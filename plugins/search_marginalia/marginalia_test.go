package search_marginalia

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DNahar74/enigma/core/query"
)

func TestMarginaliaSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("API-Key") != "my-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("query") != "hello" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"results": [
				{"title": "T1", "url": "https://example.com/1", "description": "Snippet 1"},
				{"title": "T2", "url": "https://example.com/2", "description": "Snippet 2"}
			]
		}`))
	}))
	defer server.Close()

	s := New("my-key", 10, 1)
	s.endpoint = server.URL

	q, _ := query.Parse("hello")
	ch, err := s.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	var results []query.Result
	for r := range ch {
		results = append(results, r)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}
	if results[0].Title != "T1" || results[0].URL != "https://example.com/1" || results[0].Snippet != "Snippet 1" {
		t.Errorf("Unexpected result 1: %+v", results[0])
	}
	if results[1].Title != "T2" || results[1].URL != "https://example.com/2" || results[1].Snippet != "Snippet 2" {
		t.Errorf("Unexpected result 2: %+v", results[1])
	}
}

func TestPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := New("my-key", 10, 1)
	s.endpoint = server.URL

	if err := s.Ping(context.Background()); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestSearchTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	s := New("my-key", 10, 1)
	s.endpoint = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	q, _ := query.Parse("hello")
	ch, err := s.Search(ctx, q)
	if err != nil {
		t.Fatalf("Search returned immediate error instead of handling in goroutine: %v", err)
	}

	// Should close quickly due to context cancellation
	count := 0
	for range ch {
		count++
	}
	if count != 0 {
		t.Errorf("Expected 0 results due to timeout, got %d", count)
	}
}
