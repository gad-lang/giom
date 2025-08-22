package parser

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/gad-lang/gad/parser/node"
)

type Parser struct {
	scanner      *scanner
	filename     string
	currenttoken *token
	namedBlocks  map[string]*NamedBlock
	stack        []Node
	comps        []*Comp
}

func New(rdr io.Reader) *Parser {
	p := new(Parser)
	p.scanner = newScanner(rdr)
	p.namedBlocks = make(map[string]*NamedBlock)
	return p
}

func (p *Parser) SetFilename(filename string) *Parser {
	p.filename = filename
	return p
}

func FileParser(filename string) (*Parser, error) {
	data, err := os.ReadFile(filename)

	if err != nil {
		return nil, err
	}

	parser := New(bytes.NewReader(data))
	parser.filename = filename
	return parser, nil
}

func (p *Parser) currentComp() *Comp {
	for i := len(p.stack) - 1; i >= 0; i-- {
		switch expr := p.stack[i].(type) {
		case *Comp:
			return expr
		}
	}
	return nil
}

func (p *Parser) addComp(c *Comp) {
	pc := p.currentComp()
	if pc == nil {
		p.comps = append(p.comps, c)
	} else {
		pc.Comps = append(pc.Comps, c)
	}
}

func (p *Parser) Parse() (root *Root, err error) {
	defer func() {
		var (
			pos = p.pos()
			pe  = &ParseError{
				Column:      pos.ColNum,
				Line:        pos.LineNum,
				TokenLength: pos.TokenLength,
				Filename:    pos.Filename,
			}
		)

		if r := recover(); r != nil {
			switch t := r.(type) {
			case string:
				pe.Err = errors.New(t)
			case error:
				pe.Err = t
			}

			err = pe
		} else if err != nil {
			pe.Err = err
			err = pe
		}
	}()

	root = &Root{}
	p.next()

	for {
		if p.currenttoken == nil || p.currenttoken.Kind == tokEOF {
			break
		}

		if p.currenttoken.Kind == tokBlank {
			p.next()
			continue
		}

		root.push(p.parse())
	}

	root.Comps = p.comps
	return
}

func (p *Parser) pos() SourcePosition {
	pos := p.scanner.Pos()
	pos.Filename = p.filename
	return pos
}

func (p *Parser) parse() Node {
	switch p.currenttoken.Kind {
	case tokDoctype:
		return p.parseDoctype()
	case tokComment:
		return p.parseComment()
	case tokText:
		return p.parseText()
	case tokIf:
		node := p.parseIf()
		if node.skips {
			return nil
		}
		return node
	case tokFor:
		return p.parseFor()
	case tokImportModule:
		return p.parseImportModule()
	case tokTag:
		return p.parseTag()
	case tokAssignment:
		return p.parseAssignment()
	case tokCode:
		return p.parseCode()
	case tokSlot:
		return p.parseSlot()
	case tokSlotPass:
		return p.parseSlotPass()
	case tokWrap:
		return p.parseWrap()
	case tokIndent:
		return p.parseBlock(nil)
	case tokFunc:
		return p.parseFunc()
	case tokComp:
		c := p.parseComp()
		p.addComp(c)
		return c
	case tokCompCall:
		return p.parseCompCall()
	case tokSwitch:
		return p.parseSwitch()
	case tokExport:
		return p.parseExport()
	}

	panic(fmt.Sprintf("Unexpected token: %d", p.currenttoken.Kind))
}

func (p *Parser) expect(typ ...Kind) *token {
	for _, r := range typ {
		if p.currenttoken.Kind == r {
			curtok := p.currenttoken
			p.next()
			return curtok
		}
	}
	panic("Unexpected token `" + p.currenttoken.Kind.String() + "`!")
}

func (p *Parser) next() {
	p.currenttoken = p.scanner.Next()
}

func (p *Parser) parseBlock(parent Node) *Block {
	p.expect(tokIndent)
	block := newBlock()
	block.SourcePosition = p.pos()
	p.stack = append(p.stack, parent)
	defer func() {
		p.stack = p.stack[:len(p.stack)-1]
	}()

	for {
		if p.currenttoken == nil || p.currenttoken.Kind == tokEOF || p.currenttoken.Kind == tokOutdent {
			break
		}

		if p.currenttoken.Kind == tokBlank {
			p.next()
			continue
		}

		if p.currenttoken.Kind == tokId ||
			p.currenttoken.Kind == tokClassName ||
			p.currenttoken.Kind == tokAttribute {

			if tag, ok := parent.(*Tag); ok {
				attr := p.expect(p.currenttoken.Kind)
				cond := attr.Data["Condition"]

				switch attr.Kind {
				case tokId:
					tag.Attributes = append(tag.Attributes, Attribute{p.pos(), "id", attr.Value, true, false, cond, nil})
				case tokClassName:
					tag.Attributes = append(tag.Attributes, Attribute{p.pos(), "class", attr.Value, true, false, cond, nil})
				case tokAttribute:
					var elements *node.KeyValueArrayLit
					if attr.AnyValue != nil {
						elements = attr.AnyValue.(*node.KeyValueArrayLit)
					}
					tag.Attributes = append(tag.Attributes, Attribute{p.pos(), attr.Value, attr.Data["Content"], attr.Data["Mode"] == "raw", false, cond, elements})
				}

				continue
			} else {
				if cond, ok := parent.(*If); ok {
					if tag, ok := p.stack[len(p.stack)-2].(*Tag); ok {
						// do not include it as block
						cond.skips = true

						attr := p.expect(p.currenttoken.Kind)
						expr := cond.Positives[0].Expression
						if cond.Positives[0] != nil {
							expr = "!(" + expr + ")"
						}
						switch attr.Kind {
						case tokId:
							tag.Attributes = append(tag.Attributes, Attribute{p.pos(), "id", attr.Value, true, false, expr, nil})
						case tokClassName:
							tag.Attributes = append(tag.Attributes, Attribute{p.pos(), "class", attr.Value, true, false, expr, nil})
						case tokAttribute:
							var elements *node.KeyValueArrayLit
							if attr.AnyValue != nil {
								elements = attr.AnyValue.(*node.KeyValueArrayLit)
							}
							tag.Attributes = append(tag.Attributes, Attribute{p.pos(), attr.Value, attr.Data["Content"], attr.Data["Mode"] == "raw", false, expr, elements})
						}

						continue
					}
				}
				panic("Conditional attributes must be placed immediately within a parent tag.")
			}
		}

		block.push(p.parse())
	}

	p.expect(tokOutdent)

	return block
}

func (p *Parser) parseIf() *If {
	condTok := p.expect(tokIf)
	cnd := &If{
		SourcePosition: p.pos(),
	}

	pos := &Condition{
		SourcePosition: p.pos(),
		Expression:     condTok.Value,
	}

	if p.currenttoken.Kind == tokIndent {
		pos.Block = p.parseBlock(pos)
	}
	cnd.Positives = append(cnd.Positives, pos)

readmore:
	switch p.currenttoken.Kind {
	case tokIndent:
		if cnd.skips && pos.Block != nil && len(pos.Block.Children) > 0 {
			panic("Conditional for tag attributes does not accepts children on Positive Block.")
		}
		goto readmore
	case tokElseIf:
		tok := p.expect(tokElseIf)
		pos = &Condition{
			SourcePosition: p.pos(),
			Expression:     tok.Value,
		}
		if p.currenttoken.Kind == tokIndent {
			pos.Block = p.parseBlock(pos)
		}
		cnd.Positives = append(cnd.Positives, pos)
		goto readmore
	case tokElse:
		p.expect(tokElse)
		if p.currenttoken.Kind == tokIndent {
			cnd.Negative = p.parseBlock(pos)
		}
		if cnd.skips && cnd.Negative != nil && len(cnd.Negative.Children) > 0 {
			panic("Conditional for tag attributes does not accepts children on Negative Block.")
		}
		goto readmore
	}

	return cnd
}

func (p *Parser) parseFor() *For {
	tok := p.expect(tokFor)
	ech := newFor(tok.Value)
	ech.SourcePosition = p.pos()

readmore:
	switch p.currenttoken.Kind {
	case tokIndent:
		ech.Block = p.parseBlock(ech)
		goto readmore
	case tokElse:
		p.expect(tokElse)
		if p.currenttoken.Kind == tokIndent {
			ech.Else = p.parseBlock(ech)
		} else {
			panic("Unexpected token!")
		}
	}

	return ech
}

func (p *Parser) parseImportModule() *Code {
	tok := p.expect(tokImportModule)
	ident := tok.Data["ident"]
	node := &Code{
		Expressions: []string{
			"const " + ident + " = import(" + strconv.Quote(tok.Value[1:len(tok.Value)-1]+".giom") + ")",
		}}
	node.SourcePosition = p.pos()
	return node
}

func (p *Parser) parseSlot() (s *Slot) {
	var (
		tok = p.expect(tokSlot)
		c   = mustParseFirstStmt("func("+tok.Data["Args"]+"){}", false)
	)

	s = &Slot{
		SourcePosition: p.pos(),
		Name:           tok.Value,
		Scope:          &c.(*node.ExprStmt).Expr.(*node.FuncExpr).Type.Params,
		ID:             strings.ReplaceAll(tok.Value, "-", "__"),
	}

	p.currentComp().Slots = append(p.currentComp().Slots, s)

	if p.currenttoken.Kind == tokIndent {
		s.Block = p.parseBlock(s)
		if len(s.Block.Children) > 0 {
			if w, ok := s.Block.Children[0].(*Wrap); ok {
				s.Block.Children = s.Block.Children[1:]
				s.Wrap = w
			}
		}
	}
	return
}

func (p *Parser) parseWrap() (w *Wrap) {
	p.expect(tokWrap)
	w = &Wrap{
		SourcePosition: p.pos(),
	}
	w.Block = p.parseBlock(w)
	return
}

func (p *Parser) parseSlotPass() (s *SlotPass) {
	var (
		tok     = p.expect(tokSlotPass)
		c       = mustParseFirstStmt(tok.Data["Header"], false).(*node.ExprStmt).Expr.(*node.CallExpr)
		ft, err = c.CallArgs.ToFuncParams()
	)

	if err != nil {
		panic(fmt.Errorf("Parsing slot pass failed: %v", err))
	}

	s = &SlotPass{
		SourcePosition: p.pos(),
		Name:           c.Func,
		FuncType: &node.FuncType{
			Params: *ft,
		},
	}

	if p.currenttoken.Kind == tokIndent {
		s.Block = p.parseBlock(s)
	}
	return
}

func (p *Parser) parseDoctype() *Doctype {
	tok := p.expect(tokDoctype)
	node := newDoctype(tok.Value)
	node.SourcePosition = p.pos()
	return node
}

func (p *Parser) parseComment() *Comment {
	tok := p.expect(tokComment)
	cmnt := newComment(tok.Value)
	cmnt.SourcePosition = p.pos()
	cmnt.Silent = tok.Data["Mode"] == "silent"

	if p.currenttoken.Kind == tokIndent {
		cmnt.Block = p.parseBlock(cmnt)
	}

	return cmnt
}

func (p *Parser) parseText() *Text {
	tok := p.expect(tokText)
	stmts, err := parse(tok.Value, true)
	if err != nil {
		panic(err)
	}
	node := newText(stmts)
	node.SourcePosition = p.pos()
	return node
}

func (p *Parser) parseAssignment() *Assignment {
	tok := p.expect(tokAssignment)
	node := newAssignment(tok.Data["X"], tok.Data["Op"], tok.Value)
	node.SourcePosition = p.pos()
	return node
}

func (p *Parser) parseCode() *Code {
	tok := p.expect(tokCode)
	node := newCode(tok.Values)
	node.SourcePosition = p.pos()
	return node
}

func (p *Parser) parseTag() *Tag {
	tok := p.expect(tokTag)
	tag := newTag(tok.Value)
	tag.SourcePosition = p.pos()

	ensureBlock := func() {
		if tag.Block == nil {
			tag.Block = newBlock()
		}
	}

readmore:
	switch p.currenttoken.Kind {
	case tokIndent:
		if tag.IsRawText() {
			p.scanner.readRaw = true
		}

		block := p.parseBlock(tag)
		if tag.Block == nil {
			tag.Block = block
		} else {
			for _, c := range block.Children {
				tag.Block.push(c)
			}
		}
	case tokId:
		id := p.expect(tokId)
		if len(id.Data["Condition"]) > 0 {
			panic("Conditional attributes must be placed in a block within a tag.")
		}
		tag.Attributes = append(tag.Attributes, Attribute{p.pos(), "id", id.Value, true, false, "", nil})
		goto readmore
	case tokClassName:
		cls := p.expect(tokClassName)
		if len(cls.Data["Condition"]) > 0 {
			panic("Conditional attributes must be placed in a block within a tag.")
		}
		tag.Attributes = append(tag.Attributes, Attribute{p.pos(), "class", cls.Value, true, false, "", nil})
		goto readmore
	case tokAttribute:
		attr := p.expect(tokAttribute)
		if len(attr.Data["Condition"]) > 0 {
			panic("Conditional attributes must be placed in a block within a tag.")
		}

		var elements *node.KeyValueArrayLit
		if attr.AnyValue != nil {
			elements = attr.AnyValue.(*node.KeyValueArrayLit)
		}
		tag.Attributes = append(tag.Attributes, Attribute{p.pos(), attr.Value, attr.Data["Content"], attr.Data["Mode"] == "raw", attr.Data["Flag"] == "true", "", elements})
		goto readmore
	case tokText:
		if p.currenttoken.Data["Mode"] != "piped" {
			ensureBlock()
			tag.Block.pushFront(p.parseText())
			goto readmore
		}
	}

	return tag
}

func (p *Parser) parseFunc() *Func {
	tok := p.expect(tokFunc)
	c := mustParseFirstStmt("func("+tok.Data["Args"]+"){}", false)
	f := &Func{
		Name:           tok.Value,
		Params:         &c.(*node.ExprStmt).Expr.(*node.FuncExpr).Type.Params,
		Exported:       tok.Data["Exported"] == "true",
		SourcePosition: p.pos(),
	}

	if p.currenttoken.Kind == tokIndent {
		f.Block = p.parseBlock(f)
	}
	return f
}

func (p *Parser) parseComp() *Comp {
	tok := p.expect(tokComp)
	c := mustParseFirstStmt("func("+tok.Data["Args"]+"){}", false)
	comp := newComp(tok.Value, &c.(*node.ExprStmt).Expr.(*node.FuncExpr).Type.Params, tok.Data["Exported"] == "true")
	comp.SourcePosition = p.pos()

	if p.currenttoken.Kind == tokIndent {
		comp.Block = p.parseBlock(comp)
	}

	return comp
}

func (p *Parser) parseCompCall() *CompCall {
	tok := p.expect(tokCompCall)
	call := &CompCall{
		Name: tok.Value,
	}

	if header := tok.Data["Args"]; header != "" {
		c := mustParseFirstStmt("x("+header+")", false)
		call.Args = c.(*node.ExprStmt).Expr.(*node.CallExpr).CallArgs
	}

	call.SourcePosition = p.pos()
	if p.currenttoken.Kind == tokIndent {
		block := p.parseBlock(call)
		var newChild []Node
		for _, child := range block.Children {
			switch t := child.(type) {
			case *SlotPass:
				call.SlotPass = append(call.SlotPass, t)
			default:
				newChild = append(newChild, t)
			}
		}

		if tok.Data["WithCode"] == "true" {
			if len(newChild) > 0 {
				var ok bool
				if call.InitCode, ok = newChild[0].(*Code); ok {
					newChild = newChild[1:]
				}
			}
		}

		if len(newChild) > 0 {
			block.Children = newChild
			call.SlotPass = append(call.SlotPass, &SlotPass{
				SourcePosition: newChild[0].Pos(),
				Name:           node.EIdent("main", 0),
				FuncType:       node.ProxyFuncType(),
				Block:          block,
			})
		}
	}
	return call
}

func (p *Parser) parseSwitch() *Switch {
	tok := p.expect(tokSwitch)
	sw := &Switch{
		Expr:           tok.Value,
		SourcePosition: p.pos(),
	}

	if p.currenttoken.Kind == tokIndent {
		p.next()
	next:
		switch p.currenttoken.Kind {
		case tokCase:
			tok := p.expect(tokCase)
			swCase := &Case{
				Expr:           tok.Value,
				SourcePosition: p.pos(),
			}
			if p.currenttoken.Kind == tokIndent {
				swCase.Content = p.parseBlock(swCase)
			}
			sw.Cases = append(sw.Cases, swCase)
			goto next
		case tokDefault:
			p.expect(tokDefault)
			def := &Default{
				SourcePosition: p.pos(),
			}
			sw.Default = def
			if p.currenttoken.Kind == tokIndent {
				def.Content = p.parseBlock(sw)
			}
			p.expect(tokOutdent)
		default:
			p.expect(tokCase, tokDefault, tokOutdent)
		}
	}

	return sw
}

func (p *Parser) parseExport() *Export {
	tok := p.expect(tokExport)
	ex := &Export{
		SourcePosition: p.pos(),
		Name:           tok.Data["Name"],
		Value:          tok.Data["Value"],
	}
	return ex
}
