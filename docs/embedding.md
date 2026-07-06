# Embedding In Go

Giom is embedded through Gad bytecode compilation and VM execution.

## Minimal Renderer

```go
package render

import (
    "bytes"

    "github.com/gad-lang/gad"
    "github.com/gad-lang/giom"
)

func Render(src []byte, globals gad.Dict) (string, error) {
    builtins := giom.AppendBuiltins(gad.NewBuiltins())
    st := gad.NewSymbolTable(builtins.NameSet)

    names := make([]string, 0, len(globals))
    for name := range globals {
        names = append(names, name)
    }
    if _, err := st.DefineGlobals(names); err != nil {
        return "", err
    }

    _, bc, err := giom.Compile(st, src, gad.CompileOptions{})
    if err != nil {
        return "", err
    }

    var out bytes.Buffer
    vm := gad.NewVM(builtins.Build(), bc)
    if _, err := vm.RunOpts(&gad.RunOpts{StdOut: &out, Globals: globals}); err != nil {
        return "", err
    }
    return out.String(), nil
}
```

## Render Struct (Cached Compilation)

`giom.Render` provides a ready-to-use template engine with bytecode caching and
automatic recompilation on file changes. It is safe for concurrent use.

```go
import "github.com/gad-lang/giom"

r := &giom.Render{
    TemplateDelay: 5 * time.Second, // debounce before recompiling
    WorkDir:       "./templates",
    TranspilePath: func(src string) string {
        return strings.TrimSuffix(src, ".giom") + ".gad"
    },
}

var out bytes.Buffer
err := r.Render(&out, "template.giom", "Model", gad.Dict{
    "Title": gad.Str("Home"),
})
```

The `TemplateDelay` (default 5s) prevents recompilation on rapid file saves.
`WorkDir` is the base for resolving `@import` lines via `FileImporter`.
`TranspilePath` is optional — when set, transpiled `.gad` files are written
for inspection.

Internally, `Render` tracks all files accessed during compilation (the
template and its imports). If any change is detected, compilation is
deferred until `TemplateDelay` elapses, then the bytecode is rebuilt.

## Builtins Rule

Use the same `*gad.Builtins` value for symbol-table creation and VM creation:

```go
builtins := giom.AppendBuiltins(gad.NewBuiltins())
st := gad.NewSymbolTable(builtins.NameSet)
vm := gad.NewVM(builtins.Build(), bc)
```

Do not call `giom.AppendBuiltins(gad.NewBuiltins())` separately for compile and
run steps.

## Globals

If the template uses `Model`, define it before compilation:

```go
_, err := st.DefineGlobals([]string{"Model"})
```

Then pass it at runtime:

```go
globals := gad.Dict{
    "Model": gad.Dict{
        "Title": gad.Str("Home"),
    },
}
```

Template:

```giom
@main
    h1 #{= Model.Title}
```

## Trusted HTML

```go
model := gad.Dict{
    "Body": gad.RawStr("<p>Already sanitized HTML.</p>"),
}
```

Template:

```giom
article #{= Model.Body}
```

Only use `gad.RawStr` for trusted content.

## Import Resolution

Giom uses `giom.FileImporter` to resolve `@import` lines during compilation.
The importer is set via `gad.ModuleMap.SetExtImporter`:

```go
imports := gad.NewModuleMap().SetExtImporter(&giom.FileImporter{
    WorkDir:       "./templates",
    FileReader:    os.ReadFile,
    TranspilePath: nil, // optional: write .gad files for inspection
})
```

`FileImporter` also handles named imports (`@import "file.giom" as name`)
and compiles imported Giom files to Gad bytecode transparently.

The CMS example in `examples/cms` demonstrates this in production.

## Transpiling For Inspection

You can parse and convert a Giom file to Gad statements:

```go
fs := source.NewFileSet()
f := fs.AddFileData("template.giom", -1, src)
p := parser.NewParser(f)
file, err := p.ParseFile()
if err != nil {
    return err
}

stmts := node.Convert(file.Stmts)
var buf bytes.Buffer
gnode.CodeW(&buf, stmts, gnode.CodeWithPrefix("\t"), gnode.CodeFormat())
```

This is useful for debugging, teaching, and generated-template review.
