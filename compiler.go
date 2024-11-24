package gber

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gad-lang/gad"
	gp "github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/parser/node"
	"github.com/gad-lang/gad/parser/utils"
	gt "github.com/gad-lang/gad/token"
	"github.com/gad-lang/gber/parser"
)

type FileSystem = parser.FileSystem

// Compiler is the main interface of Gber Template Engine.
// In order to use an Gber template, it is required to create a Compiler and
// compile an Gber source to native Go template.
//
//	compiler := gber.New()
//	// Parse the input file
//	err := compiler.ParseFile("./input.gber")
//	if err == nil {
//		// Compile input file to Go template
//		tpl, err := compiler.Compile()
//		if err == nil {
//			// Check built in html/template documentation for further details
//			tpl.Execute(os.Stdout, somedata)
//		}
//	}
type Compiler struct {
	// Compiler options
	Options
	Module       *gad.ModuleInfo
	root         *parser.Root
	indentLevel  int
	newline      bool
	writer       *writer
	tempvarIndex int
	mixins       map[string]*parser.Mixin
	visited      map[uintptr]any
	exports      map[string]string
}

// New creates and initialize a new Compiler.
func New() *Compiler {
	compiler := new(Compiler)
	compiler.Module = &gad.ModuleInfo{}
	compiler.tempvarIndex = 0
	compiler.PrettyPrint = true
	compiler.Options = DefaultOptions
	compiler.mixins = make(map[string]*parser.Mixin)
	compiler.exports = make(map[string]string)

	return compiler
}

// Options defines template output behavior.
type Options struct {
	// Setting if pretty printing is enabled.
	// Pretty printing ensures that the output html is properly indented and in human readable form.
	// If disabled, produced HTML is compact. This might be more suitable in production environments.
	// Default: true
	PrettyPrint bool
	// Setting if line number emitting is enabled
	// In this form, Gber emits line number comments in the output template. It is usable in debugging environments.
	// Default: false
	LineNumbers bool
	// Setting the virtual filesystem to use
	// If set, will attempt to use a virtual filesystem provided instead of os.
	// Default: nil
	VirtualFilesystem FileSystem
	// Setting Builtin funcs names
	// If set, when identifier matches key, disable to DIT convertion.
	// Default: nil
	BuiltinNames map[string]any
	GlobalNames  []string
	Code         bool

	Builtins  *gad.Builtins
	ModuleMap *gad.ModuleMap
	PreCode   string
	Context   context.Context
}

// DirOptions is used to provide options to directory compilation.
type DirOptions struct {
	// File extension to match for compilation
	Ext string
	// Whether or not to walk subdirectories
	Recursive bool
}

// DefaultOptions sets pretty-printing to true and line numbering to false.
var DefaultOptions = Options{PrettyPrint: true}

// DefaultDirOptions sets expected file extension to ".gber" and recursive search for templates within a directory to true.
var DefaultDirOptions = DirOptions{".gber", true}

// Compile parses and compiles the supplied gber template string. Returns corresponding Go Template (html/templates) instance.
// Necessary runtime functions will be injected and the template will be ready to be executed.
func Compile(input string, options Options) (*Template, error) {
	comp := New()
	comp.Options = options

	err := comp.Parse(input)
	if err != nil {
		return nil, err
	}

	return comp.Compile()
}

// CompileToString parses and compiles the supplied gber template string.
// Necessary runtime functions will be injected and the template will be ready to be executed.
func CompileToString(input string, options Options) (compiled string, err error) {
	comp := New()
	comp.Options = options

	if err = comp.Parse(input); err != nil {
		return
	}

	return comp.CompileString()
}

// CompileToWriter parses and compiles the supplied gber template.
// Necessary runtime functions will be injected and the template will be ready to be executed.
func CompileToWriter(dst io.Writer, input string, options Options) (err error) {
	comp := New()
	comp.Options = options

	if err = comp.Parse(input); err != nil {
		return
	}

	return comp.CompileWriter(dst)
}

// Compile parses and compiles the supplied gber template []byte.
// Returns corresponding Go Template (html/templates) instance.
// Necessary runtime functions will be injected and the template will be ready to be executed.
func CompileData(input []byte, filename string, options Options) (*Template, error) {
	comp := New()
	comp.Options = options

	err := comp.ParseData(input, filename)
	if err != nil {
		return nil, err
	}

	return comp.Compile()
}

// MustCompile is the same as Compile, except the input is assumed error free. If else, panic.
func MustCompile(input string, options Options) *Template {
	t, err := Compile(input, options)
	if err != nil {
		panic(err)
	}
	return t
}

// CompileFile parses and compiles the contents of supplied filename. Returns corresponding Go Template (html/templates) instance.
// Necessary runtime functions will be injected and the template will be ready to be executed.
func CompileFile(filename string, options Options) (*Template, error) {
	comp := New()
	comp.Options = options

	err := comp.ParseFile(filename)
	if err != nil {
		return nil, err
	}

	return comp.Compile()
}

// MustCompileFile is the same as CompileFile, except the input is assumed error free. If else, panic.
func MustCompileFile(filename string, options Options) *Template {
	t, err := CompileFile(filename, options)
	if err != nil {
		panic(err)
	}
	return t
}

// CompileDir parses and compiles the contents of a supplied directory path, with options.
// Returns a map of a template identifier (key) to a Go Template instance.
// Ex: if the dirname="templates/" had a file "index.gber" the key would be "index"
// If option for recursive is True, this parses every file of relevant extension
// in all subdirectories. The key then is the path e.g: "layouts/layout"
func CompileDir(dirname string, dopt DirOptions, opt Options) (map[string]*Template, error) {
	dir, err := os.Open(dirname)
	if err != nil && opt.VirtualFilesystem != nil {
		vdir, err := opt.VirtualFilesystem.Open(dirname)
		if err != nil {
			return nil, err
		}
		dir = vdir.(*os.File)
	} else if err != nil {
		return nil, err
	}
	defer dir.Close()

	files, err := dir.Readdir(0)
	if err != nil {
		return nil, err
	}

	compiled := make(map[string]*Template)
	for _, file := range files {
		// filename is for example "index.gber"
		filename := file.Name()
		fileext := filepath.Ext(filename)

		// If recursive is true and there's a subdirectory, recurse
		if dopt.Recursive && file.IsDir() {
			dirpath := filepath.Join(dirname, filename)
			subcompiled, err := CompileDir(dirpath, dopt, opt)
			if err != nil {
				return nil, err
			}
			// Copy templates from subdirectory into parent template mapping
			for k, v := range subcompiled {
				// Concat with parent directory name for unique paths
				key := filepath.Join(filename, k)
				compiled[key] = v
			}
		} else if fileext == dopt.Ext {
			// Otherwise compile the file and add to mapping
			fullpath := filepath.Join(dirname, filename)
			tmpl, err := CompileFile(fullpath, opt)
			if err != nil {
				return nil, err
			}
			// Strip extension
			key := filename[0 : len(filename)-len(fileext)]
			compiled[key] = tmpl
		}
	}

	return compiled, nil
}

// MustCompileDir is the same as CompileDir, except input is assumed error free. If else, panic.
func MustCompileDir(dirname string, dopt DirOptions, opt Options) map[string]*Template {
	m, err := CompileDir(dirname, dopt, opt)
	if err != nil {
		panic(err)
	}
	return m
}

// Parse given raw gber template string.
func (c *Compiler) Parse(input string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(r.(string))
		}
	}()

	parser, err := parser.StringParser(input)

	if err != nil {
		return
	}

	c.root = parser.Parse()
	return
}

// Parse given raw gber template bytes, and the filename that belongs with it
func (c *Compiler) ParseData(input []byte, filename string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(r.(string))
		}
	}()

	parser, err := parser.ByteParser(input)
	parser.SetFilename(filename)
	if c.VirtualFilesystem != nil {
		parser.SetVirtualFilesystem(c.VirtualFilesystem)
	}

	if err != nil {
		return
	}

	c.root = parser.Parse()
	return
}

// ParseFile parses the gber template file in given path.
func (c *Compiler) ParseFile(filename string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(r.(string))
		}
	}()

	p, err := parser.FileParser(filename)
	if err != nil && c.VirtualFilesystem != nil {
		p, err = parser.VirtualFileParser(filename, c.VirtualFilesystem)
	}
	if err != nil {
		return
	}

	c.root = p.Parse()
	c.Module.Name = filename
	return
}

func (c *Compiler) SetFileName(name string) *Compiler {
	c.Module.Name = name
	return c
}

// Compile gber and create a Go Template (html/templates) instance.
// Necessary runtime functions will be injected and the template will be ready to be executed.
func (c *Compiler) CompileToGad() (data strings.Builder, err error) {
	if len(c.GlobalNames) > 0 {
		data.WriteString(`global (` + strings.Join(c.GlobalNames, ",") + `)` + "\n")
	}

	if c.PreCode != "" {
		data.WriteString(c.PreCode + "\n")
	}
	data.WriteString("# gad: mixed\n")

	if err = c.CompileWriter(&data); err != nil {
		return
	}

	return
}

// Compile gber and create a Go Template (html/templates) instance.
// Necessary runtime functions will be injected and the template will be ready to be executed.
func (c *Compiler) Compile() (t *Template, err error) {
	var data strings.Builder
	if data, err = c.CompileToGad(); err != nil {
		if c.Code {
			t = &Template{Code: data.String()}
		}
		return
	}

	if c.Builtins == nil {
		c.Builtins = AppendBuiltins(gad.NewBuiltins())
	} else {
		c.Builtins = AppendBuiltins(c.Builtins)
	}

	if c.ModuleMap == nil {
		c.ModuleMap = gad.NewModuleMap()
	}

	var bc *gad.Bytecode
	if bc, err = gad.Compile([]byte(data.String()), gad.CompileOptions{
		CompilerOptions: gad.CompilerOptions{
			Module:      c.Module,
			ModuleMap:   c.ModuleMap,
			SymbolTable: gad.NewSymbolTable(c.Builtins),
			Context:     c.Options.Context,
		},
		ScannerOptions: gp.ScannerOptions{
			MixedExprRune: '$',
		},
	}); err == nil || c.Code {
		t = &Template{
			BC:       bc,
			Builtins: c.Builtins,
		}
		if c.Code {
			t.Code = data.String()
		}
	}

	return
}

// CompileWriter compiles gber and writes the Go Template source into given io.Writer instance.
// You would not be using this unless debugging / checking the output. Please use Compile
// method to obtain a template instance directly.
func (c *Compiler) CompileWriter(out io.Writer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(r.(string))
		}
	}()

	c.writer = &writer{Writer: out}
	c.visited = map[uintptr]any{}
	c.visit(c.root)

	if c.writer.len > 0 {
		c.write("\n")
	}

	c.write(`${if __is_module__ {` + "\n")
	c.write("\treturn {\n")
	var names []string
	for name := range c.mixins {
		if name[0] >= 'A' && name[0] <= 'Z' {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		c.write("\t\t" + name + ": $$mixins." + name + ",\n")
	}

	names = nil

	for name := range c.exports {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		c.write("\t\t" + name + ": " + c.exports[name] + ",\n")
	}

	c.write("\t}\n")
	c.write("}}")

	return
}

// CompileString compiles the template and returns the Go Template source.
// You would not be using this unless debugging / checking the output. Please use Compile
// method to obtain a template instance directly.
func (c *Compiler) CompileString() (string, error) {
	var buf bytes.Buffer

	if err := c.CompileWriter(&buf); err != nil {
		return "", err
	}

	result := buf.String()

	return result, nil
}

func (c *Compiler) visit(node parser.Node) {
	defer func() {
		if r := recover(); r != nil {
			if rs, ok := r.(string); ok && rs[:len("Gber Error")] == "Gber Error" {
				panic(r)
			}

			pos := node.Pos()

			if len(pos.Filename) > 0 {
				panic(fmt.Sprintf("Gber Error in <%s>: %v - Line: %d, Column: %d, Length: %d", pos.Filename, r, pos.LineNum, pos.ColNum, pos.TokenLength))
			} else {
				panic(fmt.Sprintf("Gber Error: %v - Line: %d, Column: %d, Length: %d", r, pos.LineNum, pos.ColNum, pos.TokenLength))
			}
		}
	}()

	switch node.(type) {
	case *parser.Root:
		c.visitRoot(node.(*parser.Root))
	case *parser.Block:
		c.visitBlock(node.(*parser.Block))
	case *parser.Doctype:
		c.visitDoctype(node.(*parser.Doctype))
	case *parser.Comment:
		c.visitComment(node.(*parser.Comment))
	case *parser.Tag:
		c.visitTag(node.(*parser.Tag))
	case *parser.Text:
		c.visitText(node.(*parser.Text))
	case *parser.If:
		c.visitCondition(node.(*parser.If))
	case *parser.For:
		c.visitFor(node.(*parser.For))
	case *parser.Assignment:
		c.visitAssignment(node.(*parser.Assignment))
	case *parser.Mixin:
		c.visitMixin(node.(*parser.Mixin))
	case *parser.MixinCall:
		c.visitMixinCall(node.(*parser.MixinCall))
	case *parser.Code:
		c.visitCode(node.(*parser.Code))
	case *parser.Switch:
		c.visitSwitch(node.(*parser.Switch))
	case *parser.Export:
		c.visitExport(node.(*parser.Export))
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
	var exprs []string
	for _, init := range root.Inits {
		exprs = append(exprs, init.Exprs...)
	}
	c.visitCode(&parser.Code{Expressions: exprs, TrimRigth: true, TrimLeft: true})

	if len(root.Mixins) > 0 {
		sort.Slice(root.Mixins, func(i, j int) bool {
			return root.Mixins[i].Name < root.Mixins[j].Name
		})

		for _, mixin := range root.Mixins {
			c.visitMixin(mixin)
		}
	}

	c.visitBlock(&root.Block)
}

func (c *Compiler) visitBlock(block *parser.Block) {
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
		c.write(`${unescaped("<!-- ` + c.escape(comment.Value) + ` -->")}`)
	} else {
		c.write(`<!-- ` + comment.Value)
		c.visitBlock(comment.Block)
		c.write(` -->`)
	}
}

func (c *Compiler) visitCondition(condition *parser.If) {
	positives := condition.Positives
	positive := positives[0]
	c.write(`${if ` + positive.Expression + ` then}`)
	if positive.Block != nil {
		c.visitBlock(positive.Block)
	}
	for _, positive = range positives[1:] {
		c.write(`${else if ` + positive.Expression + ` then}`)
		if positive.Block != nil {
			c.visitBlock(positive.Block)
		}
	}
	if condition.Negative != nil {
		c.write(`${else then}`)
		c.visitBlock(condition.Negative)
	}
	c.write(`${end}`)
}

func (c *Compiler) visitFor(for_ *parser.For) {
	if for_.Block == nil {
		return
	}
	c.write(`${` + strings.TrimSpace(for_.Expression) + ` do}`)
	c.visitBlock(for_.Block)
	if for_.Else != nil {
		c.write(`${else}`)
		c.visitBlock(for_.Else)
	}
	c.write(`${end}`)
}

func (c *Compiler) visitAssignment(assgn *parser.Assignment) {
	op := assgn.Op
	c.write(`${` + assgn.X + op + `=` + assgn.Expression + `;}`)
}

func (c *Compiler) visitCode(cd *parser.Code) {
	var (
		l     = len(cd.Expressions)
		exprs = cd.Expressions
	)

	if l > 0 {
		c.write(`${`)
		c.write(`- `)
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
		c.write(right + `}`)
	}
}

func gadParser(value string) *gp.Parser {
	fs := gp.NewFileSet()
	sf := fs.AddFile("-", -1, len(value))
	return gp.NewParser(sf, []byte(value), nil)
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
					qcond := utils.Quote(item.Condition, '\'')
					qcond = strconv.Quote(item.Condition)
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
			kv.Value = &node.ArrayLit{Elements: a.Values}
		}
		attribs = append(attribs, kv)
	}

	c.indent(0, true)

	c.write("<" + tag.Name)

	if len(attribs) > 0 {
		c.write("${=attrs")
		c.write((&node.KeyValueArrayLit{Elements: attribs}).String())
		c.write("}")
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

var textInterpolateRegexp = regexp.MustCompile(`${(.*?)}`)
var textEscapeRegexp = regexp.MustCompile(`${(.*?)}`)

func (c *Compiler) visitText(txt *parser.Text) {
	for _, stmt := range txt.Stmts {
		switch t := stmt.(type) {
		case *node.RawStringStmt:
			lines := strings.Split(t.Value(), "\n")
			for i := 0; i < len(lines); i++ {
				c.write(lines[i])

				if i < len(lines)-1 {
					c.write("\n")
					c.indent(0, false)
				}
			}
		case *node.ExprToTextStmt:
			c.write(t.StartLit.Value)
			c.write(t.Expr.String())
			c.write(t.EndLit.Value)
		}
	}
}

func (c *Compiler) visitMixin(mixin *parser.Mixin) {
	if !mixin.Override && c.mixins[mixin.Name] != nil {
		panic(fmt.Sprintf(
			"mixin duplicate %q",
			mixin.Name,
		))
	}

	if len(c.mixins) == 0 {
		c.write(`${const $$mixins = {}}`)
	}

	c.mixins[mixin.Name] = mixin
	c.write(`${$$mixins.` + mixin.ID + ` = func(` + mixin.Args + `) do}`)
	c.visitBlock(mixin.Block)
	c.write(`${end}`)
}

func (c *Compiler) visitMixinCall(mixinCall *parser.MixinCall) {
	var (
		name      = strings.ReplaceAll(mixinCall.Name, "-", "__")
		blockName string
		args      = mixinCall.Args
	)

	if mixinCall.Block != nil {
		blockName = c.tempVar()
		if args != "" {
			args += ","
		}
		args += "$body=" + blockName
		c.write(`${` + blockName + ` := func() do}`)
		c.visitBlock(mixinCall.Block)
		c.write(`${end}`)
	}

	if pos := strings.LastIndex(name, "."); pos > 0 {
		c.write(`${` + name + `(` + args + `)}`)
	} else {
		c.write(`${$$mixins.` + name + `(` + args + `)}`)
	}
}

func (c *Compiler) visitSwitch(sw *parser.Switch) {
	if len(sw.Cases) == 0 {
		if sw.Default != nil && sw.Default.Content != nil {
			c.visitBlock(sw.Default.Content)
		}
		return
	}

	cases := sw.Cases
	fmt.Fprintf(c.writer, "${if %s == %s then}", sw.Expr, cases[0].Expr)
	if cases[0].Content != nil {
		c.visitBlock(cases[0].Content)
	}
	cases = cases[1:]
	for len(cases) > 0 {
		fmt.Fprintf(c.writer, "${else if %s == %s then}", sw.Expr, cases[0].Expr)
		if cases[0].Content != nil {
			c.visitBlock(cases[0].Content)
		}
		cases = cases[1:]
	}
	if sw.Default != nil {
		c.write("${else then}")
		if sw.Default.Content != nil {
			c.visitBlock(sw.Default.Content)
		}
	}
	c.write("${end}")
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
