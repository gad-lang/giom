package v2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gad-lang/gad"
	gp "github.com/gad-lang/gad/parser"
	gnode "github.com/gad-lang/gad/parser/node"
	"github.com/gad-lang/gad/parser/source"
	giom "github.com/gad-lang/giom"
	giomnode "github.com/gad-lang/giom/v2/node"
	giomparser "github.com/gad-lang/giom/v2/parser"
	"github.com/stretchr/testify/require"
)

func parseV2(t *testing.T, src string) *giomnode.File {
	t.Helper()
	fs := source.NewFileSet()
	f := fs.AddFileData("test.giom", -1, []byte(src))
	p := giomparser.NewParser(f)
	file, err := p.ParseFile()
	require.NoError(t, err)
	return file
}

func transpileV2(t *testing.T, src string) string {
	t.Helper()
	file := parseV2(t, src)
	stmts := giomnode.Convert(file.Stmts)
	var out bytes.Buffer
	gnode.CodeW(&out, stmts, gnode.CodeWithPrefix("\t"), gnode.CodeFormat())
	return out.String()
}

func runV2(t *testing.T, src string, globals gad.Dict) string {
	t.Helper()
	_, bc, err := Compile(nil, []byte(src), gad.CompileOptions{})
	require.NoError(t, err)

	var out bytes.Buffer
	vm := gad.NewVM(giom.AppendBuiltins(gad.NewBuiltins()).Build(), bc)
	_, err = vm.RunOpts(&gad.RunOpts{StdOut: &out, Globals: globals})
	require.NoError(t, err)
	return out.String()
}

func TestCompileFallbackTag(t *testing.T) {
	_, bc, err := Compile(nil, []byte("div\n"), gad.CompileOptions{})
	require.NoError(t, err)

	var out bytes.Buffer
	vm := gad.NewVM(giom.AppendBuiltins(gad.NewBuiltins()).Build(), bc)
	_, err = vm.RunOpts(&gad.RunOpts{StdOut: &out})
	require.NoError(t, err)
	require.Equal(t, "<div></div>", out.String())
}

func TestCompileFallbackRawCode(t *testing.T) {
	_, bc, err := Compile(nil, []byte("~ write(raw \"x\")\n"), gad.CompileOptions{})
	require.NoError(t, err)

	var out bytes.Buffer
	vm := gad.NewVM(giom.AppendBuiltins(gad.NewBuiltins()).Build(), bc)
	_, err = vm.RunOpts(&gad.RunOpts{StdOut: &out})
	require.NoError(t, err)
	require.Equal(t, "x", out.String())
}

func TestCompileRuntimeElements(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"nested tags", "section\n\tdiv\n", "<section><div></div></section>"},
		{"raw code", "~ write(raw \"ok\")\n", "ok"},
		{"if true", "@if true\n\tspan\n", "<span></span>"},
		{"if false else", "@if false\n\tspan\n@else\n\tem\n", "<em></em>"},
		{"switch match", "@switch \"a\"\n\t@case \"a\"\n\t\ta\n\t@default\n\t\tb\n", "<a></a>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, runV2(t, tt.src, nil))
		})
	}
}

func TestTranspileToGadSourceBasicStatements(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		contains []string
	}{
		{
			name: "doctype tag attrs",
			src:  "!!!\ninput#email.form-control[type=\"email\"]\n",
			contains: []string{
				`giom$write("<!DOCTYPE html>")`,
				`write(raw "<input")`,
				`giom$attrs(;
	id="email",
	class="form-control",
	type="email"
)`,
				`write(raw " />")`,
			},
		},
		{
			name: "code and assignment",
			src:  "~ const Label = \"x\"\n$value := 2\n",
			contains: []string{
				`const Label = "x"`,
				`$value := 2`,
			},
		},
		{
			name: "if else",
			src:  "@if ok\n\tspan\n@else\n\tem\n",
			contains: []string{
				`if ok {`,
				`write(raw "<span")`,
				`} else {`,
				`write(raw "<em")`,
			},
		},
		{
			name: "for else",
			src:  "@for item in items\n\tli\n@else\n\tp\n",
			contains: []string{
				`for _, item in items {`,
				`write(raw "<li"`,
			},
		},
		{
			name: "switch",
			src:  "@switch kind\n\t@case \"a\"\n\t\ta\n\t@default\n\t\tb\n",
			contains: []string{
				`match kind {`,
				`"a" {`,
				`write(raw "<a")`,
				`else {`,
				`write(raw "<b")`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transpileV2(t, tt.src)
			for _, want := range tt.contains {
				require.Contains(t, got, want)
			}
		})
	}
}

func TestTranspileToGadSourceComponentsAndSlots(t *testing.T) {
	got := transpileV2(t, `@comp card(title)
	@slot header()
		h1
	div.card

@func helper(v)
	span

+card("Hi")
	@slot #header()
		h2

@export Result = helper
`)

	for _, want := range []string{
		`card = func(`,
		`title`,
		`; $slots={}`,
		`const $slot$header$ = func() {`,
		`$slots["header"]`,
		`write(raw "<div")`,
		`class="card"`,
		`helper = func(`,
		`card(`,
		`$$slots["header"] = $slot0`,
		`$slots=$$slots`,
		`export Result = helper`,
	} {
		require.Contains(t, got, want)
	}
}

func TestTranspileToGadSourceIsParseable(t *testing.T) {
	sources := []string{
		"!!!\nmain\n",
		"@if ok\n\tspan\n@else\n\tem\n",
		"@for item in items\n\tli\n",
		"@comp box\n\tdiv\n+box()\n",
		"@switch kind\n\t@case 1\n\t\tone\n\t@default\n\t\tother\n",
	}

	for _, src := range sources {
		t.Run(strings.ReplaceAll(strings.TrimSpace(src), "\n", "|"), func(t *testing.T) {
			got := transpileV2(t, src)
			_, err := gp.Parse(got, "test.gad", nil, nil)
			require.NoError(t, err)
		})
	}
}
