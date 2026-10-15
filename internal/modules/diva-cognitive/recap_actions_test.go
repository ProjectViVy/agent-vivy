package divacognitive

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	action "agent-vivy/sdk/port/controlaction"
)

type recapTestControl struct {
	fakeControl
	deny    bool
	binding cognitivecontract.RecapBinding
}

func (c *recapTestControl) AuthorizeRecap(_ context.Context, sid domain.SessionID, rid domain.RunID, name string) (cognitivecontract.RecapBinding, error) {
	if c.deny || sid != "main" || (name != "" && (rid != "run-main" || name == "unknown")) {
		return cognitivecontract.RecapBinding{}, errors.New("denied")
	}
	return c.binding, nil
}
func managedRecap(t *testing.T) (cognitivecontract.Bundle, *recapTestControl) {
	t.Helper()
	c := &recapTestControl{binding: cognitivecontract.RecapBinding{SubjectID: "subject", DomainID: "personal"}}
	in := testFactoryInput(t)
	in.ResolveRecapBinding = func(context.Context, string) (cognitivecontract.RecapBinding, error) { return c.binding, nil }
	b, err := Open(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	if err = b.AttachRuntime(c); err != nil {
		t.Fatal(err)
	}
	return b, c
}

type recapWireTest struct {
	Status string `json:"status"`
	Head   *struct {
		Revision          uint64 `json:"revision"`
		Text              string `json:"text"`
		Cause             string `json:"cause"`
		CoveredThrough    uint64 `json:"covered_through"`
		InvalidationEpoch uint64 `json:"invalidation_epoch"`
	} `json:"head"`
	Revisions []struct {
		Revision uint64 `json:"revision"`
		Text     string `json:"text"`
		Cause    string `json:"cause"`
	} `json:"revisions"`
	Reason     string `json:"reason"`
	Activation string `json:"activation"`
}

func recapCall(t *testing.T, b cognitivecontract.Bundle, id string, input any) recapWireTest {
	t.Helper()
	raw, _ := json.Marshal(input)
	result, err := b.(cognitivecontract.DispatcherProvider).Dispatcher().Invoke(context.Background(), "vivy.recap."+id, raw)
	if err != nil {
		t.Fatal(err)
	}
	var out recapWireTest
	if err = json.Unmarshal(result, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestIssue58RecapActionsCAS(t *testing.T) {
	b, _ := managedRecap(t)
	original := "完整中文\r\n\n# Recap\n---\n"
	seeded, err := recapPort(t, b).CommitRecap(context.Background(), cognitivecontract.RecapCommit{OperationID: "summary", Cause: "summary", Text: original, Sources: cognitivecontract.SourceRange{Through: 7, Digest: "runtime-only"}})
	if err != nil {
		t.Fatal(err)
	}
	got := recapCall(t, b, "read", map[string]any{"session_id": "main"})
	if got.Status != "listed" || got.Head == nil || got.Head.Text != original || got.Head.Revision != seeded.Revision {
		t.Fatalf("read=%+v", got)
	}
	input := map[string]any{"session_id": "main", "base_revision": 1, "operation_id": "edit-1", "text": "手工全文\n第二行\n"}
	edited := recapCall(t, b, "edit", input)
	if edited.Status != "applied" || edited.Head == nil || edited.Head.Revision != 2 || edited.Head.CoveredThrough != 7 || edited.Activation != "next_run" {
		t.Fatalf("edit=%+v", edited)
	}
	replay := recapCall(t, b, "edit", input)
	if replay.Status != "applied" || replay.Head.Revision != 2 {
		t.Fatalf("replay=%+v", replay)
	}
	input["operation_id"] = "stale"
	input["text"] = "must survive client conflict"
	stale := recapCall(t, b, "edit", input)
	if stale.Status != "conflict" || stale.Head == nil || stale.Head.Revision != 2 || stale.Head.Text != "手工全文\n第二行\n" {
		t.Fatalf("stale=%+v", stale)
	}
	cleared := recapCall(t, b, "clear", map[string]any{"session_id": "main", "base_revision": 2, "operation_id": "clear"})
	if cleared.Status != "applied" || cleared.Head.Text != "" || cleared.Head.CoveredThrough != 7 || cleared.Head.InvalidationEpoch != 1 || cleared.Activation != "next_safe_boundary" {
		t.Fatalf("clear=%+v", cleared)
	}
	restored := recapCall(t, b, "restore", map[string]any{"session_id": "main", "base_revision": 3, "operation_id": "restore", "revision": 1})
	if restored.Status != "applied" || restored.Head.Text != original || restored.Head.Revision != 4 || restored.Head.CoveredThrough != 7 || restored.Head.InvalidationEpoch != 1 {
		t.Fatalf("restore=%+v", restored)
	}
	head, err := recapPort(t, b).ReadRecap(context.Background())
	if err != nil || head.Sources != (cognitivecontract.SourceRange{}) {
		t.Fatalf("certificate=%+v %v", head, err)
	}
	history := recapCall(t, b, "history", map[string]any{"session_id": "main", "limit": 10})
	if history.Status != "listed" || len(history.Revisions) != 4 || history.Revisions[3].Text != original {
		t.Fatalf("history=%+v", history)
	}
}
func TestIssue58RecapActionsAuthority(t *testing.T) {
	b, c := managedRecap(t)
	for _, sid := range []string{"ordinary", "worker", "private", "unknown", ""} {
		got := recapCall(t, b, "read", map[string]any{"session_id": sid})
		if got.Status != "failed" || got.Reason != "recap_scope_denied" || got.Head != nil {
			t.Fatalf("%s=%+v", sid, got)
		}
	}
	c.deny = true
	got := recapCall(t, b, "read", map[string]any{"session_id": "main"})
	if got.Head != nil || got.Reason != "recap_scope_denied" {
		t.Fatalf("revoked=%+v", got)
	}
	c.deny = false
	for _, extra := range []string{"subject_id", "domain_id", "cause", "sources", "receipt", "binding"} {
		input := map[string]any{"session_id": "main", "base_revision": 0, "operation_id": "spoof", "text": "x", extra: "spoof"}
		raw, _ := json.Marshal(input)
		_, err := b.(cognitivecontract.DispatcherProvider).Dispatcher().Invoke(context.Background(), "vivy.recap.edit", raw)
		if !errors.Is(err, action.ErrInvalidInput) {
			t.Fatalf("%s accepted: %v", extra, err)
		}
	}
	for _, input := range []string{`{"session_id":"main","operation_id":"missing-base","text":"x"}`, `{"session_id":"main","base_revision":0,"text":"x"}`, `{"session_id":"main","base_revision":0,"operation_id":"","text":"x"}`} {
		_, err := b.(cognitivecontract.DispatcherProvider).Dispatcher().Invoke(context.Background(), "vivy.recap.edit", json.RawMessage(input))
		if !errors.Is(err, action.ErrInvalidInput) {
			t.Fatalf("missing required accepted: %v", err)
		}
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	got = recapCall(t, b, "read", map[string]any{"session_id": "main"})
	if got.Status != "failed" || got.Head != nil {
		t.Fatalf("closed=%+v", got)
	}
}
func TestIssue58HistoricalCorrectionSources(t *testing.T) {
	b, _ := managedRecap(t)
	_, err := recapPort(t, b).CommitRecap(context.Background(), cognitivecontract.RecapCommit{OperationID: "summary", Cause: "summary", Text: "before\n", Sources: cognitivecontract.SourceRange{Through: 7, Digest: "runtime-only"}})
	if err != nil {
		t.Fatal(err)
	}
	corrected, err := recapPort(t, b).CommitRecap(context.Background(), cognitivecontract.RecapCommit{BaseRevision: 1, OperationID: "runtime-reconcile", Cause: "reconcile", Text: "准确的修订\n\n", Sources: cognitivecontract.SourceRange{After: 7, Through: 7, Digest: "runtime-owned-envelope"}})
	if err != nil {
		t.Fatal(err)
	}
	got := recapCall(t, b, "revision", map[string]any{"session_id": "main", "revision": 2})
	if got.Head == nil || got.Head.Text != corrected.Text || got.Head.Cause != "reconcile" || got.Head.CoveredThrough != 7 || got.Head.InvalidationEpoch != 0 {
		t.Fatalf("reconcile=%+v", got)
	}
	sources := recapCall(t, b, "sources", map[string]any{"session_id": "main", "revision": 2})
	if sources.Status != "failed" || sources.Reason != "recap_sources_unavailable" {
		t.Fatalf("sources fabricated: %+v", sources)
	}
	denied := recapCall(t, b, "sources", map[string]any{"session_id": "worker", "revision": 2})
	if denied.Reason != "recap_scope_denied" {
		t.Fatalf("denied indistinct=%+v", denied)
	}
}

func TestIssue58RecapStrictExactJSON(t *testing.T) {
	b, _ := managedRecap(t)
	prefix := `{"session_id":"main","base_revision":0,"operation_id":"invalid","text":`
	cases := []json.RawMessage{
		append(append([]byte(prefix+`"`), 0xff), []byte(`"}`)...),
		json.RawMessage(prefix + `"\ud800"}`), json.RawMessage(prefix + `"\udc00"}`),
		json.RawMessage(prefix + `null}`), json.RawMessage(prefix + `"first","text":"second"}`),
		json.RawMessage(`{"session_id":"main","base_revision":null,"operation_id":"invalid","text":"x"}`),
		json.RawMessage(`{"session_id":"main","base_revision":0,"operation_id":null,"text":"x"}`),
	}
	for i, raw := range cases {
		_, err := b.(cognitivecontract.DispatcherProvider).Dispatcher().Invoke(context.Background(), "vivy.recap.edit", raw)
		if !errors.Is(err, action.ErrInvalidInput) {
			t.Fatalf("case %d silently normalized/ambiguous: %v", i, err)
		}
	}
	head, _ := recapPort(t, b).ReadRecap(context.Background())
	if head.Revision != 0 {
		t.Fatalf("invalid JSON wrote owner revision %d", head.Revision)
	}
}

func TestIssue58RecapValidUnicodeRemainsExact(t *testing.T) {
	b, _ := managedRecap(t)
	raw := json.RawMessage(`{"session_id":"main","base_revision":0,"operation_id":"unicode","text":"中文\r\n\ud83d\ude00\n�\\ud800\/"}`)
	result, err := b.(cognitivecontract.DispatcherProvider).Dispatcher().Invoke(context.Background(), "vivy.recap.edit", raw)
	if err != nil {
		t.Fatal(err)
	}
	var out recapWireTest
	json.Unmarshal(result, &out)
	if out.Status != "applied" || out.Head == nil || out.Head.Text != "中文\r\n😀\n�\\ud800/" {
		t.Fatalf("valid UTF-8 changed: %s", result)
	}
}
