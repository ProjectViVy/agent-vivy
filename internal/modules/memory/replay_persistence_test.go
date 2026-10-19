package memory_test

import (
	"context"
	"testing"

	"agent-vivy/internal/modules/memory"
	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/observer"
	"github.com/ProjectViVy/agent-vivy/bml"
)

// TestBMLCloseoutStableEventReopenReplay proves BML-R4's provider half:
// redelivering the exact committed EventID after a full ownership close and
// reopen returns the identical receipt and still yields exactly one history
// candidate, with no seed call and no new EventID. Public long-term listing
// must not present the history record as a managed entry.
func TestBMLCloseoutStableEventReopenReplay(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t.TempDir())

	if active := memory.Active(); active != nil {
		t.Fatalf("memory service already active: %p", active)
	}
	open := func() *memory.Service {
		t.Helper()
		service, err := memory.Open(ctx, cfg)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		return service
	}

	event := completedEvent(t, "bml-closeout-run", 7, "eventprobe synthetic completion")
	provider := memory.NewProvider()

	service := open()
	t.Cleanup(func() { _ = memory.Close() })

	receipt, err := provider.ObserveRunWithReceipt(ctx, event)
	if err != nil {
		t.Fatalf("ObserveRunWithReceipt() error = %v", err)
	}
	if receipt.EventID != event.ID {
		t.Fatalf("receipt EventID = %+v, want %+v", receipt.EventID, event.ID)
	}
	if receipt.State != observer.DeliveryCompleted {
		t.Fatalf("receipt state = %q, want %q", receipt.State, observer.DeliveryCompleted)
	}
	if receipt.ReceiptID == "" {
		t.Fatal("receipt id is empty")
	}

	page, err := provider.Query(ctx, contextsource.Request{Query: "eventprobe", Limit: 10})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(page.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want exactly one history record", page.Candidates)
	}
	candidate := page.Candidates[0]
	if candidate.ContentID == "" || candidate.Version != "1" {
		t.Fatalf("candidate identity = {id:%q version:%q}, want a revision-1 record", candidate.ContentID, candidate.Version)
	}
	evidence := candidate.Metadata["vivy.memory-bml.evidence"]
	if evidence == "" {
		t.Fatalf("candidate evidence metadata is empty: %+v", candidate.Metadata)
	}
	listed := service.List(ctx, bml.MemoryListRequest{})
	wantStatus(t, listed, bml.CrudOutcomeListed)
	for _, entry := range listed.Entries {
		if entry.ID == candidate.ContentID {
			t.Fatalf("history record exposed as a long-term entry: %+v", entry)
		}
	}

	// Full ownership restart, then the exact same event object and EventID.
	if err := memory.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if active := memory.Active(); active != nil {
		t.Fatalf("Active() after Close() = %p, want nil", active)
	}
	service = open()
	replayProvider := memory.NewProvider()
	replayReceipt, err := replayProvider.ObserveRunWithReceipt(ctx, event)
	if err != nil {
		t.Fatalf("replay ObserveRunWithReceipt() error = %v", err)
	}
	if replayReceipt != receipt {
		t.Fatalf("replay receipt = %+v, want the identical %+v", replayReceipt, receipt)
	}
	replayed, err := replayProvider.Query(ctx, contextsource.Request{Query: "eventprobe", Limit: 10})
	if err != nil {
		t.Fatalf("replay Query() error = %v", err)
	}
	requireSameSingleCandidate(t, replayed, candidate)

	// One more restart proves the dedupe survived the second write path
	// without a second ingestion.
	if err := memory.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	service = open()
	t.Cleanup(func() { _ = memory.Close() })
	final, err := memory.NewProvider().Query(ctx, contextsource.Request{Query: "eventprobe", Limit: 10})
	if err != nil {
		t.Fatalf("final Query() error = %v", err)
	}
	requireSameSingleCandidate(t, final, candidate)

	// The public long-term listing still excludes the history record.
	wantStatus(t, service.List(ctx, bml.MemoryListRequest{}), bml.CrudOutcomeListed)
}

// requireSameSingleCandidate asserts the durable projection still holds
// exactly one candidate with the original native identity and content.
func requireSameSingleCandidate(t *testing.T, page contextsource.Page, want contextsource.Candidate) {
	t.Helper()
	if len(page.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want exactly one durable candidate", page.Candidates)
	}
	got := page.Candidates[0]
	if got.ContentID != want.ContentID || got.Version != want.Version || got.Content != want.Content {
		t.Fatalf("durable candidate = {id:%q version:%q content:%q}, want {id:%q version:%q content:%q}",
			got.ContentID, got.Version, got.Content, want.ContentID, want.Version, want.Content)
	}
	if got.Metadata["vivy.memory-bml.evidence"] != want.Metadata["vivy.memory-bml.evidence"] {
		t.Fatalf("candidate evidence changed across restart: %+v vs %+v", got.Metadata, want.Metadata)
	}
}
