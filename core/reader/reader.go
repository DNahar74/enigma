package reader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/glamour"

	"github.com/DNahar74/enigma/core/render"
)

// FetchAndRender downloads a URL, cleans the HTML, converts it to Markdown,
// fetches images concurrently, and renders it into styled ANSI terminal text.
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

	// Intercept Images
	var imgTokens = make(map[string]string)
	var wg sync.WaitGroup
	var mu sync.Mutex
	imgIdx := 1

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists || strings.HasPrefix(src, "data:") {
			return
		}

		// Handle relative URLs
		if strings.HasPrefix(src, "/") {
			if strings.HasSuffix(targetURL, "/") {
				src = targetURL[:len(targetURL)-1] + src
			} else {
				// Naive handling for simplicity
				src = targetURL + src
			}
		}

		token := fmt.Sprintf("__ENIGMA_IMG_%d__", imgIdx)
		imgIdx++

		// Replace <img> with simple text node placeholder
		s.ReplaceWithHtml(token)

		wg.Add(1)
		go func(t string, url string) {
			defer wg.Done()

			imgReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			imgResp, err := client.Do(imgReq)
			if err != nil || imgResp.StatusCode != 200 {
				return
			}
			defer imgResp.Body.Close()

			imgBytes, err := io.ReadAll(imgResp.Body)
			if err != nil {
				return
			}

			asciiArt := render.RenderImage(imgBytes, width)
			mu.Lock()
			imgTokens[t] = asciiArt
			mu.Unlock()
		}(token, src)
	})

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

	// Wait for background image downloads to complete (or timeout)
	wg.Wait()

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

	// Stitch images back into the rendered ANSI output
	for token, asciiArt := range imgTokens {
		rendered = strings.ReplaceAll(rendered, token, "\n\n"+asciiArt+"\n\n")
	}

	// Remove any unmatched tokens in case image download failed
	for i := 1; i < imgIdx; i++ {
		t := fmt.Sprintf("__ENIGMA_IMG_%d__", i)
		rendered = strings.ReplaceAll(rendered, t, "[Image Failed to Load]")
	}

	return rendered, nil
}
