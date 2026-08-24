//go:build vivy_headless

package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHeadlessHandlerDoesNotServeUI(t *testing.T) {
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
