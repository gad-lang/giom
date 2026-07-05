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

Giom parses import lines, but applications usually control file loading. A
simple resolver can inline imports before compilation:

```go
var importLine = regexp.MustCompile(`(?m)^@import\s+"([^"]+)"\s*$`)

func resolve(root, name string, seen map[string]bool) (string, error) {
    clean := filepath.Clean(name)
    if seen[clean] {
        return "", nil
    }
    seen[clean] = true

    b, err := os.ReadFile(filepath.Join(root, clean))
    if err != nil {
        return "", err
    }
    src := string(b)

    var imports strings.Builder
    for _, m := range importLine.FindAllStringSubmatch(src, -1) {
        part, err := resolve(root, m[1], seen)
        if err != nil {
            return "", err
        }
        imports.WriteString(part)
        if !strings.HasSuffix(part, "\n") {
            imports.WriteByte('\n')
        }
    }
    return imports.String() + importLine.ReplaceAllString(src, ""), nil
}
```

The CMS example uses this pattern.

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
