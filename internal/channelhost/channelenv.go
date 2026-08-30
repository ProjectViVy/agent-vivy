package channelhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"agent-vivy/sdk/plugin"
)

// hostEnv is the world a channel adapter may touch: the per-plugin
// implementation of plugin.ChannelEnv handed to Channel.Start. Secrets
// fail closed on a missing grant, a name the channel envelope does not
// declare, or a missing value, and are never logged; HTTP is
// outbound-only; there is no Listen capability on this surface.
type hostEnv struct {
	host   *Host
	seam   plugin.Channel // owning adapter, for grant checks
	client *http.Client
	// tokenEnv is the env_key name the channel's config envelope declares
	// (channels.<name>.token_env). Secret is pinned to the declared names
	// (CH-C3-N2): an adapter can only ever read the env_keys its envelope
	// declares — the envelope token_env and, since CH-C6/D2, any top-level
	// `<name>_env` entry of the opaque settings block for multi-credential
	// platforms (dingtalk needs client_id_env + client_secret_env, and one
	// envelope token_env slot cannot carry both). An envelope that declares
	// neither grants no secret at all.
	tokenEnv string
}

// envFor builds the ChannelEnv handed to one adapter's Start.
func (h *Host) envFor(ch plugin.Channel) plugin.ChannelEnv {
	envelope, ok := h.deps.Config[ch.Name()]
	tokenEnv := ""
	if ok {
		tokenEnv = envelope.TokenEnv
	}
	return &hostEnv{host: h, seam: ch, client: h.client, tokenEnv: tokenEnv}
}

// Secret resolves a configured env_key name to its value. A name is
// resolvable when the channel envelope declares it: either it is the
// envelope's token_env (the single-credential pattern, e.g. telegram) or
// it is the value of a top-level `<name>_env` entry of the opaque settings
// block (the multi-credential pattern, e.g. dingtalk's client_id_env +
// client_secret_env; the settings keys declare names, never values).
// Fail-closed on every path: without the secret.read grant, when the name
// is not declared by the envelope, or when the variable is unset or empty.
// Values are never logged (D-010); error messages carry names and outcomes
// only.
func (e *hostEnv) Secret(envKey string) (string, error) {
	if !e.hasGrant(plugin.GrantSecretRead) {
		return "", plugin.ErrDenied
	}
	if strings.TrimSpace(envKey) == "" {
		return "", errors.New("channelhost: env_key must not be empty")
	}
	if !e.declaresEnvKey(envKey) {
		return "", fmt.Errorf("channelhost: env_key %q is neither the channel envelope token_env nor a settings-declared *_env name", envKey)
	}
	value := os.Getenv(envKey)
	if value == "" {
		return "", errors.New("channelhost: environment variable for env_key is empty or unset")
	}
	return value, nil
}

// declaresEnvKey reports whether this channel's config envelope grants the
// given env_key name. A name is granted when it equals the envelope's
// token_env, or when it is the value of a top-level `<name>_env` entry of
// the opaque settings mapping. Only the settings mapping's top-level
// string-valued entries count: nested mappings, non-string scalars, and
// absent or non-mapping settings declare nothing. The walk reads names
// only; values of environment variables never pass through here.
func (e *hostEnv) declaresEnvKey(envKey string) bool {
	if strings.TrimSpace(e.tokenEnv) != "" && envKey == strings.TrimSpace(e.tokenEnv) {
		return true
	}
	envelope, ok := e.host.deps.Config[e.seam.Name()]
	if !ok {
		return false
	}
	node := envelope.Settings
	if node.Kind != yaml.MappingNode {
		return false
	}
	content := node.Content
	for i := 0; i+1 < len(content); i += 2 {
		key, value := content[i], content[i+1]
		if !strings.HasSuffix(key.Value, "_env") {
			continue
		}
		if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
			// A nested mapping or a non-string scalar (int, bool, null)
			// declares no env_key name.
			continue
		}
		if value.Value == envKey {
			return true
		}
	}
	return false
}

// HTTP returns the shared outbound-only client. There is no Listen
// capability here; webhook/listen sockets belong to a later slice and to
// the host, never to the adapter.
func (e *hostEnv) HTTP() *http.Client {
	return e.client
}

// Settings serializes the opaque settings block of the channel's config
// envelope to JSON (the CH-C4 ABI addition). The kernel never inspects the
// inner keys; the owning adapter decodes the result strictly. Absent or
// null settings arrive as `{}` so a plugin-side strict decoder sees an
// empty object rather than nothing. A settings block the kernel cannot
// convert (e.g. a non-string mapping key) is logged and arrives as `{}`;
// the adapter then fails closed on whatever required key is missing.
func (e *hostEnv) Settings() json.RawMessage {
	envelope, ok := e.host.deps.Config[e.seam.Name()]
	if !ok {
		return json.RawMessage("{}")
	}
	out, err := settingsToJSON(envelope.Settings)
	if err != nil {
		e.host.logger.Error("channelhost: cannot serialize channel settings; handing empty object to the adapter",
			"channel", e.seam.Name(), "err", err)
		return json.RawMessage("{}")
	}
	return out
}

// settingsToJSON converts an opaque settings yaml.Node to JSON. Mappings,
// sequences, and scalars map through a plain any round-trip; a zero (absent)
// or null node maps to `{}` so adapters always decode an object.
func settingsToJSON(node yaml.Node) (json.RawMessage, error) {
	if node.Kind == 0 {
		return json.RawMessage("{}"), nil
	}
	if node.Kind == yaml.ScalarNode && (node.Tag == "!!null" || node.Value == "") {
		return json.RawMessage("{}"), nil
	}
	var v any
	if err := node.Decode(&v); err != nil {
		return nil, fmt.Errorf("decode settings node: %w", err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode settings to json: %w", err)
	}
	return out, nil
}

// PublishInbound is the only path an inbound message may travel into the
// kernel. It forwards to the dispatch pipeline; see dispatch.go.
func (e *hostEnv) PublishInbound(ctx context.Context, msg plugin.InboundMessage) error {
	return e.host.publishInbound(ctx, msg)
}

// Media returns the first-cut media handle: a no-op store so the
// capability surface exists before any real media pipeline (C4).
func (e *hostEnv) Media() plugin.MediaStore {
	return e.host.media
}

func (e *hostEnv) hasGrant(need plugin.Grant) bool {
	for _, grant := range e.seam.Grants() {
		if grant == need {
			return true
		}
	}
	return false
}

// noopMediaStore is the v1 Media() handle: it validates the name and
// discards the content, returning a stable opaque ref. The first real
// adapter (C4) replaces it with a durable store.
type noopMediaStore struct{}

func (noopMediaStore) Put(_ context.Context, name string, _ io.Reader) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", errors.New("channelhost: media name must not be empty")
	}
	return "noop:" + name, nil
}
