package main

import (
	"fmt"

	"github.com/gad-lang/gber"
)

func main() {
	s, err := gber.CompileToString(`
div.cls
	if true
		.cd
	else
		.de[data-v="123"]

`, gber.Options{false, false, nil, nil})
	fmt.Println(s)
	if err != nil {
		fmt.Println(err)
	}
}
