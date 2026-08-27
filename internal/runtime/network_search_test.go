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

// TestNetworkSearchServicePrefersConfiguredProvider pins the config/settings
// preferred-provider hook: a usable preference wins when the request names
// no provider, and an unusable preference falls back to the automatic
// keyless walk instead of failing the search.
func TestNetworkSearchServicePrefersConfiguredProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/searxng":
			_, _ = w.Write([]byte(`{"results":[{"title":"Go","url":"https://go.dev/","content":"A language"}]}`))
		case "/duckduckgo":
			_, _ = w.Write([]byte(`{"Heading":"Go","AbstractURL":"https://go.dev/","AbstractText":"A language"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	service := NewNetworkSearchService(server.Client(), map[string]string{
		"searxng":    server.URL + "/searxng",
		"duckduckgo": server.URL + "/duckduckgo",
	})

	// Keyless preference: searxng is usable without any env setup.
	t.Setenv("SEARXNG_SEARCH_URL", server.URL+"/searxng")
	service.SetPreferredProvider("searxng")
	response, err := service.Search(context.Background(), "", tools.SearchRequest{Query: "golang"})
	if err != nil || response.Provider != "searxng" || len(response.Results) != 1 {
		t.Fatalf("preferred searxng response=%#v err=%v", response, err)
	}

	// Unusable preference (bing has no key configured): must degrade to the
	// keyless duckduckgo walk, not fail.
	t.Setenv("BING_SEARCH_API_KEY", "")
	service.SetPreferredProvider("bing")
	response, err = service.Search(context.Background(), "", tools.SearchRequest{Query: "golang"})
	if err != nil || response.Provider != "duckduckgo" || len(response.Results) != 1 {
		t.Fatalf("unusable bing fallback response=%#v err=%v", response, err)
	}

	// An explicit request provider always wins over the preference.
	service.SetPreferredProvider("searxng")
	response, err = service.Search(context.Background(), "", tools.SearchRequest{Query: "golang", Provider: "duckduckgo"})
	if err != nil || response.Provider != "duckduckgo" {
		t.Fatalf("explicit provider response=%#v err=%v", response, err)
	}
}

// TestNetworkSearchProviderAvailability pins the D-010 surface: the roster
// reports env-key presence only, never values, and keyless providers are
// always configured.
func TestNetworkSearchProviderAvailability(t *testing.T) {
	t.Setenv("BING_SEARCH_API_KEY", "")
	t.Setenv("GOOGLE_SEARCH_API_KEY", "")
	t.Setenv("GOOGLE_SEARCH_CX", "")
	t.Setenv("SEARXNG_SEARCH_URL", "")
	roster := NetworkSearchProviderAvailability()
	byName := make(map[string]NetworkSearchProviderInfo, len(roster))
	for _, info := range roster {
		byName[info.Name] = info
	}
	if len(roster) != 5 {
		t.Fatalf("roster = %d providers, want 5", len(roster))
	}
	if info := byName["bing"]; info.Keyless || info.Configured || info.EnvKey != "BING_SEARCH_API_KEY" {
		t.Fatalf("bing (no key) = %+v", info)
	}
	for _, name := range []string{"duckduckgo", "wikipedia"} {
		if info := byName[name]; !info.Keyless || !info.Configured || info.EnvKey != "" {
			t.Fatalf("%s = %+v, want keyless+configured", name, info)
		}
	}

	t.Setenv("BING_SEARCH_API_KEY", "only-presence-matters")
	if info := byName["bing"]; !info.Configured {
		// Fresh roster to re-read env presence.
		for _, fresh := range NetworkSearchProviderAvailability() {
			if fresh.Name == "bing" {
				info = fresh
			}
		}
		if !info.Configured {
			t.Fatalf("bing with key present = %+v", info)
		}
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
