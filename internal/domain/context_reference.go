package domain

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
