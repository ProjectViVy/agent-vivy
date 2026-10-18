package runtime

import (
	"context"
	"errors"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	memorymodule "agent-vivy/internal/modules/memory"
	"agent-vivy/sdk/port/observer"
	"github.com/ProjectViVy/agent-vivy/bml"
	gardenmemory "github.com/ProjectViVy/laputa/garden/memory"
	"github.com/ProjectViVy/laputa/laputa/evolution"
)

type subjectLostReply struct {
	gardenmemory.Backend
	lost  bool
	calls int
}

func (b *subjectLostReply) Mutate(ctx context.Context, request gardenmemory.AuthorizedMutation) (gardenmemory.MutationReceipt, error) {
	b.calls++
	receipt, err := b.Backend.Mutate(ctx, request)
	if err == nil && !b.lost {
		b.lost = true
		return gardenmemory.MutationReceipt{}, errors.New("lost native commit reply")
	}
	if err == nil && b.calls == 2 {
		receipt.IndexStatus = gardenmemory.IndexPending
	}
	if err == nil && b.calls == 3 {
		receipt.IndexStatus = gardenmemory.IndexFailed
	}
	return receipt, err
}
func TestIssue58SubjectProjectionNativeRetry(t *testing.T) {
	f := newIssue58Wire(t)
	ctx := context.Background()
	rid, err := f.svc.Run(ctx, "main-reader", "native historical projection needle")
	if err != nil {
		t.Fatal(err)
	}
	if !f.svc.WaitIdle(ctx) {
		t.Fatal("not idle")
	}
	var terminal domain.RunEvent
	for _, event := range replayAll(t, f.b, rid) {
		if event.Type == domain.EventRunCompleted {
			terminal = event
		}
	}
	if terminal.RunID == "" {
		t.Fatal("terminal absent")
	}
	native, err := memorymodule.Open(ctx, config.Config{Storage: config.Storage{DataDir: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = memorymodule.Close() })
	head, err := f.owner.ReadRecap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	backend, err := memorymodule.NewBoundBackend(native, bml.OwnershipBinding{SubjectID: head.Binding.SubjectID, DomainID: head.Binding.DomainID, ScopeKind: "personal", DestinationID: "bml"})
	if err != nil {
		t.Fatal(err)
	}
	lost := &subjectLostReply{Backend: backend}
	provider, err := NewSubjectProjectionProvider(func() *Service { return f.svc }, lost, head.Binding)
	if err != nil {
		t.Fatal(err)
	}
	event := observer.NewRunEvent(observer.NewEventID(string(rid), int64(terminal.Seq)), string(terminal.Type), terminal.CreatedAt, terminal.Payload)
	if _, err = provider.ObserveRunWithReceipt(ctx, event); err == nil {
		t.Fatal("lost native receipt acknowledged")
	}
	pending, err := provider.ObserveRunWithReceipt(ctx, event)
	if err != nil || pending.State != observer.DeliveryPending {
		t.Fatalf("pending index falsely acknowledged: %+v %v", pending, err)
	}
	failed, err := provider.ObserveRunWithReceipt(ctx, event)
	if err != nil || failed.State != observer.DeliveryFailed {
		t.Fatalf("failed index falsely acknowledged: %+v %v", failed, err)
	}
	receipt, err := provider.ObserveRunWithReceipt(ctx, event)
	if err != nil || receipt.State != observer.DeliveryCompleted {
		t.Fatalf("replay=%+v %v", receipt, err)
	}
	replay, err := provider.ObserveRunWithReceipt(ctx, event)
	if err != nil || replay != receipt {
		t.Fatalf("unstable receipt: %+v %v", replay, err)
	}
	status, err := backend.MutationStatus(ctx, receipt.ReceiptID)
	if err != nil || status.Revision != 1 {
		t.Fatalf("replay duplicated native row: %+v %v", status, err)
	}
	scope := evolution.Scope{SubjectID: head.Binding.SubjectID, Kind: evolution.ScopePersonal}
	page, err := backend.Search(ctx, gardenmemory.AuthorizedSearch{Scopes: []evolution.Scope{scope}, Query: "needle", Limit: 10})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("projection count=%+v %v", page, err)
	}
	original, err := f.svc.SubjectRecent(ctx, "main-reader", 0, 1, 10)
	if err != nil || len(original) != 1 {
		t.Fatalf("core seal missing/duplicated: %+v %v", original, err)
	}
}
