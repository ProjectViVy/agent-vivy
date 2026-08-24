package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-vivy/internal/tools"
)

func TestNetworkSearchServiceNormalizesConfiguredProviders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/duckduckgo":
			_, _ = w.Write([]byte(`{"Heading":"Go","AbstractText":"A language","AbstractURL":"https://go.dev/","RelatedTopics":[{"Text":"Go tour","FirstURL":"https://go.dev/tour/"}]}`))
		case "/searxng":
			_, _ = w.Write([]byte(`{"results":[{"title":"Go","url":"https://go.dev/","content":"A language"},{"title":"Go","url":"https://go.dev/","content":"duplicate"}]}`))
		case "/wikipedia":
			_, _ = w.Write([]byte(`{"query":{"search":[{"title":"Go","pageid":123,"snippet":"<b>language</b>"}]}}`))
		case "/bing":
			_, _ = w.Write([]byte(`{"webPages":{"value":[{"name":"Go","url":"https://go.dev/","snippet":"A language"}]}}`))
		case "/google":
			_, _ = w.Write([]byte(`{"items":[{"title":"Go","link":"https://go.dev/","snippet":"A language"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("BING_SEARCH_API_KEY", "test-bing-key")
	t.Setenv("GOOGLE_SEARCH_API_KEY", "test-google-key")
	t.Setenv("GOOGLE_SEARCH_CX", "test-cx")
	service := NewNetworkSearchService(server.Client(), map[string]string{
		"duckduckgo": server.URL + "/duckduckgo",
		"searxng":    server.URL + "/searxng",
		"wikipedia":  server.URL + "/wikipedia",
		"bing":       server.URL + "/bing",
		"google":     server.URL + "/google",
	})

	for _, provider := range []string{"duckduckgo", "searxng", "wikipedia", "bing", "google"} {
		response, err := service.Search(context.Background(), "", tools.SearchRequest{Query: "golang", Provider: provider, MaxResults: 3})
		if err != nil {
			t.Fatalf("%s search: %v", provider, err)
		}
		if response.Provider != provider || !response.Untrusted || len(response.Results) == 0 {
			t.Fatalf("%s response = %#v", provider, response)
		}
		if response.Results[0].Provider != provider || !response.Results[0].Untrusted {
			t.Fatalf("%s result provenance = %#v", provider, response.Results[0])
		}
	}

	response, err := service.Search(context.Background(), "", tools.SearchRequest{Query: "golang", Provider: "searxng", MaxResults: 10})
	if err != nil || len(response.Results) != 1 {
		t.Fatalf("duplicate normalization response=%#v err=%v", response, err)
	}
	if response.Results[0].Snippet != "A language" {
		t.Fatalf("unexpected normalized snippet: %#v", response.Results[0])
	}
}

func TestNetworkSearchServiceDefaultsToSafePublicProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/duckduckgo" {
			t.Fatalf("default provider requested %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Heading":"Go","AbstractURL":"https://go.dev/","AbstractText":"A language"}`))
	}))
	defer server.Close()
	t.Setenv("BING_SEARCH_API_KEY", "")
	t.Setenv("GOOGLE_SEARCH_API_KEY", "")
	t.Setenv("GOOGLE_SEARCH_CX", "")
	service := NewNetworkSearchService(server.Client(), map[string]string{"duckduckgo": server.URL + "/duckduckgo"})
	response, err := service.Search(context.Background(), "", tools.SearchRequest{Query: "golang"})
	if err != nil || response.Provider != "duckduckgo" || len(response.Results) != 1 {
		t.Fatalf("default response=%#v err=%v", response, err)
	}
}

func TestNetworkSearchServiceRejectsInvalidAndOversizedResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/too-many":
			w.WriteHeader(http.StatusTooManyRequests)
		case "/large":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Heading":"` + strings.Repeat("x", maxSearchResponseBytes) + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	service := NewNetworkSearchService(server.Client(), map[string]string{"duckduckgo": server.URL + "/too-many"})
	if _, err := service.Search(context.Background(), "", tools.SearchRequest{Query: "x", Provider: "not-allowlisted"}); err == nil || !strings.Contains(err.Error(), "allowlisted") {
		t.Fatalf("invalid provider error = %v", err)
	}
	if _, err := service.Search(context.Background(), "", tools.SearchRequest{Query: "x", Provider: "duckduckgo"}); err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("rate limit error = %v", err)
	}
	large := NewNetworkSearchService(server.Client(), map[string]string{"duckduckgo": server.URL + "/large"})
	if _, err := large.Search(context.Background(), "", tools.SearchRequest{Query: "x", Provider: "duckduckgo"}); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("size limit error = %v", err)
	}
}
