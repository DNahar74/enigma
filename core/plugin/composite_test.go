package plugin_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
)

type mockFilter struct {
	name   string
	err    error
	called bool
}

func (m *mockFilter) Name() string { return m.name }
func (m *mockFilter) Filter(ctx context.Context, results []query.Result) ([]query.Result, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	// append a marker result
	return append(results, query.Result{Title: m.name}), nil
}

func TestCompositeFilter(t *testing.T) {
	f1 := &mockFilter{name: "f1"}
	f2 := &mockFilter{name: "f2"}
	f3 := &mockFilter{name: "f3", err: errors.New("f3 failed")}

	cf := plugin.NewCompositeFilter(f1, f2)
	results, err := cf.Filter(context.Background(), []query.Result{{Title: "initial"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 || results[1].Title != "f1" || results[2].Title != "f2" {
		t.Errorf("expected filters to run in sequence, got %v", results)
	}

	cfErr := plugin.NewCompositeFilter(f1, f3, f2)
	f1.called = false
	f2.called = false
	_, err = cfErr.Filter(context.Background(), nil)
	if err == nil || err.Error() != "f3 failed" {
		t.Errorf("expected error 'f3 failed', got %v", err)
	}
	if !f1.called || f2.called {
		t.Errorf("expected f1 to be called and f2 not to be called, got f1=%v, f2=%v", f1.called, f2.called)
	}
}

type mockRanker struct {
	name   string
	err    error
	called bool
	scores []float64
}

func (m *mockRanker) Name() string { return m.name }
func (m *mockRanker) Rank(ctx context.Context, q query.Query, results []query.Result) ([]plugin.ScoredResult, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	var scored []plugin.ScoredResult
	for i, r := range results {
		score := 1.0
		if i < len(m.scores) {
			score = m.scores[i]
		}
		scored = append(scored, plugin.ScoredResult{
			Result:    r,
			Score:     score,
			Breakdown: map[string]float64{m.name: score},
		})
	}
	return scored, nil
}

func TestCompositeRanker(t *testing.T) {
	r1 := &mockRanker{name: "r1", scores: []float64{1.0, 2.0}}
	r2 := &mockRanker{name: "r2", scores: []float64{10.0, 20.0}}
	r3 := &mockRanker{name: "r3", err: errors.New("r3 failed")}

	cr := plugin.NewCompositeRanker(r1, r2)
	results := []query.Result{{Title: "res1"}, {Title: "res2"}}
	scored, err := cr.Rank(context.Background(), query.Query{}, results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(scored) != 2 {
		t.Fatalf("expected 2 results, got %d", len(scored))
	}
	if scored[0].Score != 11.0 { // 1.0 + 10.0
		t.Errorf("expected score 11.0 for res1, got %v", scored[0].Score)
	}
	if scored[0].Breakdown["r1"] != 1.0 || scored[0].Breakdown["r2"] != 10.0 {
		t.Errorf("expected breakdown for res1 to have r1: 1.0, r2: 10.0, got %v", scored[0].Breakdown)
	}
	if scored[1].Score != 22.0 { // 2.0 + 20.0
		t.Errorf("expected score 22.0 for res2, got %v", scored[1].Score)
	}

	crErr := plugin.NewCompositeRanker(r1, r3, r2)
	r1.called = false
	r2.called = false
	_, err = crErr.Rank(context.Background(), query.Query{}, results)
	if err == nil || err.Error() != "r3 failed" {
		t.Errorf("expected error 'r3 failed', got %v", err)
	}
	if !r1.called || r2.called {
		t.Errorf("expected r1 to be called and r2 not to be called, got r1=%v, r2=%v", r1.called, r2.called)
	}

	crEmpty := plugin.NewCompositeRanker()
	scoredEmpty, err := crEmpty.Rank(context.Background(), query.Query{}, results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scoredEmpty) != 2 || scoredEmpty[0].Score != 0 || scoredEmpty[1].Score != 0 {
		t.Errorf("expected zero scores for empty composite ranker, got %v", scoredEmpty)
	}
}
