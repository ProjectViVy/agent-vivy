package memory

import (
	"context"
	"errors"

	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/observer"
)

// errUnavailable is the explicit sync-plane failure when the composition
// never opened the memory service — never a fabricated page or accepted
// receipt.
var errUnavailable = errors.New("memory: service unavailable")

// errNotImplemented is the explicit failure for the sync paths until the
// recall/ingest implementation lands.
var errNotImplemented = errors.New("memory: operation not implemented")

// Provider is the single sync-plane provider: one identity serves both the
// std/context-source@v1 recall contract and the std/observer/run@v1 ingest
// contract (receipt-aware), so the sealed manifest lists stay consistent.
type Provider struct{}

var (
	_ contextsource.Provider      = (*Provider)(nil)
	_ observer.RunProvider        = (*Provider)(nil)
	_ observer.ReceiptRunProvider = (*Provider)(nil)
)

// NewProvider is the generated binding's constructor.
func NewProvider() *Provider { return &Provider{} }

// ID satisfies both manifest lists with one identity.
func (*Provider) ID() string { return ProviderID }

// Query fails explicitly until the recall path is wired; an open service is
// resolved at call time, not captured at construction.
func (*Provider) Query(context.Context, contextsource.Request) (contextsource.Page, error) {
	if Active() == nil {
		return contextsource.Page{}, errUnavailable
	}
	return contextsource.Page{}, errNotImplemented
}

// ObserveRun delegates to the receipt-aware path so both observer contracts
// share one ingest decision.
func (p *Provider) ObserveRun(ctx context.Context, event observer.RunEvent) error {
	_, err := p.ObserveRunWithReceipt(ctx, event)
	return err
}

// ObserveRunWithReceipt reports the truthful disposition: failed when the
// service is closed or when ingestion is not yet wired. The Host retains
// its cursor and retries later deliveries.
func (*Provider) ObserveRunWithReceipt(_ context.Context, event observer.RunEvent) (observer.DeliveryReceipt, error) {
	if Active() == nil {
		return observer.NewDeliveryReceipt(event.ID, "", observer.DeliveryFailed), errUnavailable
	}
	return observer.NewDeliveryReceipt(event.ID, "", observer.DeliveryFailed), errNotImplemented
}
