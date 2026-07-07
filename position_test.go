package giom

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gad-lang/gad"
)

// runForError compiles src as a giom module, runs its main comp, and returns
// the resulting *gad.RuntimeError (or fails).
func runForError(t *testing.T, src string) *gad.RuntimeError {
	t.Helper()

	builtins := AppendBuiltins(gad.NewBuiltins())
	st := gad.NewSymbolTable(builtins.NameSet)
	names := make([]string, 0)
	// Define any globals referenced by the templates so compilation succeeds;
	// they default to nil, which is exactly what triggers the nil-call.
	for _, n := range []string{"x", "y", "z", "w", "bad"} {
		names = append(names, n)
	}
	if _, err := st.DefineGlobals(names); err != nil {
		t.Fatalf("define globals: %v", err)
	}

	opts := gad.CompileOptions{CompilerOptions: gad.CompilerOptions{
		FallbackFunc: CompileFallback,
	}}
	_, bc, err := Compile(st, []byte(src), opts)
	if err != nil {
		t.Fatalf("compile: %v\nsrc:\n%s", err, src)
	}

	var buf bytes.Buffer
	vm := gad.NewVM(builtins.Build(), bc)
	_, runErr := vm.RunOpts(&gad.RunOpts{StdOut: &buf, Globals: gad.Dict{}})
	if runErr == nil {
		t.Fatalf("expected a runtime error, got output %q\nsrc:\n%s", buf.String(), src)
	}
	re, ok := runErr.(*gad.RuntimeError)
	if !ok {
		t.Fatalf("expected *gad.RuntimeError, got %T: %v", runErr, runErr)
	}
	return re
}

// firstTraceLine returns the line number of the deepest (last) stack frame.
func firstTraceLine(re *gad.RuntimeError) int {
	trace := re.StackTrace()
	if len(trace) == 0 {
		return -1
	}
	return trace[len(trace)-1].Line
}

// TestPositionPreservationRuntime verifies that a nil-call inside various giom
// constructs reports the correct source line in the runtime error stack trace.
// Before the fix, giom never populated the source file's line table nor mapped
// fragment positions, so every position resolved to line 1.
func TestPositionPreservationRuntime(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantLine int
	}{
		{
			name: "single-line-code",
			// line 3: ~ x()
			src: "@global x\n" +
				"@main\n" +
				"    ~ x()\n",
			wantLine: 3,
		},
		{
			name: "multi-line-code",
			// line 5: y()
			src: "@global y\n" +
				"@main\n" +
				"    ~~\n" +
				"    a := 1\n" +
				"    y()\n" +
				"    ~~\n",
			wantLine: 5,
		},
		{
			name: "interpolation",
			// line 3: div {= z() }
			src: "@global z\n" +
				"@main\n" +
				"    div {= z() }\n",
			wantLine: 3,
		},
		{
			name: "if-condition",
			// line 3: @if w()
			src: "@global w\n" +
				"@main\n" +
				"    @if w()\n" +
				"        p yes\n",
			wantLine: 3,
		},
		{
			name: "deeper-line",
			// line 6: bad()
			src: "@global bad\n" +
				"@main\n" +
				"    div\n" +
				"        span\n" +
				"            ~ a := 1\n" +
				"            ~ bad()\n",
			wantLine: 6,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			re := runForError(t, tc.src)
			if !strings.Contains(re.Error(), "NotCallableError") {
				t.Fatalf("expected NotCallableError, got: %v", re.Error())
			}
			if got := firstTraceLine(re); got != tc.wantLine {
				t.Fatalf("stack trace line = %d, want %d\ntrace:\n%+v", got, tc.wantLine, re.StackTrace())
			}
		})
	}
}
