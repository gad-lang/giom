package giom

import (
	"context"

	"github.com/gad-lang/gad"
	gp "github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/stdlib/helper"
)

// TemplateBuilder builds a Template from GAD source code with configurable
// context, module, module map, and builtins.
type TemplateBuilder struct {
	input         []byte
	ctx           context.Context
	module        *gad.ModuleInfo
	moduleMap     *gad.ModuleMap
	builtins      *gad.Builtins
	handleOptions func(co *gad.CompileOptions)
}

// NewTemplateBuilder creates a new TemplateBuilder with the given GAD source code.
func NewTemplateBuilder(gadSource []byte) *TemplateBuilder {
	return &TemplateBuilder{input: gadSource}
}

// WithContext sets the context for template compilation.
func (b *TemplateBuilder) WithContext(ctx context.Context) *TemplateBuilder {
	b.ctx = ctx
	return b
}

// WithModule sets the module info for template compilation.
func (b *TemplateBuilder) WithModule(module *gad.ModuleInfo) *TemplateBuilder {
	b.module = module
	return b
}

// WithModuleMap sets the module map for template compilation.
func (b *TemplateBuilder) WithModuleMap(moduleMap *gad.ModuleMap) *TemplateBuilder {
	b.moduleMap = moduleMap
	return b
}

// WithBuiltins sets the builtins for template compilation.
func (b *TemplateBuilder) WithBuiltins(builtins *gad.Builtins) *TemplateBuilder {
	b.builtins = builtins
	return b
}

// WithHandleOptions sets a callback to configure CompileOptions before building.
func (b *TemplateBuilder) WithHandleOptions(handle func(co *gad.CompileOptions)) *TemplateBuilder {
	b.handleOptions = handle
	return b
}

// Build compiles the GAD source and returns a ready-to-execute Template.
func (b *TemplateBuilder) Build() (t *Template, err error) {
	var (
		module *gad.ModuleSpec

		ctx       = b.ctx
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

	if b.module != nil {
		module = &gad.ModuleSpec{ModuleInfo: *b.module}
	} else {
		module = &gad.ModuleSpec{ModuleInfo: gad.ModuleInfo{Name: gad.MainName}, Main: true}
	}

	co := gad.CompileOptions{
		CompilerOptions: gad.CompilerOptions{
			Context:   ctx,
			ModuleMap: moduleMap,
		},
		ScannerOptions: gp.ScannerOptions{},
	}

	if b.handleOptions != nil {
		b.handleOptions(&co)
	}

	var (
		bc              *gad.Bytecode
		buildedBuiltins = builtins.Build()
		st              = gad.NewSymbolTable(buildedBuiltins.Builtins().NameSet)
	)

	if _, bc, err = gad.CompileModule(st, module, b.input, co); err != nil {
		return
	}

	t = &Template{
		BC:       bc,
		Builtins: buildedBuiltins,
	}
	return
}
