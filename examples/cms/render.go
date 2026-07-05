package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gad-lang/gad"
	gnode "github.com/gad-lang/gad/parser/node"
	"github.com/gad-lang/gad/parser/source"
	giom "github.com/gad-lang/giom"
	giomnode "github.com/gad-lang/giom/node"
	giomparser "github.com/gad-lang/giom/parser"
)

var importLine = regexp.MustCompile(`(?m)^@import\s+"([^"]+)"\s*$`)

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
	_, bc, err := giom.Compile(st, []byte(src), gad.CompileOptions{})
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

func (a *App) loadTemplate(name string) (string, error) {
	fullSrc, err := a.resolveImports(name, map[string]bool{})
	if err != nil {
		return "", err
	}
	a.transpile(name, fullSrc)
	return fullSrc, nil
}

func (a *App) resolveImports(name string, seen map[string]bool) (string, error) {
	clean := filepath.Clean(name)
	if seen[clean] {
		return "", nil
	}
	seen[clean] = true
	path := filepath.Join(a.PublicDir, clean)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", clean, err)
	}
	src := string(b)
	var imports strings.Builder
	for _, m := range importLine.FindAllStringSubmatch(src, -1) {
		part, err := a.resolveImports(m[1], seen)
		if err != nil {
			return "", err
		}
		imports.WriteString(part)
		if !strings.HasSuffix(part, "\n") {
			imports.WriteByte('\n')
		}
	}
	src = importLine.ReplaceAllString(src, "")
	return imports.String() + src, nil
}

func (a *App) transpile(name, src string) {
	transpiledName := strings.TrimSuffix(name, ".giom") + ".gad"
	outPath := filepath.Join(a.TranspileDir, transpiledName)
	_ = os.MkdirAll(filepath.Dir(outPath), 0755)

	fs := source.NewFileSet()
	f := fs.AddFileData(name, -1, []byte(src))
	p := giomparser.NewParser(f)
	file, err := p.ParseFile()
	if err != nil {
		log.Printf("transpile parse %s: %v", name, err)
		return
	}
	stmts := giomnode.Convert(file.Stmts)
	var buf bytes.Buffer
	gnode.CodeW(&buf, stmts, gnode.CodeWithPrefix("\t"), gnode.CodeFormat())
	if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil {
		log.Printf("transpile write %s: %v", transpiledName, err)
	}
}

func (a *App) model(title string, crumbs []crumb, values gad.Dict) gad.Dict {
	model := gad.Dict{
		"SiteTitle":   gad.Str("GION CMS"),
		"Title":       gad.Str(title),
		"Menu":        a.menuGad(),
		"Breadcrumbs": crumbsToGad(crumbs),
		"Year":        gad.Int(time.Now().Year()),
	}
	for k, v := range values {
		model[k] = v
	}
	return model
}
