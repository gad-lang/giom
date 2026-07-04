package giom

import (
	"os"
	"testing"

	"github.com/gad-lang/gad"
)

func Test_PostList(t *testing.T) {
	name := "post_list"
	f, _ := os.Create("v2/samples/layouts/" + name + ".gad")
	defer f.Close()
	b, err := os.ReadFile("v2/samples/layouts/" + name + ".giom")
	if err != nil {
		panic(err)
	}
	err = CompileToGad(f, b, Options{})
	if err != nil {
		panic(err)
	}
}

func Test_RunErrorTrace(t *testing.T) {
	runExpectErrorTrace(t, `
@main
	+b()
`, ``, `Compile Error: unresolved reference "b"
	at (main):8:3
         6| const main = func(; $slots={}) {
         7| 	{
       🠆 8| 		b()
            		^
         9| 	}
        10| }
`,
		gad.ErrorHumanizing{},
	)
}

func Test_RunErrorTrace2(t *testing.T) {
	runExpectErrorTrace(t, `
~ import("alerts")
@main
	div
`, ``, `ErrCall: ‹builtinFunction: @binaryOperator›; caused by: ‹ZeroDivisionError›
	at (main):6:7
	   alerts:1:1

(main):6:7:
       🠆 6| import("alerts")
                  ^
alerts:1:1:
       🠆 1| 1 / 0
            ^
         2| export {}
         4| return @module`,
		gad.ErrorHumanizing{},
		withModule("alerts", `
~ 1/0
`),
	)
}

func Test_RunDualExportedComp(t *testing.T) {
	compileExpect(t, `
@export comp a()
	| a

@export comp b()
	+a()

@main
	+b()
`, `const (
	a = func(; $slots={}) {
		giom$write("a"		)
	}
	b = func(; $slots={}) {
		{
			a()
		}
	}
	main = func(; $slots={}) {
		{
			b()
		}
	}
)

export {
	a: a,
	b: b,
	main: main
}

return @module`)
}

func Test_RunPrintLines(t *testing.T) {
	runExpect(t, compPrintLines+`
~ const Levels = (;primary,secondary)

`, ``, nil)
	runExpect(t, compPrintLines+`
@main
	div
`, `<div></div>`, nil)

	runExpect(t, compPrintLines+`
@main
	+print_lines(["a", "b", "c", "d"])
		@slot #line(i, line)
			| line #{=str(i,"\n")}
		
`, `line 0
line 1
line 2
line 3`, nil)

	runExpect(t, compPrintLines+`
@main
	+print_lines(["a", "b", "c", "d"])
		@slot #( "line[1]" )(super, i, line)
			| line 1 #{= "\n"}
		@slot #( "line[3]" )(super, i, line)
			| line 3 @ 
			~ super(i,line)
		
`, `0: a
line 1 
2: c
line 3 @ 3: d`, nil)

	runExpect(t, compPrintLines+`
@main
	+print_lines(["a", "b", "c", "d"])
		@slot #( "line[1]" )(_, i, line)
			| line 1 #{= "\n"}
		
`, `0: a
line 1 
2: c
3: d`, nil)

	runExpect(t, compPrintLines+`
@main
	+print_lines(["a", "b", "c", "d"])
`, `0: a
1: b
2: c
3: d`, nil)
}

func Test_RunTableComp(t *testing.T) {
	runExpect(t, `
@comp table(rows, header=nil)
	@slot body(rows)
		tbody
			@for row in rows
				tr
					@for cel in row
						td #{= cel }
@main
	+table([[1,2]])
`, `<tbody><tr><td>1</td><td>2</td></tr></tbody>`, nil)
}

func Test_RunCompOverrideMainSlot(t *testing.T) {
	runExpect(t, `
@comp message()
	@slot main
		| the message
@main
	+message()
		| my msg
`, `my msg`, nil)

	runExpect(t, `
@comp message()
	@slot main
		| default message
@main
	+message()
		@slot #main(;$super)
			| my msg - 
			~ $super()
`, `my msg - default message`, nil)
}
