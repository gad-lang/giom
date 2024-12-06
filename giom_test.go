package giom

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
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
	div`, `${const $comps = {}}${$comps.Alert = func() do}
<div></div>${end}
${if __is_module__ {
	return {
		Alert: $comps.Alert,
	}
}}`)

	return
	compileExpect(t, `@export X`, `${if __is_module__ {
	return {
		X: X,
	}
}}`)
	compileExpect(t, `@export X = 2`, `${if __is_module__ {
	return {
		X: 2,
	}
}}`)
	compileExpect(t, `~ x := 100
@export Y = x`, `${- x := 100;}
${if __is_module__ {
	return {
		Y: x,
	}
}}`)
}

func Test_Each(t *testing.T) {
	compileExpect(t, `@for $a, $b, $c in values
								${=$a}`,
		`${for $a, $b, $c in values do}${=$a}${end}`)
	compileExpect(t, `
@for $a, $b, $c in values
  ${=$a}
@else
  | no values
`, `${for $a, $b, $c in values do}${=$a}${else}no values${end}`)
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
			p ${a}

		+a(1)`, `<p>1</p>`, nil)
}

func Test_CompYied(t *testing.T) {
	runExpect(t, `
		@comp Alert($body=nil)
			@if $body
				alert
					~ $body()
			@else
				no-alert

		+Alert()
		hr
		+Alert()
			p hello`, `<no-alert></no-alert><hr /><alert><p>hello</p></alert>`, nil)
}

func Test_CompYied2(t *testing.T) {
	runExpect(t, `
		@comp Alert(*args, $body=nil)
			@if args
				${args[0]}
			@else if $body
				~ $body()

		+Alert("hello")
		hr
		+Alert()
			p world`, `hello<hr /><p>world</p>`, nil)
}

func Test_Comp_Loop(t *testing.T) {
	runExpect(t, `
		@comp repeat($value, $from, $to)
			@if $from < $to
				${$value}
				+repeat($value, $from+1, $to)

		+repeat("a", 0, 3)`, `aaa`, nil)

	runExpect(t, `
		~ d := {}
		@comp m(v)
			${v}
		+m(1)
		~ d.m = $comps.m
		+d.m(2)`, `12`, nil)
}

func Test_Comp_NoArguments(t *testing.T) {
	runExpect(t, `
		@comp a()
			p Testing

		+a()`, `<p>Testing</p>`, nil)
}

func Test_Comp_MultiArguments(t *testing.T) {
	runExpect(t, `
		@comp a($a, $b, $c, $d)
			p ${$a} ${$b} ${$c} ${$d}

		+a("a", "b", "c", A)`, `<p>a b c 2</p>`, map[string]any{"A": 2})
}

func Test_Comp_NameWithDashes(t *testing.T) {
	runExpect(t, `
		@comp i-am-mixin($a, $b, $c, $d)
			p ${$a} ${$b} ${$c} ${$d}

		+i-am-mixin("a", "b", "c", A)`, `<p>a b c 2</p>`, map[string]any{"A": 2})
}

func Test_Comp_Unknown(t *testing.T) {
	_, err := run(`
		@comp foo($a)
			p ${$a}

		+bar(1)`, nil)

	expected := `NotCallableError: nil`
	if err == nil {
		t.Fatalf(`Expected {%s} error.`, expected)
	} else if !strings.Contains(err.Error(), expected) {
		t.Fatalf("Error {%s} does not contains {%s}.", err.Error(), expected)
	}
}

func Test_Comp_NotEnoughArguments(t *testing.T) {
	_, err := run(`
		mixin foo($a)
			p ${$a}

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
		mixin foo($a)
			p ${$a}

		+foo("a", "b")`, nil)

	expected := `WrongNumberOfArgumentsError: want=1 got=2`
	if err == nil {
		t.Fatalf(`Expected {%s} error.`, expected)
	} else if !strings.Contains(err.Error(), expected) {
		t.Fatalf("Error {%s} does not contains {%s}.", err.Error(), expected)
	}
}

func Test_ClassName(t *testing.T) {
	runExpect(t, `div.test
						p.test1.test2
							[class=$]
							.test3`, `<div class="test"><p class="test1 test2 test4 test3"></p></div>`,
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
		v1
	@case 2
		v2
	@default
		v0
`, `<input size="30" type="text" />`, map[string]any{"a": 1})
}

func Test_Attribute(t *testing.T) {
	runExpect(t, `a.btn[href="link",class=true?"btn-primary":"btn-outline-primary"]`, `<a class="btn btn-primary" href="link"></a>`, nil)
	runExpect(t, "input[type=`text`][name=\"\"][size=30]", `<input size="30" type="text" />`, nil)
	runExpect(t, "input[type=`text`][name][size=30]", `<input name size="30" type="text" />`, nil)
	runExpect(t, "input[type=`text`,name,size=30]", `<input name size="30" type="text" />`, nil)
	runExpect(t, "input[type=`text`,name=``,size=30]", `<input size="30" type="text" />`, nil)
	runExpect(t, "div[name=`Te>st`]",
		`<div name="Te>st"></div>`,
		nil)
	runExpect(t, `div[name="Te>st"]`,
		`<div name="Te&gt;st"></div>`,
		nil)
	runExpect(t, `div[name="Test"]["@foo.bar"="baz"].testclass
						p
							[style="text-align: center; color: maroon"]`,
		`<div @foo.bar="baz" class="testclass" name="Test">`+
			`<p style="text-align: center; color: maroon"></p></div>`,
		nil)
}

func Test_AttributeCondition(t *testing.T) {
	runExpectError(t, `
button
	.active ? $index / : ""
`, "error: parse tag 'button' attribute 'class' condition '$index / : \"\"' failed: Parse Error: expected operand, found ':'\n\tat -:1:17", nil)
}

func Test_MultipleClasses(t *testing.T) {
	res, err := run(`div.test1.test2[class="test3"][class="test4"]`, nil)

	if err != nil {
		t.Fatal(err.Error())
	} else {
		expect(res, `<div class="test1 test2 test3 test4"></div>`, t)
	}
}

func Test_EmptyAttribute(t *testing.T) {
	runExpect(t, `div[name]`, `<div name></div>`, nil)
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
	res, err := run(`${A + B * C}`, map[string]int{"A": 2, "B": 3, "C": 4})

	if err != nil {
		t.Fatal(err.Error())
	} else {
		expect(res, `14`, t)
	}
}

func Test_BooleanExpression(t *testing.T) {
	res, err := run(`${C - A < B}`, map[string]int{"A": 2, "B": 3, "C": 4})

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

func runExpect(t *testing.T, tpl, expected string, data any) {
	t.Helper()
	tmpl, res, err := runt(tpl, data)

	if err != nil {
		switch t := err.(type) {
		case *gad.RuntimeError:
			fmt.Fprintf(os.Stderr, "%+v\n", t)
			if st := t.StackTrace(); len(st) > 0 {
				t.FileSet().Position(source.Pos(st[len(st)-1].Offset)).TraceLines(os.Stderr, 20, 20)
			}
		case *parser.ErrorList, *gad.CompilerError:
			fmt.Fprintf(os.Stderr, "%+20.20v\n", t)
		default:
			if tmpl != nil {
				fmt.Println("-----------")
				fmt.Println(tmpl.Source())
				fmt.Println("-----------")
			}
		}

		t.Fatalf("%+5.5v", err)
	} else {
		require.Equal(t, expected, res)
	}
}

func runExpectError(t *testing.T, tpl, expectedError string, data any) {
	t.Helper()
	_, _, err := runt(tpl, data)

	if err == nil {
		t.Fatalf("expected error, but got nil")
	} else if err.Error() != expectedError {
		t.Fatalf("Expected error {%s} got {%s}.", expectedError, err.Error())
	}
}

func run(tpl string, data any) (string, error) {
	_, ret, err := runt(tpl, data)
	return ret, err
}

func runt(tpl string, data any) (t *Template, res string, err error) {
	var (
		globalNames []string
		dataDict    map[string]any
		i           int
	)

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

	if t, err = NewTemplateBuilder(out.Bytes()).Build(); err != nil {
		return
	}

	var buf bytes.Buffer
	if _, err = t.Executor().Out(&buf).Execute(); err != nil {
		return
	}

	res = strings.TrimSpace(buf.String())
	return
}
