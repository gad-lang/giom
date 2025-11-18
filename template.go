package giom

import (
	"github.com/gad-lang/gad"
)

type TemplateData []map[string]any

type Template struct {
	BC       *gad.Bytecode
	Global   map[string]any
	Builtins *gad.Builtins
}

func (t *Template) Executor() *TemplateExecutor {
	return NewTemplateExecutor(t)
}

func (t *Template) Source() string {
	return t.BC.FileSet.File(1).Data.ToString()
}
