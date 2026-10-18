package bml

import (
	"context"
	"testing"
)

// openReplayStore opens one disposable root and closes every handle it owns
// when the test ends.
func openReplayStore(t *testing.T, dir string) *Store {
	t.Helper()
	store, err := Open(context.Background(), dir, "workspace-1")
	if err != nil {
		t.Fatalf("Open(%q): %v", dir, err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// TestBMLCloseoutTombstoneReplayAcrossReopen proves BML-R4's library half:
// a superseding tombstone keeps its target out of visible FTS after the store
// is closed and reopened, while an insert-only same-record replay is rejected
// by the existing revision guard. Raw retention is not erasure.
func TestBMLCloseoutTombstoneReplayAcrossReopen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	const (
		targetID    = "memory-replay-target"
		tombstoneID = "memory-replay-tombstone"
	)

	first := openReplayStore(t, dir)
	meta, err := first.Metadata(ctx)
	if err != nil {
		t.Fatalf("Metadata before Put: %v", err)
	}
	target := storeRecord(targetID, "workspace-1", "replayprobe durable target")
	stored, err := first.Put(ctx, target, meta.StoreRevision, nil)
	if err != nil {
		t.Fatalf("Put target: %v", err)
	}
	if stored.Record.ID != targetID || stored.Revision != 1 {
		t.Fatalf("stored target = %+v, want revision 1", stored)
	}
	meta, err = first.Metadata(ctx)
	if err != nil {
		t.Fatalf("Metadata before tombstone: %v", err)
	}
	if err := first.PutTombstone(ctx, tombstoneRecord(tombstoneID, "workspace-1", targetID), meta.StoreRevision, targetID, stored.Revision); err != nil {
		t.Fatalf("PutTombstone: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen the same root; no reseeding and no in-memory cache stands in for
	// durability.
	second := openReplayStore(t, dir)
	meta, err = second.Metadata(ctx)
	if err != nil {
		t.Fatalf("Metadata after reopen: %v", err)
	}

	// Same existing record, insert-only replay: the revision guard must
	// reject it without another row or metadata movement.
	_, err = second.Put(ctx, target, meta.StoreRevision, nil)
	if code := storeErrCode(t, err); code != ErrRecordRevisionConflict {
		t.Fatalf("insert-only replay code = %q, want %q", code, ErrRecordRevisionConflict)
	}
	afterReplay, err := second.Metadata(ctx)
	if err != nil {
		t.Fatalf("Metadata after replay: %v", err)
	}
	if afterReplay != meta {
		t.Fatalf("metadata moved on rejected replay: %+v vs %+v", afterReplay, meta)
	}

	// The low-level API may still accept an explicit rewrite of the raw target
	// at its actual revision; the durable tombstone must keep suppressing it.
	observed := stored.Revision
	rewritten := target
	rewritten.Content = "replayprobe rewritten target"
	rewritten.Provenance.ContentDigest = MemoryContentDigest([]byte(rewritten.Content))
	if _, err := second.Put(ctx, rewritten, meta.StoreRevision, &observed); err != nil {
		t.Fatalf("rewrite target: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("Close before final reopen: %v", err)
	}

	third := openReplayStore(t, dir)
	hits, err := third.SearchVisible(ctx, searchQuery("replayprobe", "tenant-1", "workspace-1", nil, 10))
	if err != nil {
		t.Fatalf("SearchVisible: %v", err)
	}
	for _, hit := range hits {
		if hit.Record.Record.ID == targetID {
			t.Fatalf("tombstoned target resurfaced in visible FTS: %+v", hit)
		}
	}
	// The tombstone still names the target it deposed.
	tombstone, err := third.Get(ctx, tombstoneID)
	if err != nil {
		t.Fatalf("Get tombstone: %v", err)
	}
	if tombstone.Record.Tombstone == nil || tombstone.Record.Tombstone.TargetRecordID != targetID {
		t.Fatalf("tombstone record = %+v, want target %q", tombstone.Record, targetID)
	}

	// Raw listing retains exactly one target row plus the tombstone; raw Get
	// may still return the retained target, which is not public visibility.
	locked, err := third.List(ctx, 100)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var targets, tombstones int
	for _, record := range locked {
		switch record.Record.ID {
		case targetID:
			targets++
			if record.Revision != 2 {
				t.Fatalf("retained target revision = %d, want 2", record.Revision)
			}
		case tombstoneID:
			tombstones++
		}
	}
	if targets != 1 || tombstones != 1 {
		t.Fatalf("raw listing targets=%d tombstones=%d, want one of each: %+v", targets, tombstones, locked)
	}
	raw, err := third.Get(ctx, targetID)
	if err != nil || raw == nil {
		t.Fatalf("raw retained target = %v, %v; want retained record", raw, err)
	}
}
