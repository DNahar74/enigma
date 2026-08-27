// Package filter_blocklist provides a plugin to filter out search results
// from user-specified blocked domains or their subdomains.
package filter_blocklist

import (
	"context"
	"net/url"
	"strings"

	"github.com/DNahar74/enigma/core/query"
)

// Blocklist implements plugin.FilterPlugin to filter out search results
// from specifically blocked domains.
type Blocklist struct {
	blocked map[string]struct{}
}

// New creates a new Blocklist filter with the given domains.
// It normalizes domains by converting to lowercase, trimming spaces,
// and removing "www." prefixes for O(1) lookup.
func New(domains []string) *Blocklist {
	b := &Blocklist{
		blocked: make(map[string]struct{}, len(domains)),
	}
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		d = strings.TrimPrefix(d, "www.")
		if d != "" {
			b.blocked[d] = struct{}{}
		}
	}
	return b
}

// Name returns the name of the plugin.
func (b *Blocklist) Name() string {
	return "blocklist"
}

// Filter iterates over the results and returns a new slice omitting any
// results whose URLs belong to blocked domains or their subdomains.
func (b *Blocklist) Filter(ctx context.Context, results []query.Result) ([]query.Result, error) {
	if len(results) == 0 {
		return nil, nil
	}

	var filtered []query.Result
	for _, r := range results {
		// Check for cancellation before processing each result
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		u, err := url.Parse(r.URL)
		if err != nil {
			// If we can't parse the URL, we keep the result.
			filtered = append(filtered, r)
			continue
		}

		host := strings.ToLower(u.Hostname())
		host = strings.TrimPrefix(host, "www.")

		if !b.isBlocked(host) {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

// isBlocked checks if the host or any of its parent domains are blocked.
func (b *Blocklist) isBlocked(host string) bool {
	// Exact match
	if _, ok := b.blocked[host]; ok {
		return true
	}

	// Subdomain match (e.g., if host is "a.b.example.com", check "b.example.com" and "example.com")
	parts := strings.Split(host, ".")
	for i := 1; i < len(parts)-1; i++ {
		parent := strings.Join(parts[i:], ".")
		if _, ok := b.blocked[parent]; ok {
			return true
		}
	}

	return false
}
