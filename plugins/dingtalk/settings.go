package dingtalk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Settings are the first-cut adapter knobs the dingtalk channel plugin
// decodes from the opaque `channels.dingtalk.settings` block of the config
// envelope (VIVY-CHANNEL-PACK.md §11). The kernel never reads these keys;
// unknown fields fail closed so a stale adapter never silently ignores a
// new knob it cannot honor.
//
// DingTalk authenticates the stream connection with an app key + app
// secret pair, so the adapter needs TWO env_key names while the config
// envelope carries (at most) one token_env slot. The names are therefore
// declared here and resolved through ChannelEnv.Secret, which the Host
// grants for any top-level `<name>_env` entry of this settings block
// (CH-C6/D2). The values themselves must never appear in config (D-010).
type Settings struct {
	// ClientIDEnv names the environment variable holding the DingTalk app
	// key (client_id). Required; Start fails closed without it.
	ClientIDEnv string `json:"client_id_env"`
	// ClientSecretEnv names the environment variable holding the DingTalk
	// app secret (client_secret). Required; Start fails closed without it.
	ClientSecretEnv string `json:"client_secret_env"`
	// OpenAPIHost optionally overrides the DingTalk OpenAPI gateway the
	// stream client exchanges its connection ticket against, e.g. a
	// loopback stub in tests or a dedicated gateway in private
	// deployments. Empty means the official https://api.dingtalk.com.
	OpenAPIHost string `json:"open_api_host"`
}

// DecodeSettings decodes the raw settings JSON handed over by the Host
// through ChannelEnv.Settings. Absent or empty settings decode to the
// zero value; Start then fails closed on the missing env_key names.
// Unknown fields are rejected (fail-closed, §11).
func DecodeSettings(raw json.RawMessage) (Settings, error) {
	var s Settings
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return s, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return Settings{}, fmt.Errorf("dingtalk: decode settings: %w", err)
	}
	// One settings object per channel; a trailing token (e.g. two JSON
	// documents) is a config mistake, not data.
	if dec.More() {
		return Settings{}, fmt.Errorf("dingtalk: decode settings: unexpected trailing data")
	}
	s.ClientIDEnv = strings.TrimSpace(s.ClientIDEnv)
	s.ClientSecretEnv = strings.TrimSpace(s.ClientSecretEnv)
	s.OpenAPIHost = strings.TrimSpace(s.OpenAPIHost)
	return s, nil
}
