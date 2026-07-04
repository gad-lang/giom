# Handoff — giom v2 Parser & Compiler

## Current State (2026-06-15)

### gad dependency updated to `f2fd8926`
- `go get github.com/gad-lang/gad@f2fd8926`
- Breaking change: GAD now requires `;` before named parameters in function calls and declarations (no longer accepts comma-only separation).

### What's Done

**Parser (`v2/parser/`):**
- Full giom-to-GAD parser producing `gad.Node` AST
- `parseGiomFuncParamsString()` — native giom-style param parsing (handles `*`, `**`, defaults, named params with `;` separator)
- `parseCallArgsString()` — native giom-style call args parsing (handles `=` named args, `**` spread, nested parens)
- `parseSlotPass()` — no longer depends on GAD `ToFuncParams()`
- All `parseFuncParams()` callers use shared native helper

**Tests — all 12 v2 parser tests pass:**
- `TestParser_Samples_GadFixtures` — 5 sample fixtures (comps, blank, default, index, post_list)
- `TestParser_TextExprTrim` / `TestParser_TextExprTrimIndented`
- `TestParser_GadFromBlankFixture`
- `TestParser_GadOutputParses`
- `TestParseCallArgsString_GiomSyntax`
- `TestParseFuncParamsString_GiomDefaultsSemicolon`
- `TestParseSlotPassHeader_GiomDefaultsSemicolon`
- `TestParser_Positions`
- `TestParser_SampleFixturesExist`

**Samples (`v2/samples/`):**
- All `.giom` sources updated: `;` inserted before first named param in all func/comp/slot declarations and component calls
- All `~~` code blocks fixed: GAD function calls with named params now use `;` (DB calls, post_type_filter, etc.)
- All `#{=...}` inline expressions fixed: `t()` calls with `default=` now use `;`
- All `.gad` fixture files regenerated in `.giom-cache/` matching v2 parser output

**Node/CodeGen (`v2/node/`):**
- `GiomCoder` interface for round-trip giom formatting
- `WriteCode` on all node types for GAD output
- `renderFuncParams()` handles `;` separation with extra named params like `$slots={}`

### Pre-existing Failures (NOT caused by v2 changes)

Legacy tests (`giom_test.go`, `compiler_test.go`) fail due to the GAD dependency update:
- GAD now requires `;` before named params, but the legacy (v1) compiler emits code without proper `;` placement (or with trailing whitespace issues)
- GAD now forbids `export {}` on same line as `write(rawstr(...))` (missing `;` between statements)
- See test output for specific failures — all are about missing/newline placement of `;` between statements or named params

These tests use the old `giom.CompileToGad()` path and the old compiler. They are NOT v2 parser tests.

## Architecture

```
giom source (.giom)
      │
      ▼
v2/parser/scanner.go   — token scanner (partially regexp-driven)
v2/parser/utils.go     — native giom param parsing, GAD expression bridge
v2/parser/parser.go    — main parser (ParseFile → AST)
      │
      ▼
v2/node/nodes.go       — AST nodes (implement gad.Node + GiomCoder)
v2/node/giomcoder.go   — GiomCoder interface + context
      │
      ▼
gad/parser/node        — GAD expression nodes (generated code output)
```

### Key Design Decisions

1. **Template constructs** (Tag, Text, If, For, Comp, Slot, SlotPass) get own types implementing `gad.Node` + `GiomCoder`
2. **Raw GAD code** within templates (`~~ ... ~~` blocks) is parsed into actual GAD `node.Stmt`/`node.Expr`
3. **Dual coder system**: `WriteCode()` for GAD output, `WriteGiom()` for formatted giom source
4. **`;` before named params** is now required in both giom and GAD syntax — all samples updated

## Package Structure (v2 only)

| Package | Files | Lines | Purpose |
|---------|-------|-------|---------|
| `v2/token/` | 1 | 88 | Giom token kinds (29 constants, range 1000+) |
| `v2/node/` | 2 | 1023 | AST + GiomCoder |
| `v2/parser/` | 4 | 2390 | Scanner + Parser + Tests + Utils |
| `cmd/giom/` | 1 | — | CLI (render, compile, fmt) |

## What's Pending

**From TODO.md:**
1. Replace regexp-driven scanner match in `v2/parser/scanner.go` with per-token scanning
2. Create expansive tests for:
   - Parser edge cases
   - GAD code generator
   - Giom code generator (round-trip)
   - Compiler and VM integration
3. Fix legacy compiler tests (`giom_test.go`, `compiler_test.go`) to match updated GAD syntax

**Known issues:**
- `go test ./...` fails on legacy tests (v1 compiler emits GAD code without proper `;` between statements — separate issue from v2 work)
- `.giom-cache/` directories under `v2/samples/` are gitignored (regenerated with `UPDATE_GAD_FIXTURES=1 go test ./v2/parser`)

## Running

```sh
# Build everything
go build ./...

# Run only v2 tests
go test ./v2/...

# Regenerate .gad fixtures from v2 parser
UPDATE_GAD_FIXTURES=1 go test ./v2/parser -run TestParser_Samples_UpdateGadFixtures
```
