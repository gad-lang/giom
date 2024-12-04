package parser

import (
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
		po.Mode |= parser.ParseMixed
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
