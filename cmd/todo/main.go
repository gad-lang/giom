package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gad-lang/gad"
	gp "github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/parser/source"
	"github.com/gad-lang/gad/stdlib/helper"
	giom "github.com/gad-lang/giom"
	giomnode "github.com/gad-lang/giom/v2/node"
	gnode "github.com/gad-lang/gad/parser/node"
	giomparser "github.com/gad-lang/giom/v2/parser"
)

const todoDataFile = "todo.json"

// Todo represents a single todo item.
type Todo struct {
	ID    int
	Title string
	Done  bool
}

// todoData is the on-disk format for JSON persistence.
type todoData struct {
	NextID int    `json:"nextID"`
	Todos  []Todo `json:"todos"`
}

// TodoStore holds all todos and persists to todo.json.
type TodoStore struct {
	mu     sync.Mutex
	todos  []Todo
	nextID int
	path   string
}

func NewTodoStore(path string) *TodoStore {
	s := &TodoStore{path: path}
	data, err := os.ReadFile(path)
	if err == nil {
		var d todoData
		if json.Unmarshal(data, &d) == nil {
			s.todos = d.Todos
			s.nextID = d.NextID
			return s
		}
	}
	s.todos = []Todo{}
	s.nextID = 1
	s.save()
	return s
}

func (s *TodoStore) save() {
	d := todoData{NextID: s.nextID, Todos: s.todos}
	data, _ := json.MarshalIndent(d, "", "\t")
	os.WriteFile(s.path, data, 0644)
}

func (s *TodoStore) GetAll() []Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Todo, len(s.todos))
	copy(out, s.todos)
	return out
}

func (s *TodoStore) Add(title string) Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := Todo{ID: s.nextID, Title: title, Done: false}
	s.nextID++
	s.todos = append(s.todos, t)
	s.save()
	return t
}

func (s *TodoStore) Toggle(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.todos {
		if s.todos[i].ID == id {
			s.todos[i].Done = !s.todos[i].Done
			s.save()
			return
		}
	}
}

func (s *TodoStore) Delete(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.todos {
		if s.todos[i].ID == id {
			s.todos = append(s.todos[:i], s.todos[i+1:]...)
			s.save()
			return
		}
	}
}

func (s *TodoStore) ClearCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	var kept []Todo
	for _, t := range s.todos {
		if !t.Done {
			kept = append(kept, t)
		}
	}
	s.todos = kept
	s.save()
}

func main() {
	templateSrc, err := os.ReadFile("cmd/todo/templates/todo.giom")
	if err != nil {
		log.Fatalf("read template: %v", err)
	}

	fs := source.NewFileSet()
	file := fs.AddFileData("todo.giom", -1, templateSrc)
	p := giomparser.NewParser(file)
	parsed, err := p.ParseFile()
	if err != nil {
		log.Fatalf("parse: %v", err)
	}

	converted := giomnode.Convert(parsed.Stmts)

	// Append return main(;todos=Todos, filter=FilterV) to call the main function
	mainCall := &gnode.CallExpr{
		Func: gnode.EIdent("main", 0),
	}
	mainCall.NamedArgs.Append(&gnode.NamedArgExpr{Ident: gnode.EIdent("todos", 0)}, gnode.EIdent("Todos", 0))
	mainCall.NamedArgs.Append(&gnode.NamedArgExpr{Ident: gnode.EIdent("filterV", 0)}, gnode.EIdent("FilterV", 0))
	converted = append(converted, &gnode.ReturnStmt{
		Return: gnode.Return{Result: mainCall},
	})

	gadSrc, err := giomparser.Format(converted)
	if err != nil {
		log.Fatalf("format GAD: %v", err)
	}

	builtins := giom.AppendBuiltins(gad.NewBuiltins()).Build()
	st := gad.NewSymbolTable(builtins.Builtins().NameSet)
	_, bc, err := gad.Compile(st, gadSrc, gad.CompileOptions{
		CompilerOptions: gad.CompilerOptions{
			Context:   context.Background(),
			ModuleMap: helper.NewModuleMap(),
		},
		ScannerOptions: gp.ScannerOptions{},
	})
	if err != nil {
		log.Fatalf("compile bytecode: %v", err)
	}

	// compileOnce caches and reuses the compiled bytecode and builtins.
	type compiled struct {
		bc       *gad.Bytecode
		builtins *gad.StaticBuiltins
	}
	comp := &compiled{bc: bc, builtins: builtins}

	store := NewTodoStore(todoDataFile)

	mux := http.NewServeMux()

	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("cmd/todo/static"))))

	// Home page — render todo list
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		filter := r.URL.Query().Get("filter")
		if filter == "" {
			filter = "all"
		}

		allTodos := store.GetAll()
		var filtered []Todo
		switch filter {
		case "active":
			for _, t := range allTodos {
				if !t.Done {
					filtered = append(filtered, t)
				}
			}
		case "completed":
			for _, t := range allTodos {
				if t.Done {
					filtered = append(filtered, t)
				}
			}
		default:
			filtered = allTodos
		}

		var buf bytes.Buffer
		vm := gad.NewVM(comp.builtins, comp.bc)
		vm.Setup(gad.SetupOpts{
			ToRawStrHandler: func(vm *gad.VM, s gad.Str) gad.RawStr {
				return gad.RawStr(html.EscapeString(string(s)))
			},
		})
		module, err := vm.RunOpts(&gad.RunOpts{
			StdOut: &buf,
			Globals: gad.Dict{
				"Todos":   mustObj(gad.ToObject(filtered)),
				"FilterV": mustObj(gad.ToObject(filter)),
			},
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("render error: %v", err), http.StatusInternalServerError)
			return
		}
		_ = module
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.Copy(w, &buf)
	})

	// Add todo
	mux.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		title := strings.TrimSpace(r.FormValue("title"))
		if title == "" {
			title = "Untitled"
		}
		store.Add(title)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// Toggle todo done state
	mux.HandleFunc("/toggle", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.URL.Query().Get("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		store.Toggle(id)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// Delete todo
	mux.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.URL.Query().Get("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		store.Delete(id)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// Clear completed
	mux.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		store.ClearCompleted()
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	addr := ":8080"
	log.Printf("Todo app listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
func mustObj(obj gad.Object, err error) gad.Object {
	if err != nil {
		panic(err)
	}
	return obj
}
