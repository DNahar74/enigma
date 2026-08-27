package rank_personal

import (
	"context"
	"testing"

	"github.com/DNahar74/enigma/core/query"
)

func TestPersonalRank_ZeroBoost(t *testing.T) {
	// A boost of 0 should produce 0 contribution for all results without erroring
	ranker := New(0.0)

	q := query.Query{
		Raw:    "test query",
		Tokens: []string{"test", "query"},
	}

	results := []query.Result{
		{
			Title:        "Local note",
			Snippet:      "This is a test query note",
			SourcePlugin: "local",
		},
		{
			Title:        "Web page",
			Snippet:      "Also has test query",
			SourcePlugin: "tavily",
		},
	}

	scored, err := ranker.Rank(context.Background(), q, results)
	if err != nil {
		t.Fatalf("Rank() error: %v", err)
	}

	for _, s := range scored {
		if s.Score != 0.0 {
			t.Errorf("expected score 0 for %q due to boost=0, got %g", s.Result.Title, s.Score)
		}
	}
}

func TestPersonalRank_CoverageScore(t *testing.T) {
	ranker := New(5.0)

	q := query.Query{
		Raw:    "distributed consensus algorithm",
		Tokens: []string{"distributed", "consensus", "algorithm"},
	}

	results := []query.Result{
		{
			// Matches 3/3 tokens
			Title:        "Raft Note",
			Snippet:      "A distributed consensus algorithm for logs.",
			SourcePlugin: "local",
		},
		{
			// Matches 1/3 tokens
			Title:        "Other Note",
			Snippet:      "Some distributed systems stuff.",
			SourcePlugin: "local",
		},
		{
			// Matches 0/3 tokens
			Title:        "Empty Note",
			Snippet:      "Nothing relevant here.",
			SourcePlugin: "local",
		},
	}

	scored, err := ranker.Rank(context.Background(), q, results)
	if err != nil {
		t.Fatalf("Rank() error: %v", err)
	}

	// Score = coverage * boost * 10.0
	// 3/3 = 1.0 * 5.0 * 10.0 = 50.0
	if scored[0].Score != 50.0 {
		t.Errorf("Expected 3/3 match to score 50.0, got %g", scored[0].Score)
	}
	// 1/3 = 0.333 * 5.0 * 10.0 = 16.666...
	if scored[1].Score < 16.6 || scored[1].Score > 16.7 {
		t.Errorf("Expected 1/3 match to score ~16.66, got %g", scored[1].Score)
	}
	// 0/3 = 0.0
	if scored[2].Score != 0.0 {
		t.Errorf("Expected 0/3 match to score 0.0, got %g", scored[2].Score)
	}
}

func TestPersonalRank_VocabularyOverlap(t *testing.T) {
	ranker := New(2.0)

	q := query.Query{Tokens: []string{"test"}} // Query tokens don't matter for web overlap

	results := []query.Result{
		{
			// Populates local vocab with: "unique", "local", "vocabulary", "word"
			Title:        "My Note",
			Snippet:      "This has some unique local vocabulary word.",
			SourcePlugin: "local",
		},
		{
			// Overlaps: "unique", "local" -> overlap=2, score = 2 * 2.0 = 4.0
			Title:        "Web Page 1",
			Snippet:      "Unique web page with local content.",
			SourcePlugin: "tavily",
		},
		{
			// Overlaps: 0 -> score = 0
			Title:        "Web Page 2",
			Snippet:      "Generic internet stuff.",
			SourcePlugin: "tavily",
		},
	}

	scored, err := ranker.Rank(context.Background(), q, results)
	if err != nil {
		t.Fatalf("Rank() error: %v", err)
	}

	// Local result gets coverage score (not overlap). Since "test" is 1 token and not present, score=0.
	if scored[0].Score != 0.0 {
		t.Errorf("Expected local result to have score 0, got %g", scored[0].Score)
	}

	// Web result with 2 overlapping words
	if scored[1].Score != 4.0 {
		t.Errorf("Expected web result 1 to have score 4.0, got %g", scored[1].Score)
	}

	// Web result with 0 overlapping words
	if scored[2].Score != 0.0 {
		t.Errorf("Expected web result 2 to have score 0.0, got %g", scored[2].Score)
	}
}
