package bml

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAppendHistoryReusesOriginalSourceIdentityAcrossIDChange(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	h := NewHome(root)
	input := AppendHistoryInput{ID: "old-opaque-record", Content: "preserve source history", SourceID: "run/raw:7", CapturedAt: time.Now().UTC()}
	first, applied, err := h.AppendHistory(ctx, input)
	if err != nil || !applied {
		t.Fatalf("initial append: applied=%v, err=%v", applied, err)
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
	h = NewHome(root)
	t.Cleanup(func() { _ = h.Close() })
	input.ID = "new-reversible-record"
	// SourceID names the same immutable committed event. A newer projection
	// cannot replace the historical representation or allocate another record.
	input.Content = "new projection must not rewrite old history"
	got, applied, err := h.AppendHistory(ctx, input)
	if err != nil || applied || got.Record.ID != first.Record.ID || got.Revision != first.Revision || got.Record.Content != first.Record.Content {
		t.Fatalf("source replay changed stored identity: %+v, applied=%v, err=%v", got, applied, err)
	}
	store, err := h.writableStore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := store.Metadata(ctx)
	if err != nil || meta.RecordCount != 1 {
		t.Fatalf("metadata=%+v, err=%v", meta, err)
	}
}

func TestAppendHistoryConcurrentDifferentAllocationsKeepOneSource(t *testing.T) {
	ctx := context.Background()
	h := NewHome(t.TempDir())
	t.Cleanup(func() { _ = h.Close() })
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	ids := make(chan string, 8)
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				got, _, err := h.AppendHistory(ctx, AppendHistoryInput{ID: fmt.Sprintf("allocation-%d", i), Content: "same source", SourceID: "run:9"})
				if err == nil {
					ids <- got.Record.ID
					return
				}
				var se *StoreError
				if !errors.As(err, &se) || se.Code != ErrStoreRevisionConflict {
					errs <- err
					return
				}
			}
			errs <- errors.New("source append retries exhausted")
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	close(ids)
	var winner string
	for id := range ids {
		if winner != "" && winner != id {
			t.Fatalf("source has multiple allocations: %q and %q", winner, id)
		}
		winner = id
	}
	store, err := h.writableStore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := store.Metadata(ctx)
	if err != nil || meta.RecordCount != 1 {
		t.Fatalf("duplicate source records: %+v, %v", meta, err)
	}
}

func TestAppendHistoryRejectsAmbiguousSourceAndConflictingRecordID(t *testing.T) {
	ctx := context.Background()
	h := NewHome(t.TempDir())
	t.Cleanup(func() { _ = h.Close() })
	input := AppendHistoryInput{ID: "old-record", Content: "original history", SourceID: "run:7"}
	first, _, err := h.AppendHistory(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input.SourceID = "other-run:7"
	if _, _, err := h.AppendHistory(ctx, input); err == nil {
		t.Fatal("record ID reused for another source")
	}
	store, err := h.writableStore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := store.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := first.Record
	duplicate.ID = "second-historical-record"
	if _, err := store.Put(ctx, duplicate, meta.StoreRevision, nil); err != nil {
		t.Fatal(err)
	}
	input.ID, input.SourceID = "new-record", "run:7"
	_, _, err = h.AppendHistory(ctx, input)
	var storeErr *StoreError
	if !errors.As(err, &storeErr) || storeErr.Code != ErrImportConflict {
		t.Fatalf("ambiguous source accepted: %v", err)
	}
}

func TestAppendHistoryPreservesExactSourceBytes(t *testing.T) {
	ctx := context.Background()
	h := NewHome(t.TempDir())
	t.Cleanup(func() { _ = h.Close() })
	for index, source := range []string{"run:7", " run:7"} {
		id := []string{"one", "two"}[index]
		got, applied, err := h.AppendHistory(ctx, AppendHistoryInput{ID: id, Content: "distinct source", SourceID: source})
		if err != nil || !applied || got.Record.Provenance.SourceID != source {
			t.Fatalf("source bytes lost: %+v %v %v", got, applied, err)
		}
	}
}
