package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/observerhost"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/port/observer"
	gardenmemory "github.com/ProjectViVy/laputa/garden/memory"
	"github.com/ProjectViVy/laputa/laputa/evolution"
)

const SubjectProjectionProviderID = "vivy.subject-projection"

// SubjectProjectionProvider consumes sealed core evidence through the existing
// ObserverHost. The native backend owns atomic mutation/receipt replay.
type SubjectProjectionProvider struct {
	service func() *Service
	backend gardenmemory.Backend
	binding cognitivecontract.RecapBinding
}

func NewSubjectProjectionProvider(service func() *Service, backend gardenmemory.Backend, binding cognitivecontract.RecapBinding) (*SubjectProjectionProvider, error) {
	if service == nil || backend == nil || binding.SubjectID == "" || binding.DomainID == "" {
		return nil, ErrRecapUnavailable
	}
	caps := backend.Capabilities()
	if !caps.Mutate || !caps.MutationLookup {
		return nil, fmt.Errorf("subject projection: native durable writer unavailable")
	}
	return &SubjectProjectionProvider{service: service, backend: backend, binding: binding}, nil
}
func (p *SubjectProjectionProvider) ID() string { return SubjectProjectionProviderID }
func (p *SubjectProjectionProvider) Subscription() observerhost.RunSubscription {
	return observerhost.RunSubscription{Provider: p, EventTypes: append([]string(nil), CognitiveCaptureEventTypes...)}
}
func (p *SubjectProjectionProvider) ObserveRun(ctx context.Context, event observer.RunEvent) error {
	receipt, err := p.ObserveRunWithReceipt(ctx, event)
	if err == nil && receipt.State != observer.DeliveryCompleted {
		return fmt.Errorf("subject projection: %s", receipt.State)
	}
	return err
}
func (p *SubjectProjectionProvider) ObserveRunWithReceipt(ctx context.Context, event observer.RunEvent) (observer.DeliveryReceipt, error) {
	fail := func(err error) (observer.DeliveryReceipt, error) { return observer.DeliveryReceipt{}, err }
	s := p.service()
	if s == nil {
		return fail(ErrRecapUnavailable)
	}
	sources, ok := s.deps.Sessions.(storage.SubjectSourceStore)
	if !ok {
		return fail(ErrRecapUnavailable)
	}
	rid := domain.RunID(event.ID.RunID)
	pin, err := sources.GetRunSubjectSource(ctx, rid)
	if errors.Is(err, storage.ErrNotFound) || (err == nil && pin.PolicyID != domain.SubjectResidentPolicy) {
		return observer.NewDeliveryReceipt(event.ID, "policy-skipped:not-resident:"+event.ID.String(), observer.DeliveryCompleted), nil
	}
	if err != nil {
		return fail(err)
	}
	if pin.SubjectID != p.binding.SubjectID || pin.DomainID != p.binding.DomainID {
		return fail(storage.ErrSubjectBinding)
	}
	run, err := s.deps.Runs.GetRun(ctx, rid)
	if err != nil {
		return fail(err)
	}
	if !run.Status.Terminal() || run.SessionID != pin.SessionID {
		return fail(storage.ErrSubjectEvidenceIncomplete)
	}
	if _, ok := cognitiveCapturePhase(event.Type); !ok {
		return fail(storage.ErrSubjectEvidenceIncomplete)
	}
	s.projectionMu.Lock()
	err = s.sealSubjectRunLocked(ctx, pin.SessionID, rid)
	s.projectionMu.Unlock()
	if err != nil {
		return fail(err)
	}
	originals, row, err := sources.ReadSubjectRunEvidence(ctx, pin.SubjectHistoryBinding, rid)
	if err != nil {
		return fail(err)
	}
	if int64(row.TerminalSeq) != event.ID.Seq {
		return fail(storage.ErrSubjectEvidenceIncomplete)
	}
	row, err = sources.SealSubjectInteraction(ctx, pin.SubjectHistoryBinding, row)
	if err != nil {
		return fail(err)
	}
	projected, _, err := projectAutomaticRecapOriginals(originals)
	if err != nil {
		return fail(err)
	}
	// Explicit source coordinates remain authoritative; bounded excerpts are
	// disposable historical material, never promoted permanent facts.
	var body strings.Builder
	fmt.Fprintf(&body, "Historical interaction %s\nsession=%s run=%s outcome=%s source_digest=%s\n", row.ID, row.SessionID, row.RunID, run.Status, row.SourceDigest)
	for _, message := range projected {
		fmt.Fprintf(&body, "\n[%s %s]\n%s\n", message.Role, message.ID, message.Content)
	}
	text := []rune(body.String())
	if len(text) > 8192 {
		text = append(text[:8192], []rune("\n[excerpt truncated; expand the retained core original]")...)
	}
	scope := evolution.Scope{SubjectID: pin.SubjectID, Kind: evolution.ScopePersonal}
	identity, _ := json.Marshal(struct {
		Binding storage.SubjectHistoryBinding
		Event   string
		Digest  string
		Version int
	}{pin.SubjectHistoryBinding, event.ID.String(), row.SourceDigest, 1})
	id := "subject-history:" + sha256Hex(identity)
	request := gardenmemory.AuthorizedMutation{Scope: scope, DestinationID: "bml", OperationID: id, Operation: evolution.MutationCreate, RecordID: id, ExpectedAbsent: true, Body: string(text), Sources: []evolution.SourceRef{{SourceID: "vivy.core.c2", RecordID: row.ID, Revision: row.Sequence, Scope: scope}}, Inference: evolution.InferenceObserved}
	raw, err := json.Marshal(request)
	if err != nil {
		return fail(err)
	}
	request.PayloadDigest = sha256Hex(raw)
	receipt, err := p.backend.Mutate(ctx, request)
	if err != nil {
		return fail(err)
	}
	if receipt.OperationID != id || receipt.PayloadDigest != request.PayloadDigest || receipt.TargetRef != request.RecordID {
		return fail(fmt.Errorf("subject projection: native receipt does not match operation"))
	}
	ready := receipt.CanonicalStatus == gardenmemory.CanonicalCompleted && (receipt.IndexStatus == gardenmemory.IndexReady || receipt.IndexStatus == gardenmemory.IndexNotRequired) && (receipt.Status == evolution.StatusApplied || receipt.Status == evolution.StatusNoChange)
	state := observer.DeliveryPending
	if receipt.CanonicalStatus == gardenmemory.CanonicalFailed || receipt.IndexStatus == gardenmemory.IndexFailed {
		state = observer.DeliveryFailed
	}
	if ready {
		state = observer.DeliveryCompleted
	}
	return observer.NewDeliveryReceipt(event.ID, receipt.OperationID, state), nil
}
