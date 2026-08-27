package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DNahar74/enigma/core/config"
	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
	"github.com/DNahar74/enigma/internal/testutil"
)

func TestPipeline_Execute(t *testing.T) {
	q, _ := query.Parse("test query")
	cfg := &config.Config{}

	t.Run("Success Flow", func(t *testing.T) {
		fakeSearch1 := &testutil.FakeSearch{
			NameStr: "search-1",
			Results: []query.Result{
				{URL: "https://example.com/1"},
				{URL: "https://example.com/2"},
			},
		}
		fakeSearch2 := &testutil.FakeSearch{
			NameStr: "search-2",
			Results: []query.Result{
				{URL: "https://example.com/3"},
			},
		}

		fakeFilter := &testutil.FakeFilter{
			FilterFunc: func(results []query.Result) []query.Result {
				var filtered []query.Result
				for _, r := range results {
					if r.URL != "https://example.com/2" {
						filtered = append(filtered, r)
					}
				}
				return filtered
			},
		}

		fakeRank := &testutil.FakeRank{
			RankFunc: func(_ query.Query, results []query.Result) []plugin.ScoredResult {
				var scored []plugin.ScoredResult
				for _, r := range results {
					score := 1.0
					if r.URL == "https://example.com/3" {
						score = 5.0
					}
					scored = append(scored, plugin.ScoredResult{Result: r, Score: score})
				}
				return scored
			},
		}

		reg, _ := plugin.NewRegistry([]plugin.SearchPlugin{fakeSearch1, fakeSearch2}, fakeFilter, fakeRank)
		p := New(reg, cfg)
		ctx := context.Background()

		scored, err := p.Execute(ctx, q)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scored) != 2 {
			t.Fatalf("expected 2 results, got %d", len(scored))
		}

		if scored[0].Score != 5.0 || scored[0].Result.URL != "https://example.com/3" {
			t.Errorf("expected highest scored result first, got score %f for %s", scored[0].Score, scored[0].Result.URL)
		}
		if scored[1].Score != 1.0 || scored[1].Result.URL != "https://example.com/1" {
			t.Errorf("expected lower scored result second, got score %f for %s", scored[1].Score, scored[1].Result.URL)
		}
	})

	// To properly test query expansion without complicated fakes, let's create a custom SearchPlugin for remote.
	t.Run("Query Expansion Custom Fake", func(t *testing.T) {
		fakeLocal := &testutil.FakeLocalSearch{
			FakeSearch: testutil.FakeSearch{
				NameStr: "local",
				Results: []query.Result{
					{Title: "Golang testing", Snippet: "Useful testing frameworks for Golang."},
				},
			},
		}

		var expandedQuery query.Query
		fakeFilter := &testutil.FakeFilter{}
		fakeRank := &testutil.FakeRank{}

		reg, _ := plugin.NewRegistry([]plugin.SearchPlugin{fakeLocal, &remoteQueryCapturer{captured: &expandedQuery}}, fakeFilter, fakeRank)
		p := New(reg, cfg)

		_, _ = p.Execute(context.Background(), q)

		// Top keywords from "Useful testing frameworks for Golang" ignoring query "test query"
		// "golang" and "frameworks" should be extracted.
		if expandedQuery.Raw == "" {
			t.Fatalf("expected remote plugin to be called with expanded query")
		}
		if !strings.Contains(expandedQuery.Raw, "golang") {
			t.Errorf("expected expanded query to contain 'golang', got %q", expandedQuery.Raw)
		}
	})

	t.Run("Search Error", func(t *testing.T) {
		expectedErr := errors.New("search failed")
		fakeSearch := &testutil.FakeSearch{Err: expectedErr}
		fakeFilter := &testutil.FakeFilter{}
		fakeRank := &testutil.FakeRank{}

		reg, _ := plugin.NewRegistry([]plugin.SearchPlugin{fakeSearch}, fakeFilter, fakeRank)
		p := New(reg, cfg)

		_, err := p.Execute(context.Background(), q)
		if err == nil || err.Error() != "search plugin \"fake-search\" failed: search failed" {
			t.Errorf("expected search failed error, got %v", err)
		}
	})

	t.Run("Filter Error", func(t *testing.T) {
		expectedErr := errors.New("filter failed")
		fakeSearch := &testutil.FakeSearch{
			Results: []query.Result{{URL: "https://example.com"}},
		}
		fakeFilter := &testutil.FakeFilter{Err: expectedErr}
		fakeRank := &testutil.FakeRank{}

		reg, _ := plugin.NewRegistry([]plugin.SearchPlugin{fakeSearch}, fakeFilter, fakeRank)
		p := New(reg, cfg)

		_, err := p.Execute(context.Background(), q)
		if err == nil || err.Error() != "filter failed: filter failed" {
			t.Errorf("expected filter failed error, got %v", err)
		}
	})

	t.Run("Rank Error", func(t *testing.T) {
		expectedErr := errors.New("rank failed")
		fakeSearch := &testutil.FakeSearch{
			Results: []query.Result{{URL: "https://example.com"}},
		}
		fakeFilter := &testutil.FakeFilter{}
		fakeRank := &testutil.FakeRank{Err: expectedErr}

		reg, _ := plugin.NewRegistry([]plugin.SearchPlugin{fakeSearch}, fakeFilter, fakeRank)
		p := New(reg, cfg)

		_, err := p.Execute(context.Background(), q)
		if err == nil || err.Error() != "rank failed: rank failed" {
			t.Errorf("expected rank failed error, got %v", err)
		}
	})

	t.Run("Context Cancellation During Drain", func(t *testing.T) {
		fakeSearch := &testutil.FakeSearch{
			Delay: 100 * time.Millisecond,
			Results: []query.Result{
				{URL: "https://example.com/1"},
				{URL: "https://example.com/2"},
			},
		}
		fakeFilter := &testutil.FakeFilter{}
		fakeRank := &testutil.FakeRank{}

		reg, _ := plugin.NewRegistry([]plugin.SearchPlugin{fakeSearch}, fakeFilter, fakeRank)
		p := New(reg, cfg)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := p.Execute(ctx, q)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

type remoteQueryCapturer struct {
	captured *query.Query
}

func (c *remoteQueryCapturer) Name() string { return "remote" }
func (c *remoteQueryCapturer) Search(ctx context.Context, q query.Query) (<-chan query.Result, error) {
	*c.captured = q
	ch := make(chan query.Result)
	close(ch)
	return ch, nil
}
func (c *remoteQueryCapturer) Ping(ctx context.Context) error { return nil }
