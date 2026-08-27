package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
)

// Ensure fakes satisfy plugin interfaces at compile time.
var _ plugin.SearchPlugin = (*FakeSearch)(nil)
var _ plugin.FilterPlugin = (*FakeFilter)(nil)
var _ plugin.RankPlugin = (*FakeRank)(nil)

func TestFakeSearch_DelayAndContextCancellation(t *testing.T) {
	t.Run("Delay respected", func(t *testing.T) {
		start := time.Now()
		fake := &FakeSearch{
			Delay: 50 * time.Millisecond,
		}
		_, err := fake.Search(context.Background(), query.Query{Raw: "test delay"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
			t.Fatalf("expected delay of at least 50ms, got %v", elapsed)
		}
	})

	t.Run("Context cancelled during delay", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel before Search is called

		fake := &FakeSearch{
			Delay: 50 * time.Millisecond,
		}
		_, err := fake.Search(ctx, query.Query{Raw: "test ctx delay"})
		if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("Context cancelled during sending results", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		fake := &FakeSearch{
			Results: []query.Result{{URL: "url1"}, {URL: "url2"}, {URL: "url3"}},
		}
		ch, err := fake.Search(ctx, query.Query{Raw: "test ctx send"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Read one result to ensure goroutine started sending
		<-ch

		// Cancel the context, which should cause the sending goroutine to exit
		cancel()

		// Drain the channel until it's closed or timeout
		done := make(chan struct{})
		go func() {
			for range ch {
			}
			close(done)
		}()

		select {
		case <-done:
			// Goroutine exited and channel was closed
		case <-time.After(100 * time.Millisecond):
			t.Fatal("sending goroutine did not exit on context cancellation within timeout")
		}
	})
}
