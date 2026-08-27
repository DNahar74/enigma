package search_tavily

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DNahar74/enigma/core/query"
)

func TestTavilySearch(t *testing.T) {
	// Create a mock Tavily server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("expected path /search, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		var req tavilyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if req.APIKey != "test-api-key" {
			t.Errorf("expected api_key 'test-api-key', got '%s'", req.APIKey)
		}
		if req.Query != "test query" {
			t.Errorf("expected query 'test query', got '%s'", req.Query)
		}
		if req.MaxResults != 5 {
			t.Errorf("expected max_results 5, got %d", req.MaxResults)
		}
		if req.SearchDepth != "basic" {
			t.Errorf("expected search_depth 'basic', got '%s'", req.SearchDepth)
		}

		resp := tavilyResponse{
			Results: []tavilyResult{
				{
					Title:   "Result 1",
					URL:     "https://example.com/1",
					Content: "Content 1",
				},
				{
					Title:   "Result 2",
					URL:     "https://example.com/2",
					Content: "Content 2",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	plugin := New("test-api-key", 5, 10)
	plugin.baseURL = server.URL // override base URL for testing

	ctx := context.Background()
	q := query.Query{Raw: "test query"}

	out, err := plugin.Search(ctx, q)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	var results []query.Result
	for res := range out {
		results = append(results, res)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Title != "Result 1" {
		t.Errorf("expected title 'Result 1', got '%s'", results[0].Title)
	}
	if results[1].URL != "https://example.com/2" {
		t.Errorf("expected url 'https://example.com/2', got '%s'", results[1].URL)
	}
	if results[0].Snippet != "Content 1" {
		t.Errorf("expected snippet 'Content 1', got '%s'", results[0].Snippet)
	}
	if results[0].SourcePlugin != "tavily" {
		t.Errorf("expected source_plugin 'tavily', got '%s'", results[0].SourcePlugin)
	}
}

func TestTavilySearch_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	plugin := New("bad-key", 5, 10)
	plugin.baseURL = server.URL

	ctx := context.Background()
	q := query.Query{Raw: "test query"}

	_, err := plugin.Search(ctx, q)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTavilySearch_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wait longer than the context deadline
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	plugin := New("test-api-key", 5, 10)
	plugin.baseURL = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	q := query.Query{Raw: "test query"}
	_, err := plugin.Search(ctx, q)
	if err == nil {
		t.Fatal("expected error due to context timeout, got nil")
	}
}

func TestTavilySearch_Ping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := tavilyResponse{Results: []tavilyResult{}}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	plugin := New("test-api-key", 5, 10)
	plugin.baseURL = server.URL

	ctx := context.Background()
	err := plugin.Ping(ctx)
	if err != nil {
		t.Fatalf("expected ping to succeed, got %v", err)
	}
}
