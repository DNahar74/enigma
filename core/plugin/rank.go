package plugin

import (
	"context"

	"github.com/DNahar74/enigma/core/query"
)

// ScoredResult pairs a search result with its relevance score.
// Produced by RankPlugin.Rank and consumed by the pipeline for sorting.
type ScoredResult struct {
	Result query.Result
	Score  float64
	// Breakdown stores the individual score contributions from each rank plugin.
	// For example: {"bm25": 12.5, "personal": 5.0, "trust": -2.0}
	Breakdown map[string]float64
}

// RankPlugin defines the contract for plugins that score search results
// by relevance to the query.
//
// The interface uses a BATCH signature (all results at once) rather than
// per-result scoring. This is necessary because ranking algorithms like BM25
// need corpus-wide statistics (e.g., document frequency) that can only be
// computed with the full result set in hand. See CONSTITUTION.md §2.4.
type RankPlugin interface {
	// Name returns a short identifier for this plugin (e.g., "bm25").
	Name() string

	// Rank scores all results for a given query and returns them as ScoredResults.
	// The returned slice may be in any order — the pipeline handles sorting.
	// The input results slice is not modified.
	Rank(ctx context.Context, q query.Query, results []query.Result) ([]ScoredResult, error)
}
