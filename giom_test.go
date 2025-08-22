package giom

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gad-lang/gad"
	"github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/parser/source"
	"github.com/stretchr/testify/require"
)

func Test_Doctype(t *testing.T) {
	runExpect(t, `!!! 5`, `<!DOCTYPE html>`, nil)
}

func Test_Export(t *testing.T) {
	compileExpect(t, `
@comp Alert()
	div`, `const Alert = func($slots={}) {
	write(rawstr("<div></div>";cast))

}
return {}`)

	compileExpect(t, `@export X`, `return {X: X}`)
	compileExpect(t, `@export X = 2`, `return {X: 2}`)
	compileExpect(t, `~ x := 100
@export Y = x`, "x := 100\nreturn {Y: x}")
}

func Test_Each(t *testing.T) {
	cases := []struct {
		name   string
		tpl    string
		expect string
	}{
		{"", "@for $a, $b in values\n\t#{=$a}", "for $a, $b in values {\n\tgiom$write($a)\n}\nreturn {}"},
		{"", "@for $a, $b in values\n\t#{=$a}\n@else\n\t| no values\n", "for $a, $b in values {\n\tgiom$write($a)\n} else {\n\tgiom$write(\"no values\")\n}\nreturn {}"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			compileExpect(t, tt.tpl, tt.expect)
		})
	}
}

func Test_Nesting(t *testing.T) {
	runExpect(t, `html
						head
							title
						body`,
		`<html><head><title></title></head><body></body></html>`, nil)
}

func Test_Comp(t *testing.T) {
	runExpect(t, `
		@comp a(a)
			p #{=a}
		@main
			+a(1)`, `<p>1</p>`, nil)
}

func Test_CompSlot(t *testing.T) {
	tests := []struct {
		name, tpl, out string
		data           any
	}{
		{"", `
		@comp Alert
			@slot main
				| default message

		@main
			+Alert()
			| #
			+Alert()
				| custom message
`,
			"default message#custom message",
			nil,
		},
		{"", `
		@comp Alert
			~ const title = "TITLE"
			@slot main(title)
				| default title: #{=title}

		@main
			+Alert()
			| #
			+Alert()
				| no title
			| #
			+Alert()
				@slot #main(title)
					| user title: #{=title}
`,
			"default title: TITLE#no title#user title: TITLE",
			nil,
		},
		{"", `
		@comp Alert(title)
			@slot main(title)
				| default title: #{=title}

		@main
			+Alert("T1")
			| #
			+Alert("T2")
				| no title
			| #
			+Alert("T3")
				@slot #main(title)
					| user title: #{=title}
`,
			"default title: T1#no title#user title: T3",
			nil,
		},
		{"", `
		@comp Alert(title, key="no_key")
			@slot main(title,key2=key)
				| default title: #{=title}+#{=key}

		@main
			+Alert("T1")
			| #
			+Alert("T2")
				| no title
			| #
			+Alert("T3")
				@slot #main(title, key2=nil)
					| user title: #{=title}@#{=key2}
			| #
			+Alert("T1",key="xxx")
			| #
			+Alert("T2",key="xxx")
				| no title
			| #
			+Alert("T3",key="yyy")
				@slot #main(title, key2=nil)
					| user title: #{=title}@#{=key2}
			| #
			+Alert("T4",key="zzz")
				@slot #main(title, key2=nil)
					| user title: #{=title}@#{=key2}
`,
			"default title: T1+no_key#no title#user title: T3@no_key#default title: T1+xxx#no title#" +
				"user title: T3@yyy#user title: T4@zzz",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runExpect(t, tt.tpl, tt.out, tt.data)
		})
	}
}

func Test_Comp_Loop(t *testing.T) {
	runExpect(t, `
		@comp r($value, $from, $to)
			@if $from < $to
				#{=$value}
				+__callee__($value, $from+1, $to)

		@main
			+r("a", 0, 3)`, `aaa`, nil)

	runExpect(t, `
		~ d := {}
		@comp m(v)
			#{=v}
		@main
			+m(1)
			~ d.m = m
			+d.m(2)`, `12`, nil)
}

func Test_Comp_NoArguments(t *testing.T) {
	runExpect(t, `
		@comp a()
			p Testing

		@main
			+a()`, `<p>Testing</p>`, nil)
}

func Test_Comp_MultiArguments(t *testing.T) {
	runExpect(t, `
		@comp a($a, $b, $c, $d)
			p #{$a} #{$b} #{$c} #{$d}

		@main
			+a("a", "b", "c", 2)`, `<p>a b c 2</p>`, map[string]any{"A": 2})
}

func Test_Comp_NameWithDashes(t *testing.T) {
	runExpect(t, `
		@comp i-am-mixin($a, $b, $c, $d)
			p #{$a} #{$b} #{$c} #{$d}

		@main
			+i-am-mixin("a", "b", "c", 2)`, `<p>a b c 2</p>`, map[string]any{"A": 2})
}

func Test_Comp_Unknown(t *testing.T) {
	_, err := run(`
		@main
			+bar(1)`, nil)

	expected := `unresolved reference "bar"`
	if err == nil {
		t.Fatalf(`Expected {%s} error.`, expected)
	} else if !strings.Contains(err.Error(), expected) {
		t.Fatalf("Error {%s} does not contains {%s}.", err.Error(), expected)
	}
}

func Test_Comp_NotEnoughArguments(t *testing.T) {
	_, err := run(`
		@comp foo($a)
			p #{$a}

		@main
			+foo()`, nil)

	expected := `WrongNumberOfArgumentsError: want=1 got=0`
	if err == nil {
		t.Fatalf(`Expected {%s} error.`, expected)
	} else if !strings.Contains(err.Error(), expected) {
		t.Fatalf("Error {%s} does not contains {%s}.", err.Error(), expected)
	}
}

func Test_Comp_TooManyArguments(t *testing.T) {
	_, err := run(`
		@comp foo($a)
			p #{$a}

		@main
			+foo("a", "b")`, nil)

	expected := `WrongNumberOfArgumentsError: want=1 got=2`
	if err == nil {
		t.Fatalf(`Expected {%s} error.`, expected)
	} else if !strings.Contains(err.Error(), expected) {
		t.Fatalf("Error {%s} does not contains {%s}.", err.Error(), expected)
	}
}

func Test_ClassName(t *testing.T) {
	runExpect(t, `
	@comp x(cls)
		div.test
			p.c1.c2
				[class=["c3", cls, [false="c6"], [true="c7"]]]
				[class=[false="c8"], class=[true="c9"]]
				[class=[false=["c10", "c11"]], class=[true=["c12", "c13"]]]
				[class=[(1 > 0)=["c14", "c15"]], class=[(2+1 == 3)=["c16", "c17"]]]
				.c4
	@main
		+x("c5")
`, `<div class="test"><p class="c1 c2 c3 c5 c7 c9 c12 c13 c14 c15 c16 c17 c4"></p></div>`,
		"test4")
}

func Test_Id(t *testing.T) {
	res, err := run(`div#test
						p#test1#test2`, nil)

	if err != nil {
		t.Fatal(err.Error())
	} else {
		expect(res, `<div id="test"><p id="test2"></p></div>`, t)
	}
}

func Test_Switch(t *testing.T) {
	runExpect(t, `
@switch a
	@case 1
		| v1
	@case 2
		| v2
	@default
		| v0:#{a}
`, `v1`, map[string]any{"a": 1})
}

func Test_Attribute(t *testing.T) {
	runExpect(t, `hr[[b=1], [(false)=[c=2]], [(true)=[c=3]]]`, `<hr b="1" c="3" />`, nil)
	runExpect(t, `hr[[b=1], [(false)=(;c=2,d=3)], [(true)=(;c=3,d=5)]]`, `<hr b="1" c="3" d="5" />`, nil)
	runExpect(t, `hr[[b=1], [(3-2 == 1)=(;c=3,d=5)]]`, `<hr b="1" c="3" d="5" />`, nil)
	runExpect(t, `hr[[b=1], [(3-2 == 2)=(;c=3,d=5)]]`, `<hr b="1" />`, nil)
	runExpect(t, `hr[[b=2]]`, `<hr b="2" />`, nil)
	runExpect(t, "input[readonly]", `<input readonly />`, nil)
	runExpect(t, "input[readonly=false]", `<input />`, nil)
	runExpect(t, "input[type=`text`][name][size=30]", `<input type="text" name size="30" />`, nil)
	runExpect(t, "input[type=`text`][name=0][size=30]", `<input type="text" size="30" />`, nil)
	runExpect(t, "input[type=`text`]\n\t[name]", `<input type="text" name />`, nil)
	runExpect(t, `a.btn[href="link",class=true?"btn-primary":"btn-outline-primary"]`, `<a href="link" class="btn btn-primary"></a>`, nil)
	runExpect(t, "input[type=`text`][name=\"\"][size=30]", `<input type="text" size="30" />`, nil)
	runExpect(t, "input[type=`text`,name,size=30]", `<input type="text" name size="30" />`, nil)
	runExpect(t, "input[type=`text`,name=``,size=30]", `<input type="text" size="30" />`, nil)
	runExpect(t, "div[name=`Te>st`]",
		`<div name="Te>st"></div>`,
		nil)
	runExpect(t, `div[name="Te>st"]`,
		`<div name="Te&gt;st"></div>`,
		nil)
	runExpect(t, `div[name="Test"]["@foo.bar"="baz"].testclass
						p
							[style="text-align: center; color: maroon"]`,
		"<div name=\"Test\" @foo.bar=\"baz\" class=\"testclass\"><p style=\"text-align: center; color: maroon\"></p></div>",
		nil)
}

func Test_RawText(t *testing.T) {
	res, err := run(`html
						script
							var a = 5;
							alert(a)
						style
							body {
								color: white
							}`, nil)

	if err != nil {
		t.Fatal(err.Error())
	} else {
		expect(res, "<html><script>var a = 5;\nalert(a)</script><style>body {\n\tcolor: white\n}</style></html>", t)
	}
}

func Test_Empty(t *testing.T) {
	res, err := run(``, nil)

	if err != nil {
		t.Fatal(err.Error())
	} else {
		expect(res, ``, t)
	}
}

func Test_ArithmeticExpression(t *testing.T) {
	res, err := run(`#{A + B * C}`, map[string]int{"A": 2, "B": 3, "C": 4})

	if err != nil {
		t.Fatal(err.Error())
	} else {
		expect(res, `14`, t)
	}
}

func Test_BooleanExpression(t *testing.T) {
	res, err := run(`#{C - A < B}`, map[string]int{"A": 2, "B": 3, "C": 4})

	if err != nil {
		t.Fatal(err.Error())
	} else {
		expect(res, `true`, t)
	}
}

func Test_FuncCall(t *testing.T) {
	runExpect(t, `div[data-map=json.Marshal($)]`,
		`<div data-map="{&#34;A&#34;:2,&#34;B&#34;:3,&#34;C&#34;:4}"></div>`,
		map[string]any{"$": map[string]int{"A": 2, "B": 3, "C": 4}})
}

type DummyStruct struct {
	X string
}

func (d DummyStruct) MethodWithArg(s string) string {
	return d.X + " " + s
}

func Test_StructMethodCall(t *testing.T) {
	d := DummyStruct{X: "Hello"}

	res, err := run(`#{ $.MethodWithArg("world") }`, d)

	if err != nil {
		t.Fatal(err.Error())
	} else {
		expect(res, `Hello world`, t)
	}
}

func Test_Dollar_In_TagAttributes(t *testing.T) {
	res, err := run(`input[placeholder="$ per "+kwh]`, map[string]interface{}{
		"kwh": "kWh",
	})

	if err != nil {
		t.Fatal(err.Error())
	} else {
		expect(res, `<input placeholder="$ per kWh" />`, t)
	}
}

func Test_ConditionEvaluation(t *testing.T) {
	runExpect(t, `input
		[value=row?.Value ? row]`,
		`<input />`,
		map[string]interface{}{
			"row": nil,
		})

	runExpect(t, `input
		[value="test"] ? !row`,
		`<input value="test" />`,
		nil)
}

func expect(cur, expected string, t *testing.T) {
	if cur != expected {
		t.Fatalf("Expected {%s} got {%s}.", expected, cur)
	}
}

type runOption func(opts *runOpts) error

func withModule(name, source string) runOption {
	return func(opts *runOpts) error {
		if opts.modules == nil {
			opts.modules = make(map[string]string)
		}
		opts.modules[name] = source
		return nil
	}
}

func withCompileOptionsHander(handle func(co *gad.CompileOptions)) runOption {
	return func(opts *runOpts) error {
		opts.compileOptionsHandle = handle
		return nil
	}
}

type runOpts struct {
	modules              map[string]string
	compileOptionsHandle func(co *gad.CompileOptions)
}

func runExpect(t *testing.T, tpl, expected string, data any, opt ...runOption) {
	t.Helper()
	code, _, res, err := runt(tpl, data, opt...)
	printSource := func() {
		if len(code) == 0 {
			return
		}
		fmt.Fprint(os.Stderr, "\n\n%%%%%%%%%%%% BEGIN SOURCE CODE %%%%%%%%%%%%\n")
		lines := strings.Split(code, "\n")
		for i, line := range lines {
			is := strconv.Itoa(i + 1)
			if diff := 5 - len(is); diff > 0 {
				is = strings.Repeat(" ", diff) + is
			}
			fmt.Fprintf(os.Stderr, "%s | %s\n", is, line)
		}
		fmt.Fprint(os.Stderr, "%%%%%%%%%%%% END SOURCE CODE %%%%%%%%%%%%\n")
	}

	if err != nil {
		switch et := err.(type) {
		case *gad.RuntimeError:
			fmt.Fprintf(os.Stderr, "%+v\n", et)
			if st := et.StackTrace(); len(st) > 0 {
				et.FileSet().Position(source.Pos(st[len(st)-1].Offset)).TraceLines(os.Stderr, 3, 3)
			}
			printSource()
			t.Fatal(err.Error())
		case *parser.ErrorList, *gad.CompilerError:
			fmt.Fprintf(os.Stderr, "%+3.3v\n", et)
			printSource()
			t.Fatal(err.Error())
		default:
			printSource()
		}

		t.Fatalf("%+5.5v", err)
	} else {
		var ok bool
		defer func() {
			if !ok {
				printSource()
			}
		}()
		require.Equal(t, expected, res)
		ok = true
	}
}

func runExpectError(t *testing.T, tpl, expectedError string, data any, opt ...runOption) {
	t.Helper()
	_, _, _, err := runt(tpl, data, opt...)

	if err == nil {
		t.Fatalf("expected error, but got nil")
	} else if err.Error() != expectedError {
		t.Fatalf("Expected error {%s} got {%s}.", expectedError, err.Error())
	}

	switch t := err.(type) {
	case *gad.RuntimeError:
		fmt.Fprintf(os.Stderr, "%+v\n", t)
		if st := t.StackTrace(); len(st) > 0 {
			t.FileSet().Position(source.Pos(st[len(st)-1].Offset)).TraceLines(os.Stderr, 20, 20)
		}
	case *parser.ErrorList, *gad.CompilerError:
		fmt.Fprintf(os.Stderr, "%+20.20v\n", t)
	}

	t.Fatalf("%+5.5v", err)
}

func runExpectErrorTrace(t *testing.T, tpl, expectedError string, trace string, h gad.ErrorHumanizing, opt ...runOption) {
	t.Helper()
	_, _, _, err := runt(tpl, nil, opt...)

	if err == nil {
		t.Fatalf("expected error, but got nil")
	} else if expectedError != "" && err.Error() != expectedError {
		t.Fatalf("Expected error {%s} got {%s}.", expectedError, err.Error())
	}

	var out bytes.Buffer

	if me, _ := err.(*ModuleCompileError); me != nil {
		fmt.Fprintf(&out, "Compile module %q\n", me.module)
		err = me.err
	}

	h.Humanize(&out, err)
	require.Equal(t, trace, out.String())
}

func run(tpl string, data any) (string, error) {
	_, _, ret, err := runt(tpl, data)
	return ret, err
}

type ModuleCompileError struct {
	module string
	err    error
}

func (e *ModuleCompileError) Error() string {
	return fmt.Sprintf("compiling module %q: %v", e.module, e.err)
}

func runt(tpl string, data any, opt ...runOption) (code string, t *Template, res string, err error) {
	var (
		globalNames []string
		dataDict    map[string]any
		i           int
		opts        runOpts
	)

	for _, opt := range opt {
		if err = opt(&opts); err != nil {
			return
		}
	}

	if data != nil {
		switch t := data.(type) {
		case map[string]any:
			dataDict = t
		default:
			rv := reflect.ValueOf(data)
			if rv.Kind() == reflect.Map {
				dataDict = make(map[string]any, rv.Len())
				for _, k := range rv.MapKeys() {
					name := fmt.Sprint(k.Interface())
					dataDict[name] = rv.MapIndex(k).Interface()
				}
			} else {
				dataDict = map[string]any{
					"$": data,
				}
			}
		}

		globalNames = make([]string, len(dataDict))

		for name := range dataDict {
			globalNames[i] = name
			i++
		}
	}

	pc := []string{
		`strings, json := [import("strings"), import("json")]`,
	}

	if len(globalNames) > 0 {
		pc = append(pc, "global("+strings.Join(globalNames, ", ")+")")
	}

	var out bytes.Buffer
	if err = CompileToGad(&out, []byte(tpl), Options{
		PreCode: strings.Join(pc, "\n"),
	}); err != nil {
		return
	}

	code = out.String()

	modules := make(map[string]string)

	if opts.modules != nil {
		var out strings.Builder
		for k, v := range opts.modules {
			if err = CompileToGad(&out, []byte(v), Options{}); err != nil {
				err = &ModuleCompileError{k, err}
				return
			}
			modules[k] = out.String()
			out.Reset()
		}
	}

	if t, err = NewTemplateBuilder(out.Bytes()).
		WithHandleOptions(func(co *gad.CompileOptions) {
			for k, v := range modules {
				co.CompilerOptions.ModuleMap.AddSourceModule(k, []byte(v))
			}
			if opts.compileOptionsHandle != nil {
				opts.compileOptionsHandle(co)
			}
		}).Build(); err != nil {
		return
	}

	var buf bytes.Buffer
	if _, err = t.Executor().
		Out(&buf).
		Global(dataDict).
		ExecuteModule(); err != nil {
		return
	}

	res = strings.TrimSpace(buf.String())
	return
}
