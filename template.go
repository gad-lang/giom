package gber

import (
	"html"
	"io"

	"github.com/gad-lang/gad"
)

type TemplateData []map[string]any

type Template struct {
	BC          *gad.Bytecode
	GlobalNames map[string]any
	Code        string
	Builtins    *gad.Builtins
}

func (t *Template) Execute(w io.Writer, data ...map[string]any) (err error) {
	globals := make(gad.Dict, len(data))
	for _, d := range data {
		for s, a := range d {
			if globals[s], err = gad.ToObject(a); err != nil {
				return
			}
		}
	}
	vm := gad.NewVM(t.BC).
		Setup(gad.SetupOpts{
			Builtins: t.Builtins,
			ToRawStrHandler: func(vm *gad.VM, s gad.Str) gad.RawStr {
				return gad.RawStr(html.EscapeString(string(s)))
			},
		})
	_, err = vm.RunOpts(&gad.RunOpts{
		StdOut:  w,
		StdErr:  io.Discard,
		Globals: globals,
	})
	return
}
