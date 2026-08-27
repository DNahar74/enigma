package rank_trust

import (
	"context"
	"net/url"
	"strings"

	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
)

// TrustRanker boosts or penalizes results based on their domain.
type TrustRanker struct {
	boosted   map[string]struct{}
	penalized map[string]struct{}
}

// New creates a new TrustRanker.
func New(boostedDomains, penalizedDomains []string) *TrustRanker {
	boosted := make(map[string]struct{})
	for _, d := range boostedDomains {
		boosted[strings.ToLower(d)] = struct{}{}
	}

	penalized := make(map[string]struct{})
	for _, d := range penalizedDomains {
		penalized[strings.ToLower(d)] = struct{}{}
	}

	return &TrustRanker{
		boosted:   boosted,
		penalized: penalized,
	}
}

// Name returns the short identifier for this plugin ("trust").
func (t *TrustRanker) Name() string {
	return "trust"
}

// Rank adjusts the scores of the results based on domain trust.
func (t *TrustRanker) Rank(ctx context.Context, q query.Query, results []query.Result) ([]plugin.ScoredResult, error) {
	var scored []plugin.ScoredResult
	for _, r := range results {
		scoreDelta := 0.0

		u, err := url.Parse(r.URL)
		if err == nil && u.Host != "" {
			host := strings.ToLower(u.Host)
			// Remove www. if present
			if strings.HasPrefix(host, "www.") {
				host = host[4:]
			}

			if _, ok := t.boosted[host]; ok {
				scoreDelta = 20.0
			} else if _, ok := t.penalized[host]; ok {
				scoreDelta = -50.0
			}
		}

		scored = append(scored, plugin.ScoredResult{
			Result:    r,
			Score:     scoreDelta,
			Breakdown: map[string]float64{"trust": scoreDelta},
		})
	}

	return scored, nil
}
