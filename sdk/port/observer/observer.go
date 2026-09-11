// Package observer defines post-commit run observation and best-effort
// diagnostic observation. Providers receive bounded data only and have no
// mutation, Tool, Journal, Policy, or credential authority.
package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	MaxDiagnosticMessageBytes = 2048
	MaxDiagnosticFieldBytes   = 512
	MaxDiagnosticFields       = 16
)

type EventID struct {
	RunID string
	Seq   int64
}

func NewEventID(runID string, seq int64) EventID {
	return EventID{RunID: strings.TrimSpace(runID), Seq: seq}
}

func (id EventID) String() string { return fmt.Sprintf("%s:%d", id.RunID, id.Seq) }

type RunEvent struct {
	ID        EventID
	Type      string
	CreatedAt int64
	Payload   json.RawMessage
}

func NewRunEvent(id EventID, eventType string, createdAt int64, payload json.RawMessage) RunEvent {
	return RunEvent{
		ID:        id,
		Type:      strings.TrimSpace(eventType),
		CreatedAt: createdAt,
		Payload:   append(json.RawMessage(nil), payload...),
	}
}

type RunProvider interface {
	ID() string
	ObserveRun(context.Context, RunEvent) error
}

type Diagnostic struct {
	Namespace string
	Level     string
	Message   string
	Fields    map[string]string
}

func NewDiagnostic(namespace, level, message string, fields map[string]string) Diagnostic {
	out := Diagnostic{
		Namespace: strings.TrimSpace(namespace),
		Level:     strings.TrimSpace(level),
		Message:   truncateUTF8(message, MaxDiagnosticMessageBytes),
		Fields:    make(map[string]string),
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > MaxDiagnosticFields {
		keys = keys[:MaxDiagnosticFields]
	}
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		out.Fields[truncateUTF8(trimmed, MaxDiagnosticFieldBytes)] = truncateUTF8(fields[key], MaxDiagnosticFieldBytes)
	}
	return out
}

type DiagnosticProvider interface {
	ID() string
	ObserveDiagnostic(context.Context, Diagnostic)
}

func truncateUTF8(value string, budget int) string {
	if budget <= 0 || len(value) <= budget {
		if budget <= 0 {
			return ""
		}
		return value
	}
	end := budget
	for end > 0 && !utf8.ValidString(value[:end]) {
		end--
	}
	return value[:end]
}
