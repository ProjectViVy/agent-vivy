package ui

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// builtFS mimics a Vite build output; unbuiltFS mimics the committed
// placeholder state before the first npm build.
func builtFS() fs.FS {
	return fstest.MapFS{
		"index.html":       &fstest.MapFile{Data: []byte("<html>vivy</html>")},
		"assets/index.js":  &fstest.MapFile{Data: []byte("console.log(1)")},
		"assets/index.css": &fstest.MapFile{Data: []byte("body{}")},
		".keep":            &fstest.MapFile{Data: []byte("placeholder")},
	}
}

func unbuiltFS() fs.FS {
	return fstest.MapFS{".keep": &fstest.MapFile{Data: []byte("placeholder")}}
}

func doGet(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	return w
}

func TestServesIndexAtRoot(t *testing.T) {
	w := doGet(t, NewHandler(builtFS()), "/")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Body.String(); !strings.Contains(got, "vivy") {
		t.Fatalf("body = %q, want the index page", got)
	}
}

func TestServesBuildAssets(t *testing.T) {
	w := doGet(t, NewHandler(builtFS()), "/assets/index.js")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Body.String(); got != "console.log(1)" {
		t.Fatalf("body = %q", got)
	}
}

func TestSPAFallback(t *testing.T) {
	// Unknown paths fall back to the app shell, not a 404.
	w := doGet(t, NewHandler(builtFS()), "/session/sess_123")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := w.Body.String(); !strings.Contains(got, "vivy") {
		t.Fatalf("body = %q, want the index page", got)
	}
}

func TestUnbuiltReports503(t *testing.T) {
	w := doGet(t, NewHandler(unbuiltFS()), "/")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
	body, _ := io.ReadAll(w.Result().Body)
	if !strings.Contains(string(body), "npm") {
		t.Fatalf("body = %q, want actionable build hint", string(body))
	}
}

func TestRejectsNonGet(t *testing.T) {
	w := httptest.NewRecorder()
	NewHandler(builtFS()).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}

// TestEmbeddedCompiles proves the go:embed directive resolves: whether or
// not dist carries a real build, Handler() must construct and answer.
func TestEmbeddedHandlerAnswers(t *testing.T) {
	w := doGet(t, Handler(), "/")
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 200 (built) or 503 (unbuilt)", w.Code)
	}
}
