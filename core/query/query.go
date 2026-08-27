// Package query defines the foundational data types that flow through Enigma's
// search pipeline. Every other package in the system imports these types:
//
//   - Query represents a parsed user search query (tokens + raw text).
//   - Result represents a single search hit returned by a plugin.
//
// The package also provides Parse, which turns a raw query string into a
// validated, tokenized Query ready for downstream consumption by indexers,
// scorers, and rankers.
package query

import (
	"fmt"
	"strings"
)

// Query holds both the original search text and its parsed tokens.
//
// Raw preserves the trimmed, original-case input so the UI can display exactly
// what the user typed. Tokens are lowercased and deduplicated for use in
// matching and scoring (e.g. BM25 term-frequency calculations).
type Query struct {
	// Raw is the trimmed original input, preserving case for display purposes.
	Raw string

	// Tokens are the lowercased, whitespace-split, deduplicated search terms.
	// First-occurrence order is preserved so that positional heuristics remain
	// stable if we ever add phrase-proximity scoring.
	Tokens []string
}

// Result represents a single search hit returned by a source plugin.
//
// Each plugin (local index, web scraper, etc.) produces Results that are later
// merged and ranked by the scoring layer.
type Result struct {
	URL          string
	Title        string
	Snippet      string
	SourcePlugin string
}

// Parse validates and tokenizes a raw query string.
//
// Processing steps:
//  1. Trim leading/trailing whitespace.
//  2. Reject empty or whitespace-only input with a descriptive error.
//  3. Lowercase the trimmed text and split on whitespace (strings.Fields
//     handles runs of any Unicode whitespace).
//  4. Deduplicate tokens, preserving first-occurrence order.
//
// Deduplication matters because repeated terms would otherwise inflate BM25
// term-frequency counts, giving an unfair advantage to queries like
// "raft raft raft" over the equivalent single-term "raft".
func Parse(raw string) (Query, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Query{}, fmt.Errorf("query: empty or whitespace-only input")
	}

	// Lowercase for case-insensitive matching; Fields splits on any whitespace
	// run, so "  raft   consensus  " becomes ["raft", "consensus"].
	words := strings.Fields(strings.ToLower(trimmed))

	// Deduplicate while preserving insertion order. A map tracks which tokens
	// we've already seen; the slice accumulates unique tokens in order.
	seen := make(map[string]struct{}, len(words))
	tokens := make([]string, 0, len(words))
	for _, w := range words {
		if _, exists := seen[w]; exists {
			continue
		}
		seen[w] = struct{}{}
		tokens = append(tokens, w)
	}

	return Query{
		Raw:    trimmed,
		Tokens: tokens,
	}, nil
}
