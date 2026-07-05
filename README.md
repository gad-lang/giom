# Giom

Giom is an indentation-based template language for Go applications. It compiles
GION templates to Gad bytecode and is designed for server-side rendering with a
small API, reusable components, slots, imports, and HTML-oriented syntax.

The current project root contains the new implementation. The old implementation
and old samples were removed.

## Features

- Indentation-based HTML templates
- Components with named parameters and named slots
- `@import` friendly template organization
- Gad expressions and statements inside templates
- HTML tag shorthand for ids, classes, and attributes
- Transpilation to Gad AST/source for inspection
- Go embedding through `Compile` and Gad VM execution
- CMS example application in `examples/cms`

## Quick Template

```giom
@main
    !!! 5
    html[lang="en"]
        head
            title Hello
        body
            main.container
                h1 #{= Title}
                p Welcome to Giom.
```

## Component Example

```giom
@export comp page(title)
    !!! 5
    html
        head
            title #{= title}
        body
            @slot main

@main
    +page("Docs")
        h1 Documentation
        p This content is passed to the main slot.
```

## Go Usage

```go
package main

import (
    "bytes"
    "log"

    "github.com/gad-lang/gad"
    "github.com/gad-lang/giom"
)

func main() {
    src := []byte(`@main
    p Hello #{= Name}
`)

    builtins := giom.AppendBuiltins(gad.NewBuiltins())
    st := gad.NewSymbolTable(builtins.NameSet)
    if _, err := st.DefineGlobals([]string{"Name"}); err != nil {
        log.Fatal(err)
    }

    _, bc, err := giom.Compile(st, src, gad.CompileOptions{})
    if err != nil {
        log.Fatal(err)
    }

    var out bytes.Buffer
    vm := gad.NewVM(builtins.Build(), bc)
    if _, err := vm.RunOpts(&gad.RunOpts{
        StdOut:  &out,
        Globals: gad.Dict{"Name": gad.Str("Giom")},
    }); err != nil {
        log.Fatal(err)
    }

    log.Print(out.String())
}
```

## Documentation

- [Getting Started](docs/getting-started.md)
- [Template Syntax](docs/syntax.md)
- [Components And Slots](docs/components-and-slots.md)
- [Embedding In Go](docs/embedding.md)
- [API Reference](docs/api.md)
- [Examples Cookbook](docs/examples.md)
- [CMS Example](docs/cms-example.md)
- [Project Structure](docs/project-structure.md)

## Repository Layout

```text
.
├── compiler.go          # Giom compiler entry points
├── builtins.go          # HTML and write builtins
├── node/                # Giom AST nodes and Gad conversion
├── parser/              # Indentation parser and scanner
├── token/               # Giom token definitions
├── examples/cms/        # Full CMS example
└── docs/                # User documentation
```

## CMS Example

```sh
cd examples/cms
go run .
```

Open `http://localhost:8080/`. The app creates `cms.db` on first run and seeds
the database from `seed.yaml` only when `cms.db` does not already exist.

## Build

```sh
go build ./...
```

For the CMS module:

```sh
cd examples/cms
go build .
```

## License

See [LICENSE](LICENSE).
