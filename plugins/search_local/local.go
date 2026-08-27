package search_local

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/DNahar74/enigma/core/query"
	"github.com/ledongthuc/pdf"
)

// LocalSearch implements the plugin.SearchPlugin interface for local files.
type LocalSearch struct {
	rootPath string
}

// New creates a new LocalSearch plugin pointing to the given root directory.
func New(rootPath string) *LocalSearch {
	return &LocalSearch{
		rootPath: rootPath,
	}
}

// Name returns the name of the plugin.
func (l *LocalSearch) Name() string {
	return "local"
}

// Ping checks if the local search is healthy.
func (l *LocalSearch) Ping(ctx context.Context) error {
	return nil
}

// IsLocal implements plugin.LocalSearchPlugin to identify as a local searcher.
func (l *LocalSearch) IsLocal() bool {
	return true
}

// Search executes the search query against local markdown, text, and PDF files.
func (l *LocalSearch) Search(ctx context.Context, q query.Query) (<-chan query.Result, error) {
	// Validate that the root path exists
	info, err := os.Stat(l.rootPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("local search root path does not exist: %s", l.rootPath)
		}
		return nil, fmt.Errorf("failed to access root path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local search root path is not a directory: %s", l.rootPath)
	}

	results := make(chan query.Result)
	queryTerm := strings.ToLower(q.Raw)

	go func() {
		defer close(results)

		filepath.WalkDir(l.rootPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip errors like permission denied
			}
			if d.IsDir() {
				return nil
			}

			// Check context cancellation before processing a file
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".md" && ext != ".txt" && ext != ".pdf" {
				return nil
			}

			var content string
			if ext == ".pdf" {
				content, err = readPDF(path)
				if err != nil {
					return nil // skip unreadable PDFs
				}
			} else {
				bytes, err := os.ReadFile(path)
				if err != nil {
					return nil // skip unreadable files
				}
				content = string(bytes)
			}

			content = sanitizeText(content)

			lowerContent := strings.ToLower(content)
			tokens := strings.Fields(queryTerm)

			allPresent := true
			firstIdx := -1
			for _, token := range tokens {
				idx := strings.Index(lowerContent, token)
				if idx == -1 {
					allPresent = false
					break
				}
				if firstIdx == -1 || idx < firstIdx {
					firstIdx = idx
				}
			}

			if allPresent && len(tokens) > 0 {
				snippet := extractSnippet(content, firstIdx, 250)

				absPath, _ := filepath.Abs(path)
				fileURL := "file://" + filepath.ToSlash(absPath)

				res := query.Result{
					Title:        d.Name(),
					URL:          fileURL,
					Snippet:      strings.TrimSpace(snippet),
					SourcePlugin: "local",
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case results <- res:
				}
			}
			return nil
		})
	}()

	return results, nil
}

// readPDF uses the ledongthuc/pdf library to extract text from a PDF file.
func readPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	reader, err := r.GetPlainText()
	if err != nil {
		return "", err
	}

	bytes, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// sanitizeText cleans up extracted text for readability.
// Many PDFs produce text with no spaces between words (e.g. "FoundationsandTrendsRin").
// This function:
// 1. Strips non-printable control characters (common PDF artifacts like \x01).
// 2. Inserts spaces at camelCase boundaries (lowercase→uppercase transitions).
// 3. Inserts spaces between a letter and a digit (e.g. "2007J" → "2007 J").
// 4. Collapses all remaining whitespace runs into single spaces.
func sanitizeText(content string) string {
	var b strings.Builder
	runes := []rune(content)
	for i, r := range runes {
		// Strip control characters (except common whitespace)
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			b.WriteRune(' ')
			continue
		}

		if i > 0 {
			prev := runes[i-1]
			// Insert space at camelCase boundaries
			if unicode.IsUpper(r) && unicode.IsLower(prev) {
				b.WriteRune(' ')
			}
			// Insert space between digit→letter or letter→digit
			if (unicode.IsDigit(prev) && unicode.IsLetter(r)) || (unicode.IsLetter(prev) && unicode.IsDigit(r)) {
				b.WriteRune(' ')
			}
		}
		b.WriteRune(r)
	}
	// Collapse whitespace runs into single spaces
	fields := strings.Fields(b.String())
	return strings.Join(fields, " ")
}

// extractSnippet extracts a sentence-aware snippet around the match
func extractSnippet(content string, firstIdx int, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	start := firstIdx - (maxLen / 2)
	if start < 0 {
		start = 0
	} else {
		// scan backwards for a period followed by space
		foundBoundary := false
		for i := firstIdx; i >= start && i > 0; i-- {
			if content[i-1] == '.' && content[i] == ' ' {
				start = i + 1
				foundBoundary = true
				break
			}
		}
		if !foundBoundary {
			// fallback to finding a space
			for i := firstIdx; i >= start; i-- {
				if content[i] == ' ' {
					start = i + 1
					break
				}
			}
		}
	}

	end := start + maxLen
	if end > len(content) {
		end = len(content)
	} else {
		// scan forwards for a space or period
		for i := end; i < len(content); i++ {
			if content[i] == ' ' || content[i] == '.' {
				end = i
				break
			}
		}
	}

	snippet := strings.TrimSpace(content[start:end])
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}
	return snippet
}
