package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Settings are the first-cut adapter knobs the telegram channel plugin
// decodes from the opaque `channels.telegram.settings` block of the config
// envelope (VIVY-CHANNEL-PACK.md §11). The kernel never reads these keys;
// unknown fields fail closed so a stale adapter never silently ignores a
// new knob it cannot honor.
//
// token_env is duplicated by design: the envelope's token_env is the
// audited declaration the Host pins ChannelEnv.Secret to, while this
// settings copy is what the adapter itself resolves through Secret. The
// two must match or Secret fails closed, so a config that edits one side
// only can never route a secret to the wrong name.
type Settings struct {
	// TokenEnv names the environment variable holding the bot token.
	// Required; Start fails closed without it. The token value itself must
	// never appear in config or settings (D-010).
	TokenEnv string `json:"token_env"`
	// BaseURL optionally overrides the Telegram Bot API server (a local
	// bot-api sidecar). Empty means the official api.telegram.org.
	BaseURL string `json:"base_url"`
	// Proxy optionally routes the Bot API HTTP client through an HTTP
	// proxy, e.g. "http://127.0.0.1:7890". Empty means direct dialing.
	Proxy string `json:"proxy"`
}

// DecodeSettings decodes the raw settings JSON handed over by the Host
// through ChannelEnv.Settings. Absent or empty settings decode to the
// zero value; Start then fails closed on the missing token_env. Unknown
// fields are rejected (fail-closed, §11).
func DecodeSettings(raw json.RawMessage) (Settings, error) {
	var s Settings
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return s, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return Settings{}, fmt.Errorf("telegram: decode settings: %w", err)
	}
	// One settings object per channel; a trailing token (e.g. two JSON
	// documents) is a config mistake, not data.
	if dec.More() {
		return Settings{}, fmt.Errorf("telegram: decode settings: unexpected trailing data")
	}
	s.BaseURL = strings.TrimSpace(s.BaseURL)
	s.Proxy = strings.TrimSpace(s.Proxy)
	s.TokenEnv = strings.TrimSpace(s.TokenEnv)
	return s, nil
}
