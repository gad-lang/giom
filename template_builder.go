package giom

import (
	"context"

	"github.com/gad-lang/gad"
	gp "github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/stdlib/helper"
)

type TemplateBuilder struct {
	input         []byte
	ctx           context.Context
	module        *gad.ModuleInfo
	moduleMap     *gad.ModuleMap
	builtins      *gad.Builtins
	handleOptions func(co *gad.CompileOptions)
}

func NewTemplateBuilder(gadSource []byte) *TemplateBuilder {
	return &TemplateBuilder{input: gadSource}
}

func (b *TemplateBuilder) WithContext(ctx context.Context) *TemplateBuilder {
	b.ctx = ctx
	return b
}

func (b *TemplateBuilder) WithModule(module *gad.ModuleInfo) *TemplateBuilder {
	b.module = module
	return b
}

func (b *TemplateBuilder) WithModuleMap(moduleMap *gad.ModuleMap) *TemplateBuilder {
	b.moduleMap = moduleMap
	return b
}

func (b *TemplateBuilder) WithBuiltins(builtins *gad.Builtins) *TemplateBuilder {
	b.builtins = builtins
	return b
}

func (b *TemplateBuilder) WithHandleOptions(handle func(co *gad.CompileOptions)) *TemplateBuilder {
	b.handleOptions = handle
	return b
}

func (b *TemplateBuilder) Build() (t *Template, err error) {
	var (
		ctx       = b.ctx
		module    = b.module
		moduleMap = b.moduleMap
		builtins  = b.builtins
	)

	if ctx == nil {
		ctx = context.Background()
	}

	if builtins == nil {
		builtins = AppendBuiltins(gad.NewBuiltins())
	} else {
		builtins = AppendBuiltins(builtins)
	}

	if moduleMap == nil {
		moduleMap = helper.NewModuleMap()
	}

	co := gad.CompileOptions{
		CompilerOptions: gad.CompilerOptions{
			Context:     ctx,
			Module:      module,
			ModuleMap:   moduleMap,
			SymbolTable: gad.NewSymbolTable(builtins),
		},
		ScannerOptions: gp.ScannerOptions{},
	}

	if b.handleOptions != nil {
		b.handleOptions(&co)
	}

	var bc *gad.Bytecode
	if bc, err = gad.Compile(b.input, co); err != nil {
		return
	}

	t = &Template{
		BC:       bc,
		Builtins: builtins,
	}
	return
}
