package rank_trust

import (
	"context"
	"testing"

	"github.com/DNahar74/enigma/core/query"
)

func TestTrustRanker_Rank(t *testing.T) {
	// Set up a ranker with one boosted and one penalized domain.
	// These are the same defaults used in production wiring.
	ranker := New(
		[]string{"github.com", "stackoverflow.com"},
		[]string{"quora.com", "pinterest.com"},
	)

	q := query.Query{Raw: "test", Tokens: []string{"test"}}

	tests := []struct {
		name           string
		url            string
		wantScoreDelta float64
	}{
		{
			name:           "boosted domain gets +20",
			url:            "https://github.com/DNahar74/enigma",
			wantScoreDelta: 20.0,
		},
		{
			name:           "penalized domain gets -50",
			url:            "https://quora.com/What-is-Go",
			wantScoreDelta: -50.0,
		},
		{
			name:           "neutral domain gets 0",
			url:            "https://example.com/page",
			wantScoreDelta: 0.0,
		},
		{
			name:           "www prefix on boosted domain still gets boosted",
			url:            "https://www.github.com/DNahar74/enigma",
			wantScoreDelta: 20.0,
		},
		{
			name:           "empty URL does not panic and gets 0",
			url:            "",
			wantScoreDelta: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := []query.Result{
				{URL: tt.url, Title: "Test", Snippet: "test snippet"},
			}

			scored, err := ranker.Rank(context.Background(), q, results)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(scored) != 1 {
				t.Fatalf("expected 1 scored result, got %d", len(scored))
			}

			if scored[0].Score != tt.wantScoreDelta {
				t.Errorf("score delta = %f, want %f", scored[0].Score, tt.wantScoreDelta)
			}
		})
	}
}
