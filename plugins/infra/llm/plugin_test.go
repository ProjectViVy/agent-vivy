package localllm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
	controlaction "agent-vivy/sdk/port/controlaction"
	statusport "agent-vivy/sdk/port/status"
)

// fakeHost satisfies the controlaction.Host surface this Module touches:
// Grant gating only; the remaining members panic if reached.
type fakeHost struct {
	grants map[module.Grant]bool
}

func (h fakeHost) ModuleID() string { return ModuleID }
func (h fakeHost) Grant(g module.Grant) error {
	if h.grants[g] {
		return nil
	}
	return errors.New("grant denied")
}
func (h fakeHost) HasGrant(g module.Grant) bool  { return h.grants[g] }
func (h fakeHost) Secret(string) (string, error) { return "", errors.New("no secrets") }
func (h fakeHost) Settings() json.RawMessage     { return json.RawMessage(`{}`) }
func (h fakeHost) StartRun(context.Context, controlaction.RunRequest) (controlaction.RunResult, error) {
	panic("unreached")
}
func (h fakeHost) InvokeTool(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	panic("unreached")
}

func fakeResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}
}

// providerWithHooks builds a provider whose four server endpoints are
// redirected at httptest-style URLs through the httpGet hook.
func providerWithHooks(get func(ctx context.Context, url string) (*http.Response, error)) *Provider {
	p := newProvider()
	p.hooks.httpGet = get
	return p
}

func decodeResult(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("result: %v", err)
	}
	return out
}

func TestDiscoverProbesAllServers(t *testing.T) {
	p := providerWithHooks(func(ctx context.Context, url string) (*http.Response, error) {
		switch {
		case strings.Contains(url, "11434"):
			return fakeResponse(`{"models":[{"name":"qwen3:8b"},{"name":"llama3.1"}]}`), nil
		case strings.Contains(url, "8080"):
			return fakeResponse(`{"data":[{"id":"gguf-model"}]}`), nil
		default:
			return nil, errors.New("connection refused")
		}
	})
	raw, err := p.Invoke(context.Background(), fakeHost{grants: map[module.Grant]bool{}},
		json.RawMessage(`{"op":"discover"}`))
	if err != nil {
		t.Fatal(err)
	}
	result := decodeResult(t, raw)
	servers := result["servers"].([]any)
	if len(servers) != 4 {
		t.Fatalf("servers = %v", servers)
	}
	byID := map[string]map[string]any{}
	for _, s := range servers {
		m := s.(map[string]any)
		byID[m["server"].(string)] = m
	}
	if !byID["ollama"]["reachable"].(bool) {
		t.Fatal("ollama unreachable")
	}
	models := byID["ollama"]["models"].([]any)
	if len(models) != 2 || models[0] != "qwen3:8b" {
		t.Fatalf("ollama models = %v", models)
	}
	if !byID["llamacpp"]["reachable"].(bool) || byID["llamacpp"]["models"].([]any)[0] != "gguf-model" {
		t.Fatalf("llamacpp = %v", byID["llamacpp"])
	}
	if byID["vllm"]["reachable"].(bool) {
		t.Fatal("vllm should be unreachable")
	}
}

func TestStatusNeverProbes(t *testing.T) {
	calls := 0
	p := providerWithHooks(func(ctx context.Context, url string) (*http.Response, error) {
		calls++
		return fakeResponse(`{"data":[{"id":"m"}]}`), nil
	})
	// A status read before any discover reports unprobed without network.
	snap, err := p.Status(context.Background(), statusport.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("status probed the network")
	}
	if snap.Items[0].State != "unprobed" {
		t.Fatalf("state = %v", snap.Items)
	}
	if len(snap.Items) != 4 {
		t.Fatalf("items = %v", snap.Items)
	}

	// After a discover the cached snapshot feeds status without probing.
	if _, err := p.Invoke(context.Background(), fakeHost{}, json.RawMessage(`{"op":"discover"}`)); err != nil {
		t.Fatal(err)
	}
	before := calls
	snap, err = p.Status(context.Background(), statusport.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if calls != before {
		t.Fatal("status probed the network after discover")
	}
	states := map[string]string{}
	for _, item := range snap.Items {
		states[item.ID] = item.State
	}
	if states["ollama"] != "reachable" || states["vllm"] != "reachable" {
		t.Fatalf("states = %v", states)
	}
}

func TestStartStopReportSpawnUnsupported(t *testing.T) {
	p := newProvider()
	for _, server := range []string{"ollama", "llamacpp", "lmstudio", "vllm"} {
		raw, err := p.Invoke(context.Background(), fakeHost{},
			json.RawMessage(`{"op":"start","server":"`+server+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		res := decodeResult(t, raw)
		if res["ok"] != false || res["error"] != errSpawnUnsupported {
			t.Fatalf("start %s = %v", server, res)
		}
		if hint, _ := res["hint"].(string); !strings.Contains(hint, "`") {
			t.Fatalf("start %s hint = %v", server, res["hint"])
		}
	}
	raw, err := p.Invoke(context.Background(), fakeHost{},
		json.RawMessage(`{"op":"stop","server":"ollama"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res := decodeResult(t, raw); res["error"] != errSpawnUnsupported || res["ok"] != false {
		t.Fatalf("stop = %v", res)
	}
	if _, err := p.Invoke(context.Background(), fakeHost{},
		json.RawMessage(`{"op":"start","server":"bogus"}`)); err == nil {
		t.Fatal("start on unknown server must fail")
	}
}

func TestPullOllamaOnly(t *testing.T) {
	var posted string
	p := newProvider()
	p.hooks.httpPost = func(ctx context.Context, url, body string) (*http.Response, error) {
		posted = body
		return fakeResponse(`{"status":"success"}`), nil
	}
	host := fakeHost{grants: map[module.Grant]bool{module.GrantNetClient: true}}
	raw, err := p.Invoke(context.Background(), host, json.RawMessage(`{"op":"pull","server":"ollama","model":"qwen3:8b"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res := decodeResult(t, raw); res["ok"] != true || !strings.Contains(posted, "qwen3:8b") {
		t.Fatalf("pull = %v body=%q", res, posted)
	}
	raw, err = p.Invoke(context.Background(), host, json.RawMessage(`{"op":"pull","server":"llamacpp","model":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res := decodeResult(t, raw); res["ok"] != false || res["error"] != errUnsupportedServer {
		t.Fatalf("llamacpp pull = %v", res)
	}
	// Missing net.client grant fails closed before any request.
	posted = ""
	if _, err := p.Invoke(context.Background(), fakeHost{grants: map[module.Grant]bool{}},
		json.RawMessage(`{"op":"pull","server":"ollama","model":"x"}`)); err == nil {
		t.Fatal("pull without net.client must fail")
	}
	if posted != "" {
		t.Fatal("ungranted pull issued a request")
	}
}

func TestInvalidOpsAndServers(t *testing.T) {
	p := newProvider()
	host := fakeHost{grants: map[module.Grant]bool{module.GrantNetClient: true}}
	if _, err := p.Invoke(context.Background(), host, json.RawMessage(`{"op":"bogus"}`)); err == nil {
		t.Fatal("bogus op must fail")
	}
	if _, err := p.Invoke(context.Background(), host, json.RawMessage(`{"op":"status","server":"bogus"}`)); err == nil {
		t.Fatal("bogus server must fail")
	}
	if _, err := p.Invoke(context.Background(), host, json.RawMessage(`{}`)); err == nil {
		t.Fatal("missing op must fail")
	}
	if _, err := p.Invoke(context.Background(), host, json.RawMessage(`{"op":"pull","server":"ollama"}`)); err == nil {
		t.Fatal("pull without model must fail")
	}
}
