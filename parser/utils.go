package parser

import (
	"strings"

	"github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/parser/node"
	"github.com/gad-lang/gad/parser/source"
)

func parse(s string, text bool) (_ []node.Stmt, err error) {
	fileSet := source.NewFileSet()
	srcFile := fileSet.AddFileData("(main)", -1, []byte(s))
	po := &parser.ParserOptions{
		Mode: parser.ParseConfigDisabled,
	}
	so := &parser.ScannerOptions{}

	if text {
		po.Mode |= parser.ParseMixed | parser.ParseMixedExprAsValue
		so.MixedDelimiter.Start = []rune("#{")
		so.MixedDelimiter.End = []rune("}")
	}

	p := parser.NewParserWithOptions(srcFile, po, so)

	var f *parser.File
	if f, err = p.ParseFile(); err != nil {
		return
	}
	return f.Stmts, err
}

func parseFirstStmt(s string, mixed bool) (_ node.Stmt, err error) {
	var nodes []node.Stmt
	if nodes, err = parse(s, mixed); err != nil {
		return
	}
	return nodes[0], nil
}

func mustParseFirstStmt(s string, mixed bool) (stmt node.Stmt) {
	var err error
	if stmt, err = parseFirstStmt(s, mixed); err != nil {
		panic(err)
	}
	return
}

// ensureFnParamSep inserts ';' between positional and named parameters in a
// function parameter string. The new gad parser requires this separator when
// default values are present (e.g., "rows; header=nil").
func ensureFnParamSep(params string) string {
	if !strings.ContainsRune(params, '=') {
		return params
	}

	depth := 0
	firstEq := -1
	for i, r := range params {
		switch r {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '=':
			if depth == 0 {
				firstEq = i
			}
		}
		if firstEq >= 0 {
			break
		}
	}

	if firstEq < 0 {
		return params
	}

	// Find the last comma before the first '=' at depth 0
	for i := firstEq - 1; i >= 0; i-- {
		if params[i] == ',' {
			return params[:i] + "; " + params[i+1:]
		}
	}

	return params
}
