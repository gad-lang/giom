package parser

import (
	"bytes"
	"fmt"
	"strings"

	gadparser "github.com/gad-lang/gad/parser"
	gnode "github.com/gad-lang/gad/parser/node"
	"github.com/gad-lang/gad/parser/source"
	"github.com/gad-lang/gad/token"

	giomnode "github.com/gad-lang/giom/v2/node"
	giomtoken "github.com/gad-lang/giom/v2/token"
)

// Parser parses giom template source into AST nodes.
type Parser struct {
	file      *source.File
	Errors    []string
	scanner   *scanner
	Token     gadparser.PToken
	PrevToken gadparser.PToken
	filename  string
	comps     []*giomnode.CompDecl
	compStack []*giomnode.CompDecl
}

// NewParser creates a new Parser for the given source file.
func NewParser(file *source.File) *Parser {
	p := &Parser{
		file:    file,
		scanner: newScanner(file, bytes.NewReader(file.Data.Bytes())),
	}
	p.Next()
	return p
}

// Next advances the scanner and stores the current and previous tokens.
func (p *Parser) Next() {
	p.PrevToken = p.Token
	p.Token = p.scanner.Scan()
}

// expect checks the current token against the given kinds. If exactly one kind
// is provided and the token matches, it advances to the next token. If multiple
// kinds are provided, it checks without advancing. If no kind matches, it panics.
func (p *Parser) expect(kinds ...token.Token) {
	for _, kind := range kinds {
		if p.Token.Token == kind {
			if len(kinds) == 1 {
				p.Next()
			}
			return
		}
	}
	p.error(fmt.Sprintf("expected %v, got %s (%s)", kinds, giomtoken.String(p.Token.Token), p.Token.Literal))
}

func (p *Parser) error(msg string) {
	p.Errors = append(p.Errors, msg)
	panic(msg)
}

// currentComp returns the component at the top of the comp stack, or nil.
func (p *Parser) currentComp() *giomnode.CompDecl {
	if len(p.compStack) == 0 {
		return nil
	}
	return p.compStack[len(p.compStack)-1]
}

// ParseFile parses the entire giom source file into a File AST node.
func (p *Parser) ParseFile() (_ *giomnode.File, err error) {
	defer func() {
		if r := recover(); r != nil {
			if msg, ok := r.(string); ok {
				err = fmt.Errorf("parse error: %s", msg)
			} else {
				err = fmt.Errorf("parse error: %v", r)
			}
		}
	}()

	file := &giomnode.File{
		InputFile: p.scanner.SourceFile(),
	}

	for p.Token.Token != giomtoken.EOF && p.Token.Token != token.Illegal {
		stmt := p.parseStmt()
		if stmt != nil {
			file.Stmts = append(file.Stmts, stmt)
		}
	}

	file.Comps = p.comps
	return file, nil
}

// =============================================================================
// Statement parsing
// =============================================================================

// parseStmt dispatches to the appropriate parse function based on the current token.
func (p *Parser) parseStmt() gnode.Stmt {
	switch p.Token.Token {
	case giomtoken.Doctype:
		return p.parseDoctype()
	case giomtoken.Comment:
		return p.parseComment()
	case giomtoken.Text:
		return p.parseText()
	case giomtoken.Tag:
		return p.parseTag()
	case giomtoken.Id, giomtoken.ClassName, giomtoken.Attribute:
		return p.parseTag()
	case giomtoken.If:
		return p.parseIf()
	case giomtoken.ElseIf, giomtoken.Else:
		p.error(fmt.Sprintf("unexpected %s without matching @if", giomtoken.String(p.Token.Token)))
		return nil
	case giomtoken.For:
		return p.parseFor()
	case giomtoken.Assignment:
		return p.parseAssignment()
	case giomtoken.Code:
		return p.parseCode()
	case giomtoken.ImportModule:
		return p.parseImportModule()
	case giomtoken.Func:
		return p.parseFunc()
	case giomtoken.Comp:
		return p.parseComp()
	case giomtoken.Slot:
		return p.parseSlot()
	case giomtoken.SlotPass:
		return p.parseSlotPass()
	case giomtoken.Wrap:
		return p.parseWrap()
	case giomtoken.CompCall:
		return p.parseCompCall()
	case giomtoken.Switch:
		return p.parseSwitch()
	case giomtoken.Case, giomtoken.Default:
		p.error(fmt.Sprintf("unexpected %s without matching @switch", giomtoken.String(p.Token.Token)))
		return nil
	case giomtoken.Export:
		return p.parseExport()
	case giomtoken.Blank:
		p.Next()
		return nil
	case giomtoken.Outdent, giomtoken.EOF:
		return nil
	default:
		p.error(fmt.Sprintf("unexpected token: %s (%s)", giomtoken.String(p.Token.Token), p.Token.Literal))
		return nil
	}
}

// parseBlock parses an indented block of statements. It expects the current
// token to be Indent, consumes statements until Outdent, and returns them.
func (p *Parser) parseBlock(parent gnode.Stmt) gnode.Stmts {
	p.expect(giomtoken.Indent)

	var stmts gnode.Stmts
	for p.Token.Token != giomtoken.Outdent && p.Token.Token != giomtoken.EOF {
		stmt := p.parseStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}

	p.expect(giomtoken.Outdent)
	return stmts
}

// =============================================================================
// Parse functions for specific constructs
// =============================================================================

func (p *Parser) parseDoctype() *giomnode.DoctypeStmt {
	tok := p.Token
	p.expect(giomtoken.Doctype)

	value := stringData(tok, "value", "html")
	d := &giomnode.DoctypeStmt{
		NodePos: tok.Pos,
		NodeEnd: tok.Pos + source.Pos(len(tok.Literal)),
		Value:   value,
	}
	return d
}

func (p *Parser) parseComment() *giomnode.CommentStmt {
	tok := p.Token
	p.expect(giomtoken.Comment)

	mode := stringData(tok, "mode", "embed")
	text := stringData(tok, "value", "")

	c := &giomnode.CommentStmt{
		NodePos: tok.Pos,
		NodeEnd: tok.Pos + source.Pos(len(tok.Literal)),
		Text:    text,
		Silent:  mode == "silent",
	}

	if p.Token.Token == giomtoken.Indent {
		c.Body = p.parseBlock(c)
		if len(c.Body) > 0 {
			c.NodeEnd = c.Body[len(c.Body)-1].End()
		}
	}

	return c
}

func (p *Parser) parseText() *giomnode.TextStmt {
	tok := p.Token
	p.expect(giomtoken.Text)

	content := stringData(tok, "value", "")

	t := &giomnode.TextStmt{
		NodePos: tok.Pos,
		NodeEnd: tok.Pos + source.Pos(len(tok.Literal)),
	}

	if content != "" {
		stmts, err := parseTextGad(content)
		if err == nil {
			t.Stmts = stmts
		}
	}

	return t
}

func (p *Parser) parseTag() *giomnode.TagStmt {
	tok := p.Token
	p.expect(giomtoken.Tag)

	name := stringData(tok, "value", tok.Literal)

	tag := &giomnode.TagStmt{
		NodePos: tok.Pos,
		Name:    name,
	}

	// Consume inline attributes (id, class, [attr])
	for p.Token.Token == giomtoken.Id ||
		p.Token.Token == giomtoken.ClassName ||
		p.Token.Token == giomtoken.Attribute {
		attr := p.parseInlineAttribute()
		if attr != nil {
			tag.Attributes = append(tag.Attributes, attr)
		}
	}

	tag.SelfClosing = giomnode.IsSelfClosing(name)

	if p.Token.Token == giomtoken.Indent {
		tag.Body = p.parseBlock(tag)
	} else if p.Token.Token == giomtoken.Text {
		tag.Body = gnode.Stmts{p.parseText()}
	}

	if len(tag.Body) > 0 {
		tag.NodeEnd = tag.Body[len(tag.Body)-1].End()
	} else {
		tag.NodeEnd = tok.Pos + source.Pos(len(tok.Literal))
	}

	return tag
}

func (p *Parser) parseInlineAttribute() *giomnode.TagAttribute {
	tok := p.Token
	switch tok.Token {
	case giomtoken.Id:
		p.expect(giomtoken.Id)
		attr := &giomnode.TagAttribute{
			Name:  "id",
			Value: gnode.Str(stringData(tok, "value", ""), tok.Pos),
		}
		if cond := stringData(tok, "condition", ""); cond != "" {
			attr.Condition = parseExprStr(cond, tok.Pos)
		}
		return attr
	case giomtoken.ClassName:
		p.expect(giomtoken.ClassName)
		attr := &giomnode.TagAttribute{
			Name:  "class",
			Value: gnode.Str(stringData(tok, "value", ""), tok.Pos),
		}
		if cond := stringData(tok, "condition", ""); cond != "" {
			attr.Condition = parseExprStr(cond, tok.Pos)
		}
		return attr
	default:
		p.expect(giomtoken.Attribute)
		return p.parseAttribute(tok)
	}
}

func (p *Parser) parseIf() *giomnode.IfStmt {
	tok := p.Token
	p.expect(giomtoken.If)

	condStr := stringData(tok, "value", "")

	s := &giomnode.IfStmt{
		NodePos: tok.Pos,
		Cond:    parseExprStr(condStr, tok.Pos),
	}

	if p.Token.Token == giomtoken.Indent {
		s.Body = p.parseBlock(s)
	}

	// Handle optional else-ifs and else
	for p.Token.Token == giomtoken.ElseIf {
		elseIfTok := p.Token
		p.expect(giomtoken.ElseIf)
		clause := &giomnode.ElseIfClause{
			Cond: parseExprStr(stringData(elseIfTok, "value", ""), elseIfTok.Pos),
		}
		if p.Token.Token == giomtoken.Indent {
			clause.Body = p.parseBlock(s)
		}
		s.ElseIfs = append(s.ElseIfs, clause)
	}

	if p.Token.Token == giomtoken.Else {
		p.expect(giomtoken.Else)
		if p.Token.Token == giomtoken.Indent {
			s.Else = p.parseBlock(s)
		}
	}

	s.NodeEnd = bodyEndIf(s)
	return s
}

func (p *Parser) parseFor() *giomnode.ForStmt {
	tok := p.Token
	p.expect(giomtoken.For)

	condStr := stringData(tok, "value", "")

	s := &giomnode.ForStmt{
		NodePos: tok.Pos,
		Cond:    parseExprStr(condStr, tok.Pos),
	}

	if p.Token.Token == giomtoken.Indent {
		s.Body = p.parseBlock(s)
	}

	if p.Token.Token == giomtoken.Else {
		p.expect(giomtoken.Else)
		if p.Token.Token == giomtoken.Indent {
			s.Else = p.parseBlock(s)
		}
	}

	s.NodeEnd = bodyEndFor(s)
	return s
}

func (p *Parser) parseAssignment() *giomnode.AssignStmt {
	tok := p.Token
	p.expect(giomtoken.Assignment)

	x := stringData(tok, "x", "")
	op := stringData(tok, "op", "")
	valueStr := stringData(tok, "value", "")

	s := &giomnode.AssignStmt{
		NodePos: tok.Pos,
		NodeEnd: tok.Pos + source.Pos(len(tok.Literal)),
		Op:      op,
		RHS:     parseExprStr(valueStr, tok.Pos),
	}

	if x != "" {
		s.LHS = gnode.EIdent(x, tok.Pos)
	}

	return s
}

func (p *Parser) parseCode() *giomnode.CodeStmt {
	tok := p.Token
	p.expect(giomtoken.Code)

	s := &giomnode.CodeStmt{
		NodePos: tok.Pos,
		NodeEnd: tok.Pos + source.Pos(len(tok.Literal)),
	}

	if values, ok := tok.GetOk("values"); ok {
		switch v := values.(type) {
		case []string:
			// Multi-line code (~~) — join lines and parse as full GAD source
			if tok.Literal == "" {
				joined := strings.Join(v, "\n")
				if trimmed := strings.TrimSpace(joined); trimmed != "" {
					stmts, err := parseGad(trimmed, p.scanner.SourceFile(), false)
					if err == nil {
						s.Stmts = stmts
					}
				}
				if s.Stmts != nil && len(s.Stmts) > 0 {
					s.NodeEnd = s.Stmts[len(s.Stmts)-1].End()
				}
				return s
			}
			// Single-line code (~) — parse each value as individual statement
			for _, line := range v {
				line = strings.TrimSpace(line)
				if line != "" {
					stmt, err := parseGadFirstStmt(line, nil, false)
					if err == nil {
						s.Stmts = append(s.Stmts, stmt)
					}
				}
			}
		}
	}

	return s
}

func (p *Parser) parseImportModule() *giomnode.CodeStmt {
	tok := p.Token
	p.expect(giomtoken.ImportModule)

	path := stringData(tok, "value", "")
	ident := stringData(tok, "ident", "")

	var gadSrc string
	if ident != "" {
		gadSrc = fmt.Sprintf("const %s = import(%s)", ident, path)
	} else {
		gadSrc = fmt.Sprintf("import(%s)", path)
	}

	stmt, err := parseGadFirstStmt(gadSrc, nil, false)
	s := &giomnode.CodeStmt{
		NodePos: tok.Pos,
		NodeEnd: tok.Pos + source.Pos(len(tok.Literal)),
	}

	if err == nil && stmt != nil {
		s.Stmts = gnode.Stmts{stmt}
	}

	return s
}

// =============================================================================
// Function and component parameter parsing
// =============================================================================

// parseFuncParams parses function parameters from an args string.
func (p *Parser) parseFuncParams(argsStr string, pos source.Pos) *gnode.FuncParams {
	params, err := parseFuncParamsString(argsStr)
	if err != nil {
		panic(fmt.Errorf("parsing func params failed: %v", err))
	}
	return params
}

// parseCompParams parses component parameters from an args string.
func (p *Parser) parseCompParams(argsStr string, pos source.Pos) *gnode.FuncParams {
	params, err := parseFuncParamsString(argsStr)
	if err != nil {
		panic(fmt.Errorf("parsing comp params failed: %v", err))
	}
	return params
}

// =============================================================================
// Surviving content below — preserved exactly as-is
// =============================================================================

func (p *Parser) parseAttribute(pt gadparser.PToken) *giomnode.TagAttribute {
	name := stringData(pt, "value", pt.Literal)
	mode := stringData(pt, "mode", "")
	content := stringData(pt, "content", "")
	isFlag := stringData(pt, "flag", "") == "true"

	attr := &giomnode.TagAttribute{
		Name:   name,
		IsRaw:  mode == "raw",
		IsFlag: isFlag,
	}
	if content != "" {
		if attr.IsRaw {
			attr.Value = gnode.Str(content, pt.Pos)
		} else {
			attr.Value = parseExprStr(content, pt.Pos)
		}
	}
	return attr
}

func (p *Parser) parseFunc() *giomnode.FuncDecl {
	tok := p.Token
	p.expect(giomtoken.Func)

	argsStr := stringData(tok, "args", "")
	exported := stringData(tok, "exported", "") == "true"
	name := stringData(tok, "value", tok.Literal)
	params := p.parseFuncParams(argsStr, tok.Pos)

	f := &giomnode.FuncDecl{
		NodePos:   tok.Pos,
		Name:      name,
		Params:    params,
		ParamsRaw: strings.TrimSpace(argsStr),
		Exported:  exported,
	}

	if p.Token.Token == giomtoken.Indent {
		f.Body = p.parseBlock(f)
	}

	if len(f.Body) > 0 {
		f.NodeEnd = f.Body[len(f.Body)-1].End()
	} else {
		f.NodeEnd = tok.Pos + source.Pos(len(tok.Literal))
	}
	return f
}

func (p *Parser) parseComp() *giomnode.CompDecl {
	tok := p.Token
	p.expect(giomtoken.Comp)

	name := stringData(tok, "value", tok.Literal)
	argsStr := stringData(tok, "args", "")
	exported := stringData(tok, "exported", "") == "true"
	params := p.parseCompParams(argsStr, tok.Pos)

	comp := &giomnode.CompDecl{
		NodePos:   tok.Pos,
		Name:      name,
		ID:        strings.ReplaceAll(name, "-", "__"),
		Params:    params,
		ParamsRaw: strings.TrimSpace(argsStr),
		Exported:  exported,
	}

	p.compStack = append(p.compStack, comp)
	if p.Token.Token == giomtoken.Indent {
		comp.Body = p.parseBlock(comp)
	}
	p.compStack = p.compStack[:len(p.compStack)-1]

	if len(comp.Body) > 0 {
		comp.NodeEnd = comp.Body[len(comp.Body)-1].End()
	} else {
		comp.NodeEnd = tok.Pos + source.Pos(len(tok.Literal))
	}
	p.comps = append(p.comps, comp)
	return comp
}

func (p *Parser) parseSlot() *giomnode.SlotDecl {
	tok := p.Token
	p.expect(giomtoken.Slot)

	name := stringData(tok, "value", tok.Literal)
	argsStr := stringData(tok, "args", "")
	scope := p.parseFuncParams(argsStr, tok.Pos)

	s := &giomnode.SlotDecl{
		NodePos:  tok.Pos,
		Name:     name,
		ID:       strings.ReplaceAll(name, "-", "__"),
		Scope:    scope,
		ScopeRaw: strings.TrimSpace(argsStr),
	}

	if comp := p.currentComp(); comp != nil {
		comp.Slots = append(comp.Slots, s)
	}

	if p.Token.Token == giomtoken.Indent {
		s.Body = p.parseBlock(s)
		if len(s.Body) > 0 {
			if w, ok := s.Body[0].(*giomnode.WrapStmt); ok {
				s.Wrap = w
				s.Body = s.Body[1:]
			}
		}
	}

	if len(s.Body) > 0 {
		s.NodeEnd = s.Body[len(s.Body)-1].End()
	} else {
		s.NodeEnd = tok.Pos + source.Pos(len(tok.Literal))
	}
	return s
}

func (p *Parser) parseSlotPass() *giomnode.SlotPassStmt {
	tok := p.Token
	p.expect(giomtoken.SlotPass)

	header := strings.TrimSpace(stringData(tok, "header", stringData(tok, "value", "")))
	name := header
	argsStr := ""
	if i := strings.Index(header, "("); i >= 0 && strings.HasSuffix(header, ")") {
		name = strings.TrimSpace(header[:i])
		argsStr = strings.TrimSpace(header[i+1 : len(header)-1])
	}

	params, err := parseFuncParamsString(argsStr)
	if err != nil {
		panic(fmt.Errorf("parsing slot pass failed: %v", err))
	}

	s := &giomnode.SlotPassStmt{
		NodePos: tok.Pos,
		Name:    gnode.EIdent(name, tok.Pos),
		FuncType: &gnode.FuncType{
			FuncHeader: gnode.FuncHeader{Params: *params},
		},
	}

	if p.Token.Token == giomtoken.Indent {
		s.Body = p.parseBlock(s)
	}

	if len(s.Body) > 0 {
		s.NodeEnd = s.Body[len(s.Body)-1].End()
	} else {
		s.NodeEnd = tok.Pos + source.Pos(len(tok.Literal))
	}
	return s
}

func (p *Parser) parseWrap() *giomnode.WrapStmt {
	tok := p.Token
	p.expect(giomtoken.Wrap)

	w := &giomnode.WrapStmt{
		NodePos: tok.Pos,
	}
	w.Body = p.parseBlock(w)

	if len(w.Body) > 0 {
		w.NodeEnd = w.Body[len(w.Body)-1].End()
	}
	return w
}

func (p *Parser) parseCompCall() *giomnode.CompCallStmt {
	tok := p.Token
	p.expect(giomtoken.CompCall)

	name := stringData(tok, "value", tok.Literal)
	call := &giomnode.CompCallStmt{
		NodePos: tok.Pos,
		Name:    name,
	}

	if header := stringData(tok, "args", ""); header != "" {
		args, err := parseCallArgsString(header)
		if err != nil {
			panic(err)
		}
		call.Args = *args
	}

	if p.Token.Token == giomtoken.Indent {
		block := p.parseBlock(call)
		var lastMainSlot *giomnode.SlotPassStmt
		for _, child := range block {
			switch t := child.(type) {
			case *giomnode.SlotPassStmt:
				call.SlotPass = append(call.SlotPass, t)
				lastMainSlot = nil
			default:
				if t != nil {
					if lastMainSlot != nil {
						lastMainSlot.Body.Append(t)
						lastMainSlot.NodeEnd = t.End()
					} else {
						lastMainSlot = &giomnode.SlotPassStmt{
							NodePos:  t.Pos(),
							NodeEnd:  t.End(),
							Name:     gnode.EIdent("main", 0),
							FuncType: gnode.ProxyFuncType(),
							Body:     gnode.Stmts{t},
						}
						call.SlotPass = append(call.SlotPass, lastMainSlot)
					}
				}
			}
		}
		if withCode, _ := tok.GetOk("withCode"); withCode == "true" {
			if len(block) > 0 {
				if cs, ok := block[0].(*giomnode.CodeStmt); ok {
					call.InitCode = cs
				}
			}
		}
	}

	call.NodeEnd = callEnd(call)
	return call
}

func (p *Parser) parseSwitch() *giomnode.SwitchStmt {
	tok := p.Token
	p.expect(giomtoken.Switch)

	tagExpr := parseExprStr(stringData(tok, "value", ""), tok.Pos)
	s := &giomnode.SwitchStmt{
		NodePos: tok.Pos,
		Tag:     tagExpr,
	}

	if p.Token.Token == giomtoken.Indent {
		p.Next()
	next:
		switch p.Token.Token {
		case giomtoken.Case:
			caseTok := p.Token
			p.expect(giomtoken.Case)
			cc := &giomnode.CaseClause{
				Expr: parseExprStr(stringData(caseTok, "value", ""), caseTok.Pos),
			}
			if p.Token.Token == giomtoken.Indent {
				cc.Body = p.parseBlock(s)
			}
			s.Cases = append(s.Cases, cc)
			goto next
		case giomtoken.Default:
			p.expect(giomtoken.Default)
			if p.Token.Token == giomtoken.Indent {
				s.Default = p.parseBlock(s)
			}
		default:
			p.expect(giomtoken.Case, giomtoken.Default, giomtoken.Outdent)
		}
		p.expect(giomtoken.Outdent)
	}

	s.NodeEnd = switchEnd(s)
	return s
}

func (p *Parser) parseExport() *giomnode.ExportStmt {
	tok := p.Token
	p.expect(giomtoken.Export)

	name := stringData(tok, "name", stringData(tok, "value", ""))
	valueStr := stringData(tok, "value", "")

	e := &giomnode.ExportStmt{
		NodePos: tok.Pos,
		NodeEnd: tok.Pos + source.Pos(len(tok.Literal)),
		Name:    name,
	}
	if valueStr != "" {
		e.Value = gnode.EIdent(valueStr, tok.Pos)
	}
	return e
}

// =============================================================================
// Helpers
// =============================================================================

func stringData(pt gadparser.PToken, key, def string) string {
	if v, ok := pt.GetOk(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func parseExprStr(s string, pos source.Pos) gnode.Expr {
	s = strings.TrimSpace(s)
	if s == "" {
		return gnode.EIdent("", pos)
	}
	stmt, err := parseGadFirstStmt(s, nil, false)
	if err != nil {
		return gnode.EIdent(s, pos)
	}
	if es, ok := stmt.(*gnode.ExprStmt); ok {
		return es.Expr
	}
	return gnode.EIdent(s, pos)
}

func splitTopLevelArgs(s string) []string {
	var (
		parts   []string
		start   int
		depth   int
		quote   byte
		escaped bool
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',', ';':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func topLevelAssignIndex(s string) int {
	var (
		depth   int
		quote   byte
		escaped bool
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case '=':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func callEnd(call *giomnode.CompCallStmt) source.Pos {
	if len(call.SlotPass) > 0 {
		return call.SlotPass[len(call.SlotPass)-1].End()
	}
	return call.Pos()
}

func switchEnd(s *giomnode.SwitchStmt) source.Pos {
	if len(s.Default) > 0 {
		return s.Default[len(s.Default)-1].End()
	}
	if len(s.Cases) > 0 {
		last := s.Cases[len(s.Cases)-1]
		if len(last.Body) > 0 {
			return last.Body[len(last.Body)-1].End()
		}
	}
	return s.Pos()
}

func bodyEndIf(s *giomnode.IfStmt) source.Pos {
	if len(s.Else) > 0 {
		return s.Else[len(s.Else)-1].End()
	}
	if len(s.ElseIfs) > 0 {
		last := s.ElseIfs[len(s.ElseIfs)-1]
		if len(last.Body) > 0 {
			return last.Body[len(last.Body)-1].End()
		}
	}
	if len(s.Body) > 0 {
		return s.Body[len(s.Body)-1].End()
	}
	return s.Pos()
}

func bodyEndFor(s *giomnode.ForStmt) source.Pos {
	if len(s.Else) > 0 {
		return s.Else[len(s.Else)-1].End()
	}
	if len(s.Body) > 0 {
		return s.Body[len(s.Body)-1].End()
	}
	return s.Pos()
}
