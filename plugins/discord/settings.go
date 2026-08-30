package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Settings are the first-cut adapter knobs the discord channel plugin
// decodes from the opaque `channels.discord.settings` block of the config
// envelope (VIVY-CHANNEL-PACK.md §11). The kernel never reads these keys;
// unknown fields fail closed so a stale adapter never silently ignores a
// new knob it cannot honor.
//
// token_env is duplicated by design: the envelope's token_env is the
// audited declaration the Host pins ChannelEnv.Secret to, while this
// settings copy is what the adapter itself resolves through Secret. The
// two must match or Secret fails closed, so a config that edits one side
// only can never route a secret to the wrong name. Discord uses a single
// bot token (no second credential), so this is the only env_key name the
// adapter needs.
type Settings struct {
	// TokenEnv names the environment variable holding the Discord bot
	// token (the "Bot ..." token from the developer portal's Bot page).
	// Required; Start fails closed without it. The token value itself must
	// never appear in config or settings (D-010).
	TokenEnv string `json:"token_env"`
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
		return Settings{}, fmt.Errorf("discord: decode settings: %w", err)
	}
	// One settings object per channel; a trailing token (e.g. two JSON
	// documents) is a config mistake, not data.
	if dec.More() {
		return Settings{}, fmt.Errorf("discord: decode settings: unexpected trailing data")
	}
	s.TokenEnv = strings.TrimSpace(s.TokenEnv)
	return s, nil
}
