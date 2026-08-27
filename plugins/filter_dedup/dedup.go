package filter_dedup

import (
	"context"
	"strings"

	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
)

// DedupFilter implements plugin.FilterPlugin to remove near-duplicate results.
// It compares results using exact URL matches or high textual similarity in Title+Snippet.
type DedupFilter struct {
	jaccardThreshold float64
}

// New creates a new DedupFilter.
func New() *DedupFilter {
	return &DedupFilter{
		jaccardThreshold: 0.8, // 80% word overlap is considered a duplicate
	}
}

func (d *DedupFilter) Name() string { return "dedup" }

func (d *DedupFilter) Filter(ctx context.Context, results []query.Result) ([]query.Result, error) {
	if len(results) == 0 {
		return results, nil
	}

	var filtered []query.Result
	seenURLs := make(map[string]bool)

	// Keep track of the token sets for accepted results to compute Jaccard similarity
	acceptedTokens := make([]map[string]bool, 0, len(results))

	for _, r := range results {
		// 1. Exact URL match deduplication
		if r.URL != "" && seenURLs[r.URL] {
			continue
		}

		// 2. Near-duplicate text deduplication
		tokens := tokenize(r.Title + " " + r.Snippet)
		if len(tokens) == 0 {
			// If it has no text, just pass it through
			filtered = append(filtered, r)
			if r.URL != "" {
				seenURLs[r.URL] = true
			}
			continue
		}

		isDuplicate := false
		for _, existing := range acceptedTokens {
			if jaccard(tokens, existing) >= d.jaccardThreshold {
				isDuplicate = true
				break
			}
		}

		if isDuplicate {
			continue
		}

		// Accept the result
		filtered = append(filtered, r)
		acceptedTokens = append(acceptedTokens, tokens)
		if r.URL != "" {
			seenURLs[r.URL] = true
		}
	}

	return filtered, nil
}

func tokenize(s string) map[string]bool {
	tokens := make(map[string]bool)
	words := strings.Fields(strings.ToLower(s))
	for _, w := range words {
		// strip basic punctuation
		w = strings.Trim(w, ".,!?\"'()[]{}")
		if w != "" {
			tokens[w] = true
		}
	}
	return tokens
}

func jaccard(a, b map[string]bool) float64 {
	intersection := 0
	union := len(a)

	for k := range b {
		if a[k] {
			intersection++
		} else {
			union++
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

var _ plugin.FilterPlugin = (*DedupFilter)(nil)
