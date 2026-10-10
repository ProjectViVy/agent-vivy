package localllm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agent-vivy/sdk/module"
	controlaction "agent-vivy/sdk/port/controlaction"
	statusport "agent-vivy/sdk/port/status"
)

// manageInput is the op-dispatched action input. `op` is required; `server`
// selects one of ollama|llamacpp|lmstudio|vllm where an op needs it;
// `model` feeds start (llamacpp/vllm require it) and pull (ollama only).
type manageInput struct {
	Op     string `json:"op"`
	Server string `json:"server,omitempty"`
	Model  string `json:"model,omitempty"`
}

// Provider implements controlaction.Provider and statusport.Provider on one
// value: the external binding convention gives a Module one NewProvider(),
// and the two Port contracts expose disjoint method names.
type Provider struct {
	hooks    hooks
	registry *registry
}

var (
	_ controlaction.Provider = (*Provider)(nil)
	_ statusport.Provider    = (*Provider)(nil)
)

// newProvider constructs the provider with production hooks.
func newProvider() *Provider {
	return &Provider{hooks: defaultHooks(), registry: newRegistry()}
}

const (
	errGrantUnavailable  = "grant_unavailable"
	errUnsupportedServer = "unsupported_server"
	errSpawnUnsupported  = "spawn_unsupported"
)

// Definition returns the sealed action metadata. The action performs all
// lifecycle verbs under op dispatch, so its effect class is the strictest
// one any op can reach: external-effect (spawning, stopping, downloading).
// Read-only ops share the envelope but never exceed their schema.
func (*Provider) Definition() controlaction.Definition {
	return controlaction.Definition{
		ID:             ActionID,
		Description:    "Discover, list, start, stop and pull models on local OpenAI-compatible LLM servers (ollama, llama.cpp, LM Studio, vLLM)",
		Owner:          ModuleID,
		ModuleID:       ModuleID,
		InputSchema:    json.RawMessage(`{"type":"object","additionalProperties":false,"required":["op"],"properties":{"op":{"type":"string","enum":["status","discover","models","start","stop","pull"]},"server":{"type":"string","enum":["ollama","llamacpp","lmstudio","vllm"]},"model":{"type":"string","maxLength":512}}}`),
		ResultSchema:   json.RawMessage(`{"type":"object","additionalProperties":true}`),
		Effect:         controlaction.EffectExternalEffect,
		RequiredGrants: []module.Grant{module.GrantNetClient},
		Timeout:        120 * time.Second,
		MaxInputBytes:  8192,
		MaxOutputBytes: 256 << 10,
	}
}

// Invoke dispatches on input.op. Status, discover, and models are pure
// reads; pull downloads an ollama model under the net.client grant, bounded
// by the action deadline. start/stop are honest no-ops: control actions may
// not exec and the only sanctioned spawn (tool-world) dies with the session,
// so both report spawn_unsupported plus the server's external start hint.
func (p *Provider) Invoke(ctx context.Context, host controlaction.Host, input json.RawMessage) (json.RawMessage, error) {
	var parsed manageInput
	if err := json.Unmarshal(input, &parsed); err != nil || parsed.Op == "" {
		return nil, controlaction.ErrInvalidInput
	}
	switch parsed.Op {
	case "status":
		return p.opStatus(ctx, parsed.Server)
	case "discover":
		return p.opDiscover(ctx)
	case "models":
		return p.opModels(ctx, parsed.Server)
	case "start":
		return p.opStart(parsed.Server)
	case "stop":
		return p.opStop(parsed.Server)
	case "pull":
		if err := requireGrant(host, module.GrantNetClient); err != nil {
			return nil, err
		}
		return p.opPull(ctx, parsed.Server, parsed.Model)
	default:
		return nil, controlaction.ErrInvalidInput
	}
}

func requireGrant(host controlaction.Host, grant module.Grant) error {
	if host == nil {
		return errors.New(errGrantUnavailable + ": host unavailable")
	}
	if err := host.Grant(grant); err != nil {
		return fmt.Errorf("%s: %s", errGrantUnavailable, grant)
	}
	return nil
}

func marshalResult(v any) (json.RawMessage, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func (p *Provider) opStatus(ctx context.Context, server string) (json.RawMessage, error) {
	if server != "" {
		spec, ok := specFor(server)
		if !ok {
			return nil, errors.New(errUnsupportedServer)
		}
		probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
		defer cancel()
		st := p.probe(probeCtx, spec)
		return marshalResult(map[string]any{"ok": true, "servers": []serverStatus{st}})
	}
	return marshalResult(map[string]any{"ok": true, "servers": p.discover(ctx)})
}

func (p *Provider) opDiscover(ctx context.Context) (json.RawMessage, error) {
	return marshalResult(map[string]any{"ok": true, "servers": p.discover(ctx)})
}

func (p *Provider) opModels(ctx context.Context, server string) (json.RawMessage, error) {
	if server == "" {
		return marshalResult(map[string]any{"ok": true, "servers": p.discover(ctx)})
	}
	spec, ok := specFor(server)
	if !ok {
		return nil, errors.New(errUnsupportedServer)
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	st := p.probe(probeCtx, spec)
	if !st.Reachable {
		return marshalResult(map[string]any{"ok": false, "server": st.Server, "error": coalesce(st.Error, "unreachable")})
	}
	return marshalResult(map[string]any{"ok": true, "server": st.Server, "models": st.Models})
}

func (p *Provider) opStart(server string) (json.RawMessage, error) {
	spec, ok := specFor(server)
	if !ok {
		return nil, errors.New(errUnsupportedServer)
	}
	return marshalResult(map[string]any{
		"ok":      false,
		"server":  spec.ID,
		"error":   errSpawnUnsupported,
		"hint":    spec.startHint,
		"reason":  "control actions cannot spawn daemons; session-scoped spawn would die with the run, so supervision stays external",
		"managed": false,
	})
}

func (p *Provider) opStop(server string) (json.RawMessage, error) {
	spec, ok := specFor(server)
	if !ok {
		return nil, errors.New(errUnsupportedServer)
	}
	return marshalResult(map[string]any{
		"ok":      false,
		"server":  spec.ID,
		"error":   errSpawnUnsupported,
		"hint":    "stop the server in its own terminal/process manager; VIVY never owns externally started daemons",
		"managed": false,
	})
}

func (p *Provider) opPull(ctx context.Context, server, model string) (json.RawMessage, error) {
	if server != "ollama" {
		return marshalResult(map[string]any{"ok": false, "server": server, "error": errUnsupportedServer,
			"hint": "pull is ollama-only; download llama.cpp/vllm weights manually (GGUF/HF), lmstudio via lms"})
	}
	if strings.TrimSpace(model) == "" {
		return nil, controlaction.ErrInvalidInput
	}
	spec, _ := specFor("ollama")
	resp, err := p.hooks.httpPost(ctx, spec.pullURL, fmt.Sprintf(`{"name":%q,"stream":false}`, model))
	if err != nil {
		return marshalResult(map[string]any{"ok": false, "server": spec.ID, "error": shortErr(err)})
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return marshalResult(map[string]any{
		"ok":       resp.StatusCode == http.StatusOK,
		"server":   spec.ID,
		"model":    model,
		"status":   resp.StatusCode,
		"response": string(body),
	})
}

func coalesce(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
