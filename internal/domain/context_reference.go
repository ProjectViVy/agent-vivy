package domain

import (
	"fmt"
	"unicode/utf8"
)

// ContextReference is the destination-owned, sanitized copied snapshot. Its
// source workspace label is display metadata, never authority.
type ContextReference struct {
	ID                   string        `json:"id"`
	DestinationSessionID SessionID     `json:"destination_session_id"`
	DestinationRunID     RunID         `json:"destination_run_id"`
	SourceSessionID      SessionID     `json:"source_session_id"`
	SourceWorkspace      string        `json:"source_workspace"`
	CapturedAt           int64         `json:"captured_at"`
	Items                []HistoryItem `json:"items"`
	Digest               string        `json:"digest"`
	Origin               string        `json:"origin"`
}

// Validate checks the captured snapshot without consulting its live source.
func (r ContextReference) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if r.ID == "" || r.DestinationSessionID == "" || r.DestinationRunID == "" || r.SourceSessionID == "" || r.Digest == "" {
		return fmt.Errorf("context reference identifiers are invalid")
	}
	if r.CapturedAt < 0 || !utf8.ValidString(r.ID) || !utf8.ValidString(string(r.DestinationSessionID)) || !utf8.ValidString(string(r.DestinationRunID)) || !utf8.ValidString(string(r.SourceSessionID)) || !utf8.ValidString(r.SourceWorkspace) || !utf8.ValidString(r.Digest) {
		return fmt.Errorf("context reference metadata is invalid")
	}
	if r.Origin != "user_selection" && r.Origin != "model_tool" {
		return fmt.Errorf("context reference origin %q is unknown", r.Origin)
	}
	if err := validateHistoryItems(r.Items, limits, limits.ReferenceBytes); err != nil {
		return err
	}
	return validateJSONBytes("context reference", r, limits.ReferenceBytes)
}
