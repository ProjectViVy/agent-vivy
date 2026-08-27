package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

const (
	maxSearchResponseBytes = 2 << 20
	maxSearchResults       = 10
)

type SearchProvider interface {
	Name() string
	Search(context.Context, SearchProviderRequest) ([]tools.SearchResult, error)
}

type SearchProviderRequest struct {
	Query      string
	MaxResults int
	Language   string
}

type NetworkSearchService struct {
	client    *http.Client
	providers map[string]SearchProvider
	order     []string
	preferred string
}

var _ tools.SearchOperations = (*NetworkSearchService)(nil)

func NewNetworkSearchService(client *http.Client, endpoints map[string]string) *NetworkSearchService {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	providers := map[string]SearchProvider{}
	providers["bing"] = &bingSearchProvider{searchHTTPProvider: searchHTTPProvider{client: client, endpoint: endpointOr(endpoints, "bing", "https://api.bing.microsoft.com/v7.0/search")}, apiKeyEnv: "BING_SEARCH_API_KEY"}
	providers["google"] = &googleSearchProvider{searchHTTPProvider: searchHTTPProvider{client: client, endpoint: endpointOr(endpoints, "google", "https://customsearch.googleapis.com/customsearch/v1")}, apiKeyEnv: "GOOGLE_SEARCH_API_KEY", cxEnv: "GOOGLE_SEARCH_CX"}
	providers["duckduckgo"] = &duckDuckGoSearchProvider{searchHTTPProvider: searchHTTPProvider{client: client, endpoint: endpointOr(endpoints, "duckduckgo", "https://api.duckduckgo.com/")}}
	providers["searxng"] = &searxNGSearchProvider{searchHTTPProvider: searchHTTPProvider{client: client, endpoint: endpointOr(endpoints, "searxng", os.Getenv("SEARXNG_SEARCH_URL"))}}
	providers["wikipedia"] = &wikipediaSearchProvider{searchHTTPProvider: searchHTTPProvider{client: client, endpoint: endpointOr(endpoints, "wikipedia", "https://en.wikipedia.org/w/api.php")}}
	return &NetworkSearchService{client: client, providers: providers, order: []string{"bing", "google", "duckduckgo", "searxng", "wikipedia"}}
}

// SetPreferredProvider sets the provider used when a request does not name
// one (config tools.network_search.provider / Settings UI). An unusable
// preference — e.g. bing without BING_SEARCH_API_KEY — silently falls back
// to the automatic keyless walk instead of failing the search.
func (s *NetworkSearchService) SetPreferredProvider(name string) {
	s.preferred = strings.ToLower(strings.TrimSpace(name))
}

// NetworkSearchProviderInfo is the non-secret per-provider status surfaced
// to the Settings UI: whether it needs a key, whether that key (or the
// searxng URL) is present, and which environment variable to set.
type NetworkSearchProviderInfo struct {
	Name       string `json:"name"`
	Keyless    bool   `json:"keyless"`
	Configured bool   `json:"configured"`
	EnvKey     string `json:"env_key,omitempty"`
}

// searchProviderEnvNames carries environment variable NAMES only; the
// presence check reads emptiness, never values. Credential values are read
// exclusively by the provider implementations (D-010 secret audit: no
// literal key Getenv outside internal/provider).
var searchProviderEnvNames = map[string][]string{
	"bing":    {"BING_SEARCH_API_KEY"},
	"google":  {"GOOGLE_SEARCH_API_KEY", "GOOGLE_SEARCH_CX"},
	"searxng": {"SEARXNG_SEARCH_URL"},
}

// NetworkSearchProviderAvailability reports the provider roster in
// preference order. It reads only environment variable presence, never
// values (D-010).
func NetworkSearchProviderAvailability() []NetworkSearchProviderInfo {
	roster := []NetworkSearchProviderInfo{
		{Name: "bing", EnvKey: "BING_SEARCH_API_KEY"},
		{Name: "google", EnvKey: "GOOGLE_SEARCH_API_KEY"},
		{Name: "duckduckgo", Keyless: true, Configured: true},
		{Name: "searxng", EnvKey: "SEARXNG_SEARCH_URL"},
		{Name: "wikipedia", Keyless: true, Configured: true},
	}
	for i := range roster {
		envNames := searchProviderEnvNames[roster[i].Name]
		if len(envNames) == 0 {
			continue
		}
		configured := true
		for _, name := range envNames {
			if os.Getenv(name) == "" {
				configured = false
				break
			}
		}
		roster[i].Configured = configured
	}
	return roster
}

// usable reports whether the named provider can serve a request right now.
func (s *NetworkSearchService) usable(name string) bool {
	if p := s.providers[name]; p == nil {
		return false
	}
	for _, info := range NetworkSearchProviderAvailability() {
		if info.Name == name {
			return info.Configured
		}
	}
	return false
}

func (s *NetworkSearchService) Search(ctx context.Context, _ domain.RunID, req tools.SearchRequest) (tools.SearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return tools.SearchResponse{}, errors.New("network search: query must not be empty")
	}
	providerName := strings.ToLower(strings.TrimSpace(req.Provider))
	if providerName == "" && s.preferred != "" && s.usable(s.preferred) {
		providerName = s.preferred
	}
	if providerName == "" {
		for _, name := range s.order {
			if p := s.providers[name]; p != nil && (name == "duckduckgo" || name == "wikipedia" || os.Getenv(apiKeyEnvFor(name)) != "") {
				providerName = name
				break
			}
		}
	}
	p, ok := s.providers[providerName]
	if !ok || p == nil {
		return tools.SearchResponse{}, fmt.Errorf("network search: provider %q is not allowlisted or configured", providerName)
	}
	maxResults := req.MaxResults
	if maxResults <= 0 || maxResults > maxSearchResults {
		maxResults = maxSearchResults
	}
	results, err := p.Search(ctx, SearchProviderRequest{Query: query, MaxResults: maxResults, Language: req.Language})
	if err != nil {
		return tools.SearchResponse{}, err
	}
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	for i := range results {
		results[i].Provider = providerName
		results[i].Untrusted = true
	}
	return tools.SearchResponse{Query: query, Provider: providerName, Results: results, Untrusted: true}, nil
}

type searchHTTPProvider struct {
	client   *http.Client
	endpoint string
}

func (p *searchHTTPProvider) get(ctx context.Context, params url.Values, headers map[string]string, out any) error {
	if p.endpoint == "" {
		return errors.New("search provider endpoint is not configured")
	}
	u, err := url.Parse(p.endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("search provider endpoint must be an http(s) URL")
	}
	query := u.Query()
	for key, values := range params {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("search provider request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("search provider rate limited")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("search provider returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSearchResponseBytes+1))
	if err != nil {
		return err
	}
	if len(body) > maxSearchResponseBytes {
		return errors.New("search provider response exceeds size limit")
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("search provider JSON: %w", err)
	}
	return nil
}

type bingSearchProvider struct {
	searchHTTPProvider
	apiKeyEnv string
}

func (p *bingSearchProvider) Name() string { return "bing" }
func (p *bingSearchProvider) Search(ctx context.Context, req SearchProviderRequest) ([]tools.SearchResult, error) {
	key := os.Getenv(p.apiKeyEnv)
	if key == "" {
		return nil, errors.New("bing search API key is not configured")
	}
	// Decode the fields separately because Bing returns lower-case JSON keys.
	var raw struct {
		WebPages struct {
			Value []struct {
				Name    string `json:"name"`
				URL     string `json:"url"`
				Snippet string `json:"snippet"`
			} `json:"value"`
		} `json:"webPages"`
	}
	if err := p.get(ctx, url.Values{"q": {req.Query}, "count": {fmt.Sprint(req.MaxResults)}}, map[string]string{"Ocp-Apim-Subscription-Key": key}, &raw); err != nil {
		return nil, err
	}
	out := make([]tools.SearchResult, 0, len(raw.WebPages.Value))
	for _, item := range raw.WebPages.Value {
		out = append(out, tools.SearchResult{Title: item.Name, URL: item.URL, Snippet: item.Snippet})
	}
	return normalizeSearchResults(out), nil
}

type googleSearchProvider struct {
	searchHTTPProvider
	apiKeyEnv, cxEnv string
}

func (p *googleSearchProvider) Name() string { return "google" }
func (p *googleSearchProvider) Search(ctx context.Context, req SearchProviderRequest) ([]tools.SearchResult, error) {
	key, cx := os.Getenv(p.apiKeyEnv), os.Getenv(p.cxEnv)
	if key == "" || cx == "" {
		return nil, errors.New("google search API key or cx is not configured")
	}
	var raw struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}
	if err := p.get(ctx, url.Values{"q": {req.Query}, "num": {fmt.Sprint(req.MaxResults)}, "key": {key}, "cx": {cx}}, nil, &raw); err != nil {
		return nil, err
	}
	out := make([]tools.SearchResult, 0, len(raw.Items))
	for _, item := range raw.Items {
		out = append(out, tools.SearchResult{Title: item.Title, URL: item.Link, Snippet: item.Snippet})
	}
	return normalizeSearchResults(out), nil
}

type duckDuckGoSearchProvider struct{ searchHTTPProvider }

func (p *duckDuckGoSearchProvider) Name() string { return "duckduckgo" }
func (p *duckDuckGoSearchProvider) Search(ctx context.Context, req SearchProviderRequest) ([]tools.SearchResult, error) {
	var raw struct {
		Heading       string `json:"Heading"`
		AbstractText  string `json:"AbstractText"`
		AbstractURL   string `json:"AbstractURL"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
			Topics   []any  `json:"Topics"`
		} `json:"RelatedTopics"`
	}
	if err := p.get(ctx, url.Values{"q": {req.Query}, "format": {"json"}, "no_html": {"1"}, "skip_disambig": {"1"}}, nil, &raw); err != nil {
		return nil, err
	}
	var out []tools.SearchResult
	if raw.AbstractURL != "" {
		out = append(out, tools.SearchResult{Title: raw.Heading, URL: raw.AbstractURL, Snippet: raw.AbstractText})
	}
	for _, item := range raw.RelatedTopics {
		if item.FirstURL != "" {
			out = append(out, tools.SearchResult{Title: item.Text, URL: item.FirstURL, Snippet: item.Text})
		}
	}
	return normalizeSearchResults(out), nil
}

type searxNGSearchProvider struct{ searchHTTPProvider }

func (p *searxNGSearchProvider) Name() string { return "searxng" }
func (p *searxNGSearchProvider) Search(ctx context.Context, req SearchProviderRequest) ([]tools.SearchResult, error) {
	var actual struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := p.get(ctx, url.Values{"q": {req.Query}, "format": {"json"}, "language": {req.Language}}, nil, &actual); err != nil {
		return nil, err
	}
	out := make([]tools.SearchResult, 0, len(actual.Results))
	for _, item := range actual.Results {
		out = append(out, tools.SearchResult{Title: item.Title, URL: item.URL, Snippet: item.Content})
	}
	return normalizeSearchResults(out), nil
}

type wikipediaSearchProvider struct{ searchHTTPProvider }

func (p *wikipediaSearchProvider) Name() string { return "wikipedia" }
func (p *wikipediaSearchProvider) Search(ctx context.Context, req SearchProviderRequest) ([]tools.SearchResult, error) {
	var raw struct {
		Query struct {
			Search []struct {
				Title   string `json:"title"`
				Snippet string `json:"snippet"`
				PageID  int    `json:"pageid"`
			} `json:"search"`
		} `json:"query"`
	}
	if err := p.get(ctx, url.Values{"action": {"query"}, "list": {"search"}, "srsearch": {req.Query}, "format": {"json"}, "srlimit": {fmt.Sprint(req.MaxResults)}}, nil, &raw); err != nil {
		return nil, err
	}
	out := make([]tools.SearchResult, 0, len(raw.Query.Search))
	for _, item := range raw.Query.Search {
		out = append(out, tools.SearchResult{Title: item.Title, URL: fmt.Sprintf("https://en.wikipedia.org/?curid=%d", item.PageID), Snippet: stripHTML(item.Snippet)})
	}
	return normalizeSearchResults(out), nil
}

func normalizeSearchResults(results []tools.SearchResult) []tools.SearchResult {
	out := results[:0]
	seen := make(map[string]struct{}, len(results))
	for _, item := range results {
		item.Title, item.URL, item.Snippet = strings.TrimSpace(item.Title), strings.TrimSpace(item.URL), strings.TrimSpace(item.Snippet)
		u, err := url.Parse(item.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || item.Title == "" {
			continue
		}
		if len(item.Title) > 512 {
			item.Title = item.Title[:safeUTF8Prefix(item.Title, 512)]
		}
		if len(item.Snippet) > 2048 {
			item.Snippet = item.Snippet[:safeUTF8Prefix(item.Snippet, 2048)]
		}
		if _, ok := seen[item.URL]; ok {
			continue
		}
		seen[item.URL] = struct{}{}
		out = append(out, item)
	}
	return out
}

func endpointOr(endpoints map[string]string, name, fallback string) string {
	if value := strings.TrimSpace(endpoints[name]); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}
func apiKeyEnvFor(name string) string {
	switch name {
	case "bing":
		return "BING_SEARCH_API_KEY"
	case "google":
		return "GOOGLE_SEARCH_API_KEY"
	default:
		return ""
	}
}
func stripHTML(value string) string {
	var out strings.Builder
	inside := false
	for _, r := range value {
		if r == '<' {
			inside = true
			continue
		}
		if r == '>' {
			inside = false
			continue
		}
		if !inside {
			out.WriteRune(r)
		}
	}
	return out.String()
}
