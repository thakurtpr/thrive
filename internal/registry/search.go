package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// SearchResult is one Docker Hub search hit.
type SearchResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Stars       int    `json:"stars"`
	Pulls       int    `json:"pulls"`
	Official    bool   `json:"official"`
}

// Search queries the Docker Hub registry search API.
func Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("registry: search requires a query")
	}
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	endpoint := "https://hub.docker.com/v2/search/repositories/?query=" +
		url.QueryEscape(query) + fmt.Sprintf("&page_size=%d", limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("registry: search: request: %w", err)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registry: search: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry: search: hub returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("registry: search: read: %w", err)
	}
	var payload struct {
		Results []struct {
			RepoName         string `json:"repo_name"`
			ShortDescription string `json:"short_description"`
			StarCount        int    `json:"star_count"`
			PullCount        int    `json:"pull_count"`
			IsOfficial       bool   `json:"is_official"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("registry: search: parse: %w", err)
	}
	out := make([]SearchResult, 0, len(payload.Results))
	for _, r := range payload.Results {
		out = append(out, SearchResult{
			Name: r.RepoName, Description: r.ShortDescription,
			Stars: r.StarCount, Pulls: r.PullCount, Official: r.IsOfficial,
		})
	}
	return out, nil
}
