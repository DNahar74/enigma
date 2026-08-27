package reader

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/glamour"
)

// FetchAndRender downloads a URL, cleans the HTML, converts it to Markdown,
// and renders it into styled ANSI terminal text using Glamour.
func FetchAndRender(ctx context.Context, targetURL string, width int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	req.Header.Set("User-Agent", "Enigma-Terminal-Reader/0.1")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Clean up noisy tags
	doc.Find("script, style, nav, footer, aside, header, iframe, noscript").Remove()

	// Try to find main content block, fallback to body
	var content *goquery.Selection
	if doc.Find("article").Length() > 0 {
		content = doc.Find("article").First()
	} else if doc.Find("main").Length() > 0 {
		content = doc.Find("main").First()
	} else {
		content = doc.Find("body")
	}

	htmlString, err := content.Html()
	if err != nil || strings.TrimSpace(htmlString) == "" {
		return "", fmt.Errorf("failed to extract article content")
	}

	// Convert to Markdown
	mdText, err := htmltomarkdown.ConvertString(htmlString)
	if err != nil {
		return "", fmt.Errorf("failed to convert HTML to Markdown: %w", err)
	}

	// Add title
	title := doc.Find("title").Text()
	if title != "" {
		mdText = fmt.Sprintf("# %s\n\n%s", strings.TrimSpace(title), mdText)
	}

	// Render with Glamour
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", fmt.Errorf("failed to initialize markdown renderer: %w", err)
	}

	rendered, err := r.Render(mdText)
	if err != nil {
		return "", fmt.Errorf("failed to render markdown: %w", err)
	}

	return rendered, nil
}
