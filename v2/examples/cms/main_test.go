package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAppSetup(t *testing.T) {
	app := newTestApp(t)
	if app.DB == nil {
		t.Fatal("expected DB to be set")
	}
	if app.PublicDir == "" {
		t.Fatal("expected PublicDir to be set")
	}
	mux := http.NewServeMux()
	app.routes(mux)
}

func TestIndexRoute(t *testing.T) {
	_, mux := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assertOK(t, rec)
	body := rec.Body.String()
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("expected DOCTYPE html")
	}
	if !strings.Contains(body, "GION CMS") {
		t.Error("expected site title in response")
	}
}

func TestPageRoutes(t *testing.T) {
	for _, title := range []string{"About", "Guides", "Contact"} {
		t.Run(title, func(t *testing.T) {
			_, mux := newTestServer(t)
			slug := strings.ToLower(title)
			req := httptest.NewRequest(http.MethodGet, "/pages/"+slug, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			assertOK(t, rec)
			if !strings.Contains(rec.Body.String(), title) {
				t.Errorf("expected body to contain %q", title)
			}
		})
	}
}

func TestPostRoutes(t *testing.T) {
	posts := []struct{ title, slug string }{
		{"Designing fast editorial pages", "designing-fast-editorial-pages"},
		{"SQLite is enough for a compact CMS", "sqlite-compact-cms"},
		{"Modern admin interfaces without ceremony", "modern-admin-interfaces"},
		{"Reusable GION components", "reusable-gion-components"},
		{"Shipping a friendly first page", "shipping-friendly-first-page"},
		{"Building a gallery component", "building-gallery-component"},
	}
	for _, p := range posts {
		t.Run(p.slug, func(t *testing.T) {
			_, mux := newTestServer(t)
			req := httptest.NewRequest(http.MethodGet, "/posts/"+p.slug, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			assertOK(t, rec)
			if !strings.Contains(rec.Body.String(), p.title) {
				t.Errorf("expected body to contain %q", p.title)
			}
		})
	}
}

func TestTagRoutes(t *testing.T) {
	_, mux := newTestServer(t)
	for _, name := range []string{"Design", "Engineering", "News", "Tutorials"} {
		t.Run(name, func(t *testing.T) {
			slug := strings.ToLower(name)
			req := httptest.NewRequest(http.MethodGet, "/tags/"+slug, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			assertOK(t, rec)
			if !strings.Contains(rec.Body.String(), "Posts tagged "+name) {
				t.Errorf("expected body to contain %q", "Posts tagged "+name)
			}
		})
	}
}

func TestTagPagination(t *testing.T) {
	_, mux := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/tags/design?page=1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assertOK(t, rec)
}

func TestAdminRoute(t *testing.T) {
	_, mux := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assertOK(t, rec)
}

func TestAPIDashboard(t *testing.T) {
	_, mux := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assertOK(t, rec)
	var result map[string]float64
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	for _, key := range []string{"pages", "posts", "tags", "menus"} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected %q in dashboard response", key)
		}
	}
}

func TestAPICollections(t *testing.T) {
	_, mux := newTestServer(t)

	t.Run("GET /api/pages", func(t *testing.T) {
		var pages []Page
		getJSON(t, mux, "/api/pages", &pages)
		if len(pages) != 3 {
			t.Errorf("expected 3 pages, got %d", len(pages))
		}
	})

	t.Run("GET /api/tags", func(t *testing.T) {
		var tags []Tag
		getJSON(t, mux, "/api/tags", &tags)
		if len(tags) != 4 {
			t.Errorf("expected 4 tags, got %d", len(tags))
		}
	})

	t.Run("GET /api/menus", func(t *testing.T) {
		var menus []MenuItem
		getJSON(t, mux, "/api/menus", &menus)
		if len(menus) != 7 {
			t.Errorf("expected 7 menu items, got %d", len(menus))
		}
	})

	t.Run("GET /api/posts", func(t *testing.T) {
		var posts []Post
		getJSON(t, mux, "/api/posts", &posts)
		if len(posts) != 6 {
			t.Errorf("expected 6 posts, got %d", len(posts))
		}
	})
}

func TestSeedDataIntegrity(t *testing.T) {
	_, mux := newTestServer(t)
	var result map[string]float64
	getJSON(t, mux, "/api/dashboard", &result)
	if result["pages"] != 3 {
		t.Errorf("expected 3 pages, got %v", result["pages"])
	}
	if result["tags"] != 4 {
		t.Errorf("expected 4 tags, got %v", result["tags"])
	}
	if result["posts"] != 6 {
		t.Errorf("expected 6 posts, got %v", result["posts"])
	}
	if result["menus"] != 7 {
		t.Errorf("expected 7 menus, got %v", result["menus"])
	}
}

func TestConsecutiveNavigationDoesNotError(t *testing.T) {
	_, mux := newTestServer(t)
	routes := []string{
		"/",
		"/pages/about",
		"/pages/guides",
		"/pages/contact",
		"/posts/designing-fast-editorial-pages",
		"/posts/sqlite-compact-cms",
		"/posts/modern-admin-interfaces",
		"/posts/reusable-gion-components",
		"/posts/shipping-friendly-first-page",
		"/posts/building-gallery-component",
		"/tags/design",
		"/tags/engineering",
		"/tags/news",
		"/tags/tutorials",
		"/tags/design?page=1",
		"/admin",
		"/api/dashboard",
	}
	for i, route := range routes {
		t.Run(fmt.Sprintf("route_%d", i), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: expected 200, got %d: %s", route, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMenuNavigation(t *testing.T) {
	_, mux := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assertOK(t, rec)
	body := rec.Body.String()

	links := []struct{ label, href string }{
		{"Home", "/"},
		{"About", "/pages/about"},
		{"Guides", "/pages/guides"},
		{"Design", "/tags/design"},
		{"Engineering", "/tags/engineering"},
		{"Tutorials", "/tags/tutorials"},
	}
	for _, link := range links {
		if !strings.Contains(body, link.href) {
			t.Errorf("expected menu link %q (href=%q) not found in home page", link.label, link.href)
		}
	}
}

func TestNonExistentRoutes(t *testing.T) {
	_, mux := newTestServer(t)
	for _, route := range []string{"/nonexistent", "/pages/nonexistent", "/posts/nonexistent", "/tags/nonexistent"} {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Errorf("expected 404, got %d", rec.Code)
			}
		})
	}
}

func TestStaticFileServing(t *testing.T) {
	_, mux := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/seed-data/images/about-hero.jpg", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assertOK(t, rec)
}

func TestTranspiledFilesCleanup(t *testing.T) {
	root := t.TempDir()
	publicDir := filepath.Join(root, "public")
	os.MkdirAll(publicDir, 0755)
	transpiledDir := filepath.Join(publicDir, ".transpiled")
	os.MkdirAll(transpiledDir, 0755)

	stale := filepath.Join(transpiledDir, "stale.gad")
	os.WriteFile(stale, []byte("stale"), 0644)

	db, _ := gorm.Open(sqlite.Open(filepath.Join(root, "cms.db")), &gorm.Config{})
	app := &App{
		DB:           db,
		Root:         root,
		PublicDir:    publicDir,
		TranspileDir: transpiledDir,
	}
	db.AutoMigrate(&Page{}, &Tag{}, &Post{}, &MenuItem{})
	app.cleanTranspiled()
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("expected stale transpiled file to be removed")
	}
}

func TestResponseContainsNoPanic(t *testing.T) {
	_, mux := newTestServer(t)
	for _, route := range []string{"/", "/pages/about", "/posts/designing-fast-editorial-pages", "/tags/design"} {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				return
			}
			body := rec.Body.String()
			for _, sig := range []string{"panic", "runtime error", "nil pointer"} {
				if strings.Contains(body, sig) {
					t.Errorf("response contains error signal %q", sig)
				}
			}
		})
	}
}

func TestCRUDCreatePage(t *testing.T) {
	_, mux := newTestServer(t)
	payload := `{"title":"Test Page","slug":"test-page","summary":"A test page","body":"<p>hello</p>","published":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/pages", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assertOK(t, rec)
	var page Page
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if page.Title != "Test Page" {
		t.Errorf("expected title 'Test Page', got %q", page.Title)
	}
}

func TestCRUDUpdateAndDeletePage(t *testing.T) {
	_, mux := newTestServer(t)

	var pages []Page
	getJSON(t, mux, "/api/pages", &pages)
	if len(pages) == 0 {
		t.Fatal("expected some pages")
	}
	id := pages[0].ID

	payload := fmt.Sprintf(`{"title":"Updated %s","slug":"%s"}`, pages[0].Title, pages[0].Slug)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/pages/%d", id), strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assertOK(t, rec)

	var updated []Page
	getJSON(t, mux, "/api/pages", &updated)

	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/pages/%d", id), nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	var afterDelete []Page
	getJSON(t, mux, "/api/pages", &afterDelete)
	if len(afterDelete) != len(updated)-1 {
		t.Errorf("expected %d pages after delete, got %d", len(updated)-1, len(afterDelete))
	}
}

func TestPostsByTagAssociation(t *testing.T) {
	_, mux := newTestServer(t)
	var posts []Post
	getJSON(t, mux, "/api/posts", &posts)

	tagPostCounts := map[string]int{
		"design":      3,
		"engineering": 3,
		"news":        2,
		"tutorials":   2,
	}
	for _, post := range posts {
		for _, tag := range post.Tags {
			tagPostCounts[tag.Slug]--
		}
	}
	for slug, remaining := range tagPostCounts {
		if remaining != 0 {
			t.Errorf("tag %q has %d remaining (expected 0)", slug, remaining)
		}
	}
}

func TestHTTPMethods(t *testing.T) {
	_, mux := newTestServer(t)
	tests := []struct {
		method string
		route  string
		code   int
	}{
		{http.MethodHead, "/", http.StatusOK},
		{http.MethodPost, "/api/dashboard", http.StatusMethodNotAllowed},
		{http.MethodPut, "/api/pages", http.StatusMethodNotAllowed},
		{http.MethodDelete, "/api/pages", http.StatusMethodNotAllowed},
	}
	for _, tc := range tests {
		t.Run(tc.method+"_"+tc.route, func(t *testing.T) {
			var reqBody io.Reader
			if tc.method == http.MethodPost || tc.method == http.MethodPut {
				reqBody = strings.NewReader(`{}`)
			}
			req := httptest.NewRequest(tc.method, tc.route, reqBody)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.code {
				t.Errorf("expected %d, got %d", tc.code, rec.Code)
			}
		})
	}
}

func TestEmptyDBDoesNotPanic(t *testing.T) {
	root := findRoot()
	db, _ := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "empty.db")), &gorm.Config{})
	emptyApp := &App{
		DB:           db,
		Root:         root,
		PublicDir:    filepath.Join(root, "public"),
		TranspileDir: filepath.Join(root, "public", ".transpiled"),
	}
	db.AutoMigrate(&Page{}, &Tag{}, &Post{}, &MenuItem{})
	emptyApp.cleanTranspiled()

	mux := http.NewServeMux()
	emptyApp.routes(mux)

	for _, route := range []string{"/", "/pages/about", "/posts/hello"} {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code == http.StatusInternalServerError {
				t.Fatalf("%s: internal server error: %s", route, rec.Body.String())
			}
		})
	}
}

// --- helpers ---

func newTestApp(t *testing.T) *App {
	t.Helper()
	root := findRoot()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cms_test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	app := &App{
		DB:           db,
		Root:         root,
		PublicDir:    filepath.Join(root, "public"),
		TranspileDir: filepath.Join(root, "public", ".transpiled"),
	}
	if err := db.AutoMigrate(&Page{}, &Tag{}, &Post{}, &MenuItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	app.cleanTranspiled()
	if err := app.seed(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return app
}

func newTestServer(t *testing.T) (*App, *http.ServeMux) {
	t.Helper()
	app := newTestApp(t)
	mux := http.NewServeMux()
	app.routes(mux)
	return app, mux
}

func findRoot() string {
	candidates := []string{".", "..", "../.."}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "seed.yaml")); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	wd, _ := os.Getwd()
	if _, err := os.Stat(filepath.Join(wd, "seed.yaml")); err == nil {
		return wd
	}
	return "."
}

func getJSON(t *testing.T, mux *http.ServeMux, route string, v any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, route, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: expected 200, got %d: %s", route, rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("%s: json decode: %v", route, err)
	}
}

func assertOK(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
