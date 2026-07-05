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
