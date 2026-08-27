package filter_antislop

import (
	"context"
	"strings"

	"github.com/DNahar74/enigma/core/query"
)

// AntiSlopFilter attempts to silently drop SEO farms and AI-generated slop
// using lightweight heuristics rather than an LLM.
type AntiSlopFilter struct {
	slopPhrases []string
}

// New creates a new AntiSlopFilter with predefined boilerplate phrases.
func New() *AntiSlopFilter {
	return &AntiSlopFilter{
		slopPhrases: []string{
			"as an ai language model",
			"in conclusion,",
			"it is important to note that",
			"when it comes to",
			"overall,",
			"delve into",
			"here are some key things to know",
			"whether you are a",
			"buckle up",
			"a testament to",
		},
	}
}

// Name returns the short identifier for this plugin ("antislop").
func (a *AntiSlopFilter) Name() string {
	return "antislop"
}

// Filter checks results and drops those that trigger too many slop heuristics.
func (a *AntiSlopFilter) Filter(ctx context.Context, results []query.Result) ([]query.Result, error) {
	var filtered []query.Result
	for _, res := range results {
		// Only filter web results, trust local notes.
		if res.SourcePlugin == "local" {
			filtered = append(filtered, res)
			continue
		}

		slopScore := 0
		content := strings.ToLower(res.Title + " " + res.Snippet)

		for _, phrase := range a.slopPhrases {
			if strings.Contains(content, phrase) {
				slopScore++
			}
		}

		// If it hits 2 or more slop phrases in just the snippet/title, it's highly likely to be slop.
		if slopScore < 2 {
			filtered = append(filtered, res)
		}
	}
	return filtered, nil
}
