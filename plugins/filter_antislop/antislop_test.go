package filter_antislop

import (
	"context"
	"testing"

	"github.com/DNahar74/enigma/core/query"
)

func TestAntiSlopFilter_Filter(t *testing.T) {
	f := New()

	tests := []struct {
		name     string
		result   query.Result
		wantKept bool // true if the result should survive filtering
	}{
		{
			name: "0 slop phrases passes through",
			result: query.Result{
				Title:        "Go Concurrency Patterns",
				Snippet:      "Learn how goroutines and channels work in Go.",
				SourcePlugin: "tavily",
			},
			wantKept: true,
		},
		{
			name: "1 slop phrase passes through (threshold is 2)",
			result: query.Result{
				Title:        "Understanding Go",
				Snippet:      "In conclusion, Go is a great language for systems programming.",
				SourcePlugin: "tavily",
			},
			wantKept: true,
		},
		{
			name: "2+ slop phrases is dropped",
			result: query.Result{
				Title:        "Delve into Go Programming",
				Snippet:      "In conclusion, Go is powerful. It is important to note that goroutines are lightweight.",
				SourcePlugin: "tavily",
			},
			wantKept: false,
		},
		{
			name: "local result always passes even with slop phrases",
			result: query.Result{
				Title:        "Delve into My Notes",
				Snippet:      "In conclusion, this is important. It is important to note that I wrote this.",
				SourcePlugin: "local",
			},
			wantKept: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []query.Result{tt.result}

			filtered, err := f.Filter(context.Background(), input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			kept := len(filtered) == 1
			if kept != tt.wantKept {
				t.Errorf("result kept = %v, want %v (got %d results)", kept, tt.wantKept, len(filtered))
			}
		})
	}
}
