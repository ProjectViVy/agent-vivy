package feishu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Settings are the first-cut adapter knobs the feishu channel plugin
// decodes from the opaque `channels.feishu.settings` block of the config
// envelope (VIVY-CHANNEL-PACK.md §11). The kernel never reads these keys;
// unknown fields fail closed so a stale adapter never silently ignores a
// new knob it cannot honor.
//
// Feishu authenticates both the websocket long connection and the OpenAPI
// client with an app id + app secret pair, so the adapter needs TWO
// env_key names while the config envelope carries (at most) one token_env
// slot. The names are therefore declared here and resolved through
// ChannelEnv.Secret, which the Host grants for any top-level `<name>_env`
// entry of this settings block (CH-C6/D2). The values themselves must
// never appear in config (D-010).
type Settings struct {
	// AppIDEnv names the environment variable holding the Feishu app id.
	// Required; Start fails closed without it.
	AppIDEnv string `json:"app_id_env"`
	// AppSecretEnv names the environment variable holding the Feishu app
	// secret. Required and must differ from AppIDEnv; Start fails closed
	// otherwise.
	AppSecretEnv string `json:"app_secret_env"`
	// EncryptKey is the event encryption key as a plain settings value
	// (contract §14.3: the Host never decodes this field). The websocket
	// long connection pushes events unencrypted — the SDK's WS dispatch
	// path performs no decryption — so the key is carried for contract
	// completeness and handed to the SDK's event dispatcher, where it only
	// matters for webhook-mode flows this adapter does not run.
	EncryptKey string `json:"encrypt_key"`
	// IsLark switches the platform domain from Feishu
	// (https://open.feishu.cn) to international Lark
	// (https://open.larksuite.com) for both the websocket gateway and the
	// OpenAPI client. Default false (Feishu).
	IsLark bool `json:"is_lark"`
	// OpenBaseURL optionally overrides the platform base URL, e.g. a
	// loopback stub in tests or a dedicated gateway in private
	// deployments. Empty means the domain chosen by IsLark. It mirrors the
	// dingtalk adapter's open_api_host precedent (CH-C6).
	OpenBaseURL string `json:"open_base_url"`
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
		return Settings{}, fmt.Errorf("feishu: decode settings: %w", err)
	}
	// One settings object per channel; a trailing token (e.g. two JSON
	// documents) is a config mistake, not data.
	if dec.More() {
		return Settings{}, fmt.Errorf("feishu: decode settings: unexpected trailing data")
	}
	s.AppIDEnv = strings.TrimSpace(s.AppIDEnv)
	s.AppSecretEnv = strings.TrimSpace(s.AppSecretEnv)
	s.EncryptKey = strings.TrimSpace(s.EncryptKey)
	s.OpenBaseURL = strings.TrimSpace(s.OpenBaseURL)
	return s, nil
}
