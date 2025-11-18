package giom

import (
	"fmt"
	"io"

	"github.com/gad-lang/gad"
	"github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/parser/source"
)

func HumanizeError(out io.Writer, err error) {
	switch t := err.(type) {
	case *gad.RuntimeError:
		fmt.Fprintf(out, "%+v\n", t)
		if st := t.StackTrace(); len(st) > 0 {
			pos := t.FileSet().Position(source.Pos(st[len(st)-1].Offset))
			pos.File.Data.TraceLines(out, pos.Line, pos.Column, 20, 20)
		}
	case *parser.ErrorList, *gad.CompilerError:
		fmt.Fprintf(out, "%+20.20v\n", t)
	default:
		fmt.Fprintf(out, "ERROR: %v\n", err)
	}
}
