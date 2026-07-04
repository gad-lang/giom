# Plan: Create v2 Parser for giom (gad.Node-compatible)

## Context

The current giom parser uses its own position tracking, own AST node interface, and stores
embedded GAD code as raw strings. The v2 parser:

1. Follows the `gad/parser.Parser` architecture (scanner interface, token pools)
2. Uses `gad/parser/source` for position tracking (`source.FileSet`, `source.File`, `source.Pos`)
3. AST nodes implement `gad.Node` (`ast.Node` + `node.Coder`)
4. Parses embedded GAD code into proper gad AST nodes (`node.Stmt`/`node.Expr`)
5. Provides a `GiomCoder` interface to regenerate formatted giom source (like `gofmt`)
6. A `giom` CLI command to: render templates, generate gad code, format giom templates
7. Replaces regexp-driven scanner matching with per-token scanning logic in `v2/parser/scanner.go`

## Package Structure

| Package | Path | Purpose |
|---------|------|---------|
| `token` | `v2/token/` | Giom token kinds (29 constants, mapped to `token.Token` range 1000+) |
| `node` | `v2/node/` | AST node types implementing `gnode.Stmt` + `GiomCoder` |
| `parser` | `v2/parser/` | Scanner (`ScannerInterface`) + Parser + utils |

## Key Design Decision

**Template-level constructs** (Tag, Text, If, For, Comp, Slot, SlotPass, etc.) get **own types**
that implement `gad.Node` + `GiomCoder`.

**Raw GAD code** within templates is parsed into **actual gad Node implementations**
(`node.Stmt`/`node.Expr`) and stored as fields within giom v2 nodes.

## Dual Coder System

1. **`WriteCode(ctx *CodeWriteContext)`** — from `gad.Node`/`node.Coder`. Generates GAD
   compiled code using proper indentation (not MixedCode `{% %}`).
2. **`WriteGiom(ctx *GiomCodeWriteContext)`** — from `GiomCoder`. Regenerates formatted
   giom template source (like `gofmt` formats Go code). Enables round-trip: parse giom →
   AST → write formatted giom.

`@switch` now targets GAD `match (...) { ... }` generation instead of an `if/else` chain.

## Files

| File | Status |
|------|--------|
| `v2/token/token.go` | DONE |
| `v2/node/nodes.go` | DONE |
| `v2/parser/scanner.go` | WIP (still regexp-driven; TODO replace with per-token scanner) |
| `v2/parser/utils.go` | DONE |
| `v2/parser/parser.go` | DONE |
| `v2/node/giomcoder.go` | DONE |
| `cmd/giom/main.go` | DONE |
| Tests | WIP (sample-based tests in progress; need parser/gad/giom/compiler/vm coverage) |

## Verification

1. Build: `go build ./...`
2. Use `v2/samples/*.giom` as template source fixtures
3. Refresh `v2/samples/**/.giom-cache/*.gad` to the current GAD output and assert against them
4. Position accuracy: `source.File.Position(pos)` returns correct line/column
5. Round-trip: Parse → WriteGiom → produces stable formatted giom source
6. GAD generation parses successfully with the current `gad/parser`
7. CLI: `giom render`, `giom compile`, `giom fmt`
8. Run current tests: `go test ./...`
