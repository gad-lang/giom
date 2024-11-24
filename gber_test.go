package gber

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func Test_Doctype(t *testing.T) {
	runExpect(t, `!!! 5`, `<!DOCTYPE html>`, nil)
}

func Test_Export(t *testing.T) {
	genExpect(t, `
@mixin Alert()
	div`, `${const $$mixins = {}}${$$mixins.Alert = func() do}
<div></div>${end}
${if __is_module__ {
	return {
		Alert: $$mixins.Alert,
	}
}}`)

	return
	genExpect(t, `@export X`, `${if __is_module__ {
	return {
		X: X,
	}
}}`)
	genExpect(t, `@export X = 2`, `${if __is_module__ {
	return {
		X: 2,
	}
}}`)
	genExpect(t, `~ x := 100
@export Y = x`, `${- x := 100;}
${if __is_module__ {
	return {
		Y: x,
	}
}}`)
}

func Test_Each(t *testing.T) {
	genExpect(t, `@for $a, $b, $c in values
								${=$a}`,
		`${for $a, $b, $c in values do}${=$a}${end}`)
	genExpect(t, `
@for $a, $b, $c in values
  ${=$a}
@else
  | no values
`, `${for $a, $b, $c in values do}${=$a}${else}no values${end}`)
}

func Test_Calls(t *testing.T) {
	res, _ := generate(`@block a
								+super`)
	fmt.Println(res)
}

func Test_Nesting(t *testing.T) {
	runExpect(t, `html
						head
							title
						body`,
		`<html><head><title></title></head><body></body></html>`, nil)
}

func Test_Mixin(t *testing.T) {
	runExpect(t, `
		@mixin a(a)
			p ${a}

		+a(1)`, `<p>1</p>`, nil)
}

func Test_MixinYied(t *testing.T) {
	runExpect(t, `
		@mixin Alert($body=nil)
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

func Test_MixinYied2(t *testing.T) {
	runExpect(t, `
		@mixin Alert(*args, $body=nil)
			@if args
				${args[0]}
			@else if $body
				~ $body()

		+Alert("hello")
		hr
		+Alert()
			p world`, `hello<hr /><p>world</p>`, nil)
}

func Test_Mixin_Loop(t *testing.T) {
	runExpect(t, `
		@mixin repeat($value, $from, $to)
			@if $from < $to
				${$value}
				+repeat($value, $from+1, $to)

		+repeat("a", 0, 3)`, `aaa`, nil)

	runExpect(t, `
		~ d := {}
		@mixin m(v)
			${v}
		+m(1)
		~ d.m = $$mixins.m
		+d.m(2)`, `12`, nil)
}

func Test_Mixin_NoArguments(t *testing.T) {
	runExpect(t, `
		@mixin a()
			p Testing

		+a()`, `<p>Testing</p>`, nil)
}

func Test_Mixin_MultiArguments(t *testing.T) {
	runExpect(t, `
		@mixin a($a, $b, $c, $d)
			p ${$a} ${$b} ${$c} ${$d}

		+a("a", "b", "c", A)`, `<p>a b c 2</p>`, map[string]any{"A": 2})
}

func Test_Mixin_NameWithDashes(t *testing.T) {
	runExpect(t, `
		@mixin i-am-mixin($a, $b, $c, $d)
			p ${$a} ${$b} ${$c} ${$d}

		+i-am-mixin("a", "b", "c", A)`, `<p>a b c 2</p>`, map[string]any{"A": 2})
}

func Test_Mixin_Unknown(t *testing.T) {
	_, err := run(`
		@mixin foo($a)
			p ${$a}

		+bar(1)`, nil)

	expected := `NotCallableError: nil`
	if err == nil {
		t.Fatalf(`Expected {%s} error.`, expected)
	} else if !strings.Contains(err.Error(), expected) {
		t.Fatalf("Error {%s} does not contains {%s}.", err.Error(), expected)
	}
}

func Test_Mixin_NotEnoughArguments(t *testing.T) {
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

func Test_Mixin_TooManyArguments(t *testing.T) {
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

func Test_Multiple_File_Inheritance(t *testing.T) {
	tmpl, err := CompileDir("samples/", DefaultDirOptions, DefaultOptions)
	if err != nil {
		t.Fatal(err.Error())
	}

	t1a, ok := tmpl["multilevel.inheritance.a"]
	if ok != true || t1a == nil {
		t.Fatal("CompileDir, template not found.")
	}

	t1b, ok := tmpl["multilevel.inheritance.b"]
	if ok != true || t1b == nil {
		t.Fatal("CompileDir, template not found.")
	}

	t1c, ok := tmpl["multilevel.inheritance.c"]
	if ok != true || t1c == nil {
		t.Fatal("CompileDir, template not found.")
	}

	var res bytes.Buffer
	t1c.Execute(&res, nil)
	expect(strings.TrimSpace(res.String()), "<p>This is C</p>", t)
}

func Test_Recursion_In_Blocks(t *testing.T) {
	tmpl, err := CompileDir("samples/", DefaultDirOptions, DefaultOptions)
	if err != nil {
		t.Fatal(err.Error())
	}

	top, ok := tmpl["recursion.top"]
	if !ok || top == nil {
		t.Fatal("template not found.")
	}

	var res bytes.Buffer
	top.Execute(&res, nil)
	expect(strings.TrimSpace(res.String()), "content", t)
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

func Failing_Test_CompileDir(t *testing.T) {
	tmpl, err := CompileDir("samples/", DefaultDirOptions, DefaultOptions)

	// Test Compilation
	if err != nil {
		t.Fatal(err.Error())
	}

	// Make sure files are added to map correctly
	val1, ok := tmpl["basic"]
	if ok != true || val1 == nil {
		t.Fatal("CompileDir, template not found.")
	}
	val2, ok := tmpl["inherit"]
	if ok != true || val2 == nil {
		t.Fatal("CompileDir, template not found.")
	}
	val3, ok := tmpl["compiledir_test/basic"]
	if ok != true || val3 == nil {
		t.Fatal("CompileDir, template not found.")
	}
	val4, ok := tmpl["compiledir_test/compiledir_test/basic"]
	if ok != true || val4 == nil {
		t.Fatal("CompileDir, template not found.")
	}

	// Make sure file parsing is the same
	var doc1, doc2 bytes.Buffer
	val1.Execute(&doc1, nil)
	val4.Execute(&doc2, nil)
	expect(doc1.String(), doc2.String(), t)

	// Check against CompileFile
	compilefile, err := CompileFile("samples/basic.gber", DefaultOptions)
	if err != nil {
		t.Fatal(err.Error())
	}
	var doc3 bytes.Buffer
	compilefile.Execute(&doc3, nil)
	expect(doc1.String(), doc3.String(), t)
	expect(doc2.String(), doc3.String(), t)

}

func Benchmark_Parse(b *testing.B) {
	code := `
	!!! 5
	html
		head
			title Test Title
		body
			nav#mainNav[data-foo="bar"]
			div#content
				div.left
				div.center
					block center
						p Main Content
							.long ? somevar && someothervar
				div.right`

	for i := 0; i < b.N; i++ {
		cmp := New()
		cmp.Parse(code)
	}
}

func Benchmark_Compile(b *testing.B) {
	b.StopTimer()

	code := `
	!!! 5
	html
		head
			title Test Title
		body
			nav#mainNav[data-foo="bar"]
			div#content
				div.left
				div.center
					block center
						p Main Content
							.long ? somevar && someothervar
				div.right`

	cmp := New()
	cmp.Parse(code)

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		cmp.CompileString()
	}
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
		if tmpl != nil {
			fmt.Println("-----------")
			fmt.Println(tmpl.Code)
			fmt.Println("-----------")
		}
		t.Fatal(err.Error())
	} else {
		if res != expected {
			if tmpl != nil {
				fmt.Println("-----------")
				fmt.Println(tmpl.Code)
				fmt.Println("-----------")
			}
			t.Fatalf("Expected {%s} got {%s}.", expected, res)
		}
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

	t, err = Compile(tpl, Options{
		PrettyPrint:       false,
		LineNumbers:       false,
		VirtualFilesystem: nil,
		BuiltinNames:      nil,
		GlobalNames:       globalNames,
		Code:              true,
		PreCode:           `strings, json := [import("strings"), import("json")]`,
	})

	if err != nil {
		return
	}
	var buf bytes.Buffer
	if err = t.Execute(&buf, dataDict); err != nil {
		return
	}

	res = strings.TrimSpace(buf.String())
	return
}

func generate(tpl string) (string, error) {
	c := New()
	if err := c.ParseData([]byte(tpl), "test.gber"); err != nil {
		return "", err
	}
	return c.CompileString()
}

func genExpect(t *testing.T, tpl, expected string) {
	res, err := generate(tpl)
	if err != nil {
		t.Fatal(err.Error())
	} else {
		expect(strings.TrimSpace(res), expected, t)
	}
}
