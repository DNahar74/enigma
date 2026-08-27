package rank_personal

import (
	"context"
	"strings"
	"unicode"

	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
)

// PersonalRank implements plugin.RankPlugin to boost results based on local knowledge.
type PersonalRank struct {
	boostMultiplier float64
}

// New creates a new PersonalRank plugin.
func New(boostMultiplier float64) *PersonalRank {
	return &PersonalRank{
		boostMultiplier: boostMultiplier,
	}
}

// Name returns the name of the plugin.
func (p *PersonalRank) Name() string {
	return "personal"
}

// Rank scores results based on vocabulary overlap with local notes.
func (p *PersonalRank) Rank(ctx context.Context, q query.Query, results []query.Result) ([]plugin.ScoredResult, error) {
	// Fast path: if boost is 0, this ranker contributes nothing.
	if p.boostMultiplier == 0 {
		scored := make([]plugin.ScoredResult, len(results))
		for i, r := range results {
			scored[i] = plugin.ScoredResult{
				Result:    r,
				Score:     0.0,
				Breakdown: map[string]float64{"personal": 0.0},
			}
		}
		return scored, nil
	}

	// 1. Build a vocabulary of significant words from local notes.
	localVocab := make(map[string]struct{})
	for _, r := range results {
		if r.SourcePlugin == "local" {
			extractVocab(r.Title, localVocab)
			extractVocab(r.Snippet, localVocab)
		}
	}

	var scored []plugin.ScoredResult
	for _, r := range results {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		score := 0.0

		if r.SourcePlugin == "local" {
			// Score local notes by how many query tokens appear in their content,
			// so a generic PDF matching on one word doesn't dominate results.
			localText := strings.ToLower(r.Title + " " + r.Snippet)
			matchCount := 0
			for _, tok := range q.Tokens {
				if strings.Contains(localText, strings.ToLower(tok)) {
					matchCount++
				}
			}
			// Proportion of query terms matched × boost, capped so it doesn't
			// overwhelm great web results.
			if len(q.Tokens) > 0 {
				coverage := float64(matchCount) / float64(len(q.Tokens))
				score = coverage * p.boostMultiplier * 10.0
			}
		} else if len(localVocab) > 0 {
			// For web results, count vocabulary overlap
			webVocab := make(map[string]struct{})
			extractVocab(r.Title, webVocab)
			extractVocab(r.Snippet, webVocab)

			overlap := 0
			for word := range webVocab {
				if _, exists := localVocab[word]; exists {
					overlap++
				}
			}
			score = float64(overlap) * p.boostMultiplier
		}

		scored = append(scored, plugin.ScoredResult{
			Result:    r,
			Score:     score,
			Breakdown: map[string]float64{"personal": score},
		})
	}

	return scored, nil
}

// extractVocab extracts words longer than 3 characters into a set.
func extractVocab(text string, vocab map[string]struct{}) {
	words := strings.FieldsFunc(text, func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	})
	for _, w := range words {
		w = strings.ToLower(w)
		if len(w) > 3 { // skip small stop words
			vocab[w] = struct{}{}
		}
	}
}
