package app

import (
	"context"
	"errors"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/port/observer"
)

// Preserve each old provider's identity and receipt semantics for ordinary
// runs, but explicitly acknowledge the persisted resident policy exclusion.
// Profile names and event payloads cannot select the subject route.
type legacySubjectObserver struct {
	observer.RunProvider
	sources storage.SubjectSourceStore
}

func (p legacySubjectObserver) ObserveRun(ctx context.Context, event observer.RunEvent) error {
	_, err := p.ObserveRunWithReceipt(ctx, event)
	return err
}
func (p legacySubjectObserver) ObserveRunWithReceipt(ctx context.Context, event observer.RunEvent) (observer.DeliveryReceipt, error) {
	if p.sources != nil {
		pin, err := p.sources.GetRunSubjectSource(ctx, domain.RunID(event.ID.RunID))
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return observer.DeliveryReceipt{}, err
		}
		if err == nil && pin.PolicyID == domain.SubjectResidentPolicy {
			return observer.NewDeliveryReceipt(event.ID, "policy-skipped:subject-resident:"+event.ID.String(), observer.DeliveryCompleted), nil
		}
	}
	if receiver, ok := p.RunProvider.(observer.ReceiptRunProvider); ok {
		return receiver.ObserveRunWithReceipt(ctx, event)
	}
	if err := p.RunProvider.ObserveRun(ctx, event); err != nil {
		return observer.DeliveryReceipt{}, err
	}
	return observer.NewDeliveryReceipt(event.ID, fmt.Sprintf("%s:%s", p.ID(), event.ID.String()), observer.DeliveryCompleted), nil
}
