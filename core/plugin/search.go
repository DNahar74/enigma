// Package plugin defines the contracts (interfaces) that all Enigma plugins
// must implement. V0.1 has three plugin types:
//
//   - SearchPlugin — fetches raw results from an external source (e.g., Tavily Search API).
//   - FilterPlugin — removes unwanted results (e.g., blocked domains, sponsored content).
//   - RankPlugin   — scores results by relevance to the query (e.g., BM25).
//
// The [Registry] wires exactly one instance of each plugin type into the system.
// No other package (pipeline, cmd) should import plugin implementations directly —
// they depend only on these interfaces. This keeps the dependency graph clean and
// makes it trivial to swap implementations for testing.
package plugin

import (
	"context"

	"github.com/DNahar74/enigma/core/query"
)

// SearchPlugin defines the contract for plugins that fetch search results
// from an external source (e.g., Tavily Search API).
//
// Search returns a channel so results can be streamed as they arrive,
// which matters when V0.2+ adds multiple providers running in parallel.
// For V0.1 with a single provider, the channel is simply drained into a slice
// by the pipeline.
type SearchPlugin interface {
	// Name returns a short identifier for this plugin (e.g., "tavily").
	// Used in logs and the stats summary line.
	Name() string

	// Search executes a search query and streams results on the returned channel.
	// The channel is closed when all results have been sent.
	// The caller must drain the channel or cancel the context to avoid goroutine leaks.
	Search(ctx context.Context, q query.Query) (<-chan query.Result, error)

	// Ping checks whether the plugin's backing service is reachable.
	// Used for health checks and validating configuration (e.g., API key works).
	Ping(ctx context.Context) error
}

// LocalSearchPlugin is implemented by search plugins that read the user's
// own data (files, notes) rather than a remote API. The pipeline type-asserts
// for this interface to decide ordering and query-expansion eligibility.
type LocalSearchPlugin interface {
	SearchPlugin
	IsLocal() bool
}
