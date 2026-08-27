package plugin

import (
	"context"

	"github.com/DNahar74/enigma/core/query"
)

// CompositeFilter chains multiple FilterPlugins together sequentially.
// The output of one filter becomes the input to the next.
type CompositeFilter struct {
	Filters []FilterPlugin
}

// NewCompositeFilter creates a new CompositeFilter.
func NewCompositeFilter(filters ...FilterPlugin) *CompositeFilter {
	return &CompositeFilter{Filters: filters}
}

// Name returns the name of the composite filter.
func (c *CompositeFilter) Name() string { return "composite_filter" }

// Filter executes all underlying filters in sequence.
func (c *CompositeFilter) Filter(ctx context.Context, results []query.Result) ([]query.Result, error) {
	var err error
	for _, f := range c.Filters {
		results, err = f.Filter(ctx, results)
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// CompositeRanker executes multiple RankPlugins and sums their scores.
type CompositeRanker struct {
	Rankers []RankPlugin
}

// NewCompositeRanker creates a new CompositeRanker.
func NewCompositeRanker(rankers ...RankPlugin) *CompositeRanker {
	return &CompositeRanker{Rankers: rankers}
}

// Name returns the name of the composite ranker.
func (c *CompositeRanker) Name() string { return "composite_ranker" }

// Rank scores the results using all underlying rankers and aggregates the scores.
func (c *CompositeRanker) Rank(ctx context.Context, q query.Query, results []query.Result) ([]ScoredResult, error) {
	if len(c.Rankers) == 0 {
		scored := make([]ScoredResult, len(results))
		for i, r := range results {
			scored[i] = ScoredResult{Result: r, Score: 0}
		}
		return scored, nil
	}

	// Run first ranker
	scored, err := c.Rankers[0].Rank(ctx, q, results)
	if err != nil {
		return nil, err
	}

	// Ensure Breakdown is initialized for all results
	for i := range scored {
		if scored[i].Breakdown == nil {
			scored[i].Breakdown = make(map[string]float64)
		}
	}

	// Run remaining rankers and sum scores
	for i := 1; i < len(c.Rankers); i++ {
		additionalScores, err := c.Rankers[i].Rank(ctx, q, results)
		if err != nil {
			return nil, err
		}
		for j := range scored {
			scored[j].Score += additionalScores[j].Score
			for k, v := range additionalScores[j].Breakdown {
				scored[j].Breakdown[k] = v
			}
		}
	}
	return scored, nil
}
