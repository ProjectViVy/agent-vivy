// Package scxreference is the bounded local Provider used by the SCX release
// candidate. It has no network, filesystem, storage, or Eino dependency.
package scxreference

import (
	"context"
	"sync"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/observer"
)

const providerID = "scx.reference"
const maxMemoryUpdates = 256

type vivyModule struct{}

func New() module.Module { return vivyModule{} }
func (vivyModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return moduleInstance{}, nil
}

type moduleInstance struct{}

func (moduleInstance) Start(context.Context) error { return nil }
func (moduleInstance) Ready(context.Context) error { return nil }
func (moduleInstance) Stop(context.Context) error  { return nil }
func (moduleInstance) Close(context.Context) error { return nil }

type Provider struct {
	mu       sync.Mutex
	receipts map[observer.EventID]observer.DeliveryReceipt
	updates  []MemoryUpdate
}

// MemoryUpdate is the bounded, observable logical effect of the reference
// memory receiver. It deliberately stores only the Host-projected payload.
type MemoryUpdate struct {
	EventID observer.EventID
	Payload []byte
}

func NewProvider() *Provider {
	return &Provider{receipts: make(map[observer.EventID]observer.DeliveryReceipt)}
}

func (*Provider) ID() string { return providerID }

func (*Provider) Query(ctx context.Context, request contextsource.Request) (contextsource.Page, error) {
	if err := ctx.Err(); err != nil {
		return contextsource.Page{}, err
	}
	reference := contextsource.ResourceReference{
		URI: "project://demo/plan.txt", Version: "f1", MediaType: "text/plain", SizeHint: 50,
		Scope:       contextsource.Scope{TenantID: request.TenantID, WorkspaceID: request.WorkspaceID, SessionID: request.SessionID},
		VersionMode: contextsource.VersionExact, Replayable: true,
	}
	return contextsource.NewPage([]contextsource.Candidate{
		{SourceID: providerID, ContentID: "persona-1", MediaType: "text/plain", Content: "Speak calmly. Admit uncertainty. Keep explanations concise.", Version: "p1", Confidence: .8, Treatment: contextsource.TreatmentReserved},
		{SourceID: providerID, ContentID: "emotion-1", MediaType: "text/plain", Content: "Agent state: concerned; prefer a measured tone.", Version: "e1", Confidence: .7, Treatment: contextsource.TreatmentCompetitive},
		{SourceID: providerID, ContentID: "plan.txt", Version: "f1", Confidence: 1, Treatment: contextsource.TreatmentRequired, Resource: &reference},
	}, ""), nil
}

func (*Provider) Resolve(ctx context.Context, request contextsource.ResolveRequest) (contextsource.Resource, error) {
	if err := ctx.Err(); err != nil {
		return contextsource.Resource{}, err
	}
	if request.Reference.URI != "project://demo/plan.txt" || request.Reference.Version != "f1" {
		return contextsource.Resource{}, contextsource.ErrVersionUnavailable
	}
	return contextsource.NewResource(request.Reference, []byte("Keep summary and retrieval strategies replaceable.")), nil
}

func (provider *Provider) ObserveRun(ctx context.Context, event observer.RunEvent) error {
	_, err := provider.ObserveRunWithReceipt(ctx, event)
	return err
}

func (provider *Provider) ObserveRunWithReceipt(ctx context.Context, event observer.RunEvent) (observer.DeliveryReceipt, error) {
	if err := ctx.Err(); err != nil {
		return observer.DeliveryReceipt{}, err
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if receipt, ok := provider.receipts[event.ID]; ok {
		return receipt, nil
	}
	if len(provider.updates) >= maxMemoryUpdates {
		return observer.NewDeliveryReceipt(event.ID, "scx-reference:capacity", observer.DeliveryFailed), nil
	}
	receipt := observer.NewDeliveryReceipt(event.ID, "scx-reference:"+event.ID.String(), observer.DeliveryCompleted)
	provider.receipts[event.ID] = receipt
	provider.updates = append(provider.updates, MemoryUpdate{EventID: event.ID, Payload: append([]byte(nil), event.Payload...)})
	return receipt, nil
}

// Updates returns defensive copies for conformance/status inspection.
func (provider *Provider) Updates() []MemoryUpdate {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	out := make([]MemoryUpdate, len(provider.updates))
	for index, update := range provider.updates {
		out[index] = MemoryUpdate{EventID: update.EventID, Payload: append([]byte(nil), update.Payload...)}
	}
	return out
}
