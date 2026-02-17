package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestRouterStatic(t *testing.T) {
	r := New()
	matched := false
	r.GET("/api/users", func(w http.ResponseWriter, req *http.Request, ps Params) {
		matched = true
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users", nil)
	r.ServeHTTP(w, req)

	if !matched {
		t.Error("Static route not matched")
	}
}

func TestRouterParam(t *testing.T) {
	r := New()
	var userID string
	r.GET("/api/users/:id", func(w http.ResponseWriter, req *http.Request, ps Params) {
		userID = ps.ByName("id")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/users/123", nil)
	r.ServeHTTP(w, req)

	if userID != "123" {
		t.Errorf("Param route failed, expected 123, got %s", userID)
	}
}

func TestRouterWildcard(t *testing.T) {
	r := New()
	var filepath string
	r.GET("/files/*filepath", func(w http.ResponseWriter, req *http.Request, ps Params) {
		filepath = ps.ByName("filepath")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/files/css/style.css", nil)
	r.ServeHTTP(w, req)

	if filepath != "/css/style.css" {
		t.Errorf("Wildcard route failed, expected /css/style.css, got %s", filepath)
	}
}

func TestRouterPriority(t *testing.T) {
	r := New()
	result := ""

	// Order shouldn't matter
	r.GET("/api/users/:id", func(w http.ResponseWriter, req *http.Request, ps Params) {
		result = "param"
	})
	r.GET("/api/users/me", func(w http.ResponseWriter, req *http.Request, ps Params) {
		result = "static"
	})
	r.GET("/api/*all", func(w http.ResponseWriter, req *http.Request, ps Params) {
		result = "wildcard"
	})

	// Test Static Priority
	req, _ := http.NewRequest("GET", "/api/users/me", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)
	if result != "static" {
		t.Errorf("Priority failed, expected static, got %s", result)
	}

	// Test Param Priority
	req, _ = http.NewRequest("GET", "/api/users/123", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)
	if result != "param" {
		t.Errorf("Priority failed, expected param, got %s", result)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	r := New()
	var wg sync.WaitGroup

	// Writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			path := fmt.Sprintf("/api/dynamic/%d", i)
			r.GET(path, func(w http.ResponseWriter, req *http.Request, ps Params) {})
		}
	}()

	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				// Try to hit potentially existing routes
				path := fmt.Sprintf("/api/dynamic/%d", j%100) // hit first 100 repeatedly
				req, _ := http.NewRequest("GET", path, nil)
				r.ServeHTTP(httptest.NewRecorder(), req)
			}
		}()
	}

	wg.Wait()
}

func BenchmarkRouterStatic(b *testing.B) {
	r := New()
	r.GET("/api/users", func(w http.ResponseWriter, req *http.Request, ps Params) {})

	req, _ := http.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterParam(b *testing.B) {
	r := New()
	r.GET("/api/users/:id", func(w http.ResponseWriter, req *http.Request, ps Params) {})

	req, _ := http.NewRequest("GET", "/api/users/123", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}
