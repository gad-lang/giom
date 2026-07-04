package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gad-lang/gad"
	gp "github.com/gad-lang/gad/parser"
	gnode "github.com/gad-lang/gad/parser/node"
	"github.com/gad-lang/gad/parser/source"
	cmdctx "github.com/moisespsena-go/command-context"

	"github.com/gad-lang/giom"
	giomnode "github.com/gad-lang/giom/v2/node"
	giomparser "github.com/gad-lang/giom/v2/parser"
)

func main() {
	root := &cmdctx.Command{
		Name:        "giom",
		Description: "Giom template engine — render, compile, and format giom templates.",
		Run: func(ctx *cmdctx.CommandContext) error {
			return ctx.Help()
		},
	}

	root.Sub(compileCmd())
	root.Sub(renderCmd())
	root.Sub(fmtCmd())

	if _, err := root.Parse(&cmdctx.CommandContext{
		Context: context.Background(),
		Out:     os.Stdout,
		Err:     os.Stderr,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// =============================================================================
// compile — parse giom and output GAD source code
// =============================================================================

func compileCmd() *cmdctx.Command {
	var outFile string

	return &cmdctx.Command{
		Name:        "compile",
		Usage:       "[flags] <file>",
		Description: "Parse a giom template and output compiled GAD source code.",
		New: func(ctx *cmdctx.CommandContext) error {
			ctx.Flags().StringVar(&outFile, "o", "", "Output file (default: stdout)")
			ctx.Flags().StringVar(&outFile, "out", "", "Output file (default: stdout)")
			return nil
		},
		Run: func(ctx *cmdctx.CommandContext) error {
			if len(ctx.Args) == 0 {
				return errors.New("file name is required")
			}
			return withFile(ctx.Args[0], outFile, func(input io.Reader, out io.Writer, fname string) error {
				return compileTemplate(input, out, fname)
			})
		},
	}
}

func compileTemplate(input io.Reader, out io.Writer, fname string) error {
	src, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	return giom.CompileToGad(out, src, giom.Options{FileName: fname})
}

// =============================================================================
// fmt — parse giom and output formatted giom source
// =============================================================================

func fmtCmd() *cmdctx.Command {
	var outFile string
	var outToSelf, listFiles bool

	return &cmdctx.Command{
		Name:        "fmt",
		Usage:       "[flags] <file>",
		Description: "Parse a giom template and output formatted giom source.",
		New: func(ctx *cmdctx.CommandContext) error {
			ctx.Flags().StringVar(&outFile, "o", "", "Output file (default: stdout)")
			ctx.Flags().StringVar(&outFile, "out", "", "Output file (default: stdout)")
			ctx.Flags().BoolVar(&outToSelf, "w", false, "Write result to source file instead of stdout")
			ctx.Flags().BoolVar(&listFiles, "l", false, "List files whose formatting differs")
			return nil
		},
		Run: func(ctx *cmdctx.CommandContext) error {
			if len(ctx.Args) == 0 {
				return errors.New("file name is required")
			}

			fname := ctx.Args[0]
			input, err := openInput(fname)
			if err != nil {
				return err
			}
			defer input.Close()

			var buf strings.Builder
			if err := formatGiom(input, &buf, fname); err != nil {
				return err
			}

			result := buf.String()

			if listFiles {
				input2, err := openInput(fname)
				if err != nil {
					return err
				}
				defer input2.Close()
				orig, _ := io.ReadAll(input2)
				if strings.TrimSpace(string(orig)) != strings.TrimSpace(result) {
					fmt.Println(fname)
				}
				return nil
			}

			out := openOutput(outFile, outToSelf, fname)
			if out != os.Stdout {
				defer out.Close()
			}
			_, err = io.WriteString(out, result)
			return err
		},
	}
}

func formatGiom(input io.Reader, out io.Writer, fname string) error {
	src, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	fs := source.NewFileSet()
	f := fs.AddFile(fname, 0, len(src))
	f.Data = source.NewData(src)

	p := giomparser.NewParser(f)
	file, err := p.ParseFile()
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	gctx := giomnode.NewGiomCodeContext(out)
	for i, stmt := range file.Stmts {
		if gc, ok := stmt.(giomnode.GiomCoder); ok {
			if i > 0 {
				io.WriteString(out, "\n\n")
			}
			gc.WriteGiom(gctx)
		}
	}
	return nil
}

// =============================================================================
// render — parse giom, compile to GAD, execute via VM, output HTML
// =============================================================================

func renderCmd() *cmdctx.Command {
	var outFile string
	var prettyPrint bool
	var lineNumbers bool

	return &cmdctx.Command{
		Name:        "render",
		Usage:       "[flags] <file>",
		Description: "Parse and render a giom template to output.",
		New: func(ctx *cmdctx.CommandContext) error {
			ctx.Flags().StringVar(&outFile, "o", "", "Output file (default: stdout)")
			ctx.Flags().StringVar(&outFile, "out", "", "Output file (default: stdout)")
			ctx.Flags().BoolVar(&prettyPrint, "prettyprint", true, "Pretty-print output")
			ctx.Flags().BoolVar(&lineNumbers, "linenos", false, "Emit line number comments")
			return nil
		},
		Run: func(ctx *cmdctx.CommandContext) error {
			if len(ctx.Args) == 0 {
				return errors.New("file name is required")
			}
			return withFile(ctx.Args[0], outFile, func(input io.Reader, out io.Writer, fname string) error {
				return renderTemplate(input, out, fname, prettyPrint, lineNumbers)
			})
		},
	}
}

func renderTemplate(input io.Reader, out io.Writer, fname string, prettyPrint, lineNumbers bool) error {
	src, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	var gadSrc strings.Builder
	if err := giom.CompileToGad(&gadSrc, src, giom.Options{
		PrettyPrint: prettyPrint,
		LineNumbers: lineNumbers,
		FileName:    fname,
	}); err != nil {
		return fmt.Errorf("compile to GAD: %w", err)
	}

	t, err := giom.NewTemplateBuilder([]byte(gadSrc.String())).Build()
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}

	if _, err := t.Executor().Out(out).ExecuteModule(); err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	return nil
}

// =============================================================================
// Helpers
// =============================================================================

func withFile(fname, outFile string, fn func(io.Reader, io.Writer, string) error) error {
	input, err := openInput(fname)
	if err != nil {
		return err
	}
	defer input.Close()

	out := openOutput(outFile, false, "")
	if out != os.Stdout {
		defer out.Close()
	}

	return fn(input, out, fname)
}

func openInput(fname string) (io.ReadCloser, error) {
	if fname == "-" {
		return os.Stdin, nil
	}
	return os.Open(fname)
}

func openOutput(outFile string, outToSelf bool, fname string) io.WriteCloser {
	if outToSelf && fname != "" {
		f, err := os.Create(fname)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return f
	}
	if outFile != "" {
		f, err := os.Create(outFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return f
	}
	return os.Stdout
}

// checkErr prints an error to stderr, including stack traces for GAD errors.
func checkErr(err error) {
	if err == nil {
		return
	}
	switch t := err.(type) {
	case *gad.RuntimeError:
		fmt.Fprintf(os.Stderr, "%+v\n", t)
		if st := t.StackTrace(); len(st) > 0 {
			t.FileSet().Position(source.Pos(st[len(st)-1].Offset)).TraceLines(os.Stderr, 20, 20)
		}
	case *gp.ErrorList, *gad.CompilerError:
		fmt.Fprintf(os.Stderr, "%+20.20v\n", t)
	default:
		fmt.Fprintln(os.Stderr, err)
	}
}

var (
	_ io.Reader = (*strings.Reader)(nil)
	_ gnode.Stmt = (*giomnode.TextStmt)(nil)
	_ fmt.Stringer = (*giomnode.File)(nil)
)
