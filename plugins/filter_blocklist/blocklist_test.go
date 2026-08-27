package filter_blocklist

import (
	"context"
	"reflect"
	"testing"

	"github.com/DNahar74/enigma/core/query"
)

func TestBlocklist_Filter(t *testing.T) {
	tests := []struct {
		name      string
		domains   []string
		results   []query.Result
		want      []query.Result
		cancelCtx bool
		wantErr   bool
	}{
		{
			name:    "Empty blocklist -> all results kept",
			domains: []string{},
			results: []query.Result{
				{URL: "https://example.com/page1"},
				{URL: "https://google.com/search"},
			},
			want: []query.Result{
				{URL: "https://example.com/page1"},
				{URL: "https://google.com/search"},
			},
		},
		{
			name:    "Exact domain match -> result removed",
			domains: []string{"example.com"},
			results: []query.Result{
				{URL: "https://example.com/page1"},
				{URL: "https://google.com/search"},
			},
			want: []query.Result{
				{URL: "https://google.com/search"},
			},
		},
		{
			name:    "Subdomain match -> result removed",
			domains: []string{"example.com"},
			results: []query.Result{
				{URL: "https://blog.example.com/post"},
				{URL: "https://example.org/home"},
			},
			want: []query.Result{
				{URL: "https://example.org/home"},
			},
		},
		{
			name:    "www. prefix handling (blocklist has example.com, result has www.) -> removed",
			domains: []string{"example.com"},
			results: []query.Result{
				{URL: "https://www.example.com/page"},
			},
			want: nil,
		},
		{
			name:    "www. prefix handling (blocklist has www.example.com, result has example.com) -> removed",
			domains: []string{"www.example.com"},
			results: []query.Result{
				{URL: "https://example.com/page"},
			},
			want: nil,
		},
		{
			name:    "URL parse failure -> result kept",
			domains: []string{"example.com"},
			results: []query.Result{
				{URL: "://invalid-url"}, // will fail to parse
			},
			want: []query.Result{
				{URL: "://invalid-url"},
			},
		},
		{
			name:    "Empty results -> returns empty slice",
			domains: []string{"example.com"},
			results: []query.Result{},
			want:    nil,
		},
		{
			name:      "Context cancellation mid-filter -> returns context.Canceled",
			domains:   []string{"example.com"},
			results:   []query.Result{{URL: "https://test.com"}},
			cancelCtx: true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel() // cancel immediately
			}

			b := New(tt.domains)
			got, err := b.Filter(ctx, tt.results)

			if (err != nil) != tt.wantErr {
				t.Errorf("Blocklist.Filter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Normalize empty slices to nil for easier reflect.DeepEqual comparison
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Blocklist.Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlocklist_Name(t *testing.T) {
	b := New([]string{"example.com"})
	if b.Name() != "blocklist" {
		t.Errorf("Name() = %v, want %v", b.Name(), "blocklist")
	}
}
