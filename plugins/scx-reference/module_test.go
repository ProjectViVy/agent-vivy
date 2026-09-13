package scxreference

import (
	"context"
	"testing"

	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/observer"
)

func TestProviderExposesBoundedVersionedFixtures(t *testing.T) {
	provider := NewProvider()
	page, err := provider.Query(context.Background(), contextsource.Request{TenantID: "tenant", WorkspaceID: "demo", SessionID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Candidates) != 3 || page.Candidates[2].Resource == nil || page.Candidates[2].Resource.VersionMode != contextsource.VersionExact {
		t.Fatalf("candidates = %#v", page.Candidates)
	}
	resource, err := provider.Resolve(context.Background(), contextsource.ResolveRequest{Reference: page.Candidates[2].Resource.Clone()})
	if err != nil || resource.Reference.Version != "f1" || len(resource.Content) == 0 {
		t.Fatalf("resource = %#v, err = %v", resource, err)
	}
}

func TestProviderDeduplicatesStableEventReceipts(t *testing.T) {
	provider := NewProvider()
	event := observer.NewRunEvent(observer.NewEventID("run-1", 2), "run.completed", 0, []byte(`{"outcome":"completed"}`))
	first, err := provider.ObserveRunWithReceipt(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	second, err := provider.ObserveRunWithReceipt(context.Background(), event)
	if err != nil || first != second || first.State != observer.DeliveryCompleted {
		t.Fatalf("receipts = %#v / %#v, err = %v", first, second, err)
	}
	updates := provider.Updates()
	if len(updates) != 1 || updates[0].EventID != event.ID || string(updates[0].Payload) != string(event.Payload) {
		t.Fatalf("logical memory updates = %#v, want one projected update", updates)
	}
}
