package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/testsupport"
)

// admittedReference runs one real admission turn and returns both the run and
// the committed snapshot.
func admittedReference(t *testing.T, f *referenceFixture, requestID string) (domain.RunID, domain.ContextReference) {
	t.Helper()
	preview := f.PreviewAsOperator(t, "B", safeSelectionA())
	runID, err := f.AdmitAsOperator(t, &domain.ContinuityInput{
		RequestID: requestID,
		References: []domain.ReferenceSelection{
			{Selection: preview.Selection, ExpectedDigest: preview.Digest},
		},
	})
	if err != nil {
		t.Fatalf("admission: %v", err)
	}
	waitForRunStatus(t, f.backend, runID, domain.RunCompleted)
	events := committedReferenceEvents(t, f.backend, runID)
	if len(events) != 1 {
		t.Fatalf("committed references = %#v", events)
	}
	return runID, events[0]
}

// TestReferenceLifecycleCompactionManifestKeepsIDs: folded turns keep their
// reference IDs in the compaction record and the compacted event payload.
func TestReferenceLifecycleCompactionManifestKeepsIDs(t *testing.T) {
	cfg := EngineConfig{
		StreamBuffer:         8,
		MaxEventPayloadBytes: 64 << 10,
		MaxContextBytes:      256,
		Compaction:           &CompactionPolicy{Enabled: true, MaxTokens: 200, TriggerPercent: 1, KeepRecent: 1},
	}
	f := newReferenceFixtureWithConfig(t, WrapModel(testsupport.NewEchoModel()), cfg)
	runID, reference := admittedReference(t, f, "req-compact")
	_ = runID
	result, err := f.svc.CompactSession(context.Background(), "B")
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if result.Folded == 0 || result.Skipped {
		t.Fatalf("compaction folded nothing: %#v", result)
	}
	rec, ok, err := f.backend.LatestSessionCompaction(context.Background(), "B")
	if err != nil || !ok {
		t.Fatalf("latest compaction: %v ok=%v", err, ok)
	}
	if !strings.Contains(rec.Summary, reference.ID) {
		t.Fatalf("compaction record lost the reference manifest: %q", rec.Summary)
	}
	// The durable event payload carries the same explicit ID list.
	iter, err := f.backend.Replay(context.Background(), rec.RunID, 0)
	if err != nil {
		t.Fatalf("replay compaction run: %v", err)
	}
	defer func() { _ = iter.Close() }()
	found := false
	for iter.Next() {
		entry := iter.Value()
		if entry.Event.Type != domain.EventContextCompacted {
			continue
		}
		var payload payloadContextCompacted
		if err := json.Unmarshal(entry.Event.Payload, &payload); err != nil {
			t.Fatalf("decode compacted payload: %v", err)
		}
		for _, id := range payload.ReferenceIDs {
			if id == reference.ID {
				found = true
			}
		}
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !found {
		t.Fatalf("context.compacted lacks reference_ids for %s", reference.ID)
	}
	if _, err := f.refs.Lookup(context.Background(), "B", reference.ID); err != nil {
		t.Fatalf("compaction broke reference lookup: %v", err)
	}
}

// TestReferenceLifecycleSourceDeleteKeepsCopy: deleting the source session
// leaves the destination-owned copy readable with an honest source status.
func TestReferenceLifecycleSourceDeleteKeepsCopy(t *testing.T) {
	f := newReferenceFixture(t)
	_, reference := admittedReference(t, f, "req-deleted-source")
	view, err := f.refs.Get(context.Background(), "B", reference.ID)
	if err != nil {
		t.Fatalf("reference get: %v", err)
	}
	if view.SourceStatus != "ok" || view.FeedStatus != "included" {
		t.Fatalf("live view = %#v", view)
	}
	if err := f.backend.DeleteSession(context.Background(), "A"); err != nil {
		t.Fatalf("delete source: %v", err)
	}
	got, err := f.refs.Lookup(context.Background(), "B", reference.ID)
	if err != nil {
		t.Fatalf("lookup after source delete: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Text != "release notes draft for alpha" {
		t.Fatalf("retained copy drifted: %#v", got)
	}
	view, err = f.refs.Get(context.Background(), "B", reference.ID)
	if err != nil {
		t.Fatalf("get after source delete: %v", err)
	}
	if view.SourceStatus != "source_unavailable" {
		t.Fatalf("source status = %q", view.SourceStatus)
	}
}

// TestReferenceLifecycleRewindHidesTurn: rewinding past the owning turn hides
// its imported blocks from the feed while the copy stays readable by ID.
func TestReferenceLifecycleRewindHidesTurn(t *testing.T) {
	rec := &recordingModel{inner: WrapModel(testsupport.NewEchoModel())}
	f := newReferenceFixtureWithConfig(t, rec, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	first, err := f.svc.RunWithOptions(context.Background(), "B", "first turn", RunOptions{})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	waitForRunStatus(t, f.backend, first, domain.RunCompleted)
	_, reference := admittedReference(t, f, "req-rewind")
	messages, err := f.backend.ListMessages(context.Background(), "B")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	owningID := ""
	for _, message := range messages {
		if message.RunID == reference.DestinationRunID && message.Role == domain.RoleUser {
			owningID = message.ID
		}
	}
	if owningID == "" {
		t.Fatal("owning user message not found")
	}
	if _, err := f.svc.RewindSession(context.Background(), "B", owningID); err != nil {
		t.Fatalf("rewind: %v", err)
	}
	third, err := f.svc.RunWithOptions(context.Background(), "B", "after rewind", RunOptions{})
	if err != nil {
		t.Fatalf("third run: %v", err)
	}
	waitForRunStatus(t, f.backend, third, domain.RunCompleted)
	if copies := countImportedBlocks(latestFeed(rec)); copies != 0 {
		t.Fatalf("rewound reference still projected %d blocks", copies)
	}
	if _, err := f.refs.Lookup(context.Background(), "B", reference.ID); err != nil {
		t.Fatalf("rewound reference unreadable by id: %v", err)
	}
}

// TestReferenceLifecycleForkCopiesPrefixRefs: a fork copies only the included
// prefix's snapshots into the child with fresh destination IDs, keeping the
// original provenance and carrying no scope or approval state.
func TestReferenceLifecycleForkCopiesPrefixRefs(t *testing.T) {
	rec := &recordingModel{inner: WrapModel(testsupport.NewEchoModel())}
	f := newReferenceFixtureWithConfig(t, rec, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	runOne, refOne := admittedReference(t, f, "req-fork-one")
	_, refTwo := admittedReference(t, f, "req-fork-two")
	messages, err := f.backend.ListMessages(context.Background(), "B")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	cutoffID := ""
	for _, message := range messages {
		if message.RunID == runOne {
			cutoffID = message.ID
		}
	}
	if cutoffID == "" {
		t.Fatal("turn-one cutoff message missing")
	}
	result, err := f.svc.ForkSession(context.Background(), "B", cutoffID, "fork")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	child := domain.SessionID(result.SessionID)
	// The child's snapshot copies live under its own derived history run.
	copies := committedReferenceEvents(t, f.backend, forkHistoryRunID(child))
	if len(copies) != 1 {
		t.Fatalf("forked reference copies = %#v", copies)
	}
	copiedRef := copies[0]
	if copiedRef.ID == refOne.ID || copiedRef.DestinationSessionID != child {
		t.Fatalf("copied reference identity = %#v", copiedRef)
	}
	if copiedRef.DestinationRunID != runOne || copiedRef.SourceSessionID != "A" || copiedRef.Digest != refOne.Digest || copiedRef.Origin != refOne.Origin {
		t.Fatalf("copied provenance drifted: %#v", copiedRef)
	}
	if _, err := f.refs.Lookup(context.Background(), child, copiedRef.ID); err != nil {
		t.Fatalf("child cannot read its copy: %v", err)
	}
	if _, err := f.refs.Lookup(context.Background(), child, refTwo.ID); err == nil {
		t.Fatal("excluded-suffix reference leaked into the fork")
	}
	// No scope or approval travels: the child has no run rows yet, so it
	// cannot carry a run.started scope, and its receipts list stays empty.
	runs, err := f.backend.ListRunsBySession(context.Background(), child)
	if err != nil {
		t.Fatalf("list child runs: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("fork created %d unexpected child run rows", len(runs))
	}
	// The child feed projects the copied snapshot once.
	childRun, err := f.svc.RunWithOptions(context.Background(), child, "inside fork", RunOptions{})
	if err != nil {
		t.Fatalf("child run: %v", err)
	}
	waitForRunStatus(t, f.backend, childRun, domain.RunCompleted)
	if copies := countImportedBlocks(latestFeed(rec)); copies != 1 {
		t.Fatalf("child feed imported blocks = %d, want 1", copies)
	}
}

// TestReferenceLifecycleReopenKeepsIDs: a rebuilt service over the same
// journal resolves the same durable reference IDs (checkpoint-resume shape).
func TestReferenceLifecycleReopenKeepsIDs(t *testing.T) {
	f := newReferenceFixture(t)
	_, reference := admittedReference(t, f, "req-reopen")
	rebuilt := NewReferenceService(f.history, f.backend, f.backend, f.backend, f.backend)
	got, err := rebuilt.Lookup(context.Background(), "B", reference.ID)
	if err != nil {
		t.Fatalf("rebuilt lookup: %v", err)
	}
	if got.ID != reference.ID || got.Digest != reference.Digest || len(got.Items) != 1 {
		t.Fatalf("rebuilt reference drifted: %#v", got)
	}
}

// TestReferenceLifecycleCorruptSnapshotIsExplicit: an undecodable committed
// event surfaces as an explicit error, never as an empty successful copy.
func TestReferenceLifecycleCorruptSnapshotIsExplicit(t *testing.T) {
	f := newReferenceFixture(t)
	f.createLiveRun(t, "run-corrupt")
	if _, err := f.backend.Append(context.Background(), storage.Commit{
		RunID: "run-corrupt",
		Events: []domain.RunEvent{{
			RunID: "run-corrupt", Type: domain.EventContextReferenceAttached,
			CreatedAt: 30, PayloadVersion: 1, Payload: []byte("{corrupt"),
		}},
	}); err != nil {
		t.Fatalf("append corrupt event: %v", err)
	}
	_, err := f.refs.Lookup(context.Background(), "B", "ref_whatever")
	if err == nil {
		t.Fatal("corrupt journal returned an empty successful reference")
	}
	// The feed scan tolerates the corrupt event instead of crashing.
	if _, err := f.refs.AttachedReferences(context.Background(), "B"); err != nil {
		t.Fatalf("feed scan failed on corrupt event: %v", err)
	}
}
