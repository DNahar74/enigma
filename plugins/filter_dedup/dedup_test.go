package filter_dedup

import (
	"context"
	"testing"

	"github.com/DNahar74/enigma/core/query"
)

func TestDedupFilter(t *testing.T) {
	filter := New()

	results := []query.Result{
		{
			Title:   "Enigma Project Overview",
			URL:     "https://example.com/enigma",
			Snippet: "This is a CLI search tool written in Go.",
		},
		{
			// Exact URL duplicate
			Title:   "Enigma Tool",
			URL:     "https://example.com/enigma",
			Snippet: "Different snippet",
		},
		{
			// Different URL, but near-duplicate text
			Title:   "Enigma Project Overview",
			URL:     "https://example.com/enigma-mirror",
			Snippet: "This is a CLI search tool written in Go.",
		},
		{
			// Different text entirely
			Title:   "Learn Go",
			URL:     "https://golang.org",
			Snippet: "The Go programming language is an open source project to make programmers more productive.",
		},
	}

	filtered, err := filter.Filter(context.Background(), results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(filtered) != 2 {
		t.Fatalf("expected 2 results, got %d", len(filtered))
	}

	if filtered[0].Title != "Enigma Project Overview" {
		t.Errorf("expected first result to be Enigma Project Overview")
	}

	if filtered[1].Title != "Learn Go" {
		t.Errorf("expected second result to be Learn Go")
	}
}
