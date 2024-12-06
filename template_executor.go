package giom

import (
	"html"
	"io"
	"os"

	"github.com/gad-lang/gad"
)

type TemplateExecutor struct {
	t             *Template
	args          gad.Args
	namedArgs     *gad.NamedArgs
	global        []map[string]any
	out           io.Writer
	err           io.Writer
	vmOptsSetuper func(opts *gad.SetupOpts)
	vmOptsRunner  func(opts *gad.RunOpts)
}

func NewTemplateExecutor(t *Template) *TemplateExecutor {
	return &TemplateExecutor{
		t:         t,
		out:       os.Stdout,
		err:       os.Stderr,
		namedArgs: gad.NewNamedArgs(),
	}
}

func (e *TemplateExecutor) Template() *Template {
	return e.t
}

func (e *TemplateExecutor) ManyArgs(args ...gad.Array) *TemplateExecutor {
	e.args = args
	return e
}

func (e *TemplateExecutor) Args(arg ...gad.Object) *TemplateExecutor {
	return e.ManyArgs(arg)
}

func (e *TemplateExecutor) NamedArgs(na *gad.NamedArgs) *TemplateExecutor {
	e.namedArgs = na
	return e
}

func (e *TemplateExecutor) Global(g ...map[string]any) *TemplateExecutor {
	e.global = append(e.global, g...)
	return e
}

func (e *TemplateExecutor) Out(out io.Writer) *TemplateExecutor {
	e.out = out
	return e
}

func (e *TemplateExecutor) Err(out io.Writer) *TemplateExecutor {
	e.err = out
	return e
}

func (e *TemplateExecutor) VmOptsSetuper(f func(opts *gad.SetupOpts)) *TemplateExecutor {
	e.vmOptsSetuper = f
	return e
}

func (e *TemplateExecutor) VmOptsRunner(f func(opts *gad.RunOpts)) *TemplateExecutor {
	e.vmOptsRunner = f
	return e
}

func (e *TemplateExecutor) Execute() (result gad.Object, err error) {
	globals := make(gad.Dict)

	for _, d := range append([]map[string]any{e.t.Global}, e.global...) {
		for s, a := range d {
			if globals[s], err = gad.ToObject(a); err != nil {
				return
			}
		}
	}

	var (
		setupOpts = &gad.SetupOpts{
			Builtins: e.t.Builtins,
			ToRawStrHandler: func(vm *gad.VM, s gad.Str) gad.RawStr {
				return gad.RawStr(html.EscapeString(string(s)))
			},
		}

		runOpts = &gad.RunOpts{
			StdOut:  e.out,
			StdErr:  e.err,
			Globals: globals,
		}

		vm = gad.NewVM(e.t.BC)
	)

	if e.vmOptsSetuper != nil {
		e.vmOptsSetuper(setupOpts)
	}

	if e.vmOptsRunner != nil {
		e.vmOptsRunner(runOpts)
	}

	vm.Setup(*setupOpts)

	var module gad.Object
	if module, err = vm.RunOpts(runOpts); err != nil {
		return
	}

	if d, ok := module.(gad.Dict); ok {
		if main, ok := d["main"]; ok {
			result, err = gad.NewInvoker(vm, main).Invoke(e.args, e.namedArgs)
		}
	}
	return
}
