package storage

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var ErrWorkflowCursorInvalid = errors.New("storage: invalid workflow cursor")

const maxWorkflowRunCursorBytes = 2048

type WorkflowRunCursor struct {
	CreatedAt int64  `json:"created_at"`
	RunID     string `json:"run_id"`
}

func invalidWorkflowCursor(reason string) error {
	return fmt.Errorf("%w: %s; refresh the list", ErrWorkflowCursorInvalid, reason)
}

func EncodeWorkflowRunCursor(createdAt int64, runID string) string {
	payload, _ := json.Marshal(WorkflowRunCursor{CreatedAt: createdAt, RunID: runID})
	return "v1." + base64.RawURLEncoding.EncodeToString(payload)
}

func DecodeWorkflowRunCursor(cursor string) (WorkflowRunCursor, error) {
	if cursor == "" || len(cursor) > maxWorkflowRunCursorBytes {
		return WorkflowRunCursor{}, invalidWorkflowCursor("empty or oversized token")
	}
	const prefix = "v1."
	if !strings.HasPrefix(cursor, prefix) {
		return WorkflowRunCursor{}, invalidWorkflowCursor("unsupported token version")
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(strings.TrimPrefix(cursor, prefix))
	if err != nil {
		return WorkflowRunCursor{}, invalidWorkflowCursor("invalid token encoding")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var decoded WorkflowRunCursor
	if err := decoder.Decode(&decoded); err != nil {
		return WorkflowRunCursor{}, invalidWorkflowCursor("invalid token payload")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return WorkflowRunCursor{}, invalidWorkflowCursor("trailing token payload")
	}
	if decoded.CreatedAt <= 0 || strings.TrimSpace(decoded.RunID) == "" || len(decoded.RunID) > 256 {
		return WorkflowRunCursor{}, invalidWorkflowCursor("missing or invalid run position")
	}
	return decoded, nil
}

func ParseWorkflowDefinitionCursor(cursor string) (workflowID string, revision uint64, err error) {
	if cursor == "" {
		return "", 0, nil
	}
	separator := strings.LastIndexByte(cursor, ':')
	if separator <= 0 || separator == len(cursor)-1 {
		return "", 0, invalidWorkflowCursor("malformed workflow revision cursor")
	}
	workflowID = cursor[:separator]
	if len(workflowID) > 256 {
		return "", 0, invalidWorkflowCursor("workflow id is oversized")
	}
	revisionText := cursor[separator+1:]
	for _, digit := range revisionText {
		if digit < '0' || digit > '9' {
			return "", 0, invalidWorkflowCursor("revision must be a positive decimal integer")
		}
	}
	parsed, parseErr := strconv.ParseInt(revisionText, 10, 64)
	if parseErr != nil || parsed <= 0 {
		return "", 0, invalidWorkflowCursor("revision must be a positive signed SQL integer")
	}
	return workflowID, uint64(parsed), nil
}
