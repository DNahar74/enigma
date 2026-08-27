// Package testutil provides test doubles (fakes) for Enigma's plugin interfaces.
// These fakes allow for hermetic testing of the pipeline and other components
// without hitting real external networks or services.
package testutil

import (
	"context"
	"time"

	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
)

// FakeSearch is a test double for plugin.SearchPlugin.
// It allows simulating search results, errors, and network latency.
type FakeSearch struct {
	NameStr string
	Results []query.Result
	Err     error
	Delay   time.Duration // Time to sleep before returning results (for concurrency testing)
	PingErr error
}

// Name returns the fake's name, defaulting to "fake-search".
func (f *FakeSearch) Name() string {
	if f.NameStr == "" {
		return "fake-search"
	}
	return f.NameStr
}

// Search simulates executing a search. It respects the Delay and ctx.Done().
func (f *FakeSearch) Search(ctx context.Context, q query.Query) (<-chan query.Result, error) {
	if f.Err != nil {
		return nil, f.Err
	}

	if f.Delay > 0 {
		select {
		case <-time.After(f.Delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	ch := make(chan query.Result)
	go func() {
		defer close(ch)
		for _, res := range f.Results {
			select {
			case ch <- res:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// Ping checks if the simulated plugin is healthy.
func (f *FakeSearch) Ping(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return f.PingErr
}

// FakeLocalSearch embeds FakeSearch and adds the IsLocal method.
type FakeLocalSearch struct {
	FakeSearch
}

// IsLocal identifies this fake as a local search plugin.
func (f *FakeLocalSearch) IsLocal() bool {
	return true
}

// FakeFilter is a test double for plugin.FilterPlugin.
// It allows simulating filtering logic or errors.
type FakeFilter struct {
	NameStr    string
	FilterFunc func([]query.Result) []query.Result // if nil, acts as passthrough
	Err        error
}

// Name returns the fake's name, defaulting to "fake-filter".
func (f *FakeFilter) Name() string {
	if f.NameStr == "" {
		return "fake-filter"
	}
	return f.NameStr
}

// Filter simulates filtering search results.
func (f *FakeFilter) Filter(ctx context.Context, results []query.Result) ([]query.Result, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if f.Err != nil {
		return nil, f.Err
	}

	if f.FilterFunc != nil {
		return f.FilterFunc(results), nil
	}
	return results, nil
}

// FakeRank is a test double for plugin.RankPlugin.
// It allows simulating relevance scoring or errors.
type FakeRank struct {
	NameStr  string
	RankFunc func(query.Query, []query.Result) []plugin.ScoredResult // if nil, assigns index-based scores
	Err      error
}

// Name returns the fake's name, defaulting to "fake-rank".
func (f *FakeRank) Name() string {
	if f.NameStr == "" {
		return "fake-rank"
	}
	return f.NameStr
}

// Rank simulates ranking search results.
func (f *FakeRank) Rank(ctx context.Context, q query.Query, results []query.Result) ([]plugin.ScoredResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if f.Err != nil {
		return nil, f.Err
	}

	if f.RankFunc != nil {
		return f.RankFunc(q, results), nil
	}

	scored := make([]plugin.ScoredResult, len(results))
	for i, res := range results {
		scored[i] = plugin.ScoredResult{
			Result: res,
			Score:  float64(len(results) - i),
		}
	}
	return scored, nil
}
