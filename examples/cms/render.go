package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gad-lang/gad"
	giom "github.com/gad-lang/giom"
)

type trackingReader struct {
	files map[string]struct{}
}

func newTrackingReader() *trackingReader {
	return &trackingReader{files: make(map[string]struct{})}
}

func (r *trackingReader) Read(path string) ([]byte, string, error) {
	r.files[path] = struct{}{}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return data, "file:" + path, nil
}

func (a *App) render(w http.ResponseWriter, name string, model gad.Dict) {
	clean := filepath.Clean(name)
	srcPath := filepath.Join(a.PublicDir, clean)

	src, err := os.ReadFile(srcPath)
	if err != nil {
		a.serverError(w, fmt.Errorf("read %s: %w", clean, err))
		return
	}

	// Check cache and decide whether to recompile.
	a.mu.Lock()
	if a.templateCache == nil {
		a.templateCache = make(map[string]*templateCacheEntry)
	}
	entry := a.templateCache[clean]
	needsCompile := entry == nil
	if entry != nil {
		if filesChanged(entry.files) {
			entry.changedAt = time.Now()
		}
		if !entry.changedAt.IsZero() && time.Since(entry.changedAt) >= a.TemplateDelay {
			needsCompile = true
		}
	}
	a.mu.Unlock()

	if needsCompile {
		entry, err = a.compileTemplate(clean, src, srcPath)
		if err != nil {
			a.serverError(w, err)
			return
		}
	}

	a.transpile(srcPath, src)

	var out bytes.Buffer
	st := gad.NewSymbolTable(entry.builtins.NameSet)
	if _, err := st.DefineGlobals([]string{"Model"}); err != nil {
		a.serverError(w, err)
		return
	}
	vm := gad.NewVM(entry.builtins.Build(), entry.bc)
	_, err = vm.RunOpts(&gad.RunOpts{StdOut: &out, Globals: gad.Dict{"Model": model}})
	if err != nil {
		a.serverError(w, fmt.Errorf("render %s: %w", name, err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(out.Bytes())
}

func (a *App) compileTemplate(clean string, src []byte, srcPath string) (*templateCacheEntry, error) {
	builtins := giom.AppendBuiltins(gad.NewBuiltins())

	tr := newTrackingReader()
	mm := gad.NewModuleMap().SetExtImporter(&giom.FileImporter{
		WorkDir:       a.PublicDir,
		FileReader:    tr.Read,
		TranspilePath: a.transpilePath,
	})

	opts := gad.CompileOptions{CompilerOptions: gad.CompilerOptions{
		ModuleFile:   srcPath,
		ModuleMap:    mm,
		FallbackFunc: giom.CompileFallback,
	}}

	st := gad.NewSymbolTable(builtins.NameSet)
	if _, err := st.DefineGlobals([]string{"Model"}); err != nil {
		return nil, err
	}
	_, bc, err := giom.Compile(st, src, opts)
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", clean, err)
	}

	files := make(map[string]time.Time)
	for p := range tr.files {
		if fi, err := os.Stat(p); err == nil {
			files[p] = fi.ModTime()
		}
	}

	entry := &templateCacheEntry{
		bc:       bc,
		builtins: builtins,
		files:    files,
	}

	a.mu.Lock()
	a.templateCache[clean] = entry
	a.mu.Unlock()

	return entry, nil
}

func filesChanged(files map[string]time.Time) bool {
	for p, mod := range files {
		fi, err := os.Stat(p)
		if err != nil || !fi.ModTime().Equal(mod) {
			return true
		}
	}
	return false
}

func (a *App) transpile(srcPath string, src []byte) {
	_ = giom.Transpile(srcPath, src, a.transpilePath(srcPath))
}

func (a *App) transpilePath(srcPath string) string {
	rel, err := filepath.Rel(a.PublicDir, srcPath)
	if err != nil {
		rel = filepath.Base(srcPath)
	}
	return filepath.Join(a.TranspileDir, strings.TrimSuffix(rel, filepath.Ext(rel))+".gad")
}

func (a *App) model(title string, crumbs []crumb, values gad.Dict) gad.Dict {
	model := gad.Dict{
		"SiteTitle":   gad.Str("GION CMS"),
		"Title":       gad.Str(title),
		"Menu":        gad.MustNewReflectValue(a.menuItems()),
		"Breadcrumbs": crumbsValue(crumbs),
		"Year":        gad.Int(time.Now().Year()),
	}
	for k, v := range values {
		model[k] = v
	}
	return model
}
