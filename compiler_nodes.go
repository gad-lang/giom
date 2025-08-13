package giom

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	gp "github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/parser/node"
	"github.com/gad-lang/gad/parser/source"
	gt "github.com/gad-lang/gad/token"
	"github.com/gad-lang/giom/parser"
)

func (c *Compiler) visit(node parser.Node) {
	defer func() {
		if r := recover(); r != nil {
			if rs, ok := r.(string); ok && rs[:len("giom Error")] == "giom Error" {
				panic(r)
			}

			pos := node.Pos()

			if len(pos.Filename) > 0 {
				panic(fmt.Sprintf("giom Error in <%s>: %v - Line: %d, Column: %d, Length: %d", pos.Filename, r, pos.LineNum, pos.ColNum, pos.TokenLength))
			} else {
				panic(fmt.Sprintf("giom Error: %v - Line: %d, Column: %d, Length: %d", r, pos.LineNum, pos.ColNum, pos.TokenLength))
			}
		}
	}()

	switch t := node.(type) {
	case *parser.Root:
		c.visitRoot(t)
	case *parser.Block:
		c.visitBlock(t)
	case *parser.Doctype:
		c.visitDoctype(t)
	case *parser.Comment:
		c.visitComment(t)
	case *parser.Tag:
		c.visitTag(t)
	case *parser.Text:
		c.visitText(t)
	case *parser.If:
		c.visitCondition(t)
	case *parser.For:
		c.visitFor(t)
	case *parser.Assignment:
		c.visitAssignment(t)
	case *parser.Func:
		c.visitFunc(t)
	case *parser.Comp:
	case *parser.CompCall:
		c.visitCompCall(t)
	case *parser.Code:
		c.visitCode(t)
	case *parser.Switch:
		c.visitSwitch(t)
	case *parser.Export:
		c.visitExport(t)
	case *parser.Slot:
		c.visitSlot(t)
	}
}

func (c *Compiler) write(value string) {
	c.writer.Write([]byte(value))
}

func (c *Compiler) indent(offset int, newline bool) {
	if !c.PrettyPrint {
		return
	}

	if newline && c.writer.len > 0 {
		c.write("\n")
	}

	for i := 0; i < c.indentLevel+offset; i++ {
		c.write("\t")
	}
}

func (c *Compiler) escape(input string) string {
	return strings.Replace(strings.Replace(input, `\`, `\\`, -1), `"`, `\"`, -1)
}

func (c *Compiler) visitRoot(root *parser.Root) {
	c.visitBlock(&root.Block)
	for _, mixin := range root.Comps {
		c.visitComp(mixin)
	}
}

func (c *Compiler) visitBlock(block *parser.Block) {
	if block == nil {
		return
	}
	for _, node := range block.Children {
		if _, ok := node.(*parser.Text); !block.CanInline() && ok {
			c.indent(0, true)
		}

		c.visit(node)
	}
}

func (c *Compiler) visitDoctype(doctype *parser.Doctype) {
	c.write(doctype.String())
}

func (c *Compiler) visitComment(comment *parser.Comment) {
	if comment.Silent {
		return
	}

	c.indent(0, false)

	if comment.Block == nil {
		c.write(`{% unescaped("<!-- ` + c.escape(comment.Value) + ` -->") %}`)
	} else {
		c.write(`<!-- ` + comment.Value)
		c.visitBlock(comment.Block)
		c.write(` -->`)
	}
}

func (c *Compiler) visitCondition(condition *parser.If) {
	positives := condition.Positives
	positive := positives[0]
	c.write(`{% if ` + positive.Expression + ` %}`)
	if positive.Block != nil {
		c.visitBlock(positive.Block)
	}
	for _, positive = range positives[1:] {
		c.write(`{% else if ` + positive.Expression + ` %}`)
		if positive.Block != nil {
			c.visitBlock(positive.Block)
		}
	}
	if condition.Negative != nil {
		c.write(`{% else %}`)
		c.visitBlock(condition.Negative)
	}
	c.write(`{% end %}`)
}

func (c *Compiler) visitFor(for_ *parser.For) {
	if for_.Block == nil {
		return
	}
	c.write(`{% ` + strings.TrimSpace(for_.Expression) + ` %}`)
	c.visitBlock(for_.Block)
	if for_.Else != nil {
		c.write(`{% else %}`)
		c.visitBlock(for_.Else)
	}
	c.write(`{% end %}`)
}

func (c *Compiler) visitAssignment(assgn *parser.Assignment) {
	op := assgn.Op
	c.write(`{% ` + assgn.X + op + `=` + assgn.Expression + `; %}`)
}

func (c *Compiler) visitCode(cd *parser.Code) {
	var (
		l     = len(cd.Expressions)
		exprs = cd.Expressions
	)

	if l > 0 {
		c.write(`{% `)
		if l == 1 {
			c.write(strings.TrimSpace(exprs[0]))
			c.write(";")
		} else {
			c.write("\n")
			for i, expr := range exprs {
				c.write(expr)
				if i < l-1 {
					c.write("\n")
				}
			}
			c.write("\n")
		}
		var right string
		if cd.TrimRigth {
			right = "-"
		}
		c.write(right + ` %}`)
	}
}

func gadParser(value string) *gp.Parser {
	fs := source.NewFileSet()
	sf := fs.AddFileData("", -1, []byte(value))
	return gp.NewParser(sf, nil)
}

func (c *Compiler) visitTag(tag *parser.Tag) {
	parseAttr := func(value string) (items []*node.KeyValueLit) {
		p := gadParser("[" + value + "]")
		lbrack := p.Expect(gt.LBrack)
		ret := p.ParseKeyValueArrayLitAt(lbrack, gt.RBrack)
		if err := p.Errors.Err(); err != nil {
			panic(err)
		}
		return ret.Elements
	}

	var (
		attribs []*node.KeyValueLit
		am      = map[string]*struct {
			Key    node.Expr
			Values []node.Expr
		}{}
	)

	for _, item := range tag.Attributes {
		if item.Elements != nil {
			attribs = append(attribs, item.Elements...)
			continue
		}
		value := item.Value

		if item.IsRaw && item.Value != "" {
			if item.Condition != "" {
				p := gadParser("true ? " + item.Condition)
				exp := p.ParseExpr()
				if err := p.Errors.Err(); err != nil {
					qcond := strconv.Quote(item.Condition)
					qcond = qcond[1 : len(qcond)-1]
					value = `(throw error("parse tag '` + tag.Name + `': attribute '` + item.Name +
						`': condition '` + qcond + `': ` + strconv.Quote(err.Error())[1:] + "))"
				} else if cond, ok := exp.(*node.CondExpr); ok {
					value = cond.True.String() + " ? " + strconv.Quote(value) + ":" + cond.False.String()
				}
			} else {
				value = strconv.Quote(value)
			}
		}

		if strings.Contains(item.Name, "-") || strings.Contains(item.Name, ":") {
			item.Name = strconv.Quote(item.Name)
		}

		attribs = append(attribs, parseAttr(item.Name+"="+value)...)
	}

	var keys []string

	for _, at := range attribs {
		k := at.Key.String()
		if old, ok := am[k]; ok {
			old.Values = append(old.Values, at.Value)
		} else {
			keys = append(keys, k)
			am[k] = &struct {
				Key    node.Expr
				Values []node.Expr
			}{Key: at.Key, Values: []node.Expr{at.Value}}
		}
	}

	sort.Strings(keys)

	attribs = nil

	for _, key := range keys {
		a := am[key]
		kv := &node.KeyValueLit{Key: a.Key}
		switch len(a.Values) {
		case 0:
		case 1:
			kv.Value = a.Values[0]
		default:
			kv.Value = &node.ArrayExpr{Elements: a.Values}
		}
		attribs = append(attribs, kv)
	}

	c.indent(0, true)

	c.write("<" + tag.Name)

	if len(attribs) > 0 {
		c.write("{%=attrs")
		c.write((&node.KeyValueArrayLit{Elements: attribs}).String())
		c.write("%}")
	}

	if tag.IsSelfClosing() {
		c.write(` />`)
	} else {
		c.write(`>`)

		if tag.Block != nil {
			if !tag.Block.CanInline() {
				c.indentLevel++
			}

			c.visitBlock(tag.Block)

			if !tag.Block.CanInline() {
				c.indentLevel--
				c.indent(0, true)
			}
		}

		c.write(`</` + tag.Name + `>`)
	}
}

func (c *Compiler) visitText(txt *parser.Text) {
	var (
		ident = node.EIdent(BuiltinTextWrite.Name, 0)
		nodes []node.Node
	)

	var newCall = func(expr ...node.Expr) *node.CallExpr {
		return node.ECall(ident, 0, 0, node.NewCallExprArgs(nil, expr...))
	}

	txt.Stmts.Each(func(i int, sep bool, s node.Stmt) {
		switch t := s.(type) {
		case *node.CodeBeginStmt, *node.CodeEndStmt:
		case *node.MixedTextStmt:
			nodes = append(nodes, newCall(node.String(t.Value(), 0)))
		case *node.MixedValueStmt:
			nodes = append(nodes, newCall(t.Expr))
		default:
			nodes = append(nodes, s)
		}
	})

	var stmts node.Stmts
	var prev node.Node

	for i, n := range nodes {
		var s, _ = n.(node.Stmt)

		if i > 0 {
			if call, _ := n.(*node.CallExpr); call != nil {
				if prevC, _ := prev.(*node.CallExpr); prevC != nil {
					prevC.Args.Values = append(prevC.Args.Values, call.Args.Values...)
					continue
				} else {
					s = node.SExpr(n.(node.Expr))
				}
			}
		} else if s == nil {
			s = node.SExpr(n.(node.Expr))
		}

		stmts.Append(s)
		prev = n
	}

	c.write("{% ")
	node.CodeStmtsW(c.writer, stmts)
	c.write(" %}")
}

func (c *Compiler) visitFunc(f *parser.Func) {
	c.write(`{% const ` + f.Name + ` = func` + f.Params.String() + ` %}`)
	c.visitBlock(f.Block)
	c.write(`{% end %}`)

	if f.Exported {
		c.exports[f.Name] = f.Name
	}
}

func (c *Compiler) visitComp(comp *parser.Comp) {
	args := comp.Params
	args.NamedArgs.Add(node.ETypedIdent(node.EIdent("$slots", 0)), node.EDict(0, 0))
	c.write(`{% const ` + comp.ID + ` = func` + args.String() + ` %}`)

	if comp.Block != nil {
		var (
			funcs    []parser.Node
			children []parser.Node
		)

		if code, ok := comp.Block.Children[0].(*parser.Code); ok {
			comp.Block.Children = comp.Block.Children[1:]
			c.visit(code)
		}

		for _, child := range comp.Block.Children {
			if f, _ := child.(*parser.Func); f != nil {
				funcs = append(funcs, f)
			} else {
				children = append(children, child)
			}
		}
		if len(funcs) > 0 {
			comp.Block.Children = children
			blockFuncs := *comp.Block
			blockFuncs.Children = funcs
			c.visitBlock(&blockFuncs)
		}
	}

	for _, child := range comp.Comps {
		c.visitComp(child)
	}
	for _, slot := range comp.Slots {
		c.visitSlotDef(slot)
	}
	c.visitBlock(comp.Block)
	c.write(`{% end %}`)

	if comp.Exported {
		c.exportedComps[comp.Name] = comp
	}
}

func (c *Compiler) visitSlotDef(slot *parser.Slot) {
	localName := "$slot$" + slot.ID
	defScope := slot.Scope.WithNamedValuesNil().String()
	c.write(`{% const ` + localName + `$ = func` + defScope + ` %}`)
	c.visitBlock(slot.Block)
	c.write(`{% end %}`)
	if slot.Wrap != nil {
		c.write(`{% const ` + localName + `$wrap = func(slot$) %}`)
		c.write(`{% return func(*args, **kwargs) %}`)
		c.write(`{% const (user_slot = $slots.` + slot.ID + `) %}`)
		c.write(`{% const slot = (*args, **kwargs) => slot$(*args, **kwargs) %}`)
		c.visitBlock(slot.Wrap.Block)
		c.write(`{% end %}`)
		c.write(`{% end %}`)
	}

	if slot.Wrap != nil {
		c.write(`{% var ` + localName + ` = ` + localName + `$wrap($slots.` + slot.ID + ` ?? ` + localName + `$) %}`)
	} else {
		c.write(`{% var ` + localName + ` = $slots.` + slot.ID + ` ?? ` + localName + `$ %}`)
	}
}

func (c *Compiler) visitSlot(slot *parser.Slot) {
	localName := "$slot$" + slot.ID
	slot.Scope.NamedArgs.Add(node.ETypedIdent(node.EIdent("default_slot", 0)), node.EIdent(localName+"$", 0))
	call := slot.Scope.Caller().String()
	c.write(`{% ` + localName + call + ` %}`)
}

func (c *Compiler) visitCompCall(call *parser.CompCall) {
	var (
		name       = strings.ReplaceAll(call.Name, "-", "__")
		slotsNames []string
	)

	c.write("{% do %}")

	if len(call.SlotPass) > 0 {
		for i, slot := range call.SlotPass {
			localName := "slot$" + strconv.Itoa(i)
			slotsNames = append(slotsNames, localName)
			c.write(`{% const ` + localName + ` = func` + slot.FuncType.String() + ` %}`)
			c.visitBlock(slot.Block)
			c.write(`{% end %}`)
		}

		c.write("{% var $$slots = {}; ")
		for i, slot := range call.SlotPass {
			var name string
			switch t := slot.Name.(type) {
			case *node.IdentExpr:
				name = strconv.Quote(t.Name)
			case *node.ParenExpr:
				name = t.Expr.String()
			default:
				name = t.String()
			}
			c.write("$$slots[" + name + "] = slot$" + strconv.Itoa(i) + "; ")
		}
		c.write(" %}")
		if call.InitCode != nil {
			c.visitCode(call.InitCode)
		}
		call.Args.NamedArgs.AppendS("$slots", node.EIdent("$$slots", 0))
	}

	c.write(`{% ` + name + call.Args.String() + ` %}`)

	c.write("{% end %}")
}

func (c *Compiler) visitSwitch(sw *parser.Switch) {
	if len(sw.Cases) == 0 {
		if sw.Default != nil && sw.Default.Content != nil {
			c.visitBlock(sw.Default.Content)
		}
		return
	}

	cases := sw.Cases
	fmt.Fprintf(c.writer, "{%% if %s == %s %%}", sw.Expr, cases[0].Expr)
	if cases[0].Content != nil {
		c.visitBlock(cases[0].Content)
	}
	cases = cases[1:]
	for len(cases) > 0 {
		fmt.Fprintf(c.writer, "{%% else if %s == %s %%}", sw.Expr, cases[0].Expr)
		if cases[0].Content != nil {
			c.visitBlock(cases[0].Content)
		}
		cases = cases[1:]
	}
	if sw.Default != nil {
		c.write("{% else %}")
		if sw.Default.Content != nil {
			c.visitBlock(sw.Default.Content)
		}
	}
	c.write("{% end %}")
}

func (c *Compiler) visitExport(ex *parser.Export) {
	value := ex.Value
	if value == "" {
		value = ex.Name
	}
	c.exports[ex.Name] = value
}

func (c *Compiler) tempVar() string {
	c.tempvarIndex++
	return fmt.Sprint("$$", c.tempvarIndex)
}
