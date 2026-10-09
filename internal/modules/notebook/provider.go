package notebook

import (
	"context"
	"errors"
	"sync/atomic"

	nb "agent-vivy/internal/notebookcontract"
)

// Open is the typed core/notebook-service@v1 factory binding emitted into
// selected Generations. It wraps the N1 Store and trusted scope resolver —
// both host-owned — into the owner Bundle.
func Open(_ context.Context, in nb.FactoryInput) (nb.Bundle, error) {
	if in.Store == nil {
		return nil, errors.New("notebook: factory requires the storage-owned Store")
	}
	if in.Scopes == nil {
		return nil, errors.New("notebook: factory requires the trusted scope resolver")
	}
	return &bundle{store: in.Store, scopes: in.Scopes, generation: in.GenerationID}, nil
}

type bundle struct {
	store      nb.Store
	scopes     nb.ScopeResolver
	generation string
	closed     atomic.Bool
}

func (b *bundle) Service() nb.Store { return b.store }

// Actions binds the facade to the host-supplied scope and actor. Neither is
// caller-controlled: the ActionHost resolves them from the authenticated
// transport before reaching this seam.
func (b *bundle) Actions(scope nb.ScopeID, actor nb.Actor) nb.ScopedActions {
	return &scopedActions{bundle: b, scope: scope, actor: actor}
}

func (b *bundle) Close(context.Context) error {
	b.closed.Store(true)
	return nil
}

// scopedActions forwards typed requests into MutationContext envelopes; the
// store re-verifies the canonical digest so a forged key cannot replay a
// different payload under an existing receipt.
type scopedActions struct {
	bundle *bundle
	scope  nb.ScopeID
	actor  nb.Actor
}

var _ nb.ScopedActions = (*scopedActions)(nil)

func (s *scopedActions) mc(key string) (nb.MutationContext, error) {
	if s.bundle.closed.Load() {
		return nb.MutationContext{}, &nb.Error{Code: nb.CodeCapabilityUnavailable, Message: "notebook bundle is closed"}
	}
	if key == "" {
		return nb.MutationContext{}, &nb.Error{Code: nb.CodeInvalidRequest, Message: "operation_key is required"}
	}
	return nb.MutationContext{ScopeID: s.scope, Actor: s.actor, OperationKey: key}, nil
}

func (s *scopedActions) ListSections(ctx context.Context, req nb.ListSectionsRequest) (nb.SectionPage, error) {
	return s.bundle.store.ListSections(ctx, s.scope, req)
}

func (s *scopedActions) CreateSection(ctx context.Context, req nb.OperationKeyed[nb.CreateSectionRequest]) (nb.MutationReceipt, error) {
	mc, err := s.mc(req.OperationKey)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	return s.bundle.store.CreateSection(ctx, mc, req.Request)
}

func (s *scopedActions) UpdateSection(ctx context.Context, req nb.OperationKeyed[nb.UpdateSectionRequest]) (nb.MutationReceipt, error) {
	mc, err := s.mc(req.OperationKey)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	return s.bundle.store.UpdateSection(ctx, mc, req.Request)
}

func (s *scopedActions) DeleteSection(ctx context.Context, req nb.OperationKeyed[nb.DeleteSectionRequest]) (nb.MutationReceipt, error) {
	mc, err := s.mc(req.OperationKey)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	return s.bundle.store.DeleteSection(ctx, mc, req.Request)
}

func (s *scopedActions) RestoreSection(ctx context.Context, req nb.OperationKeyed[nb.RestoreSectionRequest]) (nb.MutationReceipt, error) {
	mc, err := s.mc(req.OperationKey)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	return s.bundle.store.RestoreSection(ctx, mc, req.Request)
}

func (s *scopedActions) ListEntries(ctx context.Context, req nb.ListEntriesRequest) (nb.EntryPage, error) {
	return s.bundle.store.ListEntries(ctx, s.scope, req)
}

func (s *scopedActions) GetEntry(ctx context.Context, req nb.GetEntryRequest) (nb.EntryView, error) {
	return s.bundle.store.GetEntry(ctx, s.scope, req)
}

func (s *scopedActions) CreateEntry(ctx context.Context, req nb.OperationKeyed[nb.CreateEntryRequest]) (nb.MutationReceipt, error) {
	mc, err := s.mc(req.OperationKey)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	return s.bundle.store.CreateEntry(ctx, mc, req.Request)
}

func (s *scopedActions) SaveEntry(ctx context.Context, req nb.OperationKeyed[nb.SaveEntryRequest]) (nb.MutationReceipt, error) {
	mc, err := s.mc(req.OperationKey)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	return s.bundle.store.SaveEntry(ctx, mc, req.Request)
}

func (s *scopedActions) MoveEntry(ctx context.Context, req nb.OperationKeyed[nb.MoveEntryRequest]) (nb.MutationReceipt, error) {
	mc, err := s.mc(req.OperationKey)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	return s.bundle.store.MoveEntry(ctx, mc, req.Request)
}

func (s *scopedActions) DeleteEntry(ctx context.Context, req nb.OperationKeyed[nb.DeleteEntryRequest]) (nb.MutationReceipt, error) {
	mc, err := s.mc(req.OperationKey)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	return s.bundle.store.DeleteEntry(ctx, mc, req.Request)
}

func (s *scopedActions) RestoreEntry(ctx context.Context, req nb.OperationKeyed[nb.RestoreEntryRequest]) (nb.MutationReceipt, error) {
	mc, err := s.mc(req.OperationKey)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	return s.bundle.store.RestoreEntry(ctx, mc, req.Request)
}

func (s *scopedActions) ListRevisions(ctx context.Context, req nb.ListRevisionsRequest) (nb.RevisionPage, error) {
	return s.bundle.store.ListRevisions(ctx, s.scope, req)
}

func (s *scopedActions) AdoptRevision(ctx context.Context, req nb.OperationKeyed[nb.AdoptRevisionRequest]) (nb.MutationReceipt, error) {
	mc, err := s.mc(req.OperationKey)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	return s.bundle.store.AdoptRevision(ctx, mc, req.Request)
}

func (s *scopedActions) ListComments(ctx context.Context, req nb.ListCommentsRequest) (nb.CommentPage, error) {
	return s.bundle.store.ListComments(ctx, s.scope, req)
}

func (s *scopedActions) CreateComment(ctx context.Context, req nb.OperationKeyed[nb.CreateCommentRequest]) (nb.MutationReceipt, error) {
	mc, err := s.mc(req.OperationKey)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	return s.bundle.store.CreateComment(ctx, mc, req.Request)
}

func (s *scopedActions) UpdateComment(ctx context.Context, req nb.OperationKeyed[nb.UpdateCommentRequest]) (nb.MutationReceipt, error) {
	mc, err := s.mc(req.OperationKey)
	if err != nil {
		return nb.MutationReceipt{}, err
	}
	return s.bundle.store.UpdateComment(ctx, mc, req.Request)
}

// Export returns one exact revision plus provenance and comment sidecar
// data; it writes nothing and stays inside the Host wire ceiling.
func (s *scopedActions) Export(ctx context.Context, req nb.GetEntryRequest) (nb.ExportBundle, error) {
	if s.bundle.closed.Load() {
		return nb.ExportBundle{}, &nb.Error{Code: nb.CodeCapabilityUnavailable, Message: "notebook bundle is closed"}
	}
	if req.RevisionID == "" {
		return nb.ExportBundle{}, &nb.Error{Code: nb.CodeInvalidRequest, Message: "export requires an exact revision id"}
	}
	view, err := s.bundle.store.GetEntry(ctx, s.scope, req)
	if err != nil {
		return nb.ExportBundle{}, err
	}
	comments, err := s.bundle.store.ListComments(ctx, s.scope, nb.ListCommentsRequest{EntryID: req.EntryID})
	if err != nil {
		return nb.ExportBundle{}, err
	}
	return nb.ExportBundle{Entry: view.Entry, Revision: view.Revision, Comments: comments.Comments}, nil
}
