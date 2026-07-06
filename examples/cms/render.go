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

func (a *App) render(w http.ResponseWriter, name string, model gad.Dict) {
	src, err := a.loadTemplate(name)
	if err != nil {
		a.serverError(w, err)
		return
	}
	builtins := giom.AppendBuiltins(gad.NewBuiltins())
	st := gad.NewSymbolTable(builtins.NameSet)
	if _, err := st.DefineGlobals([]string{"Model"}); err != nil {
		a.serverError(w, err)
		return
	}
	opts := gad.CompileOptions{CompilerOptions: gad.CompilerOptions{
		ModuleFile:   filepath.Join(a.PublicDir, name),
		ModuleMap:    a.moduleMap(),
		FallbackFunc: giom.CompileFallback,
	}}
	_, bc, err := giom.Compile(st, src, opts)
	if err != nil {
		a.serverError(w, fmt.Errorf("compile %s: %w", name, err))
		return
	}
	var out bytes.Buffer
	vm := gad.NewVM(builtins.Build(), bc)
	_, err = vm.RunOpts(&gad.RunOpts{StdOut: &out, Globals: gad.Dict{"Model": model}})
	if err != nil {
		a.serverError(w, fmt.Errorf("render %s: %w", name, err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(out.Bytes())
}

func (a *App) loadTemplate(name string) ([]byte, error) {
	clean := filepath.Clean(name)
	path := filepath.Join(a.PublicDir, clean)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", clean, err)
	}
	a.transpile(path, b)
	return b, nil
}

func (a *App) moduleMap() *gad.ModuleMap {
	return gad.NewModuleMap().SetExtImporter(&giom.FileImporter{
		WorkDir:       a.PublicDir,
		TranspilePath: a.transpilePath,
	})
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
