// Package pipeline coordinates the execution of search, filter, and rank plugins.
// It is responsible for orchestrating the flow of data:
// 1. Sending the query to the search plugin.
// 2. Collecting the raw results.
// 3. Passing results through the filter plugin.
// 4. Scoring the remaining results using the rank plugin.
// 5. Sorting the final results by score in descending order.
package pipeline

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/sync/errgroup"

	"github.com/DNahar74/enigma/core/config"
	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
)

// Pipeline manages the end-to-end search process.
type Pipeline struct {
	reg *plugin.Registry
	cfg *config.Config
}

// New constructs a new Pipeline instance.
func New(reg *plugin.Registry, cfg *config.Config) *Pipeline {
	return &Pipeline{
		reg: reg,
		cfg: cfg,
	}
}

// Execute runs the complete search pipeline for a given query.
func (p *Pipeline) Execute(ctx context.Context, q query.Query) ([]plugin.ScoredResult, error) {
	// 1. Separate local plugins and remote plugins
	var localPlugins []plugin.SearchPlugin
	var remotePlugins []plugin.SearchPlugin
	for _, sp := range p.reg.SearchPlugins() {
		if _, ok := sp.(plugin.LocalSearchPlugin); ok {
			localPlugins = append(localPlugins, sp)
		} else {
			remotePlugins = append(remotePlugins, sp)
		}
	}

	var results []query.Result

	// 2. Run local plugins first
	if len(localPlugins) > 0 {
		localResults, err := p.runPlugins(ctx, localPlugins, q)
		if err != nil {
			return nil, err
		}
		results = append(results, localResults...)

		// 3. Extract top keywords from local results for Query Expansion
		if len(localResults) > 0 {
			expansionTerms := extractKeywords(localResults, q)
			if len(expansionTerms) > 0 {
				q.Raw = q.Raw + " " + strings.Join(expansionTerms, " ")
				q.Tokens = append(q.Tokens, expansionTerms...)
			}
		}
	}

	// 4. Run remote plugins with expanded query
	if len(remotePlugins) > 0 {
		remoteResults, err := p.runPlugins(ctx, remotePlugins, q)
		if err != nil {
			return nil, err
		}
		results = append(results, remoteResults...)
	}

	filteredResults, err := p.reg.Filter().Filter(ctx, results)
	if err != nil {
		return nil, fmt.Errorf("filter failed: %w", err)
	}

	scoredResults, err := p.reg.Rank().Rank(ctx, q, filteredResults)
	if err != nil {
		return nil, fmt.Errorf("rank failed: %w", err)
	}

	sort.SliceStable(scoredResults, func(i, j int) bool {
		return scoredResults[i].Score > scoredResults[j].Score
	})

	return scoredResults, nil
}

// runPlugins executes a list of search plugins concurrently and merges their results.
func (p *Pipeline) runPlugins(ctx context.Context, plugins []plugin.SearchPlugin, q query.Query) ([]query.Result, error) {
	eg, egCtx := errgroup.WithContext(ctx)
	mergedResults := make(chan query.Result)

	for _, sp := range plugins {
		sp := sp // capture loop variable
		eg.Go(func() error {
			ch, err := sp.Search(egCtx, q)
			if err != nil {
				return fmt.Errorf("search plugin %q failed: %w", sp.Name(), err)
			}
			for {
				select {
				case <-egCtx.Done():
					return egCtx.Err()
				case res, ok := <-ch:
					if !ok {
						return nil
					}
					select {
					case <-egCtx.Done():
						return egCtx.Err()
					case mergedResults <- res:
					}
				}
			}
		})
	}

	go func() {
		_ = eg.Wait()
		close(mergedResults)
	}()

	var results []query.Result
drainLoop:
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case res, ok := <-mergedResults:
			if !ok {
				break drainLoop
			}
			results = append(results, res)
		}
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

// extractKeywords finds the most frequent significant terms in the local results
// that are not already in the query.
func extractKeywords(results []query.Result, q query.Query) []string {
	freq := make(map[string]int)
	querySet := make(map[string]bool)
	for _, t := range q.Tokens {
		querySet[strings.ToLower(t)] = true
	}

	// Common stop words to ignore in expansion
	stopWords := map[string]bool{
		"this": true, "that": true, "with": true, "from": true, "have": true,
		"they": true, "will": true, "would": true, "there": true, "their": true,
		"what": true, "when": true, "where": true, "which": true, "who": true,
		"some": true, "more": true, "about": true, "other": true, "into": true,
		"only": true, "also": true, "could": true, "than": true, "then": true,
		"because": true, "these": true, "those": true, "been": true, "much": true,
	}

	for _, r := range results {
		// simple tokenization
		tokens := strings.Fields(strings.ToLower(r.Title + " " + r.Snippet))
		for _, t := range tokens {
			t = strings.TrimFunc(t, func(r rune) bool {
				return !unicode.IsLetter(r) && !unicode.IsNumber(r)
			})
			if len(t) > 4 && !querySet[t] && !stopWords[t] {
				freq[t]++
			}
		}
	}

	// Sort by frequency
	type kv struct {
		k string
		v int
	}
	var ss []kv
	for k, v := range freq {
		ss = append(ss, kv{k, v})
	}
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].v > ss[j].v
	})

	var expanded []string
	// Take top 2 keywords
	for i := 0; i < len(ss) && i < 2; i++ {
		expanded = append(expanded, ss[i].k)
	}
	return expanded
}
