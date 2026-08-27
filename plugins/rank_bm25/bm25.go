package rank_bm25

import (
	"context"
	"math"
	"strings"
	"unicode"

	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
)

// BM25 implements a true Okapi BM25 ranker over a pseudo-corpus of fetched results.
// It computes per-query IDF based on the document frequency of the results.
// See CONSTITUTION.md §2.4.
type BM25 struct {
	k1 float64
	b  float64
}

// New constructs a new BM25 ranker with standard parameters k1 and b.
func New(k1, b float64) *BM25 {
	return &BM25{k1: k1, b: b}
}

// Name returns the short identifier for this plugin ("bm25").
func (bm *BM25) Name() string {
	return "bm25"
}

// tokenize lowercases and splits the string on whitespace and punctuation.
// This ensures that terms match against the lowercased query tokens.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	f := func(c rune) bool {
		return unicode.IsSpace(c) || unicode.IsPunct(c)
	}
	return strings.FieldsFunc(s, f)
}

// Rank scores a slice of results using BM25 against the query terms.
// It operates in two phases: building corpus stats from the results, and scoring.
func (bm *BM25) Rank(ctx context.Context, q query.Query, results []query.Result) ([]plugin.ScoredResult, error) {
	if len(results) == 0 {
		return []plugin.ScoredResult{}, nil
	}

	// Phase 1: Build corpus stats
	df := make(map[string]int)
	docTokens := make([][]string, len(results))
	totalTokens := 0

	for i, r := range results {
		// Check for cancellation to prevent long-running tasks blocking
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Tokenize Title + Snippet
		tokens := tokenize(r.Title + " " + r.Snippet)
		docTokens[i] = tokens

		seen := make(map[string]struct{})
		for _, t := range tokens {
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				df[t]++
			}
		}
		totalTokens += len(tokens)
	}

	// Phase 2: Score
	N := float64(len(results))
	avgdl := float64(totalTokens) / N

	// Prevent divide-by-zero if all documents are completely empty
	if avgdl == 0 {
		avgdl = 1.0
	}

	var scored []plugin.ScoredResult
	// Preallocate the slice for performance
	scored = make([]plugin.ScoredResult, 0, len(results))

	for i, r := range results {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		tokens := docTokens[i]
		dl := float64(len(tokens))
		score := 0.0

		for _, qt := range q.Tokens {
			// Calculate Term Frequency (tf)
			tfCount := 0
			for _, t := range tokens {
				if t == qt {
					tfCount++
				}
			}

			if tfCount == 0 {
				continue
			}

			tf := float64(tfCount)
			docFreq := float64(df[qt])

			// Compute IDF with +0.5 smoothing
			idf := math.Log(1.0 + (N-docFreq+0.5)/(docFreq+0.5))

			// Compute TF component with length normalization
			tfComp := (tf * (bm.k1 + 1.0)) / (tf + bm.k1*(1.0-bm.b+bm.b*(dl/avgdl)))

			score += idf * tfComp
		}

		scored = append(scored, plugin.ScoredResult{
			Result:    r,
			Score:     score,
			Breakdown: map[string]float64{"bm25": score},
		})
	}

	return scored, nil
}
