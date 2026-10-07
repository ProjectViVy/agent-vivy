package facerun

import (
	"encoding/json"
	"fmt"
	"io"
)

// TextSink renders pi-style print output: assistant text on Out, tool and
// diagnostic noise on Err.
type TextSink struct {
	Out, Err io.Writer
	streamed bool
}

func (s *TextSink) AssistantDelta(text string) {
	_, _ = fmt.Fprint(s.Out, text)
	s.streamed = true
}
func (s *TextSink) AssistantMessageEnd() {
	if s.streamed {
		_, _ = fmt.Fprintln(s.Out)
		s.streamed = false
	}
}
func (s *TextSink) ToolStarted(name string) { _, _ = fmt.Fprintf(s.Err, "> %s\n", name) }
func (s *TextSink) ToolFinished(name, errText string) {
	if errText != "" {
		_, _ = fmt.Fprintf(s.Err, "  %s failed: %s\n", name, errText)
	}
}
func (s *TextSink) ApprovalBlocked(notice string) { _, _ = fmt.Fprint(s.Err, notice) }
func (s *TextSink) QuestionBlocked(notice string) { _, _ = fmt.Fprint(s.Err, notice) }
func (s *TextSink) RunFailed(category, message string) {
	_, _ = fmt.Fprintf(s.Err, "vivy: run failed (%s): %s\n", category, message)
}
func (s *TextSink) RunCancelled()            { _, _ = fmt.Fprintln(s.Err, "vivy: run cancelled") }
func (s *TextSink) ProtocolError(msg string) { _, _ = fmt.Fprintln(s.Err, "vivy: "+msg) }

// recordVersion is the JSONL record schema version (RQ-JSON).
const recordVersion = 1

// JSONLSink projects the run into pi-compatible JSONL records on Out. Every
// record carries the version and the raw vivy event type under "vivy" so the
// stream stays lossless even where names map onto the pi vocabulary.
type JSONLSink struct {
	Out io.Writer
}

func (s JSONLSink) emit(rec map[string]any) {
	rec["v"] = recordVersion
	line, err := json.Marshal(rec)
	if err != nil {
		return
	}
	_, _ = s.Out.Write(append(line, '\n'))
}

func (s JSONLSink) AssistantDelta(text string) {
	s.emit(map[string]any{"type": "message_update", "vivy": "model.delta", "delta": text})
}

// AssistantMessageEnd is a no-op: message_end is emitted by JournalEvent's
// model.completed mapping (the event carries the integrity digest).
func (s JSONLSink) AssistantMessageEnd() {}
func (s JSONLSink) ToolStarted(name string) {
	s.emit(map[string]any{"type": "tool_execution_start", "vivy": "tool.started", "tool_name": name})
}
func (s JSONLSink) ToolFinished(name, errText string) {
	s.emit(map[string]any{"type": "tool_execution_end", "vivy": "tool.finished", "tool_name": name, "error": errText})
}
func (s JSONLSink) ApprovalBlocked(notice string) {
	s.emit(map[string]any{"type": "tool_execution_update", "vivy": "tool.approval_required", "state": "blocked", "message": notice})
}
func (s JSONLSink) QuestionBlocked(notice string) {
	s.emit(map[string]any{"type": "error", "vivy": "user.question_required", "message": notice})
}
func (s JSONLSink) RunFailed(category, message string) {
	s.emit(map[string]any{"type": "error", "vivy": "run.failed", "cause_category": category, "message": message})
}
func (s JSONLSink) RunCancelled() {
	s.emit(map[string]any{"type": "error", "vivy": "run.cancelled", "message": "run cancelled"})
}
func (s JSONLSink) ProtocolError(msg string) {
	s.emit(map[string]any{"type": "error", "vivy": "protocol", "message": msg})
}

// Emit writes one JSONL record with the schema version added. Face glue uses
// it for lifecycle records that are not journal events (session, turn_start,
// turn_end, agent_settled).
func (s JSONLSink) Emit(name string, fields map[string]any) {
	rec := map[string]any{"type": name}
	for k, v := range fields {
		rec[k] = v
	}
	s.emit(rec)
}

// RunStarted/RunSettled implement LifecycleSink: the session, turn_start,
// turn_end, agent_end and agent_settled records that pi emits around the
// journal stream. The whole terminal tail lives in RunSettled so its order
// (turn_end → agent_end → agent_settled) holds for every outcome.
func (s JSONLSink) RunStarted(sessionID, runID string) {
	s.Emit("session", map[string]any{"session_id": sessionID})
	s.Emit("turn_start", map[string]any{"session_id": sessionID, "run_id": runID})
}

func (s JSONLSink) RunSettled(status string) {
	s.Emit("turn_end", map[string]any{"status": status})
	s.Emit("agent_end", map[string]any{"status": status})
	s.Emit("agent_settled", map[string]any{"status": status})
}

// JournalEvent implements RawSink: events the semantic sink already renders
// are skipped (they were emitted there); mapped types get their pi record
// name; everything else passes through as "journal_event" so the stream is
// complete and lossless.
func (s JSONLSink) JournalEvent(typ string, seq int64, payload json.RawMessage) {
	switch typ {
	case "model.delta", "tool.started", "tool.finished",
		"tool.approval_required", "user.question_required",
		"run.completed", "run.failed", "run.cancelled":
		return
	}
	mapped := map[string]string{
		"run.started":           "agent_start",
		"model.request":         "message_start",
		"model.reasoning_delta": "message_update",
		"model.usage":           "message_usage",
		"model.completed":       "message_end",
		"context.compacted":     "compaction_end",
		"provider.retry":        "auto_retry_start",
		"provider.stall":        "auto_retry_update",
		"tool.requested":        "tool_execution_update",
		"tool.approval_decided": "tool_execution_update",
		"tool.operation":        "tool_execution_update",
		"session.truncated":     "session_update",
		"session.forked":        "session_update",
	}
	if name, ok := mapped[typ]; ok {
		s.emit(map[string]any{"type": name, "vivy": typ, "seq": seq, "payload": payload})
		return
	}
	s.emit(map[string]any{"type": "journal_event", "vivy": typ, "seq": seq, "payload": payload})
}
