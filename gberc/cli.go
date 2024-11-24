package main

import (
	"flag"
	"fmt"
	"os"

	gber "github.com/gad-lang/gber"
)

var prettyPrint bool
var lineNumbers bool

func init() {
	flag.BoolVar(&prettyPrint, "prettyprint", true, "Use pretty indentation in output html.")
	flag.BoolVar(&prettyPrint, "pp", true, "Use pretty indentation in output html.")

	flag.BoolVar(&lineNumbers, "linenos", true, "Enable debugging information in output html.")
	flag.BoolVar(&lineNumbers, "ln", true, "Enable debugging information in output html.")

	flag.Parse()
}

func main() {
	input := flag.Arg(0)

	if len(input) == 0 {
		fmt.Fprintln(os.Stderr, "Please provide an input file. (gberc input.gber)")
		os.Exit(1)
	}

	cmp := gber.New()
	cmp.PrettyPrint = prettyPrint
	cmp.LineNumbers = lineNumbers

	err := cmp.ParseFile(input)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = cmp.CompileWriter(os.Stdout)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
