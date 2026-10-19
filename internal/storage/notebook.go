package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"agent-vivy/internal/notebookcontract"
)

// NotebookStore is the narrow scoped-content contract added to the Engine
// surface (N1). Backends expose it via Engine.Notebook(); the alias keeps
// consumers decoupled from the contract package.
type NotebookStore = notebookcontract.Store

// NotebookGeneratedWriter is the reserved report-publication seam defined in
// N1 and implemented with provenance in R1.
type NotebookGeneratedWriter = notebookcontract.GeneratedWriter

// NewNotebookID mints a unique resource ID; prefixes keep the kind legible in
// dumps and exports.
func NewNotebookID(prefix string) (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("notebook: id entropy: %w", err)
	}
	return prefix + hex.EncodeToString(raw[:]), nil
}

// notebookCursor is the stable page cursor. F binds the cursor to scope plus
// the caller's filter tuple so a cursor minted under another scope or filter
// is rejected instead of leaking a position.
type NotebookCursor struct {
	U int64  `json:"u"`
	I string `json:"i"`
	F string `json:"f"`
}

func NotebookFilterTag(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:16]
}

func EncodeNotebookCursor(u int64, id, tag string) string {
	raw, _ := json.Marshal(NotebookCursor{U: u, I: id, F: tag})
	return "nb1." + base64.RawURLEncoding.EncodeToString(raw)
}

func DecodeNotebookCursor(raw, tag string) (NotebookCursor, error) {
	if raw == "" {
		return NotebookCursor{}, nil
	}
	const prefix = "nb1."
	if !strings.HasPrefix(raw, prefix) {
		return NotebookCursor{}, &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "malformed page cursor"}
	}
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(raw, prefix))
	if err != nil {
		return NotebookCursor{}, &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "malformed page cursor"}
	}
	var c NotebookCursor
	if err := json.Unmarshal(data, &c); err != nil {
		return NotebookCursor{}, &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "malformed page cursor"}
	}
	if c.F != tag {
		return NotebookCursor{}, &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "cursor does not match this scope or filter"}
	}
	return c, nil
}

// NotebookValidateMutation checks the trusted envelope and returns the
// canonical request digest. A caller-supplied digest that disagrees is an
// invalid_request (host bug), never silently trusted.
func NotebookValidateMutation(mc notebookcontract.MutationContext, kind string, req any) (string, error) {
	if mc.ScopeID == "" {
		return "", &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "missing trusted scope"}
	}
	if mc.OperationKey == "" {
		return "", &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "missing operation key"}
	}
	if _, err := mc.Actor.Kind.OriginFor(); err != nil {
		return "", &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "unsupported actor kind " + string(mc.Actor.Kind)}
	}
	if mc.Actor.Ref == "" {
		return "", &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "missing actor reference"}
	}
	digest, err := notebookcontract.RequestDigest(kind, req)
	if err != nil {
		return "", err
	}
	if mc.RequestDigest != "" && mc.RequestDigest != digest {
		return "", &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "request digest mismatch"}
	}
	return digest, nil
}

func NotebookValidateTitle(title string) error {
	if strings.TrimSpace(title) == "" {
		return &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "empty title"}
	}
	return nil
}

func NotebookValidateBody(markdown string) error {
	if !utf8.ValidString(markdown) {
		return &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "body is not valid UTF-8"}
	}
	if len(markdown) > notebookcontract.MaxBodyBytes {
		return &notebookcontract.Error{Code: notebookcontract.CodeLimitExceeded, Message: fmt.Sprintf("body exceeds %d bytes", notebookcontract.MaxBodyBytes)}
	}
	return nil
}

func NotebookValidateCommentBody(body string) error {
	if !utf8.ValidString(body) {
		return &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "comment is not valid UTF-8"}
	}
	if len(body) == 0 {
		return &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "empty comment"}
	}
	if len(body) > notebookcontract.MaxCommentBytes {
		return &notebookcontract.Error{Code: notebookcontract.CodeLimitExceeded, Message: fmt.Sprintf("comment exceeds %d bytes", notebookcontract.MaxCommentBytes)}
	}
	return nil
}

// NotebookPageLimit clamps the requested page size to the contract bound.
func NotebookPageLimit(limit int) int {
	if limit <= 0 || limit > notebookcontract.MaxPageRows {
		return notebookcontract.MaxPageRows
	}
	return limit
}
