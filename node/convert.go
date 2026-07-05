package node

import (
	"fmt"

	gnode "github.com/gad-lang/gad/parser/node"
	"github.com/gad-lang/gad/parser/source"
	"github.com/gad-lang/gad/token"
)

// Convert recursively converts giom-specific AST nodes to GAD AST nodes,
// returning pure GAD statements suitable for Format.
// Consecutive const/var declarations are merged into grouped declarations.
func Convert(stmts gnode.Stmts) gnode.Stmts {
	var out gnode.Stmts
	for _, s := range stmts {
		out = append(out, convertStmt(s)...)
	}
	return mergeDecls(out)
}

// mergeDecls merges consecutive const/var GenDecl statements into grouped declarations.
func mergeDecls(stmts gnode.Stmts) gnode.Stmts {
	var out gnode.Stmts
	var pending *gnode.GenDecl
	for i, s := range stmts {
		ds, ok := s.(*gnode.DeclStmt)
		if !ok {
			out = appendPending(out, &pending)
			out = append(out, s)
			continue
		}
		gd, ok := ds.Decl.(*gnode.GenDecl)
		if !ok || (gd.Tok != token.Const && gd.Tok != token.Var) {
			out = appendPending(out, &pending)
			out = append(out, s)
			continue
		}
		if pending != nil && pending.Tok == gd.Tok {
			if !pending.Lparen.IsValid() {
				pending.Lparen = stmts[i-1].Pos()
			}
			pending.Specs = append(pending.Specs, gd.Specs...)
			if gd.Rparen.IsValid() {
				pending.Rparen = gd.Rparen
			}
			if i == len(stmts)-1 {
				out = append(out, gnode.SDecl(pending))
			}
		} else {
			out = appendPending(out, &pending)
			pending = gd
			if i == len(stmts)-1 {
				out = append(out, gnode.SDecl(pending))
			}
		}
	}
	return out
}

// appendPending flushes a pending GenDecl to out if it has multiple specs,
// setting Lparen/Rparen for grouped syntax.
func appendPending(out gnode.Stmts, pending **gnode.GenDecl) gnode.Stmts {
	if *pending == nil {
		return out
	}
	gd := *pending
	if len(gd.Specs) > 1 && !gd.Lparen.IsValid() {
		gd.Lparen = 1
	}
	if len(gd.Specs) > 1 && !gd.Rparen.IsValid() {
		gd.Rparen = 1
	}
	out = append(out, gnode.SDecl(gd))
	*pending = nil
	return out
}

func funcType(params *gnode.FuncParams) *gnode.FuncType {
	return &gnode.FuncType{
		FuncPos:    1,
		FuncHeader: gnode.FuncHeader{Params: *params},
	}
}

func funcExpr(params *gnode.FuncParams, body gnode.Stmts, pos, end source.Pos) *gnode.FuncExpr {
	return &gnode.FuncExpr{
		Type: funcType(params),
		Body: gnode.SBlock(pos, end, body...),
	}
}

func convertStmt(s gnode.Stmt) gnode.Stmts {
	switch st := s.(type) {
	case *FuncDecl:
		return convertFuncDecl(st)
	case *CompDecl:
		return convertCompDecl(st)
	case *CompCallStmt:
		return convertCompCall(st)
	case *SwitchStmt:
		return convertSwitch(st)
	case *ExportStmt:
		return convertExport(st)
	case *SlotDecl:
		return convertSlot(st)
	case *SlotPassStmt:
		return convertSlotPass(st)
	case *CodeStmt:
		return st.Stmts
	case *AssignStmt:
		return convertAssign(st)
	case *ForStmt:
		return convertFor(st)
	case *IfStmt:
		return convertIf(st)
	case *DoctypeStmt:
		return convertDoctype(st)
	case *TextStmt:
		return convertText(st)
	case *TagStmt:
		return convertTag(st)
	default:
		return gnode.Stmts{s}
	}
}

func convertAssign(s *AssignStmt) gnode.Stmts {
	return gnode.Stmts{
		&gnode.AssignStmt{
			LHS:      []gnode.Expr{s.LHS},
			RHS:      []gnode.Expr{s.RHS},
			Token:    assignToken(s.Op),
			TokenPos: s.NodePos,
		},
	}
}

func assignToken(op string) token.Token {
	switch op {
	case ":=", ":":
		return token.Define
	case "=":
		return token.Assign
	case "+=":
		return token.AddAssign
	case "-=":
		return token.SubAssign
	case "*=":
		return token.MulAssign
	case "/=":
		return token.QuoAssign
	case "%=":
		return token.RemAssign
	case "??=":
		return token.NullichAssign
	default:
		return token.Assign
	}
}

func convertBody(stmts gnode.Stmts) gnode.Stmts {
	return Convert(stmts)
}

func convertFuncDecl(f *FuncDecl) gnode.Stmts {
	params := addSlotsParam(f.Params)
	return gnode.Stmts{
		gnode.SDecl(&gnode.GenDecl{
			Tok:    token.Const,
			TokPos: f.Pos(),
			Specs: []gnode.Spec{
				&gnode.ValueSpec{
					Idents: []*gnode.IdentExpr{gnode.EIdent(f.Name, f.Pos())},
					Values: []gnode.Expr{
						funcExpr(params, convertBody(f.Body), f.Pos(), f.End()),
					},
				},
			},
		}),
	}
}

func addSlotsParam(params *gnode.FuncParams) *gnode.FuncParams {
	if params == nil {
		return nil
	}
	for _, n := range params.NamedArgs.Names {
		if n != nil && n.Ident != nil && n.Ident.Name == "$slots" {
			return params
		}
	}
	out := *params
	out.NamedArgs.Names = append(out.NamedArgs.Names, &gnode.TypedIdentExpr{Ident: gnode.EIdent("$slots", 0)})
	out.NamedArgs.Values = append(out.NamedArgs.Values, &gnode.DictExpr{})
	return &out
}

func convertCompDecl(c *CompDecl) gnode.Stmts {
	var body gnode.Stmts
	for _, comp := range c.Comps {
		body = append(body, comp)
	}
	body = append(body, convertBody(c.Body)...)

	if c.Name == "main" {
		return body
	}

	return gnode.Stmts{
		gnode.SDecl(&gnode.GenDecl{
			Tok:    token.Const,
			TokPos: c.Pos(),
			Specs: []gnode.Spec{
				&gnode.ValueSpec{
					Idents: []*gnode.IdentExpr{gnode.EIdent(c.ID, c.Pos())},
					Values: []gnode.Expr{
						funcExpr(addSlotsParam(c.Params), body, c.Pos(), c.End()),
					},
				},
			},
		}),
	}
}

func convertCompCall(c *CompCallStmt) gnode.Stmts {
	call := &gnode.CallExpr{
		Func: gnode.EIdent(c.Name, c.Pos()),
	}
	if !c.Args.LParen.IsValid() {
		call.LParen = c.Pos()
	}
	if !c.Args.RParen.IsValid() {
		call.RParen = c.End()
	}
	call.Args = c.Args.Args
	call.NamedArgs = c.Args.NamedArgs

	if len(c.SlotPass) == 0 {
		return gnode.Stmts{gnode.SExpr(call)}
	}

	// With slot passes, wrap in a block:
	//   const $slot0 = func(...) { ... }
	//   var $$slots = {}
	//   $$slots["main"] = $slot0
	//   page_wrapper(args; $slots=$$slots)
	var stmts gnode.Stmts
	for i, sp := range c.SlotPass {
		ft := sp.FuncType
		if ft != nil && !ft.FuncPos.IsValid() {
			ft.FuncPos = sp.Pos()
		}
		stmts.Append(gnode.SDecl(&gnode.GenDecl{
			Tok:    token.Const,
			TokPos: sp.Pos(),
			Specs: []gnode.Spec{
				&gnode.ValueSpec{
					Idents: []*gnode.IdentExpr{gnode.EIdent(fmt.Sprintf("$slot%d", i), sp.Pos())},
					Values: []gnode.Expr{
						&gnode.FuncExpr{
							Type: ft,
							Body: gnode.SBlock(sp.Pos(), sp.End(), convertBody(sp.Body)...),
						},
					},
				},
			},
		}))
	}
	stmts.Append(gnode.SDecl(&gnode.GenDecl{
		Tok:    token.Var,
		TokPos: c.Pos(),
		Specs: []gnode.Spec{
			&gnode.ValueSpec{
				Idents: []*gnode.IdentExpr{gnode.EIdent("$$slots", c.Pos())},
				Values: []gnode.Expr{gnode.EDict(c.Pos(), c.End())},
			},
		},
	}))
	for i, sp := range c.SlotPass {
		stmts = append(stmts, &gnode.AssignStmt{
			LHS: []gnode.Expr{
				&gnode.IndexExpr{
					X:     gnode.EIdent("$$slots", 0),
					Index: gnode.Str(slotPassName(sp), 0),
				},
			},
			RHS:      []gnode.Expr{gnode.EIdent(fmt.Sprintf("$slot%d", i), 0)},
			Token:    token.Assign,
			TokenPos: sp.Pos(),
		})
	}
	call.NamedArgs.AppendS("$slots", gnode.EIdent("$$slots", 0))
	stmts.Append(gnode.SExpr(call))
	return stmts
}

func slotPassName(sp *SlotPassStmt) string {
	if sp.Name != nil {
		if s, ok := sp.Name.(*gnode.StrLit); ok {
			return s.Value()
		}
		if s, ok := sp.Name.(*gnode.IdentExpr); ok {
			return s.Name
		}
	}
	return "default"
}

func convertSwitch(s *SwitchStmt) gnode.Stmts {
	return gnode.Stmts{gnode.SExpr(switchMatchExpr(s))}
}

func switchMatchExpr(s *SwitchStmt) *gnode.MatchExpr {
	match := &gnode.MatchExpr{
		MatchPos: s.Pos(),
		Expr:     s.Tag,
		LBrace:   s.Pos(),
		RBrace:   s.End(),
	}
	for _, c := range s.Cases {
		match.Arms = append(match.Arms, &gnode.MatchArm{
			Conds: []gnode.Expr{c.Expr},
			Body:  gnode.SBlock(s.Pos(), s.End(), convertBody(c.Body)...),
		})
	}
	if len(s.Default) > 0 {
		match.Arms = append(match.Arms, &gnode.MatchArm{
			Body: gnode.SBlock(s.Pos(), s.End(), convertBody(s.Default)...),
		})
	}
	return match
}

func convertExport(e *ExportStmt) gnode.Stmts {
	return gnode.Stmts{
		&gnode.ExportStmt{
			TokenPos:  e.Pos(),
			KeyExpr:   gnode.EIdent(e.Name, e.Pos()),
			ValueExpr: e.Value,
		},
	}
}

func convertSlot(s *SlotDecl) gnode.Stmts {
	var stmts gnode.Stmts
	stmts.Append(gnode.SDecl(&gnode.GenDecl{
		Tok:    token.Const,
		TokPos: s.Pos(),
		Specs: []gnode.Spec{
			&gnode.ValueSpec{
				Idents: []*gnode.IdentExpr{gnode.EIdent("$slot$"+s.ID+"$", s.Pos())},
				Values: []gnode.Expr{
					funcExpr(s.Scope, convertBody(s.Body), s.Pos(), s.End()),
				},
			},
		},
	}))
	stmts.Append(gnode.SDecl(&gnode.GenDecl{
		Tok:    token.Var,
		TokPos: s.Pos(),
		Specs: []gnode.Spec{
			&gnode.ValueSpec{
				Idents: []*gnode.IdentExpr{gnode.EIdent("$slot$"+s.ID, s.Pos())},
				Values: []gnode.Expr{
					&gnode.BinaryExpr{
						LHS: &gnode.IndexExpr{
							X:     gnode.EIdent("$slots", 0),
							Index: gnode.Str(s.ID, 0),
						},
						RHS:      gnode.EIdent("$slot$"+s.ID+"$", 0),
						Token:    token.Nullich,
						TokenPos: s.Pos(),
					},
				},
			},
		},
	}))
	// Invoke the slot at this position in the body
	stmts.Append(gnode.SExpr(&gnode.CallExpr{
		Func: gnode.EIdent("$slot$"+s.ID, 0),
	}))
	return stmts
}

func convertSlotPass(s *SlotPassStmt) gnode.Stmts {
	return gnode.Stmts{
		gnode.SDecl(&gnode.GenDecl{
			Tok:    token.Const,
			TokPos: s.Pos(),
			Specs: []gnode.Spec{
				&gnode.ValueSpec{
					Idents: []*gnode.IdentExpr{gnode.EIdent("$slot", s.Pos())},
					Values: []gnode.Expr{
						&gnode.FuncExpr{
							Type: s.FuncType,
							Body: gnode.SBlock(s.Pos(), s.End(), convertBody(s.Body)...),
						},
					},
				},
			},
		}),
	}
}

func convertFor(f *ForStmt) gnode.Stmts {
	if bin, ok := f.Cond.(*gnode.BinaryExpr); ok && bin.Token == token.In {
		if val, ok := bin.LHS.(*gnode.IdentExpr); ok {
			return gnode.Stmts{
				&gnode.ForInStmt{
					ForPos:   f.Pos(),
					Key:      &gnode.IdentExpr{Name: "_"},
					Value:    val,
					Iterable: bin.RHS,
					Body:     gnode.SBlock(f.Pos(), f.End(), convertBody(f.Body)...),
				},
			}
		}
	}
	return gnode.Stmts{
		&gnode.ForStmt{
			ForPos: f.Pos(),
			Init:   f.Init,
			Cond:   f.Cond,
			Post:   f.Post,
			Body:   gnode.SBlock(f.Pos(), f.End(), convertBody(f.Body)...),
		},
	}
}

func convertIf(s *IfStmt) gnode.Stmts {
	body := gnode.SBlock(s.Pos(), s.End(), convertBody(s.Body)...)
	var elseStmt gnode.Stmt
	if len(s.Else) > 0 {
		elseStmt = gnode.SBlock(s.Pos(), s.End(), convertBody(s.Else)...)
	}
	for i := len(s.ElseIfs) - 1; i >= 0; i-- {
		eif := s.ElseIfs[i]
		eifBody := gnode.SBlock(s.Pos(), s.End(), convertBody(eif.Body)...)
		elseStmt = &gnode.IfStmt{Cond: eif.Cond, Body: eifBody, Else: elseStmt}
	}
	ifStmt := &gnode.IfStmt{Cond: s.Cond, Body: body, Else: elseStmt}
	if s.Init != nil {
		ifStmt.Init = s.Init
	}
	return gnode.Stmts{ifStmt}
}

func convertTag(t *TagStmt) gnode.Stmts {
	var stmts gnode.Stmts
	openCall := writeCallExpr("<" + t.Name)
	setParens(openCall, t.NodePos, t.NodeEnd)
	stmts.Append(gnode.SExpr(openCall))

	if len(t.Attributes) > 0 {
		stmts.Append(gnode.SExpr(buildAttrsCall(t.Attributes, t.NodePos, t.NodeEnd)))
	}
	if t.SelfClosing {
		closeCall := writeCallExpr(" />")
		setParens(closeCall, t.NodePos, t.NodeEnd)
		stmts.Append(gnode.SExpr(closeCall))
		return stmts
	}
	closeOpen := writeCallExpr(">")
	setParens(closeOpen, t.NodePos, t.NodeEnd)
	stmts.Append(gnode.SExpr(closeOpen))
	stmts.Append(convertBody(t.Body)...)
	closeTag := writeCallExpr("</" + t.Name + ">")
	setParens(closeTag, t.NodePos, t.NodeEnd)
	stmts.Append(gnode.SExpr(closeTag))
	return gnode.Stmts{
		gnode.SBlock(t.Pos(), t.End(), stmts...),
	}
}

func convertDoctype(d *DoctypeStmt) gnode.Stmts {
	call := gnode.ECall(gnode.EIdent("giom$write", 0), 0, 0)
	if !call.LParen.IsValid() {
		call.LParen = d.NodePos
	}
	if !call.RParen.IsValid() {
		call.RParen = d.NodeEnd
	}
	call.Args.Values = append(call.Args.Values, gnode.Str(doctypeValue(d.Value), 0))
	return gnode.Stmts{gnode.SExpr(call)}
}

func convertText(t *TextStmt) gnode.Stmts {
	var out gnode.Stmts
	for _, stmt := range t.Stmts {
		switch s := stmt.(type) {
		case *gnode.MixedTextStmt:
			call := gnode.ECall(gnode.EIdent("giom$write", 0), 0, 0)
			if !call.LParen.IsValid() {
				call.LParen = t.NodePos
			}
			if !call.RParen.IsValid() {
				call.RParen = t.NodeEnd
			}
			call.Args.Values = append(call.Args.Values, gnode.Str(s.Value(), s.Pos()))
			out.Append(gnode.SExpr(call))
		case *gnode.MixedValueStmt:
			call := gnode.ECall(gnode.EIdent("giom$write", 0), 0, 0)
			if !call.LParen.IsValid() {
				call.LParen = t.NodePos
			}
			if !call.RParen.IsValid() {
				call.RParen = t.NodeEnd
			}
			call.Args.Values = append(call.Args.Values, s.Expr)
			out.Append(gnode.SExpr(call))
		case gnode.Stmt:
			out.Append(s)
		}
	}
	return out
}
