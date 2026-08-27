package plugin

import (
	"context"
	"strings"
	"testing"

	"github.com/DNahar74/enigma/core/query"
)

// ---------------------------------------------------------------------------
// Minimal stubs just for testing registry construction.
// Full-featured fakes live in internal/testutil (not yet created).
// ---------------------------------------------------------------------------

type stubSearch struct{}

func (s *stubSearch) Name() string { return "stub-search" }
func (s *stubSearch) Search(_ context.Context, _ query.Query) (<-chan query.Result, error) {
	ch := make(chan query.Result)
	close(ch)
	return ch, nil
}
func (s *stubSearch) Ping(_ context.Context) error { return nil }

type stubFilter struct{}

func (s *stubFilter) Name() string { return "stub-filter" }
func (s *stubFilter) Filter(_ context.Context, results []query.Result) ([]query.Result, error) {
	return results, nil
}

type stubRank struct{}

func (s *stubRank) Name() string { return "stub-rank" }
func (s *stubRank) Rank(_ context.Context, _ query.Query, results []query.Result) ([]ScoredResult, error) {
	out := make([]ScoredResult, len(results))
	for i, r := range results {
		out[i] = ScoredResult{Result: r, Score: 1.0}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestNewRegistry(t *testing.T) {
	search := &stubSearch{}
	filter := &stubFilter{}
	rank := &stubRank{}

	tests := []struct {
		name          string
		searchPlugins []SearchPlugin
		filter        FilterPlugin
		rank          RankPlugin
		wantErr       bool
		errSubstr     string // substring that must appear in the error message
	}{
		{
			name:          "all valid plugins",
			searchPlugins: []SearchPlugin{search},
			filter:        filter,
			rank:          rank,
		},
		{
			name:          "nil search",
			searchPlugins: nil,
			filter:        filter,
			rank:          rank,
			wantErr:       true,
			errSubstr:     "search",
		},
		{
			name:          "empty search",
			searchPlugins: []SearchPlugin{},
			filter:        filter,
			rank:          rank,
			wantErr:       true,
			errSubstr:     "search",
		},
		{
			name:          "nil filter",
			searchPlugins: []SearchPlugin{search},
			filter:        nil,
			rank:          rank,
			wantErr:       true,
			errSubstr:     "filter",
		},
		{
			name:          "nil rank",
			searchPlugins: []SearchPlugin{search},
			filter:        filter,
			rank:          nil,
			wantErr:       true,
			errSubstr:     "rank",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := NewRegistry(tt.searchPlugins, tt.filter, tt.rank)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errSubstr)
				}
				if reg != nil {
					t.Error("registry should be nil on error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(reg.SearchPlugins()) != len(tt.searchPlugins) {
				t.Error("SearchPlugins() returned wrong length")
			} else {
				for i, sp := range reg.SearchPlugins() {
					if sp != tt.searchPlugins[i] {
						t.Errorf("SearchPlugins()[%d] returned wrong instance", i)
					}
				}
			}
			if reg.Filter() != tt.filter {
				t.Error("Filter() returned wrong instance")
			}
			if reg.Rank() != tt.rank {
				t.Error("Rank() returned wrong instance")
			}
		})
	}
}
