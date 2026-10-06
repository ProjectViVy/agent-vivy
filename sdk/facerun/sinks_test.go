package facerun

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// recordTypes parses the JSONL output into the ordered record type names.
func recordTypes(t *testing.T, out string) []string {
	t.Helper()
	var types []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("bad JSONL record %q: %v", line, err)
		}
		if rec["v"] != float64(1) {
			t.Fatalf("record %q missing v:1", line)
		}
		types = append(types, rec["type"].(string))
	}
	return types
}

func TestJSONLSinkMappingTable(t *testing.T) {
	cases := []struct {
		vivy string
		want string // "" means the event is rendered by the semantic sink, not JournalEvent
	}{
		{"run.started", "agent_start"},
		{"model.request", "message_start"},
		{"model.reasoning_delta", "message_update"},
		{"model.usage", "message_usage"},
		{"model.completed", "message_end"},
		{"context.compacted", "compaction_end"},
		{"provider.retry", "auto_retry_start"},
		{"provider.stall", "auto_retry_update"},
		{"tool.requested", "tool_execution_update"},
		{"tool.approval_decided", "tool_execution_update"},
		{"tool.operation", "tool_execution_update"},
		{"session.truncated", "session_update"},
		{"session.forked", "session_update"},
		{"model.call.finished", "journal_event"},
		{"hook.started", "journal_event"},
		{"workflow.started", "journal_event"},
		// semantic-sink types: no JournalEvent record
		{"model.delta", ""},
		{"tool.started", ""},
		{"tool.finished", ""},
		{"tool.approval_required", ""},
		{"user.question_required", ""},
		{"run.completed", ""},
		{"run.failed", ""},
		{"run.cancelled", ""},
	}
	for _, tc := range cases {
		var out bytes.Buffer
		sink := JSONLSink{Out: &out}
		sink.JournalEvent(tc.vivy, 7, json.RawMessage(`{"k":"v"}`))
		got := recordTypes(t, out.String())
		if tc.want == "" {
			if len(got) != 0 {
				t.Fatalf("%s emitted %v, want silent (semantic sink renders it)", tc.vivy, got)
			}
			continue
		}
		if len(got) != 1 || got[0] != tc.want {
			t.Fatalf("%s emitted %v, want [%s]", tc.vivy, got, tc.want)
		}
	}
}

func TestJSONLSinkLifecycleRecords(t *testing.T) {
	var out bytes.Buffer
	sink := JSONLSink{Out: &out}
	sink.RunStarted("sess-1", "run-1")
	sink.RunSettled("completed")
	got := recordTypes(t, out.String())
	want := []string{"session", "turn_start", "turn_end", "agent_end", "agent_settled"}
	if len(got) != len(want) {
		t.Fatalf("lifecycle records = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lifecycle records = %v, want %v", got, want)
		}
	}
}
