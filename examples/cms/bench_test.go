package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkRoutes(b *testing.B) {
	_, mux := newTestServer(b)
	templates := []string{
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
	}
	for _, route := range templates {
		b.Run(route, func(b *testing.B) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req)
			}
		})
	}
}

func BenchmarkSequentialNavigation(b *testing.B) {
	_, mux := newTestServer(b)
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
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, route := range routes {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
		}
	}
}

func BenchmarkColdRender(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app := newTestApp(b)
		mux := http.NewServeMux()
		app.routes(mux)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
	}
}

func BenchmarkMixedWorkload(b *testing.B) {
	_, mux := newTestServer(b)
	endpoints := []string{
		"/",
		"/pages/about",
		"/posts/designing-fast-editorial-pages",
		"/tags/design",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, route := range endpoints {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("%s: expected 200, got %d", route, rec.Code)
			}
		}
	}
}

func BenchmarkPostWithRelatedContent(b *testing.B) {
	_, mux := newTestServer(b)
	slugs := []string{
		"/posts/designing-fast-editorial-pages",
		"/posts/sqlite-compact-cms",
		"/posts/modern-admin-interfaces",
		"/posts/reusable-gion-components",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, slug := range slugs {
			req := httptest.NewRequest(http.MethodGet, slug, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("%s: expected 200, got %d", slug, rec.Code)
			}
		}
	}
}

func BenchmarkTagWithPagination(b *testing.B) {
	_, mux := newTestServer(b)
	pages := []string{
		"/tags/design?page=1",
		"/tags/engineering?page=1",
		"/tags/news?page=1",
		"/tags/tutorials?page=1",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, route := range pages {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				b.Fatalf("%s: expected 200, got %d", route, rec.Code)
			}
		}
	}
}
