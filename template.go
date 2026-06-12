package giom

import (
	"github.com/gad-lang/gad"
)

// TemplateData is a slice of key-value maps used as template global variables.
type TemplateData []map[string]any

// Template holds compiled GAD bytecode and builtins ready for execution.
type Template struct {
	BC       *gad.Bytecode
	Global   map[string]any
	Builtins *gad.StaticBuiltins
}

func (t *Template) Executor() *TemplateExecutor {
	return NewTemplateExecutor(t)
}

func (t *Template) Source() string {
	return t.BC.FileSet.File(1).Data.ToString()
}
