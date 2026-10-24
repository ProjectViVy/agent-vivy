package runtime

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"golang.org/x/net/http/httpproxy"

	"agent-vivy/internal/tools"
)

func testPublicLookup(_ context.Context, host string) ([]net.IPAddr, error) {
	switch host {
	case "internal.test":
		return []net.IPAddr{{IP: net.ParseIP("10.0.0.5")}}, nil
	case "loopback.test":
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	default:
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}
}

func proxyTestNetwork(t *testing.T, proxyURL string) *webNetwork {
	t.Helper()
	network := newWebNetwork(&httpproxy.Config{HTTPProxy: proxyURL, HTTPSProxy: proxyURL})
	network.lookupIP = testPublicLookup
	return network
}

func TestWebFetchUsesConfiguredProxy(t *testing.T) {
	var hits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Scheme != "http" || r.URL.Host != "example.test" {
			t.Errorf("proxy received %s %s, want absolute-form http://example.test", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "proxied body")
	}))
	defer proxy.Close()

	network := proxyTestNetwork(t, proxy.URL)
	backend := newWebFetchBackend(1<<20, nil, network)
	result, err := backend.Fetch(context.Background(), "", tools.WebFetchRequest{URL: "http://example.test/page", Format: "text"})
	if err != nil {
		t.Fatalf("fetch through proxy: %v", err)
	}
	if result.Content != "proxied body" || hits.Load() != 1 {
		t.Fatalf("result = %+v, proxy hits = %d", result, hits.Load())
	}
}

// A NO_PROXY entry must not remove the configured endpoint from the dial allow
// set: NO_PROXY decides whether a request uses a proxy, not whether the proxy
// is reachable.
func TestWebFetchProxiesHostOutsideNoProxy(t *testing.T) {
	var hits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Scheme != "http" || r.URL.Host != "other.test" {
			t.Errorf("proxy received %s %s, want absolute-form http://other.test", r.Method, r.URL)
		}
		_, _ = io.WriteString(w, "proxied body")
	}))
	defer proxy.Close()

	network := newWebNetwork(&httpproxy.Config{HTTPProxy: proxy.URL, NoProxy: "example.com"})
	network.lookupIP = testPublicLookup
	backend := newWebFetchBackend(1<<20, nil, network)
	result, err := backend.Fetch(context.Background(), "", tools.WebFetchRequest{URL: "http://other.test/page", Format: "text"})
	if err != nil {
		t.Fatalf("fetch through proxy: %v", err)
	}
	if result.Content != "proxied body" || hits.Load() != 1 {
		t.Fatalf("result = %+v, proxy hits = %d", result, hits.Load())
	}
}

func TestWebFetchRejectsPrivateTargetBehindProxy(t *testing.T) {
	var hits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, "should not be reached")
	}))
	defer proxy.Close()

	network := proxyTestNetwork(t, proxy.URL)
	backend := newWebFetchBackend(1<<20, nil, network)
	_, err := backend.Fetch(context.Background(), "", tools.WebFetchRequest{URL: "http://loopback.test/", Format: "text"})
	if !errors.Is(err, errPublicTargetDenied) {
		t.Fatalf("fetch error = %v, want errPublicTargetDenied", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("private target reached the proxy %d times", hits.Load())
	}
}

func TestWebFetchRevalidatesRedirectTargets(t *testing.T) {
	var hits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Host == "example.test" {
			http.Redirect(w, r, "http://internal.test/admin", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "internal content")
	}))
	defer proxy.Close()

	network := proxyTestNetwork(t, proxy.URL)
	backend := newWebFetchBackend(1<<20, nil, network)
	_, err := backend.Fetch(context.Background(), "", tools.WebFetchRequest{URL: "http://example.test/", Format: "text"})
	if err == nil || !strings.Contains(err.Error(), "redirect target is not public") {
		t.Fatalf("redirect error = %v, want a public-target refusal", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("redirect hop was sent: proxy hits = %d, want 1", hits.Load())
	}
}
