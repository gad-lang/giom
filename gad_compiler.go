package giom

import (
	"bytes"
	"io"
	"strings"

	"github.com/gad-lang/gad"
	gp "github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/parser/node"
)

type FormatFlag uint

const (
	Format FormatFlag = iota + 1
	FormatTranspile
)

type ToGadCompiler struct {
	c       *Compiler
	preCode string
	format  FormatFlag
}

func NewToGadCompiler(c *Compiler) *ToGadCompiler {
	return &ToGadCompiler{c: c}
}

func (w *ToGadCompiler) PreCode(s string) *ToGadCompiler {
	w.preCode = s
	return w
}

func (w *ToGadCompiler) Format(f FormatFlag) *ToGadCompiler {
	w.format = f
	return w
}

func (c *ToGadCompiler) Compile(out io.Writer) (err error) {
	var (
		data strings.Builder
	)

	if c.preCode != "" {
		data.WriteString(c.preCode + "\n")
	}

	data.WriteString("# gad: mixed\n")

	if err = c.c.Compile(&data); err != nil {
		return
	}

	if c.format > 0 {
		var f *gp.File
		if f, err = gp.Parse(data.String(), "", nil, nil); err != nil {
			return
		}

		data.Reset()
		codeOptions := []node.CodeOption{
			node.CodeWithPrefix("\t"),
		}

		if c.format == FormatTranspile {
			codeOptions = append(codeOptions, node.CodeTranspile(gad.TranspileOptions()))
		}

		node.CodeW(&firstSpacerTrimWriter{w: out}, f, codeOptions...)
		if _, err = gp.Parse(data.String(), "", nil, nil); err != nil {
			return
		}
	}

	return
}

type firstSpacerTrimWriter struct {
	w     io.Writer
	wrote bool
}

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

func (w *firstSpacerTrimWriter) Write(p []byte) (n int, err error) {
	if w.wrote {
		return w.w.Write(p)
	}

	p2 := bytes.TrimLeftFunc(p, func(r rune) bool {
		return asciiSpace[r] == 1
	})

	if len(p2) == 0 {
		n = len(p)
		return
	}

	w.wrote = true
	return w.w.Write(p2)
}
