package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

	urlStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))

	scoreStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFF00"))

	highlightStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF00"))

	explainHeaderStyle = lipgloss.NewStyle().
				Italic(true).
				Foreground(lipgloss.Color("#AAAAAA"))

	explainKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	explainValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CCCCCC"))
)

// Result renders a single search result as a styled string.
func Result(r plugin.ScoredResult, q query.Query, index int, explain bool) string {
	var sb strings.Builder

	emoji := "🌐"
	if r.Result.SourcePlugin == "local" {
		emoji = "📝"
	}

	// 1. Title Line
	titleLine := fmt.Sprintf("%d. %s %s", index, emoji, r.Result.Title)
	sb.WriteString(titleStyle.Render(titleLine) + "\n")

	// 2. URL Line
	urlLine := urlStyle.Render(r.Result.URL) + " " + scoreStyle.Render(fmt.Sprintf("(Score: %.2f)", r.Score))
	sb.WriteString(urlLine + "\n")

	// 3. Snippet with highlights
	snippet := highlightText(r.Result.Snippet, q.Tokens)
	sb.WriteString(snippet + "\n")

	// 4. Explain breakdown
	if explain && len(r.Breakdown) > 0 {
		sb.WriteString("\n" + explainHeaderStyle.Render("Score Breakdown:") + "\n")

		// Sort keys for deterministic output
		var keys []string
		for k := range r.Breakdown {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			val := r.Breakdown[k]
			line := fmt.Sprintf("  %s %s",
				explainKeyStyle.Render(fmt.Sprintf("%-10s", k+":")),
				explainValueStyle.Render(fmt.Sprintf("%.2f", val)),
			)
			sb.WriteString(line + "\n")
		}
	}

	return sb.String()
}

// highlightText wraps occurrences of tokens in the given text with lipgloss styling.
func highlightText(text string, tokens []string) string {
	for _, token := range tokens {
		if token == "" {
			continue
		}
		lowerToken := strings.ToLower(token)
		idx := 0
		for {
			lowerText := strings.ToLower(text)
			i := strings.Index(lowerText[idx:], lowerToken)
			if i == -1 {
				break
			}
			actualStart := idx + i
			actualEnd := actualStart + len(token)

			orig := text[actualStart:actualEnd]
			replacement := highlightStyle.Render(orig)

			text = text[:actualStart] + replacement + text[actualEnd:]
			idx = actualStart + len(replacement)
		}
	}
	return text
}
