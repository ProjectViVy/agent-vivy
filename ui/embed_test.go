package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testUIFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":      {Data: []byte("<html>vivy</html>")},
		"assets/index.js": {Data: []byte("console.log(1)")},
	}
}

func TestNewHandlerServesAssetsAndClientRoutes(t *testing.T) {
	handler := NewHandler(testUIFS())
	for _, target := range []string{"/", "/skills"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "vivy") {
			t.Fatalf("GET %s = (%d, %q), want app shell", target, response.Code, response.Body.String())
		}
	}

	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/index.js", nil))
	if asset.Code != http.StatusOK || asset.Body.String() != "console.log(1)" {
		t.Fatalf("asset = (%d, %q)", asset.Code, asset.Body.String())
	}
}

func TestNewHandlerRejectsWrites(t *testing.T) {
	response := httptest.NewRecorder()
	NewHandler(testUIFS()).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestEmbeddedHandlerAnswers(t *testing.T) {
	response := httptest.NewRecorder()
	Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/skills", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}
