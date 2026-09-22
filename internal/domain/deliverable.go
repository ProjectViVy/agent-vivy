package domain

import (
	"fmt"
	"unicode/utf8"
)

// Deliverable records an immutable fingerprint for one explicitly presented
// workspace-relative file. It does not promise durable file bytes.
type Deliverable struct {
	ID               string `json:"id"`
	SessionID        SessionID `json:"session_id"`
	RunID            RunID `json:"run_id"`
	WorkspaceID      string `json:"workspace_id"`
	Path             string `json:"path"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Size             int64 `json:"size"`
	SHA256           string `json:"sha256"`
	MediaType        string `json:"media_type"`
	CapturedAt       int64 `json:"captured_at"`
	OriginToolCallID string `json:"origin_tool_call_id"`
	FileVersionID    string `json:"file_version_id,omitempty"`
}

type PresentFile struct {
	Path        string `json:"path"`
	Description string `json:"description"`
}

type PresentRequest struct {
	Files []PresentFile `json:"files"`
	Title string        `json:"title,omitempty"`
}

func (r PresentRequest) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if len(r.Files) == 0 || len(r.Files) > limits.PresentPaths {
		return fmt.Errorf("present request has %d files; maximum is %d", len(r.Files), limits.PresentPaths)
	}
	if err := validateUTF8Bounded("present title", r.Title, limits.DescriptionBytes); err != nil {
		return err
	}
	for _, file := range r.Files {
		if file.Path == "" || !utf8.ValidString(file.Path) {
			return fmt.Errorf("present file path is invalid")
		}
		if err := validateUTF8Bounded("present description", file.Description, limits.DescriptionBytes); err != nil {
			return err
		}
	}
	return nil
}

type DeliveryFailure struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type DeliverySet struct {
	ID         string            `json:"id"`
	SessionID  SessionID         `json:"session_id"`
	RunID      RunID             `json:"run_id"`
	ToolCallID string            `json:"tool_call_id"`
	CreatedAt  int64             `json:"created_at"`
	Title      string            `json:"title,omitempty"`
	Items      []Deliverable     `json:"items"`
	Failures   []DeliveryFailure `json:"failures"`
	Status     string            `json:"status"`
}

type DeliveryReadRequest struct {
	ItemID         string `json:"item_id"`
	ExpectedDigest string `json:"expected_digest"`
	TransferID     string `json:"transfer_id,omitempty"`
	Offset         int64  `json:"offset"`
	Length         int    `json:"length"`
}

func (r DeliveryReadRequest) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if r.ItemID == "" || r.ExpectedDigest == "" || !utf8.ValidString(r.ItemID) || !utf8.ValidString(r.ExpectedDigest) {
		return fmt.Errorf("delivery read identifiers are invalid")
	}
	if r.Offset < 0 || r.Length < 0 || r.Length > limits.BinaryPageBytes {
		return fmt.Errorf("delivery read bounds are invalid")
	}
	return validateUTF8Strings("delivery read", r.TransferID)
}

type DeliveryChunk struct {
	TransferID string `json:"transfer_id"`
	ItemID     string `json:"item_id"`
	Digest     string `json:"digest"`
	Offset     int64  `json:"offset"`
	DataBase64 string `json:"data_base64"`
	EOF        bool   `json:"eof"`
	ExpiresAt  int64  `json:"expires_at"`
}

type ContinuityReceipt struct {
	SessionID  SessionID `json:"session_id"`
	RunID      RunID     `json:"run_id"`
	Operation  string    `json:"operation"`
	RequestID  string    `json:"request_id"`
	InputHash  string    `json:"input_hash"`
	EventSeq   EventSeq  `json:"event_seq"`
}
