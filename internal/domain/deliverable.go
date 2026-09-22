package domain

import (
	"encoding/base64"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

const (
	DeliveryStatusOK      = "ok"
	DeliveryStatusPartial = "partial"
	DeliveryStatusFailed  = "failed"
)

func validDeliveryStatus(status string) bool {
	switch status {
	case DeliveryStatusOK, DeliveryStatusPartial, DeliveryStatusFailed:
		return true
	}
	return false
}

// Deliverable records an immutable fingerprint for one explicitly presented
// workspace-relative file. It does not promise durable file bytes.
type Deliverable struct {
	ID               string    `json:"id"`
	SessionID        SessionID `json:"session_id"`
	RunID            RunID     `json:"run_id"`
	WorkspaceID      string    `json:"workspace_id"`
	Path             string    `json:"path"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	Size             int64     `json:"size"`
	SHA256           string    `json:"sha256"`
	MediaType        string    `json:"media_type"`
	CapturedAt       int64     `json:"captured_at"`
	OriginToolCallID string    `json:"origin_tool_call_id"`
	FileVersionID    string    `json:"file_version_id,omitempty"`
}

// Validate checks immutable metadata and its declared file-size ceiling. It
// does not read the workspace or attempt to verify file bytes.
func (d Deliverable) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if d.ID == "" || d.SessionID == "" || d.RunID == "" || d.WorkspaceID == "" || d.Name == "" || d.SHA256 == "" || d.MediaType == "" || d.OriginToolCallID == "" {
		return fmt.Errorf("deliverable identifiers are invalid")
	}
	if d.Size < 0 || d.Size > limits.PresentFileBytes || d.CapturedAt < 0 {
		return fmt.Errorf("deliverable size or timestamp is invalid")
	}
	if !validWorkspaceRelativePath(d.Path) {
		return fmt.Errorf("deliverable path is not normalized and workspace-relative")
	}
	if err := validateUTF8Strings("deliverable", d.ID, string(d.SessionID), string(d.RunID), d.WorkspaceID, d.Path, d.Name, d.SHA256, d.MediaType, d.OriginToolCallID, d.FileVersionID); err != nil {
		return err
	}
	return validateUTF8Bounded("deliverable description", d.Description, limits.DescriptionBytes)
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
		if !validWorkspaceRelativePath(file.Path) {
			return fmt.Errorf("present file path is not normalized and workspace-relative")
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

// Validate checks the complete declared presentation set without reading a
// file. Size values enforce the per-file and aggregate verification ceilings.
func (s DeliverySet) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if s.ID == "" || s.SessionID == "" || s.RunID == "" || s.ToolCallID == "" || s.CreatedAt < 0 {
		return fmt.Errorf("delivery set identifiers are invalid")
	}
	if !validDeliveryStatus(s.Status) {
		return fmt.Errorf("delivery set status %q is unknown", s.Status)
	}
	if len(s.Items)+len(s.Failures) == 0 || len(s.Items)+len(s.Failures) > limits.PresentPaths {
		return fmt.Errorf("delivery set has %d paths; maximum is %d", len(s.Items)+len(s.Failures), limits.PresentPaths)
	}
	if err := validateUTF8Strings("delivery set", s.ID, string(s.SessionID), string(s.RunID), s.ToolCallID); err != nil {
		return err
	}
	if err := validateUTF8Bounded("delivery set title", s.Title, limits.DescriptionBytes); err != nil {
		return err
	}
	seenPaths := make(map[string]struct{}, len(s.Items)+len(s.Failures))
	var total int64
	for _, item := range s.Items {
		if err := item.Validate(limits); err != nil {
			return err
		}
		if _, duplicate := seenPaths[item.Path]; duplicate {
			return fmt.Errorf("delivery set contains a duplicate path")
		}
		seenPaths[item.Path] = struct{}{}
		total += item.Size
		if total > limits.PresentSetBytes {
			return fmt.Errorf("delivery set exceeds %d bytes", limits.PresentSetBytes)
		}
	}
	for _, failure := range s.Failures {
		if !validWorkspaceRelativePath(failure.Path) {
			return fmt.Errorf("delivery failure path is not normalized and workspace-relative")
		}
		if err := validateUTF8Bounded("delivery failure reason", failure.Reason, limits.DescriptionBytes); err != nil || failure.Reason == "" {
			return fmt.Errorf("delivery failure reason is invalid")
		}
		if _, duplicate := seenPaths[failure.Path]; duplicate {
			return fmt.Errorf("delivery set contains a duplicate path")
		}
		seenPaths[failure.Path] = struct{}{}
	}
	return nil
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

// Validate checks the bounded, encoded binary page without allocating a
// decoded page larger than the protocol ceiling.
func (c DeliveryChunk) Validate(limits ContinuityLimits) error {
	limits = limits.Effective(0)
	if c.TransferID == "" || c.ItemID == "" || c.Digest == "" || c.Offset < 0 || c.ExpiresAt < 0 {
		return fmt.Errorf("delivery chunk metadata is invalid")
	}
	if err := validateUTF8Strings("delivery chunk", c.TransferID, c.ItemID, c.Digest, c.DataBase64); err != nil {
		return err
	}
	if len(c.DataBase64) > base64.StdEncoding.EncodedLen(limits.BinaryPageBytes) {
		return fmt.Errorf("delivery chunk exceeds %d bytes", limits.BinaryPageBytes)
	}
	decoded, err := base64.StdEncoding.DecodeString(c.DataBase64)
	if err != nil || len(decoded) > limits.BinaryPageBytes {
		return fmt.Errorf("delivery chunk data is invalid")
	}
	return nil
}

type ContinuityReceipt struct {
	SessionID SessionID `json:"session_id"`
	RunID     RunID     `json:"run_id"`
	Operation string    `json:"operation"`
	RequestID string    `json:"request_id"`
	InputHash string    `json:"input_hash"`
	EventSeq  EventSeq  `json:"event_seq"`
}

func validWorkspaceRelativePath(value string) bool {
	if value == "" || !utf8.ValidString(value) || path.IsAbs(value) || strings.HasPrefix(value, "../") {
		return false
	}
	return path.Clean(value) == value && value != "."
}
