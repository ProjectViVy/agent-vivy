package qq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Settings are the first-cut adapter knobs the qq channel plugin decodes
// from the opaque `channels.qq.settings` block of the config envelope
// (VIVY-CHANNEL-PACK.md §11). The kernel never reads these keys; unknown
// fields fail closed so a stale adapter never silently ignores a new knob
// it cannot honor.
//
// The QQ OPEN-PLATFORM bot authenticates with an app id + app secret pair
// issued on https://q.qq.com, so the adapter needs TWO env_key names while
// the config envelope carries (at most) one token_env slot. The names are
// therefore declared here and resolved through ChannelEnv.Secret, which
// the Host grants for any top-level `<name>_env` entry of this settings
// block (CH-C6/D2). The values themselves must never appear in config
// (D-010). There is no other credential shape: personal QQ accounts,
// OneBot/NapCat/gocqhttp endpoints and their tokens are out of scope by
// design — see the package comment.
type Settings struct {
	// AppIDEnv names the environment variable holding the open-platform
	// app id (the robot's AppID from the q.qq.com console). Required;
	// Start fails closed without it.
	AppIDEnv string `json:"app_id_env"`
	// AppSecretEnv names the environment variable holding the app secret
	// (AppSecret). Required and must differ from AppIDEnv; Start fails
	// closed otherwise.
	AppSecretEnv string `json:"app_secret_env"`
	// Sandbox switches the OpenAPI base domain from production
	// (https://api.sgroup.qq.com) to the official sandbox
	// (https://sandbox.api.sgroup.qq.com) via the SDK's own constructor.
	// Default false. The websocket gateway URL is discovered through the
	// same client, so one switch moves both. Default false (production).
	Sandbox bool `json:"sandbox"`
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
		return Settings{}, fmt.Errorf("qq: decode settings: %w", err)
	}
	// One settings object per channel; a trailing token (e.g. two JSON
	// documents) is a config mistake, not data.
	if dec.More() {
		return Settings{}, fmt.Errorf("qq: decode settings: unexpected trailing data")
	}
	s.AppIDEnv = strings.TrimSpace(s.AppIDEnv)
	s.AppSecretEnv = strings.TrimSpace(s.AppSecretEnv)
	return s, nil
}
