package giom

import (
	"html"
	"io"
	"os"

	"github.com/gad-lang/gad"
)

// TemplateExecutor executes a compiled Template with configurable output,
// arguments, and global variables.
type TemplateExecutor struct {
	t             *Template
	args          gad.Args
	namedArgs     *gad.NamedArgs
	global        []map[string]any
	out           io.Writer
	err           io.Writer
	vmOptsSetuper func(opts *gad.SetupOpts)
	vmOptsRunner  func(opts *gad.RunOpts)
	builtins      *gad.StaticBuiltins
}

// NewTemplateExecutor creates a new TemplateExecutor for the given Template.
func NewTemplateExecutor(t *Template) *TemplateExecutor {
	return &TemplateExecutor{
		t:         t,
		out:       os.Stdout,
		err:       os.Stderr,
		namedArgs: gad.NewNamedArgs(),
		builtins:  t.Builtins,
	}
}

// Template returns the underlying Template.
func (e *TemplateExecutor) Template() *Template {
	return e.t
}

// ManyArgs sets positional arguments for template execution from multiple gad.Array values.
func (e *TemplateExecutor) ManyArgs(args ...gad.Array) *TemplateExecutor {
	e.args = args
	return e
}

// Args sets positional arguments for template execution.
func (e *TemplateExecutor) Args(arg ...gad.Object) *TemplateExecutor {
	return e.ManyArgs(arg)
}

// NamedArgs sets named arguments for template execution.
func (e *TemplateExecutor) NamedArgs(na *gad.NamedArgs) *TemplateExecutor {
	e.namedArgs = na
	return e
}

// Global appends global variable maps for template execution.
func (e *TemplateExecutor) Global(g ...map[string]any) *TemplateExecutor {
	e.global = append(e.global, g...)
	return e
}

// Out sets the output writer for template execution (defaults to os.Stdout).
func (e *TemplateExecutor) Out(out io.Writer) *TemplateExecutor {
	e.out = out
	return e
}

// Err sets the error output writer for template execution (defaults to os.Stderr).
func (e *TemplateExecutor) Err(out io.Writer) *TemplateExecutor {
	e.err = out
	return e
}

// VmOptsSetuper sets a callback to configure SetupOpts before VM setup.
func (e *TemplateExecutor) VmOptsSetuper(f func(opts *gad.SetupOpts)) *TemplateExecutor {
	e.vmOptsSetuper = f
	return e
}

// VmOptsRunner sets a callback to configure RunOpts before VM execution.
func (e *TemplateExecutor) VmOptsRunner(f func(opts *gad.RunOpts)) *TemplateExecutor {
	e.vmOptsRunner = f
	return e
}

// Execute runs the template and returns the VM, result, and any error.
func (e *TemplateExecutor) Execute() (vm *gad.VM, result gad.Object, err error) {
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
			ToRawStrHandler: func(vm *gad.VM, s gad.Str) gad.RawStr {
				return gad.RawStr(html.EscapeString(string(s)))
			},
		}

		runOpts = &gad.RunOpts{
			StdOut:  e.out,
			StdErr:  e.err,
			Globals: globals,
		}
	)

	vm = gad.NewVM(e.builtins, e.t.BC)
	vm.Builtins = e.builtins

	if e.vmOptsSetuper != nil {
		e.vmOptsSetuper(setupOpts)
	}

	if e.vmOptsRunner != nil {
		e.vmOptsRunner(runOpts)
	}

	vm.Setup(*setupOpts)

	result, err = vm.RunOpts(runOpts)
	return
}

// ExecuteModule runs the template and invokes the "main" export with the configured arguments.
func (e *TemplateExecutor) ExecuteModule() (result gad.Object, err error) {
	var (
		module gad.Object
		vm     *gad.VM
	)

	if vm, module, err = e.Execute(); err != nil {
		return
	}

	if d, ok := module.(gad.Dict); ok {
		if main, ok := d["main"]; ok {
			result, err = gad.NewInvoker(vm, main).Invoke(e.args, e.namedArgs)
		}
	} else if m, ok := module.(*gad.Module); ok {
		if d, ok := m.Data.(gad.Dict); ok {
			if main, ok := d["main"]; ok {
				result, err = gad.NewInvoker(vm, main).Invoke(e.args, e.namedArgs)
			}
		}
	}

	return
}
