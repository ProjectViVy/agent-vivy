package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func TestHTTPBackendAllowsBoundedReadOnlyRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "text/plain")
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("untrusted response"))
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	// Create permissive sandbox for tests
	sandbox, err := NewSandboxManager(
		domain.SandboxModeDangerFullAccess,
		t.TempDir(), // Use temp dir as workspace root
		nil,
		&domain.NetworkPolicy{DenyPrivateIPs: false},
	)
	if err != nil {
		t.Fatalf("new sandbox: %v", err)
	}
	backend := NewHTTPBackend([]string{u.Hostname()}, 64, 0, sandbox)

	response, err := backend.Request(context.Background(), "", tools.HTTPRequest{URL: server.URL})
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if response.StatusCode != http.StatusOK || response.Body != "untrusted response" || !response.Untrusted {
		t.Fatalf("response = %#v", response)
	}

	head, err := backend.Request(context.Background(), "", tools.HTTPRequest{Method: "HEAD", URL: server.URL})
	if err != nil || head.StatusCode != http.StatusOK || head.Body != "" {
		t.Fatalf("HEAD response=%#v err=%v", head, err)
	}
}
func TestHTTPBackendRejectsWritesCredentialsRedirectsAndBounds(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("other"))
	}))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, other.URL, http.StatusFound)
		case "/large":
			_, _ = w.Write([]byte(strings.Repeat("x", 65)))
		default:
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	// Create permissive sandbox for tests
	sandbox, err := NewSandboxManager(
		domain.SandboxModeDangerFullAccess,
		t.TempDir(), // Use temp dir as workspace root
		nil,
		&domain.NetworkPolicy{DenyPrivateIPs: false},
	)
	if err != nil {
		t.Fatalf("new sandbox: %v", err)
	}
	backend := NewHTTPBackend([]string{u.Hostname()}, 64, 0, sandbox)

	cases := []struct {
		name  string
		input tools.HTTPRequest
		want  string
	}{
		{name: "write", input: tools.HTTPRequest{Method: "POST", URL: server.URL}, want: "only GET and HEAD"},
		{name: "header credential", input: tools.HTTPRequest{URL: server.URL, Headers: map[string]string{"Authorization": "Bearer secret"}}, want: "credential-bearing"},
		{name: "query credential", input: tools.HTTPRequest{URL: server.URL + "?api_key=secret"}, want: "credential-like"},
		{name: "host", input: tools.HTTPRequest{URL: "https://example.com/"}, want: "not allowlisted"},
		{name: "redirect", input: tools.HTTPRequest{URL: server.URL + "/redirect"}, want: "redirect leaves"},
		{name: "large", input: tools.HTTPRequest{URL: server.URL + "/large"}, want: "exceeds"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := backend.Request(context.Background(), "", test.input); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestHTTPBackendSetConfigLiveApply(t *testing.T) {
	backend := NewHTTPBackend([]string{"example.com"}, 64, 10, nil)
	if backend.client.Timeout != 10*time.Second {
		t.Fatalf("timeout = %v, want 10s", backend.client.Timeout)
	}
	if !backend.hostAllowed("example.com") || backend.hostAllowed("api.other.dev") {
		t.Fatalf("initial allowlist mismatch")
	}

	backend.SetConfig([]string{"*.other.dev", " example.org "}, 999)
	if backend.client.Timeout != maxHTTPTimeout {
		t.Fatalf("timeout = %v, want clamp to %v", backend.client.Timeout, maxHTTPTimeout)
	}
	// Wildcards cover subdomains only, never the bare base (pre-existing
	// semantics); exact entries match exactly after trimming.
	if !backend.hostAllowed("api.other.dev") || !backend.hostAllowed("example.org") ||
		backend.hostAllowed("other.dev") || backend.hostAllowed("example.com") {
		t.Fatalf("allowlist after SetConfig mismatch")
	}

	backend.SetConfig(nil, 0)
	if backend.client.Timeout != defaultHTTPTimeout {
		t.Fatalf("timeout = %v, want default %v", backend.client.Timeout, defaultHTTPTimeout)
	}
	if !backend.hostAllowed("api.other.dev") {
		t.Fatalf("nil hosts must keep the current list")
	}

	backend.SetConfig([]string{"https://api.form.dev:8443/path"}, 5)
	if !backend.hostAllowed("api.form.dev") || backend.hostAllowed("form.dev") {
		t.Fatalf("URL-form normalization mismatch")
	}
	if backend.client.Timeout != 5*time.Second {
		t.Fatalf("timeout = %v, want 5s", backend.client.Timeout)
	}
}
