package runtime

import (
	"errors"

	"agent-vivy/internal/domain"
)

// ErrInvalidThinkingMode is returned before a run is persisted when a
// caller supplies an unknown thinking preference.
var ErrInvalidThinkingMode = errors.New("runtime: invalid thinking mode")

// normalizeThinkingMode maps the empty mode to "auto" and rejects unknown
// values, mirroring normalizeRunMode.
func normalizeThinkingMode(mode domain.ThinkingMode) (domain.ThinkingMode, error) {
	if mode == "" {
		return domain.ThinkingModeAuto, nil
	}
	if !mode.Valid() {
		return "", errors.Join(ErrInvalidThinkingMode, errors.New("thinking must be auto, on or off"))
	}
	return mode, nil
}
