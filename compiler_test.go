package giom

import (
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
			(custom ? func(*args) { custom(print_line, *args) } : print_line)(i, line)
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
@comp c1
	~~
	var (x = 1, y = 2)
	~~
	@slot main()
		div

@comp c2
	+c1()
		@slot #main(x)
			a #{x}
`, `
const (
	c1 = func(; $slots={}) {
		var (
			x = 1
			y = 2
		)

		const $slot$main$ = func() {
			write(rawstr("<div></div>";cast))

		}

		var $slot$main = ($slots.main ?? $slot$main$)

		$slot$main(; $super=$slot$main$		)
	}
	c2 = func(; $slots={}) {
		{
			const slot$0 = func(x) {
				write(rawstr("<a>";cast))

				giom$write(x				)
				write(rawstr("</a>";cast))

			}

			var $$slots = {}

			$$slots["main"] = slot$0

			c1(; $slots=$$slots			)
		}
	}
)

export {}

return @module
`},

		{"", `
@main
	~~
	var (x = 1, y = 2)
	~~
	@slot main
		div
`, `
const main = func(; $slots={}) {
	var (
		x = 1
		y = 2
	)

	const $slot$main$ = func() {
		write(rawstr("<div></div>";cast))

	}

	var $slot$main = ($slots.main ?? $slot$main$)

	$slot$main(; $super=$slot$main$	)
}

export { main: main }

return @module
`},
		{"", `
@export comp c1
	@slot default
		| dv
`, `
const c1 = func(; $slots={}) {
	const $slot$default$ = func() {
		giom$write("dv"		)
	}

	var $slot$default = ($slots.default ?? $slot$default$)

	$slot$default(; $super=$slot$default$	)
}

export { c1: c1 }

return @module
`},
		{"", `
@comp print_lines(rows)
	@for i, line in rows
		@slot item(i, line)
			| #{=i} => #{=line}#{="\n"}
`, `
const print_lines = func(rows; $slots={}) {
	const $slot$item$ = func(i, line) {
		giom$write(
			i,
			" => ",
			line,
			"\n"
		)
	}

	var $slot$item = ($slots.item ?? $slot$item$)

	for i, line in rows {
		$slot$item(
			i,
			line
			; $super=$slot$item$
		)
	}
}

export {}

return @module
`},
		{"compPrintLines", compPrintLines, `
const print_lines = func(rows; $slots={}) {
	const (
		print_line = func(i, line) {
			giom$write(
				i,
				": ",
				line,
				"\n"
			)
		}
		$slot$line$ = func(i, line) {
			const custom = $slots[(("line[" + i) + "]")]

			((custom ? func(*args) {
				custom(
					print_line,
					*args
				)
			} : print_line))(
				i,
				line
			)
		}
	)

	var $slot$line = ($slots.line ?? $slot$line$)

	for i, line in rows {
		$slot$line(
			i,
			line
			; $super=$slot$line$
		)
	}
}

export {}

return @module
`},
		{"programPrintLines", compPrintLines + `
@comp run
	+print_lines(["a", "b"])
		@slot #( "item[2]" )(i, line)
			| linha 2
`, `
const (
	print_lines = func(rows; $slots={}) {
		const (
			print_line = func(i, line) {
				giom$write(
					i,
					": ",
					line,
					"\n"
				)
			}
			$slot$line$ = func(i, line) {
				const custom = $slots[(("line[" + i) + "]")]

				((custom ? func(*args) {
					custom(
						print_line,
						*args
					)
				} : print_line))(
					i,
					line
				)
			}
		)

		var $slot$line = ($slots.line ?? $slot$line$)

		for i, line in rows {
			$slot$line(
				i,
				line
				; $super=$slot$line$
			)
		}
	}
	run = func(; $slots={}) {
		{
			const slot$0 = func(i, line) {
				giom$write("linha 2"				)
			}

			var $$slots = {}

			$$slots["item[2]"] = slot$0

			print_lines(
				[
					"a",
					"b"
				]
				; $slots=$$slots
			)
		}
	}
)

export {}

return @module
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compileExpect(t, tt.tpl, strings.TrimSpace(tt.out))
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
const (
	print_lines = func(rows; $slots={}) {
		const (
			print_line = func(i, line) {
				giom$write(
					i,
					": ",
					line,
					"\n"
				)
			}
			$slot$line$ = func(i, line) {
				const custom = $slots[(("line[" + i) + "]")]

				((custom ? func(*args) {
					custom(
						print_line,
						*args
					)
				} : print_line))(
					i,
					line
				)
			}
		)

		var $slot$line = ($slots.line ?? $slot$line$)

		for i, line in rows {
			$slot$line(
				i,
				line
				; $super=$slot$line$
			)
		}
	}
	run = func(; $slots={}) {
		{
			const slot$0 = func(i, line) {
				giom$write("linha 2"				)
			}

			var $$slots = {}

			$$slots["item[2]"] = slot$0

			print_lines(
				[
					"a",
					"b"
				]
				; $slots=$$slots
			)
		}
	}
)

export {}

return @module
`},
		{"programPrintLines2", compPrintLines + `
@comp run
	+print_lines(["a", "b"]) ~
		@slot #( "item[2]" )(i, line)
			| linha 2

		~ $childSlots["item[4]"] = (i, line) => giom$write("four line", "\n")
`, `
const (
	print_lines = func(rows; $slots={}) {
		const (
			print_line = func(i, line) {
				giom$write(
					i,
					": ",
					line,
					"\n"
				)
			}
			$slot$line$ = func(i, line) {
				const custom = $slots[(("line[" + i) + "]")]

				((custom ? func(*args) {
					custom(
						print_line,
						*args
					)
				} : print_line))(
					i,
					line
				)
			}
		)

		var $slot$line = ($slots.line ?? $slot$line$)

		for i, line in rows {
			$slot$line(
				i,
				line
				; $super=$slot$line$
			)
		}
	}
	run = func(; $slots={}) {
		{
			const slot$0 = func(i, line) {
				giom$write("linha 2"				)
			}

			var $$slots = {}

			$$slots["item[2]"] = slot$0

			$childSlots["item[4]"] = (i, line) => giom$write(
				"four line",
				"\n"
			)

			print_lines(
				[
					"a",
					"b"
				]
				; $slots=$$slots
			)
		}
	}
)

export {}

return @module
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compileExpect(t, tt.tpl, strings.TrimSpace(tt.out))
		})
	}
}

func TestCompiler_Code(t *testing.T) {
	tests := []struct {
		name, tpl, out string
	}{
		{"", `~ const Levels = (;primary,secondary)`, `const Levels = (;
	primary,
	secondary
)

export {}

return @module
`},
		{"", `~ print(a)`, `print(a)
export {}

return @module
`},
		{"", "~ print(a)\n~ print(b)", "print(a)\nprint(b)\nexport {}\n\nreturn @module\n"},
		{"", `
~~
print(a)

print(b)


~~
`, "print(a)\nprint(b)\nexport {}\n\nreturn @module\n"},
		{"", `
~~
print(a)

print(b)


~~

~ x
`, "print(a)\nprint(b)\nx\nexport {}\n\nreturn @module\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compileExpect(t, tt.tpl, strings.TrimSpace(tt.out))
		})
	}
}

func TestCompiler_Text(t *testing.T) {
	tests := []struct {
		name, tpl, out string
	}{
		{"", `| #{= func(a){return {v:a}}(5).v }`, `giom$write((func(a) {
	return { v: a }
})(5).v)
export {}

return @module
`},

		{"", `| #{= x + 2 }`, `giom$write((x + 2))
export {}

return @module
`},

		{"", `| #{= x }`, `giom$write(x)
export {}

return @module
`},

		{"", `| a #{- x -} b #{-= c }`, `giom$write(
	"a",
	x,
	"b",
	c
)
export {}

return @module
`},

		{"", `| a #{- x } b #{= c }`, `giom$write(
	"a",
	x,
	" b ",
	c
)
export {}

return @module
`},

		{"", `| a #{- x } b`, `giom$write(
	"a",
	x,
	" b"
)
export {}

return @module
`},

		{"", `| a #{- x }`, `giom$write(
	"a",
	x
)
export {}

return @module
`},

		{"", `| a #{ x }`, `giom$write(
	"a ",
	x
)
export {}

return @module
`},

		{"", `| a`, `giom$write("a")
export {}

return @module
`},

		{"", `| link <a href="/">see</a> b`, `giom$write("link <a href=\"/\">see</a> b")
export {}

return @module
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compileExpect(t, tt.tpl, strings.TrimSpace(tt.out))
		})
	}
}

func TestCompiler_ClassCondition(t *testing.T) {
	compileExpect(t, `
button
	.active ? $index : ""
`, `
write(rawstr("<button";cast))

write(giom$attrs(; class=(($index ? "active" : ""))))

write(rawstr("></button>";cast))

export {}

return @module
`)
}

func TestCompiler_CompileExportComp(t *testing.T) {
	compileExpect(t, `
@export comp repeat($value, $count)
	+repeat($value, $level+1)
`, `
const repeat = func($value, $count; $slots={}) {
	{
		repeat(
			$value,
			($level + 1)
		)
	}
}

export { repeat: repeat }

return @module
`)
}

func TestCompiler_CompileComp(t *testing.T) {

	compileExpect(t, `
@comp table(rows, z=3, header=nil)
	@slot body(rows, y=z)
		tbody

`, `
const table = func(rows; z=3, header=nil, $slots={}) {
	const $slot$body$ = func(rows; y=nil) {
		write(rawstr("<tbody></tbody>";cast))

	}

	var $slot$body = ($slots.body ?? $slot$body$)

	$slot$body(
		rows
		; y=z,
		$super=$slot$body$
	)
}

export {}

return @module
`)
	compileExpect(t, `
@comp table(rows, header=nil)
	@slot body(rows)
		tbody

`, `
const table = func(rows; header=nil, $slots={}) {
	const $slot$body$ = func(rows) {
		write(rawstr("<tbody></tbody>";cast))

	}

	var $slot$body = ($slots.body ?? $slot$body$)

	$slot$body(
		rows
		; $super=$slot$body$
	)
}

export {}

return @module
`)
	compileExpect(t, `
@comp repeat($value, $count)
	+repeat($value, $level+1)

+repeat(1, 0)
`, `
{
	repeat(
		1,
		0
	)
}

const repeat = func($value, $count; $slots={}) {
	{
		repeat(
			$value,
			($level + 1)
		)
	}
}

export {}

return @module
`)
}

func TestCompiler_CompileSwitch(t *testing.T) {
	compileExpect(t, `
@switch a
`, `export {}

return @module`)
}

func compileExpect(t *testing.T, tpl, expected string) {
	t.Helper()
	tpl, expected = strings.TrimSpace(tpl), strings.TrimSpace(expected)

	var o strings.Builder
	err := CompileToGad(&o, []byte(tpl), Options{})
	if err != nil {
		t.Errorf("Compiler expect '%s', but got error: \"%v\"", expected, err)
	}
	require.Equal(t, normalizeTrailingWS(expected), normalizeTrailingWS(strings.TrimSpace(o.String())))
}

func normalizeTrailingWS(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n")
}

var _ = normalizeTrailingWS
