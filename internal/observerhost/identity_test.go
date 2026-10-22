package observerhost

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestCursorKeyPreservesEntireTuple(t *testing.T) {
	seen := map[string]bool{}
	for _, tuple := range [][2]string{{"a\x00b", "c"}, {"a", "b\x00c"}, {"a/b", "世界"}, {"a", "b"}} {
		key := cursorKey(tuple[0], domain.RunID(tuple[1]))
		if seen[key] {
			t.Fatalf("cursor identities collide: %q", key)
		}
		seen[key] = true
		parts := strings.Split(strings.TrimPrefix(key, "observer/run/v2/"), "/")
		if len(parts) != 2 {
			t.Fatal("cursor lost tuple framing")
		}
		for i, part := range parts {
			got, err := base64.RawURLEncoding.DecodeString(part)
			if err != nil || string(got) != tuple[i] {
				t.Fatalf("cursor component %d not reversible", i)
			}
		}
	}
}

func TestConvertedObserverProgressContinuesAfterRestart(t *testing.T) {
	ctx := context.Background()
	journal := &memoryJournal{}
	_, _ = journal.Append(ctx, storage.Commit{RunID: "run/raw", Events: []domain.RunEvent{{Type: domain.EventRunStarted}, {Type: domain.EventRunCompleted}}})
	key := cursorKey("fixture/run", "run/raw")
	cursors := &memorySnapshots{values: map[string][]byte{key: []byte("1")}, versions: map[string]int64{key: 7}}
	provider := &recordingRunObserver{id: "fixture/run"}
	for range 2 {
		host, err := New(Config{Journal: journal, Cursors: cursors, RunSubscriptions: []RunSubscription{{Provider: provider, EventTypes: []string{"run.started", "run.completed"}}}})
		if err != nil {
			t.Fatal(err)
		}
		if err := host.DeliverRun(ctx, "run/raw"); err != nil {
			t.Fatal(err)
		}
	}
	if len(provider.events) != 1 || provider.events[0].ID.Seq != 2 {
		t.Fatalf("already delivered events repeated: %+v", provider.events)
	}
	value, version, err := cursors.Get(ctx, key)
	if err != nil || string(value) != "2" || version != 8 {
		t.Fatalf("cursor value=%q version=%d err=%v", value, version, err)
	}
}
