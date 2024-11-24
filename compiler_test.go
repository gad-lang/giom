package gber

import (
	"fmt"
	"io"
	"os"
	"testing"
)

func TestCompiler_ClassCondition(t *testing.T) {
	err := compileW(os.Stdout, `
button
	.active ? $index / : ""
`)
	fmt.Println(err)
}

func TestCompiler_CompileExportMixin(t *testing.T) {
	err := compileW(os.Stdout, `
@exported mixin repeat($value, $count)
	+repeat($value, $level+1)
`)
	fmt.Println(err)
}

func TestCompiler_CompileMixin(t *testing.T) {
	err := compileW(os.Stdout, `
@mixin repeat($value, $count)
	+repeat($value, $level+1)

+repeat(1, 0)
`)
	fmt.Println(err)
}

func TestCompiler_CompileSwitch(t *testing.T) {
	err := compileW(os.Stdout, `
@switch a
`)
	fmt.Println(err)

	err = compileW(os.Stdout, `
@switch a
	@case 1
`)
	fmt.Println(err)

	err = compileW(os.Stdout, `
@switch a
	@case 1
	@default
`)
	fmt.Println(err)

	err = compileW(os.Stdout, `
@switch a
	@case 1
	@case 2
	@default
`)
	fmt.Println(err)

	err = compileW(os.Stdout, `
@switch a
	@case 1
	@case 2
	@case 3
	@default
`)
	fmt.Println(err)

	err = compileW(os.Stdout, `
@switch a
	@case 1
	@case 2
	@case 3
`)
	fmt.Println(err)

	err = compileW(os.Stdout, `
@switch a
	@case 1
	@case 2
	@case 3
	div
`)
	fmt.Println(err != nil)
}

func TestCompiler_CompileImport(t *testing.T) {
	err := compileW(os.Stdout, `@import "abc" as util`)
	fmt.Println(err)
}

func TestCompiler_CompileConcat(t *testing.T) {
	err := compileW(os.Stdout, `
${-1}
`)
	fmt.Println(err)
}

func TestCompiler_CompileInit(t *testing.T) {
	fmt.Println(compileW(os.Stdout, `
	~~~
		1
		2
	~~~
`))
}
func TestCompiler_CompileCode(t *testing.T) {
	fmt.Println(compileW(os.Stdout, `
	~ 1
`))
	fmt.Println(compileW(os.Stdout, `
	~~
		1
	~~
`))
	fmt.Println(compileW(os.Stdout, `
	~~
		1
		2
	~~
`))
	fmt.Println(compileW(os.Stdout, `
	~~
		1
	2
		3
	~~
`))
}

func TestCompiler_CompileMultiCode(t *testing.T) {
	err := compileW(os.Stdout, `
~~
1
2
~~
`)
	fmt.Println(err)
}

func TestCompiler_CompileIf(t *testing.T) {
	fmt.Println(compileW(os.Stdout, `
@if a
`))

	fmt.Println(compileW(os.Stdout, `
@if a
	av
@else if b
	bv
`))

	fmt.Println(compileW(os.Stdout, `
@if a
	av
@else if b
	bv
@else
	cv
`))
}

func TestCompiler_CompileTag(t *testing.T) {
	fmt.Println(compileW(os.Stdout, `
@mixin Test(yield=nil)
	div

+Fn()

+test.Fn()
	a
`))

	fmt.Println(compileW(os.Stdout, `
~ d := {}
@mixin m(v)
	${v}
+m(1)
~ d.m = $$mixins.m
+d.m(2)
`))
}

func compile(tpl string) (result string, err error) {
	result, err = CompileToString(tpl, Options{})
	return
}

func compileW(w io.Writer, tpl string) (err error) {
	return CompileToWriter(w, tpl, Options{})
}
