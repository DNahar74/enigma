package search_marginalia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
)

const (
	defaultEndpoint = "https://api2.marginalia-search.com/search"
)

// MarginaliaSearch implements plugin.SearchPlugin against the Marginalia Search API.
type MarginaliaSearch struct {
	apiKey     string
	maxResults int
	client     *http.Client
	endpoint   string
}

// New creates a new MarginaliaSearch plugin. If apiKey is empty, it uses "public".
func New(apiKey string, maxResults int, timeoutSeconds int) *MarginaliaSearch {
	if apiKey == "" {
		apiKey = "public"
	}
	return &MarginaliaSearch{
		apiKey:     apiKey,
		maxResults: maxResults,
		client: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
		endpoint: defaultEndpoint,
	}
}

func (s *MarginaliaSearch) Name() string { return "marginalia" }

type marginaliaResponse struct {
	Results []struct {
		Title       string `json:"title"`
		URL         string `json:"url"`
		Description string `json:"description"`
	} `json:"results"`
}

func (s *MarginaliaSearch) Search(ctx context.Context, q query.Query) (<-chan query.Result, error) {
	out := make(chan query.Result)

	reqURL, err := url.Parse(s.endpoint)
	if err != nil {
		return nil, fmt.Errorf("marginalia: invalid endpoint URL: %w", err)
	}

	qParams := reqURL.Query()
	qParams.Set("query", q.Raw)
	qParams.Set("count", strconv.Itoa(s.maxResults))
	reqURL.RawQuery = qParams.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("marginalia: failed to create request: %w", err)
	}

	req.Header.Set("API-Key", s.apiKey)
	req.Header.Set("Accept", "application/json")

	go func() {
		defer close(out)

		resp, err := s.client.Do(req)
		if err != nil {
			// In fan-out, we don't want to crash the whole pipeline if one provider fails,
			// but Search() returns a channel. We log or just stop sending.
			// Currently our pipeline expects to just drain what's there.
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return
		}

		var mResp marginaliaResponse
		if err := json.NewDecoder(resp.Body).Decode(&mResp); err != nil {
			return
		}

		for _, r := range mResp.Results {
			res := query.Result{
				Title:        r.Title,
				URL:          r.URL,
				Snippet:      r.Description,
				SourcePlugin: "marginalia",
			}

			select {
			case out <- res:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

func (s *MarginaliaSearch) Ping(ctx context.Context) error {
	reqURL, err := url.Parse(s.endpoint)
	if err != nil {
		return err
	}
	q := reqURL.Query()
	q.Set("query", "test")
	q.Set("count", "1")
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("API-Key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("marginalia ping failed: status %d", resp.StatusCode)
	}
	return nil
}

// Compile-time check
var _ plugin.SearchPlugin = (*MarginaliaSearch)(nil)
