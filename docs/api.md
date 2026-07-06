# API Reference

This is a practical user-level API reference. For exact signatures, use `go doc`.

## Package

```go
import "github.com/gad-lang/giom"
```

## `AppendBuiltins`

```go
func AppendBuiltins(b *gad.Builtins) *gad.Builtins
```

Registers Giom builtins in a Gad builtins set:

- `giom$escape`
- `giom$attr`
- `giom$attrs`
- `giom$write`

Use it before compiling and before constructing the VM.

```go
builtins := giom.AppendBuiltins(gad.NewBuiltins())
```

## `Compile`

```go
func Compile(st *gad.SymbolTable, input []byte, opts gad.CompileOptions) (*node.File, *gad.Bytecode, error)
```

Parses Giom source and compiles it to Gad bytecode.

```go
file, bc, err := giom.Compile(st, src, gad.CompileOptions{})
```

`file` is the parsed Giom AST. `bc` is executable Gad bytecode.

## `CompileFile`

```go
func CompileFile(st *gad.SymbolTable, module *gad.ModuleSpec, file *node.File, opts gad.CompileOptions) (*gad.Bytecode, error)
```

Compiles an already parsed Giom file. Use this when you need parser access before
compilation.

## `Render` Struct

```go
type Render struct {
    TemplateDelay time.Duration       // debounce before recompiling (default 5s)
    WorkDir       string              // base for import resolution (default: file's dir)
    TranspilePath func(srcPath string) string // optional .gad output path
    BuiltinsFunc  func() *gad.Builtins       // optional builtins factory
}
```

### `(*Render) Render`

```go
func (r *Render) Render(out io.Writer, filePath, globalName string, globalValue gad.Dict) error
```

Reads `filePath`, compiles (or retrieves cached bytecode), and executes the
template with `globalName` bound to `globalValue`. The output is written to `out`.

Caching tracks all files accessed during compilation (template + imports).
When a file change is detected, recompilation is deferred by `TemplateDelay`.

### Caching Behavior

- The first call to `Render` for a given file compiles it and caches the
  bytecode along with file modification times.
- Subsequent calls check all tracked files. If any have changed, the entry
  is marked stale but not immediately recompiled — recompilation happens
  after `TemplateDelay` elapses since the first detected change.
- This debounce prevents recompilation during rapid file-save sequences.

## `Transpile`

```go
func Transpile(name string, src []byte, outPath string) error
```

Parses Giom source, converts it to Gad statements, and writes the result
to `outPath`. Useful for inspection and debugging.

```go
giom.Transpile("template.giom", src, "template.gad")
```

## `FileImporter`

```go
type FileImporter struct {
    WorkDir       string
    FileReader    func(path string) ([]byte, string, error)
    TranspilePath func(srcPath string) string
}
```

Implements `gad.ExtImporter` for resolving `@import` lines in Giom
templates. It reads imported files via `FileReader`, compiles them to
Gad bytecode, and optionally writes transpiled `.gad` output.

Used automatically by `Render` when `WorkDir` is set. Can also be wired
manually:

```go
mm := gad.NewModuleMap().SetExtImporter(&giom.FileImporter{
    WorkDir:    "./templates",
    FileReader: os.ReadFile,
})
```

## Parser Package

```go
import "github.com/gad-lang/giom/parser"
```

```go
fs := source.NewFileSet()
f := fs.AddFileData("template.giom", -1, src)
p := parser.NewParser(f)
file, err := p.ParseFile()
```

## Node Package

```go
import giomnode "github.com/gad-lang/giom/node"
```

Convert Giom nodes to Gad nodes:

```go
stmts := giomnode.Convert(file.Stmts)
```

## Token Package

```go
import "github.com/gad-lang/giom/token"
```

Contains Giom token definitions used by the parser and scanner.
