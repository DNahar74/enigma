package plugin

import (
	"context"

	"github.com/DNahar74/enigma/core/query"
)

// FilterPlugin defines the contract for plugins that remove unwanted results
// from the search output (e.g., blocked domains, sponsored content).
//
// Filters run BEFORE ranking — no point scoring results we're going to throw away.
type FilterPlugin interface {
	// Name returns a short identifier for this plugin (e.g., "blocklist").
	Name() string

	// Filter takes a slice of results and returns a new slice with
	// unwanted results removed. The input slice is not modified.
	Filter(ctx context.Context, results []query.Result) ([]query.Result, error)
}
