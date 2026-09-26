package memory_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/modules/memory"
	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/observer"
	"github.com/ProjectViVy/agent-vivy/bml"
)

func completedEvent(t *testing.T, runID string, seq int64, summary string) observer.RunEvent {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"outcome":      "success",
		"summary":      summary,
		"session_id":   "sess-1",
		"workspace_id": "ws-1",
		"tenant_id":    "local",
		"view":         "cli",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return observer.NewRunEvent(observer.NewEventID(runID, seq), "run.completed", time.Now().UnixMilli(), payload)
}

func TestProviderQueryMapsCandidatesToEnvelope(t *testing.T) {
	ctx := context.Background()
	service := openService(t)
	outcome := service.Add(ctx, bml.MemoryAddRequest{Content: "quartz lantern repair notes"})
	if outcome.Status != bml.CrudOutcomeApplied || outcome.Entry == nil {
		t.Fatalf("Add() status = %q, want applied with entry", outcome.Status)
	}
	page, err := memory.NewProvider().Query(ctx, contextsource.Request{Query: "quartz lantern", Limit: 4})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(page.Candidates) != 1 {
		t.Fatalf("Query() candidates = %d, want 1", len(page.Candidates))
	}
	candidate := page.Candidates[0]
	if candidate.SourceID != memory.ProviderID {
		t.Fatalf("SourceID = %q, want %q", candidate.SourceID, memory.ProviderID)
	}
	if candidate.ContentID != outcome.Entry.ID {
		t.Fatalf("ContentID = %q, want record id %q", candidate.ContentID, outcome.Entry.ID)
	}
	if candidate.Content != outcome.Entry.Content {
		t.Fatalf("Content = %q, want %q", candidate.Content, outcome.Entry.Content)
	}
	if candidate.MediaType != "text/markdown" {
		t.Fatalf("MediaType = %q, want text/markdown", candidate.MediaType)
	}
	if candidate.Version != "1" {
		t.Fatalf("Version = %q, want decimal revision \"1\"", candidate.Version)
	}
	if candidate.Treatment != contextsource.TreatmentCompetitive {
		t.Fatalf("Treatment = %q, want competitive", candidate.Treatment)
	}
	if candidate.Confidence != 1.0 {
		t.Fatalf("Confidence = %v, want 1.0", candidate.Confidence)
	}
	if candidate.UpdatedAt <= 0 || candidate.ValidUntil != 0 {
		t.Fatalf("UpdatedAt = %d, ValidUntil = %d; want UpdatedAt > 0 and ValidUntil == 0",
			candidate.UpdatedAt, candidate.ValidUntil)
	}
	if got := candidate.Metadata["vivy.memory-bml.kind"]; got != "long_term" {
		t.Fatalf("metadata kind = %q, want long_term", got)
	}
	if got := candidate.Metadata["vivy.memory-bml.trust"]; got != "user_asserted" {
		t.Fatalf("metadata trust = %q, want user_asserted", got)
	}
	if got := candidate.Metadata["vivy.memory-bml.provenance"]; got != "user_input" {
		t.Fatalf("metadata provenance = %q, want user_input", got)
	}
	for key := range candidate.Metadata {
		if len(key) <= len("vivy.memory-bml.") || key[:len("vivy.memory-bml.")] != "vivy.memory-bml." {
			t.Fatalf("metadata key %q is not namespaced under vivy.memory-bml.", key)
		}
	}
}

func TestProviderQueryPaginatesWithCursor(t *testing.T) {
	ctx := context.Background()
	service := openService(t)
	for i := 0; i < 3; i++ {
		outcome := service.Add(ctx, bml.MemoryAddRequest{Content: "alphabeta memory record"})
		if outcome.Status != bml.CrudOutcomeApplied {
			t.Fatalf("Add() status = %q, want applied", outcome.Status)
		}
	}
	provider := memory.NewProvider()
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 3; page++ {
		out, err := provider.Query(ctx, contextsource.Request{Query: "alphabeta", Limit: 1, Cursor: cursor})
		if err != nil {
			t.Fatalf("Query(cursor=%q) error = %v", cursor, err)
		}
		if len(out.Candidates) != 1 {
			t.Fatalf("page %d candidates = %d, want 1", page, len(out.Candidates))
		}
		if seen[out.Candidates[0].ContentID] {
			t.Fatalf("page %d repeated ContentID %q", page, out.Candidates[0].ContentID)
		}
		seen[out.Candidates[0].ContentID] = true
		cursor = out.NextCursor
		if page < 2 && cursor == "" {
			t.Fatalf("page %d NextCursor empty, want continuation", page)
		}
	}
	if cursor != "" {
		t.Fatalf("last page NextCursor = %q, want empty (no more results)", cursor)
	}
	out, err := provider.Query(ctx, contextsource.Request{Query: "alphabeta", Limit: 1, Cursor: "3"})
	if err != nil {
		t.Fatalf("Query(cursor=\"3\") error = %v", err)
	}
	if len(out.Candidates) != 0 || out.NextCursor != "" {
		t.Fatalf("beyond-end page = %d candidates, cursor %q; want empty page", len(out.Candidates), out.NextCursor)
	}
}

func TestProviderQueryRejectsInvalidCursor(t *testing.T) {
	openService(t)
	_, err := memory.NewProvider().Query(context.Background(), contextsource.Request{Query: "x", Cursor: "bogus"})
	if err == nil {
		t.Fatal("Query() with malformed cursor returned nil error")
	}
}

func TestProviderObserveCompletedIngestsHistoryOnce(t *testing.T) {
	ctx := context.Background()
	service := openService(t)
	provider := memory.NewProvider()
	event := completedEvent(t, "run-9", 3, "finished alpha work")
	receipt, err := provider.ObserveRunWithReceipt(ctx, event)
	if err != nil {
		t.Fatalf("ObserveRunWithReceipt() error = %v", err)
	}
	if receipt.State != observer.DeliveryCompleted {
		t.Fatalf("receipt state = %q, want completed", receipt.State)
	}
	if receipt.ReceiptID == "" {
		t.Fatal("receipt id is empty; the observer host rejects it")
	}
	if receipt.EventID != event.ID {
		t.Fatalf("receipt EventID = %v, want %v", receipt.EventID, event.ID)
	}

	// A redelivery of the same EventID returns the identical receipt and
	// must not write a second record.
	replay, err := provider.ObserveRunWithReceipt(ctx, event)
	if err != nil {
		t.Fatalf("replayed ObserveRunWithReceipt() error = %v", err)
	}
	if replay != receipt {
		t.Fatalf("replayed receipt = %+v, want %+v", replay, receipt)
	}

	page, err := provider.Query(ctx, contextsource.Request{Query: "finished alpha", Limit: 10})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(page.Candidates) != 1 {
		t.Fatalf("recall candidates = %d, want exactly 1 after redelivery", len(page.Candidates))
	}
	candidate := page.Candidates[0]
	if got := candidate.Metadata["vivy.memory-bml.kind"]; got != "history" {
		t.Fatalf("metadata kind = %q, want history", got)
	}
	if got := candidate.Metadata["vivy.memory-bml.evidence"]; got != "run:run-9" {
		t.Fatalf("metadata evidence = %q, want run:run-9", got)
	}

	// History records are recallable but never listed as long-term memory.
	listed := service.List(ctx, bml.MemoryListRequest{})
	if len(listed.Entries) != 0 {
		t.Fatalf("List() entries = %d, want 0 (history is not long-term visible)", len(listed.Entries))
	}
}

func TestProviderObserveSkipsNonCompletedEvents(t *testing.T) {
	ctx := context.Background()
	openService(t)
	provider := memory.NewProvider()
	payload, err := json.Marshal(map[string]string{"outcome": "failed", "cause_category": "internal_error"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	event := observer.NewRunEvent(observer.NewEventID("run-5", 1), "run.failed", time.Now().UnixMilli(), payload)
	receipt, err := provider.ObserveRunWithReceipt(ctx, event)
	if err != nil {
		t.Fatalf("ObserveRunWithReceipt() error = %v", err)
	}
	if receipt.State != observer.DeliveryCompleted || receipt.ReceiptID == "" {
		t.Fatalf("receipt = %+v, want completed with non-empty id", receipt)
	}
	page, err := provider.Query(ctx, contextsource.Request{Query: "run-5", Limit: 10})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(page.Candidates) != 0 {
		t.Fatalf("recall candidates = %d, want 0 for a skipped event type", len(page.Candidates))
	}
}

func TestProviderObserveFailedReceiptOnStoreError(t *testing.T) {
	dir := t.TempDir()
	// A regular file where the {config}/memory directory belongs makes the
	// first store open fail inside ObserveRunWithReceipt.
	if err := os.WriteFile(filepath.Join(dir, "memory"), []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := memory.Open(context.Background(), testConfig(dir)); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = memory.Close() })

	event := completedEvent(t, "run-err", 1, "write will fail")
	receipt, err := memory.NewProvider().ObserveRunWithReceipt(context.Background(), event)
	if err == nil {
		t.Fatal("ObserveRunWithReceipt() returned nil error on a broken store")
	}
	if receipt.State != observer.DeliveryFailed {
		t.Fatalf("receipt state = %q, want failed", receipt.State)
	}
	if receipt.ReceiptID == "" {
		t.Fatal("failed receipt id is empty; the observer host rejects it")
	}
	if receipt.EventID != event.ID {
		t.Fatalf("receipt EventID = %v, want %v", receipt.EventID, event.ID)
	}
}

func TestProviderReportsUnavailableWithoutService(t *testing.T) {
	if err := memory.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	provider := memory.NewProvider()
	if _, err := provider.Query(context.Background(), contextsource.Request{Query: "x"}); err == nil {
		t.Fatal("Query() without an open service returned nil error")
	}
	event := completedEvent(t, "run-closed", 1, "no service")
	receipt, err := provider.ObserveRunWithReceipt(context.Background(), event)
	if err == nil {
		t.Fatal("ObserveRunWithReceipt() without an open service returned nil error")
	}
	if receipt.State != observer.DeliveryFailed || receipt.ReceiptID == "" || receipt.EventID != event.ID {
		t.Fatalf("receipt = %+v, want failed with non-empty id and matching EventID", receipt)
	}
}

func TestProviderObserveAcksUndecodableCompletedEvent(t *testing.T) {
	ctx := context.Background()
	openService(t)
	provider := memory.NewProvider()

	// A malformed payload can never decode, and the host never abandons a
	// failed delivery — so the only truthful non-wedging disposition is an
	// acknowledged completion with no record written.
	poison := observer.NewRunEvent(observer.NewEventID("run-poison", 1), "run.completed",
		time.Now().UnixMilli(), json.RawMessage(`{"outcome":`))
	receipt, err := provider.ObserveRunWithReceipt(ctx, poison)
	if err != nil {
		t.Fatalf("ObserveRunWithReceipt(poison) error = %v, want nil (ack)", err)
	}
	if receipt.State != observer.DeliveryCompleted || receipt.ReceiptID == "" {
		t.Fatalf("poison receipt = %+v, want completed with non-empty id", receipt)
	}
	replay, err := provider.ObserveRunWithReceipt(ctx, poison)
	if err != nil || replay != receipt {
		t.Fatalf("replayed poison receipt = %+v, err %v; want identical ack", replay, err)
	}

	// A missing required field (empty RunID) is equally permanent.
	norun := completedEvent(t, "", 7, "no run id")
	norunReceipt, err := provider.ObserveRunWithReceipt(ctx, norun)
	if err != nil {
		t.Fatalf("ObserveRunWithReceipt(empty RunID) error = %v, want nil (ack)", err)
	}
	if norunReceipt.State != observer.DeliveryCompleted {
		t.Fatalf("empty-RunID receipt state = %q, want completed", norunReceipt.State)
	}

	// The wedge regression: a well-formed event later in the same run must
	// still ingest — the poisoned event did not block the cursor.
	good := completedEvent(t, "run-poison", 2, "later work lands")
	goodReceipt, err := provider.ObserveRunWithReceipt(ctx, good)
	if err != nil {
		t.Fatalf("ObserveRunWithReceipt(good) error = %v", err)
	}
	if goodReceipt.State != observer.DeliveryCompleted {
		t.Fatalf("good receipt state = %q, want completed", goodReceipt.State)
	}
	page, err := provider.Query(ctx, contextsource.Request{Query: "run-poison", Limit: 10})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(page.Candidates) != 1 {
		t.Fatalf("recall candidates = %d, want exactly 1 (the good event only)", len(page.Candidates))
	}
}

func TestProviderIngestSeparatesCollidingRunIDs(t *testing.T) {
	ctx := context.Background()
	openService(t)
	provider := memory.NewProvider()

	// "a/b" and "a\b" sanitize to the same id stem; the raw-id hash suffix
	// must keep their dedupe keys distinct.
	slash := completedEvent(t, "a/b", 1, "collision alpha")
	backslash := completedEvent(t, "a\\b", 1, "collision alpha")
	slashReceipt, err := provider.ObserveRunWithReceipt(ctx, slash)
	if err != nil {
		t.Fatalf("ObserveRunWithReceipt(a/b) error = %v", err)
	}
	backslashReceipt, err := provider.ObserveRunWithReceipt(ctx, backslash)
	if err != nil {
		t.Fatalf("ObserveRunWithReceipt(a\\b) error = %v", err)
	}
	if slashReceipt.ReceiptID == backslashReceipt.ReceiptID {
		t.Fatalf("colliding RunIDs share receipt id %q", slashReceipt.ReceiptID)
	}
	page, err := provider.Query(ctx, contextsource.Request{Query: "collision alpha", Limit: 10})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(page.Candidates) != 2 {
		t.Fatalf("recall candidates = %d, want 2 (one per distinct RunID)", len(page.Candidates))
	}
	if page.Candidates[0].ContentID == page.Candidates[1].ContentID {
		t.Fatalf("colliding RunIDs share record id %q", page.Candidates[0].ContentID)
	}
}

func TestProviderQueryRejectsAbsurdCursor(t *testing.T) {
	openService(t)
	for _, cursor := range []string{"9223372036854775807", "4294967296"} {
		_, err := memory.NewProvider().Query(context.Background(), contextsource.Request{Query: "x", Cursor: cursor})
		if err == nil {
			t.Fatalf("Query(cursor=%q) returned nil error", cursor)
		}
		if !strings.Contains(err.Error(), bml.HomeCodeInvalidRequest) {
			t.Fatalf("Query(cursor=%q) error = %q, want the memory_invalid_request vocabulary", cursor, err)
		}
	}
}

func TestProviderObserveRunDelegatesToReceiptPath(t *testing.T) {
	openService(t)
	provider := memory.NewProvider()
	if err := provider.ObserveRun(context.Background(), completedEvent(t, "run-8", 1, "delegated write")); err != nil {
		t.Fatalf("ObserveRun() error = %v", err)
	}
	page, err := provider.Query(context.Background(), contextsource.Request{Query: "delegated write", Limit: 4})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(page.Candidates) != 1 {
		t.Fatalf("recall candidates = %d, want 1", len(page.Candidates))
	}
}
