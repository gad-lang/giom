package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gad-lang/gad"
	gp "github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/parser/source"
	"github.com/gad-lang/giom"
	"github.com/gad-lang/giom/parser"
)

var (
	prettyPrint       bool
	lineNumbers       bool
	transpileDisabled bool
	formatDisabled    bool
	outToSelf         bool
	outFile           = ""
)

func init() {
	flag.BoolVar(&prettyPrint, "prettyprint", true, "Use pretty indentation in output.")
	flag.BoolVar(&lineNumbers, "linenos", true, "Enable debugging information in output.")
	flag.BoolVar(&transpileDisabled, "no-transpile", false, "Disable to Gad code transpile.")
	flag.BoolVar(&formatDisabled, "no-format", false, "Disable to Gad code format.")
	flag.BoolVar(&outToSelf, "out-self", false, "Outputs to FILE_NAME.gad")
	flag.StringVar(&outFile, "out", outFile, "Output file.")

	flag.Parse()
}

func main() {
	var (
		err      error
		input    io.ReadCloser
		fileName string
	)

	fileName = flag.Arg(0)

	if len(fileName) == 0 {
		checkErr(errors.New("file name is required"))
	}

	if fileName == "-" {
		input = os.Stdin
		fileName = "(stdin)"
	} else {
		input, err = os.Open(fileName)
		checkErr(err)
	}

	var root *parser.Root

	{
		defer input.Close()
		p := parser.New(input)
		p.SetFilename(fileName)

		root, err = p.Parse()
		checkErr(err)
	}

	Giom := giom.New(root)
	Giom.FileName = fileName
	Giom.PrettyPrint = prettyPrint
	Giom.LineNumbers = lineNumbers

	var out io.WriteCloser
	if outToSelf {
		dir, name := filepath.Split(fileName)
		pos := strings.LastIndex(name, ".")
		if pos != -1 {
			name = name[:pos]
		}
		name = filepath.Join(dir, name+".gad")
		out, err = os.Create(name)
		checkErr(err)
	} else if outFile == "" {
		out = os.Stdout
	} else {
		out, err = os.Create(outFile)
		checkErr(err)
	}

	defer out.Close()

	if formatDisabled {
		err = Giom.Compile(out)
	} else {
		var (
			gw = giom.NewToGadCompiler(Giom)
			ff = giom.Format
		)

		if !transpileDisabled {
			ff = giom.FormatTranspile
		}

		err = gw.Format(ff).Compile(out)
	}

	checkErr(err)
}

func checkErr(err error) {
	if err != nil {
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
}
