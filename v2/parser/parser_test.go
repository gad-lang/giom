package parser_test

import (
	"bytes"
	"os"
	"testing"

	gadparser "github.com/gad-lang/gad/parser"
	gnode "github.com/gad-lang/gad/parser/node"
	"github.com/gad-lang/gad/parser/source"
	giomnode "github.com/gad-lang/giom/v2/node"
	giomparser "github.com/gad-lang/giom/v2/parser"
	"github.com/stretchr/testify/require"
)

func parseString(t *testing.T, src string) *giomnode.File {
	t.Helper()
	fs := source.NewFileSet()
	f := fs.AddFileData("test.giom", -1, []byte(src))
	p := giomparser.NewParser(f)
	file, err := p.ParseFile()
	require.NoError(t, err)
	require.NotNil(t, file)
	return file
}

func parseFile(t *testing.T, path string) *giomnode.File {
	t.Helper()
	src, err := os.ReadFile(path)
	require.NoError(t, err)
	fs := source.NewFileSet()
	f := fs.AddFileData(path, -1, src)
	p := giomparser.NewParser(f)
	file, err := p.ParseFile()
	require.NoError(t, err)
	return file
}

func writeGiom(t *testing.T, f *giomnode.File) string {
	t.Helper()
	var out bytes.Buffer
	ctx := giomnode.NewGiomCodeContext(&out)
	for i, stmt := range f.Stmts {
		gc, ok := stmt.(giomnode.GiomCoder)
		require.True(t, ok, "stmt %T does not implement GiomCoder", stmt)
		if i > 0 {
			out.WriteString("\n\n")
		}
		gc.WriteGiom(ctx)
	}
	return out.String()
}

func writeGad(t *testing.T, f *giomnode.File) string {
	t.Helper()
	var out bytes.Buffer
	gnode.CodeW(&out, f, gnode.CodeWithPrefix("\t"), gnode.CodeFormat())
	return out.String()
}

func writeGadPlain(t *testing.T, f *giomnode.File) string {
	t.Helper()
	var out bytes.Buffer
	gnode.CodeW(&out, f)
	return out.String()
}

func TestParser_TextExprTrim(t *testing.T) {
	f := parseString(t, "@if ok\n\t#{=safe(Page.Body)}\n")
	require.Len(t, f.Stmts, 1)
	ifStmt, ok := f.Stmts[0].(*giomnode.IfStmt)
	require.True(t, ok)
	require.Len(t, ifStmt.Body, 1)

	textStmt, ok := ifStmt.Body[0].(*giomnode.TextStmt)
	require.True(t, ok)
	require.Len(t, textStmt.Stmts, 1)

	mv, ok := textStmt.Stmts[0].(*gnode.MixedValueStmt)
	require.True(t, ok)
	require.Equal(t, "safe(Page.Body)", mv.Expr.String())
}

func TestParser_TextExprTrimIndented(t *testing.T) {
	f := parseString(t, "@if ok\n\t\t#{=safe(Page.Body)}\n")
	require.Len(t, f.Stmts, 1)
	ifStmt, ok := f.Stmts[0].(*giomnode.IfStmt)
	require.True(t, ok)
	require.Len(t, ifStmt.Body, 1)

	textStmt, ok := ifStmt.Body[0].(*giomnode.TextStmt)
	require.True(t, ok)
	require.Len(t, textStmt.Stmts, 1)

	mv, ok := textStmt.Stmts[0].(*gnode.MixedValueStmt)
	require.True(t, ok)
	require.Equal(t, "safe(Page.Body)", mv.Expr.String())
}

func TestParser_GadFromBlankFixture(t *testing.T) {
	f := parseFile(t, "../samples/layouts/blank.giom")
	require.Len(t, f.Stmts, 2)
	comp, ok := f.Stmts[1].(*giomnode.CompDecl)
	require.True(t, ok)
	require.Len(t, comp.Body, 1)
	ifStmt, ok := comp.Body[0].(*giomnode.IfStmt)
	require.True(t, ok)
	require.Len(t, ifStmt.Body, 1)
	textStmt, ok := ifStmt.Body[0].(*giomnode.TextStmt)
	require.True(t, ok)
	require.Len(t, textStmt.Stmts, 1)
	mv, ok := textStmt.Stmts[0].(*gnode.MixedValueStmt)
	require.True(t, ok)
	require.Equal(t, "safe(Page.Body)", mv.Expr.String())

	plainText := writeGadPlain(t, &giomnode.File{Stmts: gnode.Stmts{textStmt}})
	require.Contains(t, plainText, "safe(Page.Body)")
	plainIf := writeGadPlain(t, &giomnode.File{Stmts: gnode.Stmts{ifStmt}})
	require.Contains(t, plainIf, "safe(Page.Body)")
	plainComp := writeGadPlain(t, &giomnode.File{Stmts: gnode.Stmts{comp}})
	require.Contains(t, plainComp, "safe(Page.Body)")
	plain := writeGadPlain(t, f)
	require.Contains(t, plain, "safe(Page.Body)")
	formatted := writeGad(t, f)
	require.Contains(t, formatted, "safe(Page.Body)")

	got, err := giomparser.Format(f.Stmts)
	require.NoError(t, err)
	require.Contains(t, string(got), "safe(Page.Body)")
}

func TestParser_GadOutputParses(t *testing.T) {
	f := parseString(t, "@switch x\n\t@case 1\n\t\t| one\n\t@default\n\t\t| other\n")
	code := writeGad(t, f)
	fs := source.NewFileSet()
	sf := fs.AddFileData("out.gad", -1, []byte(code))
	p := gadparser.NewParser(sf, nil)
	_, err := p.ParseFile()
	require.NoError(t, err)
	require.Contains(t, code, "match (")
}

func TestParseCallArgsString_GiomSyntax(t *testing.T) {
	args, err := giomparser.ParseCallArgsStringForTest(`enabled=!config.post_date_disabled, append=(() => post_tags(p.Tags;inline=true))`)
	require.NoError(t, err)
	require.Len(t, args.NamedArgs.Names, 2)
	require.Equal(t, "enabled", args.NamedArgs.Names[0].Name())
	require.Equal(t, "(!config.post_date_disabled)", args.NamedArgs.Values[0].String())
	require.Equal(t, "append", args.NamedArgs.Names[1].Name())
	require.Equal(t, "(() => post_tags(p.Tags; inline=true))", args.NamedArgs.Values[1].String())
}

func TestParseFuncParamsString_GiomDefaultsSemicolon(t *testing.T) {
	params, err := giomparser.ParseFuncParamsStringForTest(`(title, adminBar={}, config={})`)
	require.NoError(t, err)
	require.NotNil(t, params)
	require.Equal(t, `(title; adminBar={}, config={})`, params.String())
}

func TestParseSlotPassHeader_GiomDefaultsSemicolon(t *testing.T) {
	f := parseString(t, "@comp x\n\t@slot #main(title, key=nil)\n\t\t| body\n")
	require.Len(t, f.Stmts, 1)
	comp, ok := f.Stmts[0].(*giomnode.CompDecl)
	require.True(t, ok)
	require.Len(t, comp.Body, 1)
	slotPass, ok := comp.Body[0].(*giomnode.SlotPassStmt)
	require.True(t, ok)
	require.NotNil(t, slotPass.FuncType)
	require.Equal(t, `(title; key=nil)`, slotPass.FuncType.Params.String())
}

func TestParser_GadFromPostList(t *testing.T) {
	f := parseFile(t, "../samples/layouts/post_list.giom")
	got, err := giomparser.Format(f.Stmts)
	require.NoError(t, err)
	data, err := os.ReadFile("../samples/layouts/post_list.gad")
	require.NoError(t, err)
	f2, err := os.Create("../samples/layouts/post_list.new.gad")
	require.NoError(t, err)
	f2.Write(got)
	require.Equal(t, string(data), string(got))
}

func TestParser_Positions(t *testing.T) {
	f := parseFile(t, "../samples/layouts/post_list.giom")
	inputFile := f.InputFile

	// 6 top-level stmts: CodeStmt (global+const), @func post_date,
	// @func post_tags, @comp post_item, @export comp post_items, @main
	require.Len(t, f.Stmts, 6)

	// --- @func post_date (line 45) ---
	fd, ok := f.Stmts[1].(*giomnode.FuncDecl)
	require.True(t, ok)
	require.Equal(t, "post_date", fd.Name)
	fp, err := inputFile.DataPosition(fd.Pos())
	require.NoError(t, err)
	require.Equal(t, 45, fp.Line, "post_date Pos line")

	// @if enabled inside post_date (line 46)
	require.Len(t, fd.Body, 1)
	ifStmt, ok := fd.Body[0].(*giomnode.IfStmt)
	require.True(t, ok)
	ifp, err := inputFile.DataPosition(ifStmt.Pos())
	require.NoError(t, err)
	require.Equal(t, 46, ifp.Line, "post_date @if Pos line")
	require.Equal(t, "enabled", ifStmt.Cond.String())

	// --- @func post_tags (line 54) ---
	ft, ok := f.Stmts[2].(*giomnode.FuncDecl)
	require.True(t, ok)
	require.Equal(t, "post_tags", ft.Name)
	ftp, err := inputFile.DataPosition(ft.Pos())
	require.NoError(t, err)
	require.Equal(t, 54, ftp.Line, "post_tags Pos line")

	// --- @comp post_item (line 67) ---
	comp, ok := f.Stmts[3].(*giomnode.CompDecl)
	require.True(t, ok)
	require.Equal(t, "post_item", comp.ID)
	compP, err := inputFile.DataPosition(comp.Pos())
	require.NoError(t, err)
	require.Equal(t, 67, compP.Line, "post_item Pos line")

	// +post_date component call inside post_item date func (line 82)
	require.Len(t, comp.Body, 4)
	dateFunc, ok := comp.Body[2].(*giomnode.FuncDecl)
	require.True(t, ok)
	require.Equal(t, "date", dateFunc.Name)
	require.Len(t, dateFunc.Body, 1)
	call, ok := dateFunc.Body[0].(*giomnode.CompCallStmt)
	require.True(t, ok)
	callP, err := inputFile.DataPosition(call.Pos())
	require.NoError(t, err)
	require.Equal(t, 82, callP.Line, "+post_date Pos line")
	require.Equal(t, "post_date", call.Name)
	require.Len(t, call.Args.NamedArgs.Names, 2)
	require.Equal(t, "enabled", call.Args.NamedArgs.Names[0].Name())

	// --- @export comp post_items (line 129) ---
	exportComp, ok := f.Stmts[4].(*giomnode.CompDecl)
	require.True(t, ok)
	require.True(t, exportComp.Exported)
	require.Equal(t, "post_items", exportComp.ID)
	ep, err := inputFile.DataPosition(exportComp.Pos())
	require.NoError(t, err)
	require.Equal(t, 129, ep.Line, "post_items Pos line")

	// --- @main (line 211) ---
	mainComp, ok := f.Stmts[5].(*giomnode.CompDecl)
	require.True(t, ok)
	require.Equal(t, "main", mainComp.ID)
	require.True(t, mainComp.Exported)
	mp, err := inputFile.DataPosition(mainComp.Pos())
	require.NoError(t, err)
	require.Equal(t, 211, mp.Line, "main Pos line")

	// @if post_id inside @main (line 216)
	require.GreaterOrEqual(t, len(mainComp.Body), 2)
	ifPost, ok := mainComp.Body[1].(*giomnode.IfStmt)
	require.True(t, ok)
	ifp2, err := inputFile.DataPosition(ifPost.Pos())
	require.NoError(t, err)
	require.Equal(t, 216, ifp2.Line, "@if post_id Pos line")
}

func TestParser_Positions_Basic(t *testing.T) {
	f := parseString(t, "div\n")
	require.Len(t, f.Stmts, 1)
	stmt := f.Stmts[0]
	require.True(t, stmt.Pos().IsValid())
	require.True(t, stmt.End().IsValid())
	require.GreaterOrEqual(t, int(stmt.End()), int(stmt.Pos()))
}
