package parser

import (
	"strings"

	gadparser "github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/parser/node"
	"github.com/gad-lang/gad/parser/source"
)

// parseGad parses a GAD expression string using gad's parser.
// If textMode is true, uses mixed mode with {...} delimiters.
func parseGad(s string, file *source.File, textMode bool) (_ node.Stmts, err error) {
	po := &gadparser.ParserOptions{
		Mode: gadparser.ParseConfigDisabled,
	}
	so := &gadparser.ScannerOptions{}

	if textMode {
		po.Mode |= gadparser.ParseMixed | gadparser.ParseMixedExprAsValue
		so.MixedDelimiter.Start = []rune("{")
		so.MixedDelimiter.End = []rune("}")
	}

	fileSet := source.NewFileSet()
	srcFile := fileSet.AddFileData("(main)", -1, []byte(s))
	p := gadparser.NewParserWithOptions(srcFile, po, so)

	var f *gadparser.File
	if f, err = p.ParseFile(); err != nil {
		return
	}
	return f.Stmts, err
}

func parseGadFirstStmt(s string, file *source.File, mixed bool) (_ node.Stmt, err error) {
	var stmts node.Stmts
	if stmts, err = parseGad(s, file, mixed); err != nil {
		return
	}
	return stmts[0], nil
}

func mustParseGadFirstStmt(s string, file *source.File, mixed bool) (stmt node.Stmt) {
	var err error
	if stmt, err = parseGadFirstStmt(s, file, mixed); err != nil {
		panic(err)
	}
	return
}

func parseTextGad(s string) (node.Stmts, error) {
	return parseGad(strings.TrimSpace(s), nil, true)
}

func parseCallArgsString(s string) (args *node.CallArgs, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return &node.CallArgs{}, nil
	}
	parts := splitTopLevelArgs(s)
	args = &node.CallArgs{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "**") {
			name := strings.TrimSpace(part[2:])
			args.NamedArgs.Append(&node.NamedArgExpr{Ident: node.EIdent(name, 0), Var: true}, nil)
			continue
		}
		if idx := topLevelAssignIndex(part); idx >= 0 {
			name := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+1:])
			args.NamedArgs.AppendS(name, parseExprStr(value, 0))
			continue
		}
		args.Args.Values = append(args.Args.Values, parseExprStr(part, 0))
	}
	return args, nil
}

func parseFuncParamsString(s string) (params *node.FuncParams, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return &node.FuncParams{}, nil
	}
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return parseGiomFuncParamsString(s), nil
}

func parseGiomFuncParamsString(s string) *node.FuncParams {
	s = strings.TrimSpace(s)
	if s == "" {
		return &node.FuncParams{}
	}

	params := &node.FuncParams{}
	parts := splitTopLevelArgs(s)
	seenNamed := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "**") {
			name := strings.TrimSpace(part[2:])
			params.NamedArgs.Var = node.EIdent(name, 0)
			seenNamed = true
			continue
		}
		if strings.HasPrefix(part, "*") {
			name := strings.TrimSpace(part[1:])
			params.Args.Var = node.ETypedIdent(node.EIdent(name, 0))
			continue
		}
		if idx := topLevelAssignIndex(part); idx >= 0 {
			seenNamed = true
			name := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+1:])
			params.NamedArgs.Add(node.ETypedIdent(node.EIdent(name, 0)), parseExprStr(value, 0))
			continue
		}
		if seenNamed {
			params.NamedArgs.Add(node.ETypedIdent(node.EIdent(strings.TrimSpace(part), 0)), nil)
			continue
		}
		params.Args.Values = append(params.Args.Values, node.ETypedIdent(node.EIdent(part, 0)))
	}
	return params
}

func ParseCallArgsStringForTest(s string) (*node.CallArgs, error) {
	return parseCallArgsString(s)
}

func ParseFuncParamsStringForTest(s string) (*node.FuncParams, error) {
	return parseFuncParamsString(s)
}
