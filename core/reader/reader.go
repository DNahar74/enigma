package reader

import (
	"bytes"
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
	if err != nil {
		// If the DOM is too deep (exceeds 512 nodes), strip structural tags to flatten it
		if strings.Contains(err.Error(), "exceeds") {
			flatHTML := string(bodyBytes)
			flatHTML = strings.ReplaceAll(flatHTML, "<div>", "")
			flatHTML = strings.ReplaceAll(flatHTML, "</div>", "")
			flatHTML = strings.ReplaceAll(flatHTML, "<span>", "")
			flatHTML = strings.ReplaceAll(flatHTML, "</span>", "")
			doc, err = goquery.NewDocumentFromReader(strings.NewReader(flatHTML))
		}

		if err != nil {
			return "", fmt.Errorf("failed to parse HTML: %w", err)
		}
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

			imgReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return
			}
			imgReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			imgReq.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
			imgReq.Header.Set("Referer", targetURL)

			imgResp, err := client.Do(imgReq)
			if err != nil {
				return
			}
			defer imgResp.Body.Close()
			if imgResp.StatusCode != 200 {
				return
			}

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
