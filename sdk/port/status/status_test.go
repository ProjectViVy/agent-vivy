package status

import "testing"

func TestSnapshotCopiesItems(t *testing.T) {
	items := []Item{{ID: "python", State: "ready", Message: "ok"}}
	snapshot := NewSnapshot("rev-1", "cursor-7", items)
	items[0].State = "mutated"
	if snapshot.Items[0].State != "ready" {
		t.Fatalf("snapshot item mutated through caller slice: %#v", snapshot.Items[0])
	}
}

func TestUnavailableSnapshotCarriesNoProviderError(t *testing.T) {
	snapshot := Unavailable("timeout")
	if snapshot.Available || snapshot.UnavailableReason != "timeout" {
		t.Fatalf("unavailable snapshot = %#v", snapshot)
	}
}
