package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"agent-vivy/internal/tools"
)

func TestEinoHTTPBackendAllowsBoundedReadOnlyRequests(t *testing.T) {
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
	backend := NewEinoHTTPBackend([]string{u.Hostname()}, 64)

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
func TestEinoHTTPBackendRejectsWritesCredentialsRedirectsAndBounds(t *testing.T) {
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
	backend := NewEinoHTTPBackend([]string{u.Hostname()}, 64)

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
