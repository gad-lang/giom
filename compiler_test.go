package giom

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	compPrintLines = `
@comp print_lines(rows)
	@func print_line(i, line)
		| #{=i}: #{=line}#{="\n"}

	@for i, line in rows
		@slot line(i, line)
			~~
			const custom = $slots["line["+i+"]"]
			(custom ? ((*args, **kwargs) => custom(print_line, *args, **kwargs)) : print_line)(i, line)
			~~
`

	programPrintLines = compPrintLines + `
@main
	+print_lines(["a", "b"])
`
)

func TestCompiler_Slots(t *testing.T) {
	tests := []struct {
		name, tpl, out string
	}{
		{"", `
@main
	~~
	var (x = 1, y = 2)
	~~
	@slot main
		div
`, `
	const c1 = func c1($slots={}) {
		const $slot$default$ = func $slot$default$() {
			giomTextWrite("dv")
		}
		var $slot$default = ($slots.default ?? (_, *args, **kwargs) => $slot$default$(*args; **kwargs))
		$slot$default($slot$default$)
	}
`},
		{"", `
@export comp c1
	@slot default
		| dv
`, `
	const c1 = func c1($slots={}) {
		const $slot$default$ = func $slot$default$() {
			giomTextWrite("dv")
		}
		var $slot$default = ($slots.default ?? (_, *args, **kwargs) => $slot$default$(*args; **kwargs))
		$slot$default($slot$default$)
	}
`},
		{"", `
@comp print_lines(rows)
	@for i, line in rows
		@slot item(i, line)
			| #{=i} => #{=line}#{="\n"}
`, `
	const print_lines = func print_lines(rows, $slots={}) {
		const $slot$item$ = func $slot$item$(i, line) {
			giomTextWrite(i, " => ", line, "\n")
		}
		var $slot$item = ($slots.item ?? (_, *args, **kwargs) => $slot$item$(*args; **kwargs))
		for i, line in rows {
			$slot$item($slot$item$, i, line)
		}
	}
`},
		{"compPrintLines", compPrintLines, `
	const print_lines = func print_lines(rows, $slots={}) {
		const print_line = func print_line(i, line) {
			giomTextWrite(i, ": ", line, "\n")
		}
		const $slot$line$ = func $slot$line$(i, line) {
			(($slots[(("line[" + i) + "]")] ?? print_line))(i, line)
		}
		var $slot$line = ($slots.line ?? (_, *args, **kwargs) => $slot$line$(*args; **kwargs))
		for i, line in rows {
			$slot$line($slot$line$, i, line)
		}
	}
`},
		{"programPrintLines", compPrintLines + `
@comp run
	+print_lines(["a", "b"])
		@slot #( "item[2]" )(i, line)
			| linha 2
`, `
	const print_lines = func print_lines(rows, $slots={}) {
		const print_line = func print_line(i, line) {
			giomTextWrite(i, ": ", line, "\n")
		}
		const $slot$line$ = func $slot$line$(i, line) {
			(($slots[(("line[" + i) + "]")] ?? print_line))(i, line)
		}
		var $slot$line = ($slots.line ?? (_, *args, **kwargs) => $slot$line$(*args; **kwargs))
		for i, line in rows {
			$slot$line($slot$line$, i, line)
		}
	}
	const run = func run($slots={}) {
		print_lines(["a", "b"])
	}
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compileExpect(t, tt.tpl, `
# gad: mixed
{%
	`+strings.TrimSpace(tt.out)+`
%}
{%
	return {}
%}
`)
		})
	}
}

func TestCompiler_Slots2(t *testing.T) {
	tests := []struct {
		name, tpl, out string
	}{
		{"programPrintLines", compPrintLines + `
@comp run
	+print_lines(["a", "b"])
		@slot #( "item[2]" )(i, line)
			| linha 2
`, `
	const print_lines = func print_lines(rows, $slots={}) {
		const print_line = func print_line(i, line) {
			giomTextWrite(i, ": ", line, "\n")
		}
		const $slot$line$ = func $slot$line$(i, line) {
			(($slots[(("line[" + i) + "]")] ?? print_line))(i, line)
		}
		var $slot$line = ($slots.line ?? (_, *args, **kwargs) => $slot$line$(*args; **kwargs))
		for i, line in rows {
			$slot$line($slot$line$, i, line)
		}
	}
	const run = func run($slots={}) {
		.{
			const $slot$0 = func $slot$0(i, line) {
				giomTextWrite("linha 2")
			}
			var $childSlots = {}
			$childSlots[("item[2]")] = $slot$0
			print_lines(; $slots=$childSlots)
		}
	}
`},
		{"programPrintLines2", compPrintLines + `
@comp run
	+print_lines(["a", "b"]) ~
		@slot #( "item[2]" )(i, line)
			| linha 2

		~ $childSlots["item[4]"] = (i, line) => giomTextWrite("four line", "\n")
`, `
	const print_lines = func print_lines(rows, $slots={}) {
		const print_line = func print_line(i, line) {
			giomTextWrite(i, ": ", line, "\n")
		}
		const $slot$line$ = func $slot$line$(i, line) {
			(($slots[(("line[" + i) + "]")] ?? print_line))(i, line)
		}
		var $slot$line = ($slots.line ?? (_, *args, **kwargs) => $slot$line$(*args; **kwargs))
		for i, line in rows {
			$slot$line($slot$line$, i, line)
		}
	}
	const run = func run($slots={}) {
		.{
			const $slot$0 = func $slot$0(i, line) {
				giomTextWrite("linha 2")
			}
			var $childSlots = {}
			$childSlots[("item[2]")] = $slot$0
			$childSlots["item[4]"] = (i, line) => giomTextWrite("four line", "\n")
			print_lines(; $slots=$childSlots)
		}
	}
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compileExpect(t, tt.tpl, `
# gad: mixed
{%
	`+strings.TrimSpace(tt.out)+`
%}
{%
	return {}
%}
`)
		})
	}
}

func TestCompiler_Code(t *testing.T) {
	tests := []struct {
		name, tpl, out string
	}{
		{"", `~ const Levels = (;primary,secondary)`, `print(a)`},
		{"", `~ print(a)`, `print(a)`},
		{"", "~ print(a)\n~ print(b)", "print(a)\n\tprint(b)"},
		{"", `
~~
print(a)

print(b)


~~
`, "print(a)\n\tprint(b)"},
		{"", `
~~
print(a)

print(b)


~~

~ x
`, "print(a)\n\tprint(b)\n\tx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compileExpect(t, tt.tpl, `
# gad: mixed
{%
	`+strings.TrimSpace(tt.out)+`
%}
{%
	return {}
%}
`)
		})
	}
}

func TestCompiler_Text(t *testing.T) {
	tests := []struct {
		name, tpl, out string
	}{
		{"", `| #{= func(a){return {v:a}}(5).v }`, `
	giomTextWrite(func(a) {
		return {v: a}
	}(5).v)`},

		{"", `| #{= x + 2 }`, `giomTextWrite(((x + 2)))`},

		{"", `| #{= x }`, `giomTextWrite(x)`},

		{"", `| a #{- x -} b #{-= c }`, `
	giomTextWrite("a")
	x
	giomTextWrite("b", c)`},

		{"", `| a #{- x } b #{= c }`, `
	giomTextWrite("a")
	x
	giomTextWrite(" b ", c)`},

		{"", `| a #{- x } b`, `
	giomTextWrite("a")
	x
	giomTextWrite(" b")`},

		{"", `| a #{- x }`, `
	giomTextWrite("a")
	x`},

		{"", `| a #{ x }`, `
	giomTextWrite("a ")
	x`},

		{"", `| a`, `giomTextWrite("a")`},

		{"", `| link <a href="/">see</a> b`, `giomTextWrite("link <a href=\"/\">see</a> b")`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compileExpect(t, tt.tpl, `
# gad: mixed
{%
	`+strings.TrimSpace(tt.out)+`
%}
{%
	return {}
%}
`)
		})
	}
}

func TestCompiler_ClassCondition(t *testing.T) {
	compileExpect(t, `
button
	.active ? $index : ""
`, `
# gad: mixed
<button{%=attrs(; class=(($index ? "active" : "")))%}></button>
{%
	return {}
%}
`)
}

func TestCompiler_CompileExportComp(t *testing.T) {
	compileExpect(t, `
@export comp repeat($value, $count)
	+repeat($value, $level+1)
`, `
# gad: mixed
{%
	func repeat($value, $count, $blocks={}) {
		$comps.repeat($value,($level + 1))
	}
%}
{%
	return {repeat: repeat}
%}
`)
}

func TestCompiler_CompileComp(t *testing.T) {

	compileExpect(t, `
@comp table(rows, z=3, header=nil)
	@slot body(rows, y=z)
		tbody

`, `
# gad: mixed
{%
	const table = func table(rows, z=3, header=nil, $blocks={}) {
		const $slot$body = func $slot$body(rows, y=nil) {
%}
<tbody></tbody>{%
		}
		(($slots.body ?? ((_, *args, **kwargs) => $slot$body(*args; **kwargs))))($slot$body,rows; y=z)
	}
%}
{%
	return {}
%}
`)
	compileExpect(t, `
@comp table(rows, header=nil)
	@slot body(rows)
		tbody

`, `
# gad: mixed
{%
	const table = func table(rows, header=nil, $blocks={}) {
		const $slot$body = func $slot$body(rows) {
%}
<tbody></tbody>{%
		}
		(($slots.body ?? ((_, *args, **kwargs) => $slot$body(*args; **kwargs))))($slot$body,rows)
	}
%}
{%
	return {}
%}
`)
	return
	compileExpect(t, `
@comp repeat($value, $count)
	+repeat($value, $level+1)

+repeat(1, 0)
`, `
# gad: mixed
{%
	func repeat($value, $count, $blocks={}) {
		$comps.repeat($value,($level + 1))
	}
	$comps.repeat(1,0)
%}
{%
	return {}
%}
`)
}

func TestCompiler_CompileSwitch(t *testing.T) {
	compileExpect(t, `
@switch a
`, `# gad: mixed
{%
	return {}
%}`)
	return

	compileExpect(t, `
@switch a
	@case 1
`, ``)

	compileExpect(t, `
@switch a
	@case 1
	@default
`, ``)
	compileExpect(t, `
@switch a
	@case 1
	@case 2
	@default
`, ``)
	compileExpect(t, `
@switch a
	@case 1
	@case 2
	@case 3
	@default
`, ``)
	compileExpect(t, `
@switch a
	@case 1
	@case 2
	@case 3
`, ``)
	compileExpect(t, `
@switch a
	@case 1
	@case 2
	@case 3
	div
`, ``)
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
@comp Test(yield=nil)
	div

+Fn()

+test.Fn()
	a
`))

	fmt.Println(compileW(os.Stdout, `
~ d := {}
@comp m(v)
	${v}
+m(1)
~ d.m = $comps.m
+d.m(2)
`))
}

func compileW(w io.Writer, tpl string) (err error) {
	panic("not implemented")
}

func compileExpect(t *testing.T, tpl, expected string) {
	tpl, expected = strings.TrimSpace(tpl), strings.TrimSpace(expected)

	var o strings.Builder
	err := CompileToGad(&o, []byte(tpl), Options{})
	if err != nil {
		t.Errorf("Compiler expect '%s', but got error: \"%+10.2v\"", expected, err)
	}
	require.Equal(t, expected, o.String())
}
