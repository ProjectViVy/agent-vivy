package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testModelsServer serves an OpenAI-compatible /models payload and records
// the Authorization header + request path it received.
func testModelsServer(t *testing.T, status int, payload any) (*httptest.Server, *string, *[]string) {
	t.Helper()
	var auth string
	var paths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if payload != nil {
			_ = json.NewEncoder(w).Encode(payload)
		}
	}))
	t.Cleanup(ts.Close)
	return ts, &auth, &paths
}

func TestModelListClientHappyPath(t *testing.T) {
	ts, auth, paths := testModelsServer(t, http.StatusOK, map[string]any{
		"data": []map[string]any{
			{"id": "gpt-4o"},
			{"id": "gpt-4o-mini"},
		},
	})
	client := &ModelListClient{HTTP: ts.Client()}
	models, err := client.List(context.Background(), ts.URL, "sk-secret")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0] != "gpt-4o" || models[1] != "gpt-4o-mini" {
		t.Fatalf("models = %v", models)
	}
	if *auth != "Bearer sk-secret" {
		t.Fatalf("authorization = %q, want Bearer sk-secret", *auth)
	}
	if len(*paths) != 1 || (*paths)[0] != "/models" {
		t.Fatalf("request path = %v, want [/models]", *paths)
	}
}

func TestModelListClientTrailingSlashOnBaseURL(t *testing.T) {
	ts, _, paths := testModelsServer(t, http.StatusOK, map[string]any{"data": []map[string]any{{"id": "m1"}}})
	client := &ModelListClient{HTTP: ts.Client()}
	if _, err := client.List(context.Background(), ts.URL+"/v1/", ""); err != nil {
		t.Fatal(err)
	}
	if len(*paths) != 1 || (*paths)[0] != "/v1/models" {
		t.Fatalf("request path = %v, want [/v1/models]", *paths)
	}
}

func TestModelListClientDedupesAndTrims(t *testing.T) {
	ts, _, _ := testModelsServer(t, http.StatusOK, map[string]any{
		"data": []map[string]any{
			{"id": "  gpt-4o  "},
			{"id": "gpt-4o"},
			{"id": ""},
			{"id": "gpt-3.5-turbo"},
		},
	})
	client := &ModelListClient{HTTP: ts.Client()}
	models, err := client.List(context.Background(), ts.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0] != "gpt-4o" || models[1] != "gpt-3.5-turbo" {
		t.Fatalf("models = %v", models)
	}
}

func TestModelListClientEmptyData(t *testing.T) {
	ts, _, _ := testModelsServer(t, http.StatusOK, map[string]any{"data": []any{}})
	client := &ModelListClient{HTTP: ts.Client()}
	models, err := client.List(context.Background(), ts.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 0 {
		t.Fatalf("models = %v, want empty", models)
	}
}

func TestModelListClientHTTPError(t *testing.T) {
	ts, _, _ := testModelsServer(t, http.StatusUnauthorized, nil)
	client := &ModelListClient{HTTP: ts.Client()}
	_, err := client.List(context.Background(), ts.URL, "sk-secret")
	if err == nil {
		t.Fatal("expected HTTP 401 to fail")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %v, want status mention", err)
	}
	if strings.Contains(err.Error(), "sk-secret") {
		t.Fatalf("error leaked the api key: %v", err)
	}
}

func TestModelListClientInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data": [{"id": "m1"}], broken`))
	}))
	t.Cleanup(ts.Close)
	client := &ModelListClient{HTTP: ts.Client()}
	if _, err := client.List(context.Background(), ts.URL, ""); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestModelListClientUnreachable(t *testing.T) {
	// A server that is closed immediately: connection refused, no live
	// network involved.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	client := &ModelListClient{HTTP: ts.Client()}
	url := ts.URL
	ts.Close()

	_, err := client.List(context.Background(), url, "sk-secret")
	if err == nil {
		t.Fatal("expected unreachable upstream to fail")
	}
	// The URL (and anything in it) must not appear in the error.
	if strings.Contains(err.Error(), url) {
		t.Fatalf("error leaked the upstream URL: %v", err)
	}
	if strings.Contains(err.Error(), "sk-secret") {
		t.Fatalf("error leaked the api key: %v", err)
	}
}
