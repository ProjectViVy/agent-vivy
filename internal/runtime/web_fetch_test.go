package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func newWebFetchTestBackend(t *testing.T, maxBodyBytes int) *EinoWebFetchBackend {
	t.Helper()
	sandbox, err := NewSandboxManager(
		domain.SandboxModeDangerFullAccess,
		t.TempDir(),
		nil,
		&domain.NetworkPolicy{DenyPrivateIPs: false},
	)
	if err != nil {
		t.Fatalf("new sandbox manager: %v", err)
	}
	backend := NewEinoWebFetchBackend(maxBodyBytes, sandbox)
	backend.allowLoopbackForTest()
	return backend
}

const webFetchHTMLFixture = `<!doctype html><html><head><title>Page</title>` +
	`<script>evil()</script><style>.x{color:red}</style></head>` +
	`<body><h1>Hello</h1>` + "\n" + `<p>World <a href="/next">link</a>.</p>` +
	`<nav>navnoise</nav><script>more()</script></body></html>`

func TestWebFetchFormatsHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(webFetchHTMLFixture))
	}))
	defer server.Close()
	backend := newWebFetchTestBackend(t, 1<<20)
	ctx := context.Background()

	fetched, err := backend.Fetch(ctx, "run_web_fetch", tools.WebFetchRequest{URL: server.URL})
	if err != nil {
		t.Fatalf("fetch markdown: %v", err)
	}
	if fetched.Format != "markdown" || fetched.StatusCode != http.StatusOK {
		t.Fatalf("unexpected result meta: %+v", fetched)
	}
	if !strings.Contains(fetched.Content, "Hello") || !strings.Contains(fetched.Content, "link") {
		t.Fatalf("markdown lost body content: %q", fetched.Content)
	}
	if strings.Contains(fetched.Content, "navnoise") || strings.Contains(fetched.Content, "evil") {
		t.Fatalf("markdown kept noise: %q", fetched.Content)
	}
	if !fetched.Untrusted || fetched.Truncated {
		t.Fatalf("unexpected trust/truncation flags: %+v", fetched)
	}

	text, err := backend.Fetch(ctx, "run_web_fetch", tools.WebFetchRequest{URL: server.URL, Format: "text"})
	if err != nil {
		t.Fatalf("fetch text: %v", err)
	}
	if text.Format != "text" || !strings.Contains(text.Content, "Hello World link") {
		t.Fatalf("unexpected text result: %+v", text)
	}

	html, err := backend.Fetch(ctx, "run_web_fetch", tools.WebFetchRequest{URL: server.URL, Format: "html"})
	if err != nil {
		t.Fatalf("fetch html: %v", err)
	}
	if html.Format != "html" || !strings.Contains(html.Content, "<h1>") {
		t.Fatalf("unexpected html result: %+v", html)
	}
	if strings.Contains(html.Content, "<script>") {
		t.Fatalf("html format kept scripts: %q", html.Content)
	}
}

func TestWebFetchPrettyPrintsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"a":1,"b":[2,3]}`))
	}))
	defer server.Close()
	backend := newWebFetchTestBackend(t, 1<<20)

	fetched, err := backend.Fetch(context.Background(), "run_web_fetch", tools.WebFetchRequest{URL: server.URL})
	if err != nil {
		t.Fatalf("fetch json: %v", err)
	}
	if fetched.Format != "text" || !strings.Contains(fetched.Content, "\n  \"a\": 1") {
		t.Fatalf("json not pretty printed: %+v", fetched)
	}
}

func TestWebFetchNon2xxIsBoundedResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("missing page"))
	}))
	defer server.Close()
	backend := newWebFetchTestBackend(t, 1<<20)

	fetched, err := backend.Fetch(context.Background(), "run_web_fetch", tools.WebFetchRequest{URL: server.URL})
	if err != nil {
		t.Fatalf("non-2xx should be a result, got error: %v", err)
	}
	if fetched.StatusCode != http.StatusNotFound || !strings.Contains(fetched.Content, "missing page") {
		t.Fatalf("unexpected non-2xx result: %+v", fetched)
	}
}

func TestWebFetchTruncatesOversizedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strings.Repeat("x", 4096)))
	}))
	defer server.Close()
	backend := newWebFetchTestBackend(t, 64)

	fetched, err := backend.Fetch(context.Background(), "run_web_fetch", tools.WebFetchRequest{URL: server.URL})
	if err != nil {
		t.Fatalf("fetch oversized: %v", err)
	}
	if !fetched.Truncated || !strings.HasSuffix(fetched.Content, "[Content truncated to 64 bytes]") {
		t.Fatalf("expected truncation marker: %+v", fetched)
	}
}

func TestWebFetchSecurityGuards(t *testing.T) {
	t.Run("private target refused without the test seam", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
		defer server.Close()
		sandbox, err := NewSandboxManager(domain.SandboxModeDangerFullAccess, t.TempDir(), nil, &domain.NetworkPolicy{DenyPrivateIPs: false})
		if err != nil {
			t.Fatalf("new sandbox manager: %v", err)
		}
		backend := NewEinoWebFetchBackend(1<<20, sandbox)
		if _, err := backend.Fetch(context.Background(), "run_web_fetch", tools.WebFetchRequest{URL: server.URL}); err == nil || !strings.Contains(err.Error(), "private or local") {
			t.Fatalf("expected private-address refusal, got %v", err)
		}
	})
	t.Run("credential-like query refused", func(t *testing.T) {
		backend := newWebFetchTestBackend(t, 1<<20)
		_, err := backend.Fetch(context.Background(), "run_web_fetch", tools.WebFetchRequest{URL: "https://example.com/path?api_key=secret"})
		if err == nil || !strings.Contains(err.Error(), "credential") {
			t.Fatalf("expected credential refusal, got %v", err)
		}
	})
	t.Run("binary content refused", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte{0x00, 0x01})
		}))
		defer server.Close()
		backend := newWebFetchTestBackend(t, 1<<20)
		_, err := backend.Fetch(context.Background(), "run_web_fetch", tools.WebFetchRequest{URL: server.URL})
		if err == nil || !strings.Contains(err.Error(), "download") {
			t.Fatalf("expected binary refusal pointing at download, got %v", err)
		}
	})
	t.Run("redirect carrying credentials refused", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", "/final?token=secret")
			w.WriteHeader(http.StatusFound)
		}))
		defer server.Close()
		backend := newWebFetchTestBackend(t, 1<<20)
		_, err := backend.Fetch(context.Background(), "run_web_fetch", tools.WebFetchRequest{URL: server.URL})
		if err == nil || !strings.Contains(err.Error(), "redirect") {
			t.Fatalf("expected redirect refusal, got %v", err)
		}
	})
	t.Run("invalid URL refused", func(t *testing.T) {
		backend := newWebFetchTestBackend(t, 1<<20)
		for _, raw := range []string{"ftp://example.com/x", "https://user:pass@example.com/", "not a url"} {
			if _, err := backend.Fetch(context.Background(), "run_web_fetch", tools.WebFetchRequest{URL: raw}); err == nil {
				t.Fatalf("expected refusal for %q", raw)
			}
		}
	})
}

func TestClampFetchTimeout(t *testing.T) {
	cases := []struct {
		seconds int
		want    time.Duration
	}{
		{0, defaultFetchTimeout},
		{-5, defaultFetchTimeout},
		{1, minFetchTimeout},
		{30, 30 * time.Second},
		{100000, maxFetchTimeout},
	}
	for _, tc := range cases {
		if got := clampFetchTimeout(tc.seconds, defaultFetchTimeout, minFetchTimeout, maxFetchTimeout); got != tc.want {
			t.Fatalf("clamp(%d) = %v, want %v", tc.seconds, got, tc.want)
		}
	}
}
