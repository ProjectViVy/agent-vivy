package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ModelListTimeout caps every upstream /models fetch. The Settings → 模型
// refresh button is synchronous, so a hung gateway must fail fast instead of
// pinning the request table for minutes.
const ModelListTimeout = 15 * time.Second

// maxModelListResponseBytes bounds the /models response body read so a
// misbehaving gateway cannot exhaust memory while decoding.
const maxModelListResponseBytes = 4 << 20

// defaultModelListHTTP is the package default transport for upstream model
// discovery. Tests inject their own HTTP client (httptest) through
// ModelListClient.HTTP so no unit test touches the live network.
var defaultModelListHTTP = &http.Client{Timeout: ModelListTimeout}

// ModelListClient discovers the model ids an OpenAI-compatible gateway
// advertises through its GET {base}/models endpoint. It is the runtime side
// of the Settings → 模型 "refresh" control: the ids returned are persisted
// into the provider registry so the UI list is a local copy of upstream.
//
// Only OpenAI-compatible endpoints are supported (v1). Anthropic's native
// API does not implement this protocol; those providers keep their
// static/manually maintained list (the RPC rejects them up front).
type ModelListClient struct {
	// HTTP is the transport used for the request. Nil means the package
	// default 15s client.
	HTTP *http.Client
}

// List fetches and normalizes the upstream model ids for baseURL. apiKey,
// when non-empty, is sent as a Bearer token; it is never included in the
// returned errors (the URL and any user-supplied query params are stripped
// from the cause chain too). The result keeps the gateway's order and
// de-duplicates by trimmed id; blank ids are dropped.
func (c *ModelListClient) List(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	httpClient := defaultModelListHTTP
	if c != nil && c.HTTP != nil {
		httpClient = c.HTTP
	}
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("list models: invalid base url: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		// Strip the *url.Error URL (which could carry a user-supplied query
		// param token) from the cause chain so no half-secret leaks into the
		// RPC error surface (D-010).
		cause := err
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			cause = urlErr.Err
		}
		return nil, fmt.Errorf("list models: %w", cause)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxModelListResponseBytes))
		return nil, fmt.Errorf("list models: upstream returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxModelListResponseBytes))
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("list models: invalid response: %w", err)
	}
	seen := make(map[string]bool, len(payload.Data))
	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		models = append(models, id)
	}
	return models, nil
}
