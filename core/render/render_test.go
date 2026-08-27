package render

import (
	"testing"

	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestHighlightText(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	text := "This is a simple test"
	tokens := []string{"simple"}

	highlighted := highlightText(text, tokens)
	// Lipgloss uses ANSI escape codes, so we just check if it's longer than the original
	if len(highlighted) <= len(text) {
		t.Errorf("expected highlighted text to be longer (contain ANSI codes), got: %s", highlighted)
	}
}

func TestResultRender(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	q, _ := query.Parse("test")
	r := plugin.ScoredResult{
		Result: query.Result{
			Title:        "Test Title",
			URL:          "https://test.com",
			Snippet:      "A simple test.",
			SourcePlugin: "local",
		},
		Score: 10.5,
		Breakdown: map[string]float64{
			"bm25": 10.5,
		},
	}

	out := Result(r, q, 1, true)
	if out == "" {
		t.Errorf("expected non-empty output")
	}
}
