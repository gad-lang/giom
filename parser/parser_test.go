package parser

import (
	"strings"
	"testing"

	"github.com/gad-lang/gad/parser/source"
	giomnode "github.com/gad-lang/giom/node"
)

func parseLine(t *testing.T, src string) *giomnode.File {
	t.Helper()
	fs := source.NewFileSet()
	f := fs.AddFileData("test.giom", -1, []byte(src))
	p := NewParser(f)
	file, err := p.ParseFile()
	if err != nil {
		t.Fatalf("parse error: %v\nsrc: %q", err, src)
	}
	return file
}

func expectStmtCount(t *testing.T, file *giomnode.File, n int) {
	t.Helper()
	if len(file.Stmts) != n {
		t.Fatalf("expected %d statements, got %d", n, len(file.Stmts))
	}
}

func expectCodeStmt(t *testing.T, file *giomnode.File, idx int, contains string) {
	t.Helper()
	if idx >= len(file.Stmts) {
		t.Fatalf("statement index %d out of range (len=%d)", idx, len(file.Stmts))
	}
	cs, ok := file.Stmts[idx].(*giomnode.CodeStmt)
	if !ok {
		t.Fatalf("expected *giomnode.CodeStmt at index %d, got %T", idx, file.Stmts[idx])
	}
	for _, stmt := range cs.Stmts {
		got := stmt.String()
		if !strings.Contains(got, contains) {
			t.Fatalf("expected CodeStmt to contain %q, got %q", contains, got)
		}
	}
}

func TestImportBare(t *testing.T) {
	file := parseLine(t, `@import "components.giom"`)
	expectStmtCount(t, file, 1)
	expectCodeStmt(t, file, 0, "import")
}

func TestImportNamed(t *testing.T) {
	file := parseLine(t, `@import "components.giom" as comps`)
	expectStmtCount(t, file, 1)
	expectCodeStmt(t, file, 0, "import")
	expectCodeStmt(t, file, 0, "comps")
}

func TestImportDestructureSingle(t *testing.T) {
	file := parseLine(t, `@import { page_wrapper } from "comps.giom"`)
	expectStmtCount(t, file, 1)
	code := codeStmtStr(t, file, 0)
	if !strings.Contains(code, "page_wrapper") {
		t.Fatalf("expected destructure entry 'page_wrapper', got: %s", code)
	}
}

func TestImportDestructureMultiple(t *testing.T) {
	file := parseLine(t, `@import { page_wrapper, hero, post_card } from "comps.giom"`)
	expectStmtCount(t, file, 1)
	code := codeStmtStr(t, file, 0)
	for _, name := range []string{"page_wrapper", "hero", "post_card"} {
		if !strings.Contains(code, name) {
			t.Fatalf("expected destructure entry %q, got: %s", name, code)
		}
	}
}

func TestImportDestructureRename(t *testing.T) {
	file := parseLine(t, `@import { page_wrapper: pw, hero: h } from "comps.giom"`)
	expectStmtCount(t, file, 1)
	code := codeStmtStr(t, file, 0)
	if !strings.Contains(code, "page_wrapper:pw") {
		t.Fatalf("expected 'page_wrapper:pw', got: %s", code)
	}
}

func TestImportDestructureDefault(t *testing.T) {
	file := parseLine(t, `@import { page_wrapper = default_wrapper } from "comps.giom"`)
	expectStmtCount(t, file, 1)
	code := codeStmtStr(t, file, 0)
	if !strings.Contains(code, "page_wrapper=default_wrapper") {
		t.Fatalf("expected 'page_wrapper=default_wrapper', got: %s", code)
	}
}

func TestImportDestructureRest(t *testing.T) {
	file := parseLine(t, `@import { page_wrapper, **rest } from "comps.giom"`)
	expectStmtCount(t, file, 1)
	code := codeStmtStr(t, file, 0)
	if !strings.Contains(code, "**rest") {
		t.Fatalf("expected '**rest', got: %s", code)
	}
}

func TestImportDestructureMixed(t *testing.T) {
	file := parseLine(t, `@import { a, b: bb, c = 5, **rest } from "comps.giom"`)
	expectStmtCount(t, file, 1)
	code := codeStmtStr(t, file, 0)
	for _, part := range []string{"a", "b:bb", "c=5", "**rest"} {
		if !strings.Contains(code, part) {
			t.Fatalf("expected part %q, got: %s", part, code)
		}
	}
}

func TestImportDestructureWithMain(t *testing.T) {
	src := `@import { page_wrapper } from "comps.giom"
@main
    +page_wrapper("Test")
        p Hello`
	file := parseLine(t, src)
	expectStmtCount(t, file, 2)
	expectCodeStmt(t, file, 0, "import")
	expectCodeStmt(t, file, 0, "page_wrapper")
}

func codeStmtStr(t *testing.T, file *giomnode.File, idx int) string {
	t.Helper()
	if idx >= len(file.Stmts) {
		t.Fatalf("statement index %d out of range (len=%d)", idx, len(file.Stmts))
	}
	cs, ok := file.Stmts[idx].(*giomnode.CodeStmt)
	if !ok {
		t.Fatalf("expected *giomnode.CodeStmt at index %d, got %T", idx, file.Stmts[idx])
	}
	var parts []string
	for _, stmt := range cs.Stmts {
		parts = append(parts, stmt.String())
	}
	return strings.Join(parts, "; ")
}
