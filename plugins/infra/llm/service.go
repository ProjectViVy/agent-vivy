package localllm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// serverSpec describes one local OpenAI-compatible model server: where it
// listens by default, how to enumerate its models, and the exact command an
// operator runs to bring it up. VIVY never spawns these daemons itself —
// control actions may not exec, and session-scoped tool-world spawn dies
// with the run — so supervision stays external and the spec carries the
// start hint the start/stop ops report as `spawn_unsupported`.
type serverSpec struct {
	// ID is the stable server key accepted by the manage action.
	ID string
	// BaseURL is the loopback base of the OpenAI-compatible surface.
	BaseURL string
	// modelsURL lists loaded/available models; empty when the server has no
	// listing endpoint.
	modelsURL string
	// modelsShape picks the response decoder: "openai" for {data:[{id}]},
	// "ollama" for {models:[{name}]}.
	modelsShape string
	// startHint is the documented way to launch the server externally.
	startHint string
	// pullURL is the model-download RPC path; only ollama has one.
	pullURL string
}

var serverSpecs = []serverSpec{
	{
		ID:          "ollama",
		BaseURL:     "http://127.0.0.1:11434",
		modelsURL:   "http://127.0.0.1:11434/api/tags",
		modelsShape: "ollama",
		startHint:   "run `ollama serve` in a terminal (or launch the Ollama app)",
		pullURL:     "http://127.0.0.1:11434/api/pull",
	},
	{
		ID:          "llamacpp",
		BaseURL:     "http://127.0.0.1:8080",
		modelsURL:   "http://127.0.0.1:8080/v1/models",
		modelsShape: "openai",
		startHint:   "run `llama-server -m <model.gguf> --jinja`",
	},
	{
		ID:          "lmstudio",
		BaseURL:     "http://127.0.0.1:1234",
		modelsURL:   "http://127.0.0.1:1234/v1/models",
		modelsShape: "openai",
		startHint:   "run `lms server start` or enable the server in the LM Studio app",
	},
	{
		ID:          "vllm",
		BaseURL:     "http://127.0.0.1:8000",
		modelsURL:   "http://127.0.0.1:8000/v1/models",
		modelsShape: "openai",
		startHint:   "run `vllm serve <model>`",
	},
}

func specFor(id string) (serverSpec, bool) {
	for _, spec := range serverSpecs {
		if spec.ID == id {
			return spec, true
		}
	}
	return serverSpec{}, false
}

// probeTimeout bounds every reachability check; discover runs the four
// probes concurrently so a dead server never stalls the result.
const probeTimeout = time.Second

// httpGet/httpPost are injectable for tests; production values hit the real
// network through bounded clients. Package-level net/http helpers stay
// unused — the source-verification capability firewall forbids them.
type hooks struct {
	httpGet  func(ctx context.Context, url string) (*http.Response, error)
	httpPost func(ctx context.Context, url, body string) (*http.Response, error)
}

func defaultHooks() hooks {
	client := &http.Client{Timeout: probeTimeout}
	// Pulls stream model downloads; the action's own Timeout (not the probe
	// budget) bounds them.
	pullClient := &http.Client{}
	return hooks{
		httpGet: func(ctx context.Context, url string) (*http.Response, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, err
			}
			return client.Do(req)
		},
		httpPost: func(ctx context.Context, url, body string) (*http.Response, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/json")
			return pullClient.Do(req)
		},
	}
}

// serverStatus is one server's reachability plus model listing.
type serverStatus struct {
	Server    string   `json:"server"`
	Reachable bool     `json:"reachable"`
	Models    []string `json:"models"`
	BaseURL   string   `json:"base_url"`
	Error     string   `json:"error,omitempty"`
}

// registry caches the last discovery snapshot for the status source. It is
// process-local: a Generation restart loses it, which is honest — VIVY does
// not own servers it did not start.
type registry struct {
	mu       sync.Mutex
	lastSeen map[string]serverStatus
}

func newRegistry() *registry {
	return &registry{lastSeen: map[string]serverStatus{}}
}

// probe checks one server's models endpoint.
func (p *Provider) probe(ctx context.Context, spec serverSpec) serverStatus {
	st := serverStatus{Server: spec.ID, BaseURL: spec.BaseURL, Models: []string{}}
	if spec.modelsURL == "" {
		st.Reachable = true
		return st
	}
	resp, err := p.hooks.httpGet(ctx, spec.modelsURL)
	if err != nil {
		st.Error = shortErr(err)
		return st
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		st.Error = shortErr(err)
		return st
	}
	if resp.StatusCode != http.StatusOK {
		st.Error = fmt.Sprintf("status %d", resp.StatusCode)
		return st
	}
	st.Reachable = true
	st.Models = decodeModels(spec.modelsShape, body)
	return st
}

// discover probes every known server concurrently and caches the result
// for the status source.
func (p *Provider) discover(ctx context.Context) []serverStatus {
	results := make([]serverStatus, len(serverSpecs))
	var wg sync.WaitGroup
	for i, spec := range serverSpecs {
		wg.Add(1)
		go func(i int, spec serverSpec) {
			defer wg.Done()
			probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
			defer cancel()
			results[i] = p.probe(probeCtx, spec)
		}(i, spec)
	}
	wg.Wait()
	p.registry.mu.Lock()
	for _, st := range results {
		p.registry.lastSeen[st.Server] = st
	}
	p.registry.mu.Unlock()
	return results
}

func decodeModels(shape string, body []byte) []string {
	models := []string{}
	switch shape {
	case "ollama":
		var payload struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if json.Unmarshal(body, &payload) == nil {
			for _, m := range payload.Models {
				if m.Name != "" {
					models = append(models, m.Name)
				}
			}
		}
	default: // openai
		var payload struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &payload) == nil {
			for _, m := range payload.Data {
				if m.ID != "" {
					models = append(models, m.ID)
				}
			}
		}
	}
	return models
}

func shortErr(err error) string {
	message := err.Error()
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return "timeout"
	}
	if len(message) > 160 {
		message = message[:160]
	}
	return message
}
