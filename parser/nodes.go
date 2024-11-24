package parser

import (
	"fmt"
	"strings"

	"github.com/gad-lang/gad/parser/node"
)

var selfClosingTags = [...]string{
	"meta",
	"img",
	"link",
	"input",
	"source",
	"area",
	"base",
	"col",
	"br",
	"hr",
}

var doctypes = map[string]string{
	"5":            `<!DOCTYPE html>`,
	"default":      `<!DOCTYPE html>`,
	"xml":          `<?xml version="1.0" encoding="utf-8" ?>`,
	"transitional": `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">`,
	"strict":       `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Strict//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-strict.dtd">`,
	"frameset":     `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Frameset//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-frameset.dtd">`,
	"1.1":          `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.1//EN" "http://www.w3.org/TR/xhtml11/DTD/xhtml11.dtd">`,
	"basic":        `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML Basic 1.1//EN" "http://www.w3.org/TR/xhtml-basic/xhtml-basic11.dtd">`,
	"mobile":       `<!DOCTYPE html PUBLIC "-//WAPFORUM//DTD XHTML Mobile 1.2//EN" "http://www.openmobilealliance.org/tech/DTD/xhtml-mobile12.dtd">`,
}

type Node interface {
	Pos() SourcePosition
}

type SourcePosition struct {
	LineNum     int
	ColNum      int
	TokenLength int
	Filename    string
}

func (s SourcePosition) String() string {
	var ret string
	if s.Filename != "" {
		ret = s.Filename + " "
	}
	return ret + fmt.Sprintf("%d:%d", s.LineNum, s.ColNum)
}

func (s *SourcePosition) Pos() SourcePosition {
	return *s
}

type Doctype struct {
	SourcePosition
	Value string
}

func newDoctype(value string) *Doctype {
	dt := new(Doctype)
	dt.Value = value
	return dt
}

func (d *Doctype) String() string {
	if defined := doctypes[d.Value]; len(defined) != 0 {
		return defined
	}

	return `<!DOCTYPE ` + d.Value + `>`
}

type Comment struct {
	SourcePosition
	Value  string
	Block  *Block
	Silent bool
}

func newComment(value string) *Comment {
	dt := new(Comment)
	dt.Value = value
	dt.Block = nil
	dt.Silent = false
	return dt
}

type Text struct {
	SourcePosition
	Stmts []node.Stmt
}

func newText(stmts []node.Stmt) *Text {
	dt := new(Text)
	dt.Stmts = stmts
	return dt
}

type Block struct {
	SourcePosition
	Children []Node
}

func newBlock() *Block {
	block := new(Block)
	block.Children = make([]Node, 0)
	return block
}

func (b *Block) push(node Node) {
	if node != nil {
		b.Children = append(b.Children, node)
	}
}

func (b *Block) pushFront(node Node) {
	if node != nil {
		b.Children = append([]Node{node}, b.Children...)
	}
}

func (b *Block) CanInline() bool {
	if len(b.Children) == 0 {
		return true
	}

	allText := true

	for _, child := range b.Children {
		if _, ok := child.(*Text); !ok {
			allText = false
			break
		}
	}

	return allText
}

const (
	NamedBlockDefault = iota
	NamedBlockAppend
	NamedBlockPrepend
)

type NamedBlock struct {
	Block
	Name     string
	Modifier int
}

func newNamedBlock(name string) *NamedBlock {
	bb := new(NamedBlock)
	bb.Name = name
	bb.Block.Children = make([]Node, 0)
	bb.Modifier = NamedBlockDefault
	return bb
}

type Attribute struct {
	SourcePosition
	Name      string
	Value     string
	IsRaw     bool
	IsFlag    bool
	Condition string
	Elements  []*node.KeyValueLit
}

type Tag struct {
	SourcePosition
	Block          *Block
	Name           string
	IsInterpolated bool
	Attributes     []Attribute
}

func newTag(name string) *Tag {
	tag := new(Tag)
	tag.Block = nil
	tag.Name = name
	tag.Attributes = make([]Attribute, 0)
	tag.IsInterpolated = false
	return tag

}

func (t *Tag) IsSelfClosing() bool {
	for _, tag := range selfClosingTags {
		if tag == t.Name {
			return true
		}
	}

	return false
}

func (t *Tag) IsRawText() bool {
	return t.Name == "style" || t.Name == "script"
}

type Condition struct {
	SourcePosition
	Block      *Block
	Expression string
	skips      bool
}

type If struct {
	SourcePosition
	Positives []*Condition
	Negative  *Block
	skips     bool
}

type For struct {
	SourcePosition
	Args       string
	Expression string
	Block      *Block
	Else       *Block
}

func newFor(exp string) *For {
	each := new(For)
	each.Expression = exp
	return each
}

type Assignment struct {
	SourcePosition
	Op         string
	X          string
	Expression string
}

func newAssignment(x, op string, expression string) *Assignment {
	assgn := new(Assignment)
	assgn.Op = op
	assgn.X = x
	assgn.Expression = expression
	return assgn
}

type Code struct {
	SourcePosition
	Expressions         []string
	TrimRigth, TrimLeft bool
}

func newCode(expressions []string) *Code {
	var (
		trimRigth bool
		trimLeft  bool
	)
	l := len(expressions)

	if l > 0 {
		if trimLeft = expressions[0] == "-"; trimLeft {
			expressions = expressions[1:]
			l--
		}
	}

	if l > 0 {
		if trimRigth = expressions[l-1] == "-"; trimRigth {
			expressions = expressions[:l-1]
		}
	}
	return &Code{Expressions: expressions, TrimRigth: trimRigth, TrimLeft: trimLeft}
}

type Mixin struct {
	SourcePosition
	Block    *Block
	Name     string
	Args     string
	Override bool
	ID       string
	Exported bool
}

func newMixin(name, args string, exported bool) *Mixin {
	mixin := new(Mixin)
	mixin.Override = name[0] == '='
	if mixin.Override {
		name = name[1:]
	}
	mixin.ID = strings.ReplaceAll(name, "-", "__")
	mixin.Name = name
	mixin.Args = args
	mixin.Exported = exported
	return mixin
}

type MixinCall struct {
	SourcePosition
	Name  string
	Args  string
	Block *Block
}

func newMixinCall(name, args string) *MixinCall {
	mixinCall := new(MixinCall)
	mixinCall.Name = name
	mixinCall.Args = args
	return mixinCall
}

type Case struct {
	SourcePosition
	Expr    string
	Content *Block
}

type Default struct {
	SourcePosition
	Content *Block
}

type Switch struct {
	SourcePosition
	Expr    string
	Cases   []*Case
	Default *Default
}

type Init struct {
	SourcePosition
	Exprs []string
}

type Root struct {
	Block
	Inits  []*Init
	Mixins []*Mixin
}

type Export struct {
	SourcePosition
	Name  string
	Value string
}
