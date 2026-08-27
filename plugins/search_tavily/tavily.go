package search_tavily

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/DNahar74/enigma/core/query"
)

// TavilySearch implements the SearchPlugin interface for the Tavily Search API.
type TavilySearch struct {
	apiKey     string
	maxResults int
	client     *http.Client
	baseURL    string // overrideable for tests
}

// New creates a new TavilySearch plugin.
func New(apiKey string, maxResults int, timeoutSeconds int) *TavilySearch {
	return &TavilySearch{
		apiKey:     apiKey,
		maxResults: maxResults,
		client: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
		baseURL: "https://api.tavily.com",
	}
}

// Name returns the name of the plugin.
func (s *TavilySearch) Name() string {
	return "tavily"
}

// Ping checks if the Tavily API is reachable and the API key is valid.
func (s *TavilySearch) Ping(ctx context.Context) error {
	// A simple search to verify connectivity and authentication
	q := query.Query{Raw: "test"}
	_, err := s.Search(ctx, q)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	return nil
}

type tavilyRequest struct {
	APIKey        string `json:"api_key"`
	Query         string `json:"query"`
	SearchDepth   string `json:"search_depth"`
	IncludeAnswer bool   `json:"include_answer"`
	MaxResults    int    `json:"max_results"`
}

type tavilyResponse struct {
	Results []tavilyResult `json:"results"`
}

type tavilyResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// Search executes a query against the Tavily Search API and streams results back.
func (s *TavilySearch) Search(ctx context.Context, q query.Query) (<-chan query.Result, error) {
	reqBody := tavilyRequest{
		APIKey:        s.apiKey,
		Query:         q.Raw,
		SearchDepth:   "basic",
		IncludeAnswer: false,
		MaxResults:    s.maxResults,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/search", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		// Drain and close the body to reuse the connection.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tavily API returned status: %s", resp.Status)
	}

	var tavilyResp tavilyResponse
	if err := json.NewDecoder(resp.Body).Decode(&tavilyResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	out := make(chan query.Result)

	go func() {
		defer close(out)

		for _, r := range tavilyResp.Results {
			res := query.Result{
				URL:          r.URL,
				Title:        r.Title,
				Snippet:      r.Content,
				SourcePlugin: s.Name(),
			}

			select {
			case <-ctx.Done():
				return // Context cancelled, stop streaming
			case out <- res:
				// Successfully sent result
			}
		}
	}()

	return out, nil
}
