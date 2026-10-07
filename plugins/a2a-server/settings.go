package a2aserver

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// a2aSettings is the channel envelope's plugin-owned settings block
// (design §10.1). The kernel keeps it opaque; this module decodes it
// fail-closed — an unknown inner key is a config error.
type a2aSettings struct {
	PublicName        string   `json:"public_name" yaml:"public_name"`
	PublicDescription string   `json:"public_description" yaml:"public_description"`
	PublicSkillIDs    []string `json:"public_skill_ids" yaml:"public_skill_ids"`
}

// decodeSettings strictly decodes the env's settings payload: the Host
// hands the envelope's settings subtree as raw JSON.
func decodeSettings(raw json.RawMessage) (a2aSettings, error) {
	var s a2aSettings
	if len(raw) == 0 || string(raw) == "null" {
		return s, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return s, fmt.Errorf("a2a settings: %w", err)
	}
	return s, nil
}
