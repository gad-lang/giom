package giom

import "github.com/gad-lang/gad"

var ModuleSpec = gad.NewModuleSpecFromName("giom")

// Module returns the `giom` builtin namespace.
func Module() gad.Dict { return newModule() }

// newModule builds the `giom` builtin namespace.
func newModule() gad.Dict {
	return gad.Dict{
		// gad:doc
		// # giom module
		// ## Types
		// Tag is a type of tag Value
		// "Tag":    TagType,
		"escape": BuiltinEscape,
		"attr":   BuiltinAttr,
		"attrs":  BuiltinAttrs,
		"write":  BuiltinTextWrite,
	}
}
