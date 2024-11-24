package parser

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/parser/node"
)

type FileSystem interface {
	Open(name string) (io.ReadCloser, error)
}

type Parser struct {
	scanner      *scanner
	filename     string
	fs           FileSystem
	currenttoken *token
	namedBlocks  map[string]*NamedBlock
	inits        []*Init
	parent       *Parser
	result       *Root
	blockStack   []Node
	mixins       []*Mixin
}

func newParser(rdr io.Reader) *Parser {
	p := new(Parser)
	p.scanner = newScanner(rdr)
	p.namedBlocks = make(map[string]*NamedBlock)
	return p
}

func StringParser(input string) (*Parser, error) {
	return newParser(bytes.NewReader([]byte(input))), nil
}

func ByteParser(input []byte) (*Parser, error) {
	return newParser(bytes.NewReader(input)), nil
}

func (p *Parser) SetFilename(filename string) {
	p.filename = filename
}

func (p *Parser) SetVirtualFilesystem(fs FileSystem) {
	p.fs = fs
}

func FileParser(filename string) (*Parser, error) {
	data, err := os.ReadFile(filename)

	if err != nil {
		return nil, err
	}

	parser := newParser(bytes.NewReader(data))
	parser.filename = filename
	return parser, nil
}

func VirtualFileParser(filename string, fs FileSystem) (*Parser, error) {
	file, err := fs.Open(filename)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	parser := newParser(bytes.NewReader(data))
	parser.filename = filename
	parser.fs = fs
	return parser, nil
}

func (p *Parser) Parse() *Root {
	if p.result != nil {
		return p.result
	}

	defer func() {
		if r := recover(); r != nil {
			if rs, ok := r.(string); ok && rs[:len("Gber Error")] == "Gber Error" {
				panic(r)
			}

			pos := p.pos()

			if len(pos.Filename) > 0 {
				panic(fmt.Sprintf("Gber Error in <%s>: %v - Line: %d, Column: %d, Length: %d", pos.Filename, r, pos.LineNum, pos.ColNum, pos.TokenLength))
			} else {
				panic(fmt.Sprintf("Gber Error: %v - Line: %d, Column: %d, Length: %d", r, pos.LineNum, pos.ColNum, pos.TokenLength))
			}
		}
	}()

	root := &Root{}
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

	if len(p.inits) > 0 {
		p.inits[0].Exprs = append([]string{"", "// from: " + p.filename}, p.inits[0].Exprs...)
	}

	root.Inits = p.inits
	root.Mixins = p.mixins

	if p.parent != nil {
		p.parent.Parse()

		p.parent.result.Inits = append(p.parent.result.Inits, root.Inits...)
		p.parent.result.Mixins = append(p.parent.result.Mixins, root.Mixins...)

		for _, prev := range p.parent.namedBlocks {
			ours := p.namedBlocks[prev.Name]

			if ours == nil {
				// Put a copy of the named block into current context, so that sub-templates can use the block
				p.namedBlocks[prev.Name] = prev
				continue
			}

			top := findTopmostParentWithNamedBlock(p, prev.Name)
			nb := top.namedBlocks[prev.Name]
			switch ours.Modifier {
			case NamedBlockAppend:
				for i := 0; i < len(ours.Children); i++ {
					nb.push(ours.Children[i])
				}
			case NamedBlockPrepend:
				for i := len(ours.Children) - 1; i >= 0; i-- {
					nb.pushFront(ours.Children[i])
				}
			default:
				nb.Children = ours.Children
			}
		}

		root = p.parent.result
	}

	p.result = root
	return root
}

func (p *Parser) pos() SourcePosition {
	pos := p.scanner.Pos()
	pos.Filename = p.filename
	return pos
}

func (p *Parser) parseRelativeFile(filename string) *Parser {
	if len(p.filename) == 0 {
		panic("Unable to import or extend " + filename + " in a non filesystem based parser.")
	}

	if filename[0] != '/' {
		filename = filepath.Join(filepath.Dir(p.filename), filename)
	}

	if strings.IndexRune(filepath.Base(filename), '.') < 0 {
		filename = filename + ".gber"
	}

	parser, err := FileParser(filename)
	if err != nil && p.fs != nil {
		parser, err = VirtualFileParser(filename, p.fs)
	}
	if err != nil {
		panic("Unable to read " + filename + ", Error: " + string(err.Error()))
	}

	return parser
}

func (p *Parser) parse() Node {
	switch p.currenttoken.Kind {
	case tokInit:
		return p.parseInit()
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
	case tokImport:
		return p.parseImport()
	case tokImportModule:
		return p.parseImportModule()
	case tokTag:
		return p.parseTag()
	case tokAssignment:
		return p.parseAssignment()
	case tokCode:
		return p.parseCode()
	case tokNamedBlock:
		return p.parseNamedBlock()
	case tokExtends:
		return p.parseExtends()
	case tokIndent:
		return p.parseBlock(nil)
	case tokMixin:
		p.mixins = append(p.mixins, p.parseMixin())
		return nil
	case tokMixinCall:
		return p.parseMixinCall()
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

func (p *Parser) parseExtends() *Block {
	if p.parent != nil {
		panic("Unable to extend multiple parent templates.")
	}

	tok := p.expect(tokExtends)
	parser := p.parseRelativeFile(tok.Value)
	parser.Parse()
	p.parent = parser
	return newBlock()
}

func (p *Parser) parseBlock(parent Node) *Block {
	p.expect(tokIndent)
	block := newBlock()
	block.SourcePosition = p.pos()
	p.blockStack = append(p.blockStack, parent)
	defer func() {
		p.blockStack = p.blockStack[:len(p.blockStack)-1]
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
					var elements []*node.KeyValueLit
					if attr.AnyValue != nil {
						elements = attr.AnyValue.([]*node.KeyValueLit)
					}
					tag.Attributes = append(tag.Attributes, Attribute{p.pos(), attr.Value, attr.Data["Content"], attr.Data["Mode"] == "raw", false, cond, elements})
				}

				continue
			} else {
				if cond, ok := parent.(*If); ok {
					if tag, ok := p.blockStack[len(p.blockStack)-2].(*Tag); ok {
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
							var elements []*node.KeyValueLit
							if attr.AnyValue != nil {
								elements = attr.AnyValue.([]*node.KeyValueLit)
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

func (p *Parser) parseImport() *Root {
	tok := p.expect(tokImport)
	node := p.parseRelativeFile(tok.Value).Parse()
	node.SourcePosition = p.pos()
	return node
}

func (p *Parser) parseImportModule() *Code {
	tok := p.expect(tokImportModule)
	ident := tok.Data["ident"]
	node := &Code{
		Expressions: []string{
			"const " + ident + " = import(" + strconv.Quote(tok.Value[1:len(tok.Value)-1]+".gber") + ")",
		}}
	node.SourcePosition = p.pos()
	return node
}

func (p *Parser) parseNamedBlock() *Block {
	tok := p.expect(tokNamedBlock)

	if p.namedBlocks[tok.Value] != nil {
		panic("Multiple definitions of named blocks are not permitted. Block " + tok.Value + " has been re defined.")
	}

	block := newNamedBlock(tok.Value)
	block.SourcePosition = p.pos()

	if tok.Data["Modifier"] == "append" {
		block.Modifier = NamedBlockAppend
	} else if tok.Data["Modifier"] == "prepend" {
		block.Modifier = NamedBlockPrepend
	}

	if p.currenttoken.Kind == tokIndent {
		block.Block = *(p.parseBlock(nil))
	}

	p.namedBlocks[block.Name] = block

	if block.Modifier == NamedBlockDefault {
		return &block.Block
	}

	return newBlock()
}

func (p *Parser) parseInit() Node {
	tok := p.expect(tokInit)
	var init = &Init{
		SourcePosition: p.pos(),
		Exprs:          tok.Values,
	}
	p.inits = append(p.inits, init)
	return nil
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
	stmts, err := parse(tok.Value)
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

		var elements []*node.KeyValueLit
		if attr.AnyValue != nil {
			elements = attr.AnyValue.([]*node.KeyValueLit)
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

func (p *Parser) parseMixin() *Mixin {
	tok := p.expect(tokMixin)
	mixin := newMixin(tok.Value, tok.Data["Args"], tok.Data["Exported"] == "true")
	mixin.SourcePosition = p.pos()

	if p.currenttoken.Kind == tokIndent {
		mixin.Block = p.parseBlock(mixin)
	}

	return mixin
}

func (p *Parser) parseMixinCall() *MixinCall {
	tok := p.expect(tokMixinCall)
	mixinCall := newMixinCall(tok.Value, tok.Data["Args"])
	mixinCall.SourcePosition = p.pos()
	if p.currenttoken.Kind == tokIndent {
		mixinCall.Block = p.parseBlock(mixinCall)
	}
	return mixinCall
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

func findTopmostParentWithNamedBlock(p *Parser, name string) *Parser {
	top := p

	for {
		if top.namedBlocks[name] == nil {
			return nil
		}
		if top.parent == nil {
			return top
		}
		if top.parent.namedBlocks[name] != nil {
			top = top.parent
		} else {
			return top
		}
	}
}

func parse(s string) (_ []node.Stmt, err error) {
	fileSet := parser.NewFileSet()
	srcFile := fileSet.AddFile("(main)", -1, len(s))
	p := parser.NewParserWithOptions(srcFile, []byte(s), &parser.ParserOptions{
		Mode: parser.ParseMixed | parser.ParseConfigDisabled | parser.ParseMixedExprAsValue,
	}, &parser.ScannerOptions{
		MixedExprRune: '$',
	})

	var f *parser.File
	if f, err = p.ParseFile(); err != nil {
		return
	}
	return f.Stmts, err
}

const dollar = "__DOLLAR__"
