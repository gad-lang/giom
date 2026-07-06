package giom

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gad-lang/gad"
)

type templateCacheEntry struct {
	bc        *gad.Bytecode
	builtins  *gad.Builtins
	files     map[string]time.Time
	changedAt time.Time
}

type trackingReader struct {
	files map[string]struct{}
}

func newTrackingReader() *trackingReader {
	return &trackingReader{files: make(map[string]struct{})}
}

func (r *trackingReader) Read(path string) ([]byte, string, error) {
	r.files[path] = struct{}{}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return data, "file:" + path, nil
}

// Render handles Giom template rendering with bytecode caching and
// automatic recompilation on file changes. It is safe for concurrent use.
type Render struct {
	// TemplateDelay is the debounce duration before recompiling after
	// a file change is detected. Defaults to 5s.
	TemplateDelay time.Duration

	// WorkDir is the directory used as the base for resolving module
	// imports via FileImporter. Defaults to the directory of the
	// rendered file if empty.
	WorkDir string

	// TranspilePath returns the output path for transpiled .gad files.
	// If nil, transpilation is skipped.
	TranspilePath func(srcPath string) string

	// BuiltinsFunc returns the Gad builtins to use for compilation.
	// If nil, defaults to AppendBuiltins(gad.NewBuiltins()).
	BuiltinsFunc func() *gad.Builtins

	mu            sync.Mutex
	templateCache map[string]*templateCacheEntry
}

// Render reads the Giom template at filePath, compiles or retrieves cached
// bytecode, and executes it with globalName bound to globalValue, writing
// the output to out.
func (r *Render) Render(out io.Writer, filePath, globalName string, globalValue gad.Dict) error {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	delay := r.TemplateDelay
	if delay <= 0 {
		delay = 5 * time.Second
	}

	r.mu.Lock()
	if r.templateCache == nil {
		r.templateCache = make(map[string]*templateCacheEntry)
	}
	entry := r.templateCache[filePath]
	needsCompile := entry == nil
	if entry != nil {
		if filesChanged(entry.files) {
			entry.changedAt = time.Now()
		}
		if !entry.changedAt.IsZero() && time.Since(entry.changedAt) >= delay {
			needsCompile = true
		}
	}
	r.mu.Unlock()

	if needsCompile {
		entry, err = r.compile(filePath, src)
		if err != nil {
			return err
		}
	}

	if r.TranspilePath != nil {
		_ = Transpile(filePath, src, r.TranspilePath(filePath))
	}

	var buf bytes.Buffer
	st := gad.NewSymbolTable(entry.builtins.NameSet)
	if _, err := st.DefineGlobals([]string{globalName}); err != nil {
		return err
	}
	vm := gad.NewVM(entry.builtins.Build(), entry.bc)
	_, err = vm.RunOpts(&gad.RunOpts{StdOut: &buf, Globals: gad.Dict{globalName: globalValue}})
	if err != nil {
		return fmt.Errorf("render %s: %w", filePath, err)
	}
	_, err = io.Copy(out, &buf)
	return err
}

func (r *Render) compile(filePath string, src []byte) (*templateCacheEntry, error) {
	builtinsFn := r.BuiltinsFunc
	if builtinsFn == nil {
		builtinsFn = func() *gad.Builtins { return gad.NewBuiltins() }
	}

	builtins := AppendBuiltins(builtinsFn())

	tr := newTrackingReader()
	workDir := r.WorkDir
	if workDir == "" {
		workDir = filepath.Dir(filePath)
	}
	mm := gad.NewModuleMap().SetExtImporter(&FileImporter{
		WorkDir:       workDir,
		FileReader:    tr.Read,
		TranspilePath: r.TranspilePath,
	})

	opts := gad.CompileOptions{CompilerOptions: gad.CompilerOptions{
		ModuleFile:   filePath,
		ModuleMap:    mm,
		FallbackFunc: CompileFallback,
	}}

	st := gad.NewSymbolTable(builtins.NameSet)
	if _, err := st.DefineGlobals([]string{"Model"}); err != nil {
		return nil, err
	}
	_, bc, err := Compile(st, src, opts)
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", filePath, err)
	}

	files := make(map[string]time.Time)
	for p := range tr.files {
		if fi, err := os.Stat(p); err == nil {
			files[p] = fi.ModTime()
		}
	}

	entry := &templateCacheEntry{
		bc:       bc,
		builtins: builtins,
		files:    files,
	}

	r.mu.Lock()
	r.templateCache[filePath] = entry
	r.mu.Unlock()

	return entry, nil
}

func filesChanged(files map[string]time.Time) bool {
	for p, mod := range files {
		fi, err := os.Stat(p)
		if err != nil || !fi.ModTime().Equal(mod) {
			return true
		}
	}
	return false
}
