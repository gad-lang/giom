package giom

import (
	"bytes"
	"io"
	"sort"

	"github.com/gad-lang/giom/parser"
)

// Compiler is the main interface of giom Template Engine.
// In order to use an giom template, it is required to create a Compiler and
// compile an giom source to native Go template.
//
//	compiler := giom.New()
//	// Parse the input file
//	err := compiler.ParseFile("./input.giom")
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
	root          *parser.Root
	indentLevel   int
	newline       bool
	writer        *writer
	tempvarIndex  int
	exportedComps map[string]*parser.Comp
	visited       map[uintptr]any
	exports       map[string]string
}

// New creates and initialize a new Compiler.
func New(root *parser.Root) *Compiler {
	compiler := new(Compiler)
	compiler.tempvarIndex = 0
	compiler.PrettyPrint = true
	compiler.Options = DefaultOptions
	compiler.exportedComps = make(map[string]*parser.Comp)
	compiler.exports = make(map[string]string)
	compiler.root = root

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
	// In this form, giom emits line number comments in the output template. It is usable in debugging environments.
	// Default: false
	LineNumbers bool
	PreCode     string
	FileName    string
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

// DefaultDirOptions sets expected file extension to ".giom" and recursive search for templates within a directory to true.
var DefaultDirOptions = DirOptions{".giom", true}

// Compile parses and compiles the supplied giom template string. Write gad gode to out writer.
func Compile(out io.Writer, input []byte, options Options) (err error) {
	var (
		p    = parser.New(bytes.NewBuffer(input))
		root *parser.Root
	)

	if root, err = p.Parse(); err != nil {
		return
	}

	comp := New(root)
	comp.Options = options

	err = comp.Compile(out)
	return
}

// CompileToGad parses and compiles the supplied giom template string. Write gad gode to out writer.
func CompileToGad(out io.Writer, input []byte, options Options) (err error) {
	var (
		p    = parser.New(bytes.NewBuffer(input))
		root *parser.Root
	)

	if root, err = p.Parse(); err != nil {
		return
	}

	comp := New(root)
	comp.Options = options

	err = NewToGadCompiler(comp).
		PreCode(options.PreCode).
		Format(FormatTranspile).
		Compile(out)
	return
}

// Compile compiles giom and writes the Go Template source into given io.Writer instance.
// You would not be using this unless debugging / checking the output. Please use Compile
// method to obtain a template instance directly.
func (c *Compiler) Compile(out io.Writer) (err error) {
	c.writer = &writer{Writer: out}
	c.visited = map[uintptr]any{}
	c.visit(c.root)

	if c.writer.len > 0 {
		c.write("\n")
	}

	c.write("{% return {")
	var names [][2]string
	for name, comp := range c.exportedComps {
		names = append(names, [2]string{name, comp.Name})
	}

	for name, value := range c.exports {
		names = append(names, [2]string{name, value})
	}

	sort.Slice(names, func(i, j int) bool { return names[i][0] < names[j][0] })

	for i, name := range names {
		if i > 0 {
			c.write(",")
		}
		c.write(name[0] + ": " + name[1])
	}

	c.write("} %}")
	return
}

// CompileString compiles the template and returns the Go Template source.
// You would not be using this unless debugging / checking the output. Please use Compile
// method to obtain a template instance directly.
func (c *Compiler) CompileString() (string, error) {
	var buf bytes.Buffer

	if err := c.Compile(&buf); err != nil {
		return "", err
	}

	result := buf.String()

	return result, nil
}
