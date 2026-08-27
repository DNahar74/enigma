package rank_bm25

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DNahar74/enigma/core/query"
)

func TestBM25_Name(t *testing.T) {
	bm := New(1.2, 0.75)
	assert.Equal(t, "bm25", bm.Name())
}

func TestBM25_EmptyResults(t *testing.T) {
	bm := New(1.2, 0.75)
	res, err := bm.Rank(context.Background(), query.Query{Tokens: []string{"test"}}, nil)
	require.NoError(t, err)
	assert.Empty(t, res)
}

func TestBM25_SingleResult(t *testing.T) {
	bm := New(1.2, 0.75)
	q := query.Query{Tokens: []string{"match"}}

	results := []query.Result{
		{Title: "A match made in heaven", Snippet: "Here is a match."},
	}

	scored, err := bm.Rank(context.Background(), q, results)
	require.NoError(t, err)
	require.Len(t, scored, 1)
	assert.Greater(t, scored[0].Score, 0.0)
}

func TestBM25_IDF(t *testing.T) {
	bm := New(1.2, 0.75)

	// Create three documents of equal length to isolate IDF's effect (no length normalization bias).
	results := []query.Result{
		{Title: "Doc 1", Snippet: "common common filler"},
		{Title: "Doc 2", Snippet: "common common filler"},
		{Title: "Doc 3", Snippet: "common rare filler"},
	}

	// Evaluate the query "common"
	qCommon := query.Query{Tokens: []string{"common"}}
	scoredCommon, err := bm.Rank(context.Background(), qCommon, results)
	require.NoError(t, err)

	// Evaluate the query "rare"
	qRare := query.Query{Tokens: []string{"rare"}}
	scoredRare, err := bm.Rank(context.Background(), qRare, results)
	require.NoError(t, err)

	// Doc 3 has one "rare" term. Doc 1 has two "common" terms.
	// But "rare" appears in 1/3 docs, while "common" appears in 3/3 docs.
	// The IDF for "rare" should be significantly higher than "common", making its overall score higher despite lower TF.
	scoreCommonInDoc1 := scoredCommon[0].Score
	scoreRareInDoc3 := scoredRare[2].Score

	assert.Greater(t, scoreRareInDoc3, scoreCommonInDoc1, "Rare term should score higher than common term due to IDF")
}

func TestBM25_LengthNorm(t *testing.T) {
	bm := New(1.2, 0.75)
	q := query.Query{Tokens: []string{"term"}}

	results := []query.Result{
		{Title: "Short", Snippet: "term"},
		{Title: "Long", Snippet: "term and a lot of other words that do not matter"},
	}

	scored, err := bm.Rank(context.Background(), q, results)
	require.NoError(t, err)
	require.Len(t, scored, 2)

	// Both have exactly 1 occurrence of "term".
	// The shorter document should have a higher score because of length normalization (b=0.75).
	assert.Greater(t, scored[0].Score, scored[1].Score)
}

func TestBM25_ContextCancellation(t *testing.T) {
	bm := New(1.2, 0.75)
	q := query.Query{Tokens: []string{"term"}}

	results := []query.Result{
		{Title: "Doc 1", Snippet: "term"},
		{Title: "Doc 2", Snippet: "term"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately, tests Phase 1 cancellation

	_, err := bm.Rank(ctx, q, results)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

type phase2CancelCtx struct {
	context.Context
	cancelCh chan struct{}
	calls    int
	once     sync.Once
}

func (c *phase2CancelCtx) Done() <-chan struct{} {
	c.calls++
	// There are len(results) checks in Phase 1.
	// We want to cancel after Phase 1, so when calls > len(results).
	if c.calls > 2 {
		c.once.Do(func() {
			close(c.cancelCh)
		})
	}
	return c.cancelCh
}

func (c *phase2CancelCtx) Err() error {
	select {
	case <-c.cancelCh:
		return context.Canceled
	default:
		return nil
	}
}

func TestBM25_ContextCancellation_Phase2(t *testing.T) {
	bm := New(1.2, 0.75)
	q := query.Query{Tokens: []string{"term"}}

	results := []query.Result{
		{Title: "Doc 1", Snippet: "term"},
		{Title: "Doc 2", Snippet: "term"},
	}

	ctx := &phase2CancelCtx{
		Context:  context.Background(),
		cancelCh: make(chan struct{}),
	}

	_, err := bm.Rank(ctx, q, results)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestBM25_EmptyDocs(t *testing.T) {
	bm := New(1.2, 0.75)
	q := query.Query{Tokens: []string{"term"}}

	// Create docs that produce no tokens to trigger avgdl == 0
	results := []query.Result{
		{Title: "", Snippet: ""},
		{Title: "   ", Snippet: "!!!"},
	}

	scored, err := bm.Rank(context.Background(), q, results)
	require.NoError(t, err)
	require.Len(t, scored, 2)
	assert.Equal(t, 0.0, scored[0].Score)
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"Hello World", "Hello World!", []string{"hello", "world"}},
		{"Punctuation", "  some-words, with punctuation.  ", []string{"some", "words", "with", "punctuation"}},
		{"Lowercase", "already_lower", []string{"already", "lower"}},
		{"Empty", "", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tokenize(tt.input))
		})
	}
}
