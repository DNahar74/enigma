package search_local

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DNahar74/enigma/core/query"
)

func TestLocalSearch_Search(t *testing.T) {
	// Create a temporary directory for tests
	tmpDir := t.TempDir()

	// Create some dummy files
	files := map[string]string{
		"note1.md":    "This is a note about rust programming.",
		"note2.txt":   "Another note mentioning golang and rust.",
		"ignore.html": "<html>rust</html>",
		"sub/doc.md":  "Deeply nested rust doc.",
		"other.md":    "This note does not contain the word.",
	}

	// Make sub directory
	if err := os.Mkdir(filepath.Join(tmpDir, "sub"), 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test file %s: %v", name, err)
		}
	}

	ls := New(tmpDir)

	q, _ := query.Parse("RUST")

	resultsChan, err := ls.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	var foundFiles []string
	for r := range resultsChan {
		foundFiles = append(foundFiles, r.Title)
		if r.SourcePlugin != "local" {
			t.Errorf("Expected SourcePlugin 'local', got %s", r.SourcePlugin)
		}
		if r.URL == "" {
			t.Errorf("Expected URL to be populated, got empty string")
		}
		if r.Snippet == "" {
			t.Errorf("Expected Snippet to be populated, got empty string")
		}
	}

	expectedCount := 3 // note1.md, note2.txt, sub/doc.md
	if len(foundFiles) != expectedCount {
		t.Errorf("Expected %d files, got %d. Files found: %v", expectedCount, len(foundFiles), foundFiles)
	}
}

func TestLocalSearch_InvalidRoot(t *testing.T) {
	ls := New("/this/path/should/not/exist/123456789")
	q, _ := query.Parse("test")
	_, err := ls.Search(context.Background(), q)
	if err == nil {
		t.Errorf("Expected error for non-existent root path, got nil")
	}
}

func TestLocalSearch_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "huge.md")
	os.WriteFile(path, []byte("rust"), 0644)

	ls := New(tmpDir)
	q, _ := query.Parse("rust")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	resultsChan, err := ls.Search(ctx, q)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	// Should not block or return any results
	select {
	case _, ok := <-resultsChan:
		if ok {
			t.Errorf("Expected channel to be closed without results due to cancellation")
		}
	case <-time.After(1 * time.Second):
		t.Errorf("Timeout waiting for channel to close")
	}
}
