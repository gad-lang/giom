package giom

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gad-lang/gad"
)

type templateCacheEntry struct {
	bc         *gad.Bytecode
	builtins   *gad.Builtins
	files      map[string]time.Time
	changedAt  time.Time
	compiledAt time.Time
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
	// a file change is detected. Defaults to 15s.
	TemplateDelay time.Duration

	// workDir is the directory used as the base for resolving module
	// imports via FileImporter. Defaults to the directory of the
	// rendered file if empty.
	workDir string

	// TranspilePath returns the output path for transpiled .gad files.
	// If nil, transpilation is skipped.
	TranspilePath func(srcPath string) string

	// BuiltinsFunc returns the Gad builtins to use for compilation.
	// If nil, defaults to AppendBuiltins(gad.NewBuiltins()).
	BuiltinsFunc func() *gad.Builtins

	mu            sync.Mutex
	compileMu     sync.Mutex
	templateCache map[string]*templateCacheEntry
	onRenderFuncs []func(first bool, mainFile string, files []string, lastTime time.Time, err error)
	cachedBuiltins *gad.Builtins
	builtinsOnce  sync.Once
}

// NewRender creates a Render with the given workDir. Non-empty paths are
// resolved to an absolute path. Other fields (TemplateDelay, TranspilePath,
// BuiltinsFunc) may be set on the returned value before use.
func NewRender(workDir string) *Render {
	if workDir != "" {
		abs, err := filepath.Abs(workDir)
		if err == nil {
			workDir = abs
		}
	}
	return &Render{
		workDir:       workDir,
		TemplateDelay: time.Second * 15,
	}
}

// WorkDir returns the base directory for resolving module imports.
func (r *Render) WorkDir() string { return r.workDir }

// OnRender appends callback functions invoked after compilation.
// On first render, first=true with empty files and zero lastTime.
// On recompilation due to file changes, first=false with the changed file paths.
// If compilation fails, err is non-nil and the cached entry is not updated.
// Returns the Render for chaining.
func (r *Render) OnRender(f ...func(first bool, mainFile string, files []string, lastTime time.Time, err error)) *Render {
	r.onRenderFuncs = append(r.onRenderFuncs, f...)
	return r
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
		delay = 15 * time.Second
	}

	var (
		first        bool
		changed      []string
		lastTime     time.Time
		needsCompile bool
		base         string
	)

	r.mu.Lock()
	if r.templateCache == nil {
		r.templateCache = make(map[string]*templateCacheEntry)
	}
	entry := r.templateCache[filePath]
	first = entry == nil
	base = r.workDir
	if base == "" {
		base = filepath.Dir(filePath)
	}
	if entry != nil {
		lastTime = entry.compiledAt
		if changedFiles := changedPaths(entry.files, base); len(changedFiles) > 0 {
			if entry.changedAt.IsZero() {
				entry.changedAt = time.Now()
			}
			changed = changedFiles
		}
		if !entry.changedAt.IsZero() && time.Since(entry.changedAt) >= delay {
			needsCompile = true
		}
	}
	r.mu.Unlock()

	if first || needsCompile {
		newEntry, cerr := r.compile(filePath, src)
		if cerr == nil {
			newEntry.compiledAt = time.Now()
			r.mu.Lock()
			r.templateCache[filePath] = newEntry
			entry = newEntry
			r.mu.Unlock()
		}
		mainRel := filePath
		if base != "" {
			if rel, err := filepath.Rel(base, filePath); err == nil {
				mainRel = rel
			}
		}
		for _, fn := range r.onRenderFuncs {
			fn(first, mainRel, changed, lastTime, cerr)
		}
		if cerr != nil {
			return cerr
		}
	}

	if r.TranspilePath != nil {
		_ = Transpile(filePath, src, r.TranspilePath(filePath))
	}

	st := gad.NewSymbolTable(entry.builtins.NameSet)
	if _, err := st.DefineGlobals([]string{globalName}); err != nil {
		return err
	}
	vm := gad.NewVM(entry.builtins.Build(), entry.bc)
	_, err = vm.RunOpts(&gad.RunOpts{StdOut: out, Globals: gad.Dict{globalName: globalValue}})
	if err != nil {
		return fmt.Errorf("render %s: %w", filePath, err)
	}
	return err
}

func (r *Render) compile(filePath string, src []byte) (*templateCacheEntry, error) {
	r.compileMu.Lock()
	defer r.compileMu.Unlock()

	r.builtinsOnce.Do(func() {
		builtinsFn := r.BuiltinsFunc
		if builtinsFn == nil {
			builtinsFn = func() *gad.Builtins { return gad.NewBuiltins() }
		}
		r.cachedBuiltins = AppendBuiltins(builtinsFn())
	})

	tr := newTrackingReader()
	workDir := r.workDir
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

	st := gad.NewSymbolTable(r.cachedBuiltins.NameSet)
	if _, err := st.DefineGlobals([]string{"Model"}); err != nil {
		return nil, err
	}
	_, bc, err := Compile(st, src, opts)
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", filePath, err)
	}

	files := make(map[string]time.Time)

	// Track imported files.
	for p := range tr.files {
		if fi, err := os.Stat(p); err == nil {
			files[p] = fi.ModTime()
		}
	}
	// Also track the main template file.
	if fi, err := os.Stat(filePath); err == nil {
		files[filePath] = fi.ModTime()
	}

	return &templateCacheEntry{
		bc:       bc,
		builtins: r.cachedBuiltins,
		files:    files,
	}, nil
}

func changedPaths(files map[string]time.Time, base string) []string {
	var out []string
	for p, mod := range files {
		fi, err := os.Stat(p)
		if err != nil || !fi.ModTime().Equal(mod) {
			rel, err := filepath.Rel(base, p)
			if err != nil {
				rel = p
			}
			out = append(out, rel)
		}
	}
	return out
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
