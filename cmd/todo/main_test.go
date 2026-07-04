package main

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gad-lang/gad"
	gp "github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/stdlib/helper"
	giom "github.com/gad-lang/giom"
)
var noRedirect = func(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}


func TestMain(m *testing.M) {
	// go test runs from the package directory; chdir to module root
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			log.Fatal("go.mod not found")
		}
		dir = parent
	}
	os.Chdir(dir)
	os.Exit(m.Run())
}

const testDataFile = "test_todo.json"

func setupTestServer(t *testing.T) (*httptest.Server, *TodoStore) {
	t.Helper()

	gadSrc, err := os.ReadFile("cmd/todo/templates/main.gad")
	if err != nil {
		t.Fatalf("read main.gad: %v", err)
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
		t.Fatalf("compile: %v", err)
	}

	type compiled struct {
		bc       *gad.Bytecode
		builtins *gad.StaticBuiltins
	}
	comp := &compiled{bc: bc, builtins: builtins}

	os.Remove(testDataFile)
	store := NewTodoStore(testDataFile)

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("cmd/todo/static"))))

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
		_, err := vm.RunOpts(&gad.RunOpts{
			StdOut: &buf,
			Globals: gad.Dict{
				"Todos":   mustObj(gad.ToObject(filtered)),
				"FilterV": mustObj(gad.ToObject(filter)),
			},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.Copy(w, &buf)
	})

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

	mux.HandleFunc("/toggle", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.URL.Query().Get("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		store.Toggle(id)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	mux.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.URL.Query().Get("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		store.Delete(id)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	mux.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		store.ClearCompleted()
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	ts := httptest.NewServer(mux)
	return ts, store
}

func TestHomeEmpty(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "No todos") {
		t.Error("expected empty state message")
	}
	if !strings.Contains(string(body), "Todo App") {
		t.Error("expected title")
	}
}

func TestAddTodo(t *testing.T) {
	ts, store := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	// Add a todo
	res, err := (&http.Client{CheckRedirect: noRedirect}).Post(ts.URL+"/add", "application/x-www-form-urlencoded",
		strings.NewReader("title=Test+Todo"))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", res.StatusCode)
	}

	// Verify it appears in the list
	res, err = http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "Test Todo") {
		t.Errorf("expected todo in list, got:\n%s", body)
	}

	// Verify store has the todo
	todos := store.GetAll()
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}
	if todos[0].Title != "Test Todo" {
		t.Errorf("expected 'Test Todo', got %q", todos[0].Title)
	}
	if todos[0].Done {
		t.Error("expected new todo to be not done")
	}
}

func TestToggleTodo(t *testing.T) {
	ts, store := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	store.Add("Item A")
	store.Add("Item B")

	// Toggle Item A (id=1)
	res, err := (&http.Client{CheckRedirect: noRedirect}).Get(ts.URL + "/toggle?id=1")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", res.StatusCode)
	}

	todos := store.GetAll()
	if len(todos) != 2 {
		t.Fatalf("expected 2 todos, got %d", len(todos))
	}
	if !todos[0].Done {
		t.Error("expected Item A to be done after toggle")
	}
	if todos[1].Done {
		t.Error("expected Item B to remain not done")
	}

	// Toggle again — back to undone
	(&http.Client{CheckRedirect: noRedirect}).Get(ts.URL + "/toggle?id=1")
	todos = store.GetAll()
	if todos[0].Done {
		t.Error("expected Item A to be undone after second toggle")
	}
}

func TestDeleteTodo(t *testing.T) {
	ts, store := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	store.Add("Delete Me")

	res, err := (&http.Client{CheckRedirect: noRedirect}).Get(ts.URL + "/delete?id=1")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", res.StatusCode)
	}

	if len(store.GetAll()) != 0 {
		t.Error("expected no todos after delete")
	}
}

func TestClearCompleted(t *testing.T) {
	ts, store := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	store.Add("Task A")
	store.Add("Task B")
	store.Add("Task C")

	store.Toggle(1) // Task A done
	store.Toggle(3) // Task C done

	res, err := (&http.Client{CheckRedirect: noRedirect}).Post(ts.URL+"/clear", "application/x-www-form-urlencoded",
		strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", res.StatusCode)
	}

	todos := store.GetAll()
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo after clear, got %d", len(todos))
	}
	if todos[0].Title != "Task B" {
		t.Errorf("expected 'Task B', got %q", todos[0].Title)
	}
}

func TestFilterAll(t *testing.T) {
	ts, store := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	store.Add("Active Item")
	store.Add("Done Item")
	store.Toggle(2)

	res, err := http.Get(ts.URL + "/?filter=all")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "Active Item") {
		t.Error("expected 'Active Item' in all filter")
	}
	if !strings.Contains(string(body), "Done Item") {
		t.Error("expected 'Done Item' in all filter")
	}
}

func TestFilterActive(t *testing.T) {
	ts, store := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	store.Add("Active Item")
	store.Add("Done Item")
	store.Toggle(2)

	res, err := http.Get(ts.URL + "/?filter=active")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "Active Item") {
		t.Error("expected 'Active Item' in active filter")
	}
	if strings.Contains(string(body), "Done Item") {
		t.Error("expected 'Done Item' NOT in active filter")
	}
}

func TestFilterCompleted(t *testing.T) {
	ts, store := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	store.Add("Active Item")
	store.Add("Done Item")
	store.Toggle(2)

	res, err := http.Get(ts.URL + "/?filter=completed")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if strings.Contains(string(body), "Active Item") {
		t.Error("expected 'Active Item' NOT in completed filter")
	}
	if !strings.Contains(string(body), "Done Item") {
		t.Error("expected 'Done Item' in completed filter")
	}
}

func TestJSONPersistence(t *testing.T) {
	ts, store := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	store.Add("Persist Me")

	// Read the JSON file
	data, err := os.ReadFile(testDataFile)
	if err != nil {
		t.Fatal(err)
	}

	var d todoData
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatal(err)
	}

	if len(d.Todos) != 1 {
		t.Fatalf("expected 1 todo in JSON, got %d", len(d.Todos))
	}
	if d.Todos[0].Title != "Persist Me" {
		t.Errorf("expected 'Persist Me', got %q", d.Todos[0].Title)
	}
	if d.Todos[0].ID != 1 {
		t.Errorf("expected ID 1, got %d", d.Todos[0].ID)
	}
}

func TestHomeShowsTodos(t *testing.T) {
	ts, store := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	store.Add("Buy milk")
	store.Add("Walk dog")

	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	if !strings.Contains(string(body), "Buy milk") {
		t.Error("expected 'Buy milk' in response")
	}
	if !strings.Contains(string(body), "Walk dog") {
		t.Error("expected 'Walk dog' in response")
	}
	if !strings.Contains(string(body), "0 active of") {
		t.Errorf("expected count '0 active of', got:\n%s", body)
	}
}

func TestAddEmptyTitle(t *testing.T) {
	ts, store := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	res, err := (&http.Client{CheckRedirect: noRedirect}).Post(ts.URL+"/add", "application/x-www-form-urlencoded",
		strings.NewReader("title="))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	todos := store.GetAll()
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}
	if todos[0].Title != "Untitled" {
		t.Errorf("expected 'Untitled', got %q", todos[0].Title)
	}
}

func TestInvalidToggle(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	res, err := ts.Client().Get(ts.URL + "/toggle?id=abc")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid id, got %d", res.StatusCode)
	}
}

func TestInvalidDelete(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	res, err := ts.Client().Get(ts.URL + "/delete?id=abc")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid id, got %d", res.StatusCode)
	}
}

func TestAddMethodNotAllowed(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	res, err := ts.Client().Get(ts.URL + "/add")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", res.StatusCode)
	}
}

func TestClearMethodNotAllowed(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()
	defer os.Remove(testDataFile)

	res, err := ts.Client().Get(ts.URL + "/clear")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", res.StatusCode)
	}
}
