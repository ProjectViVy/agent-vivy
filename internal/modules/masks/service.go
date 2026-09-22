package masks

import (
	"context"
	"errors"
	"sort"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/maskcontract"
	"agent-vivy/internal/moduleport"
	"agent-vivy/internal/storage"
)

// Service is the generation-scoped mask manager. The catalog is immutable
// compiled data; custom definitions and selections remain owned by Core
// Storage and are reached only through the narrow MaskStore interface.
type Service struct {
	store   storage.MaskStore
	catalog Catalog
}

var _ maskcontract.Service = (*Service)(nil)

// Open constructs the optional mask service from the selected generation and
// a real Core Storage extension. A selected mask capability never falls back
// to an in-memory catalog or a best-effort store.
func Open(ctx context.Context, deps moduleport.MaskDependencies) (maskcontract.Service, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if deps.Store == nil {
		return nil, errors.New("masks: storage capability is unavailable")
	}
	catalog, err := NewCatalog(strings.TrimSpace(deps.GenerationID))
	if err != nil {
		return nil, err
	}
	return &Service{store: deps.Store, catalog: catalog}, nil
}

var _ moduleport.MaskFactory = Open

// NewService is a descriptive alias useful to internal fixtures that already
// hold an immutable catalog. Production composition should use Open.
func NewService(store storage.MaskStore, catalog Catalog) (*Service, error) {
	if store == nil {
		return nil, errors.New("masks: storage capability is unavailable")
	}
	return &Service{store: store, catalog: catalog}, nil
}

func (s *Service) PromptAssets() (string, string) {
	if s == nil {
		return "", ""
	}
	return s.catalog.PromptAssets()
}

func (s *Service) ListMasks(ctx context.Context, in maskcontract.ListRequest) (maskcontract.Page, error) {
	if s == nil || s.store == nil {
		return maskcontract.Page{}, maskcontract.NewError(maskcontract.CodeUnavailable, nil)
	}
	in, err := maskcontract.NormalizeList(in)
	if err != nil {
		return maskcontract.Page{}, err
	}
	// The custom store can return at most MaxListLimit rows. That is enough to
	// merge every legal page (100 rows) with the three built-ins while retaining
	// the store's continuation bit.
	customPage, err := s.store.ListCustomMasks(ctx, maskcontract.ListRequest{AfterID: in.AfterID, Limit: maskcontract.MaxListLimit})
	if err != nil {
		return maskcontract.Page{}, err
	}
	items := make([]maskcontract.Metadata, 0, len(s.catalog.Definitions())+len(customPage.Items))
	for _, definition := range s.catalog.Definitions() {
		if definition.ID <= in.AfterID {
			continue
		}
		items = append(items, metadataOf(definition))
	}
	items = append(items, customPage.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	page := maskcontract.Page{Items: items}
	if len(items) > in.Limit {
		page.NextAfterID = items[in.Limit-1].ID
		page.Items = items[:in.Limit]
	} else if customPage.NextAfterID != "" && len(items) > 0 {
		// The custom store had more rows than it returned. If all rows in this
		// merged page fit, the last returned ID is still the exclusive cursor.
		page.NextAfterID = items[len(items)-1].ID
	}
	return page, nil
}

func (s *Service) GetMask(ctx context.Context, in maskcontract.GetRequest) (maskcontract.Definition, error) {
	if s == nil || s.store == nil {
		return maskcontract.Definition{}, maskcontract.NewError(maskcontract.CodeUnavailable, nil)
	}
	if err := maskcontract.ValidateMaskID(in.ID); err != nil {
		return maskcontract.Definition{}, maskcontract.NewError(maskcontract.CodeInvalidMask, err)
	}
	if definition, ok := s.catalog.Get(in.ID); ok {
		return cloneDefinition(definition), nil
	}
	return s.store.GetCustomMask(ctx, in.ID)
}

func (s *Service) CreateMask(ctx context.Context, in maskcontract.CreateRequest) (maskcontract.Definition, error) {
	if s == nil || s.store == nil {
		return maskcontract.Definition{}, maskcontract.NewError(maskcontract.CodeUnavailable, nil)
	}
	normalized, err := maskcontract.NormalizeCreate(in)
	if err != nil {
		return maskcontract.Definition{}, err
	}
	return s.store.CreateCustomMask(ctx, normalized)
}

func (s *Service) UpdateMask(ctx context.Context, in maskcontract.UpdateRequest) (maskcontract.Definition, error) {
	if s == nil || s.store == nil {
		return maskcontract.Definition{}, maskcontract.NewError(maskcontract.CodeUnavailable, nil)
	}
	normalized, err := maskcontract.NormalizeUpdate(in)
	if err != nil {
		return maskcontract.Definition{}, err
	}
	return s.store.UpdateCustomMask(ctx, normalized)
}

func (s *Service) DeleteMask(ctx context.Context, in maskcontract.DeleteRequest) (maskcontract.DeleteResult, error) {
	if s == nil || s.store == nil {
		return maskcontract.DeleteResult{}, maskcontract.NewError(maskcontract.CodeUnavailable, nil)
	}
	normalized, err := maskcontract.NormalizeDelete(in)
	if err != nil {
		return maskcontract.DeleteResult{}, err
	}
	return s.store.DeleteCustomMask(ctx, normalized)
}

func (s *Service) Capture(ctx context.Context, sessionID domain.SessionID) (maskcontract.Capture, error) {
	if s == nil || s.store == nil {
		return maskcontract.Capture{}, maskcontract.NewError(maskcontract.CodeUnavailable, nil)
	}
	if err := maskcontract.ValidateSessionID(sessionID); err != nil {
		return maskcontract.Capture{}, maskcontract.NewError(maskcontract.CodeInvalidMask, err)
	}
	capture, err := s.store.ReadMaskCapture(ctx, sessionID)
	if err != nil {
		return maskcontract.Capture{}, err
	}
	if capture.Mask != nil {
		capture.Mask = cloneSnapshot(capture.Mask)
		return capture, nil
	}
	if capture.Selection.MaskID == "" {
		return capture, nil
	}
	if !maskcontract.IsBuiltinID(capture.Selection.MaskID) {
		return maskcontract.Capture{}, maskcontract.NewError(maskcontract.CodeMaskUnavailable, errors.New("selected custom mask is unavailable"))
	}
	if _, ok := s.catalog.Get(capture.Selection.MaskID); !ok {
		return maskcontract.Capture{}, maskcontract.NewError(maskcontract.CodeMaskUnavailable, errors.New("built-in mask is not compiled in this generation"))
	}
	snapshot, err := s.catalog.Snapshot(ctx, capture.Selection.MaskID, capture.Selection.Revision)
	if err != nil {
		return maskcontract.Capture{}, err
	}
	capture.Mask = &snapshot
	return capture, nil
}

func (s *Service) GetMaskSelection(ctx context.Context, in maskcontract.SelectionRequest) (maskcontract.SelectionView, error) {
	if s == nil || s.store == nil {
		return maskcontract.SelectionView{}, maskcontract.NewError(maskcontract.CodeUnavailable, nil)
	}
	if err := maskcontract.ValidateSessionID(in.SessionID); err != nil {
		return maskcontract.SelectionView{}, maskcontract.NewError(maskcontract.CodeInvalidMask, err)
	}
	// Selection reads intentionally do not resolve a built-in snapshot. A
	// session can outlive the Generation that compiled its chosen built-in;
	// the control plane reports that state as inactive while runtime Capture
	// continues to fail closed before admission.
	capture, err := s.store.ReadMaskCapture(ctx, in.SessionID)
	if err != nil {
		return maskcontract.SelectionView{}, err
	}
	return s.selectionView(capture)
}

func (s *Service) SetMaskSelection(ctx context.Context, in maskcontract.SetSelectionRequest) (maskcontract.SelectionView, error) {
	if s == nil || s.store == nil {
		return maskcontract.SelectionView{}, maskcontract.NewError(maskcontract.CodeUnavailable, nil)
	}
	normalized, err := maskcontract.NormalizeSelection(in)
	if err != nil {
		return maskcontract.SelectionView{}, err
	}
	if maskcontract.IsBuiltinID(normalized.MaskID) {
		if _, ok := s.catalog.Get(normalized.MaskID); !ok {
			return maskcontract.SelectionView{}, maskcontract.NewError(maskcontract.CodeMaskUnavailable, errors.New("built-in mask is not compiled in this generation"))
		}
	}
	selection, err := s.store.SetMaskSelection(ctx, normalized)
	if err != nil {
		return maskcontract.SelectionView{}, err
	}
	// Build the response from the committed CAS result. A second Capture read
	// could observe a later writer and falsely report that writer's revision.
	return s.selectionViewForSelection(ctx, selection)
}

func (s *Service) selectionView(capture maskcontract.Capture) (maskcontract.SelectionView, error) {
	view := maskcontract.SelectionView{Selection: capture.Selection, Available: true}
	if capture.Selection.MaskID == "" {
		return view, nil
	}
	if maskcontract.IsBuiltinID(capture.Selection.MaskID) {
		if _, ok := s.catalog.Get(capture.Selection.MaskID); !ok {
			view.Available = false
			view.InactiveReason = "not_compiled"
		}
		return view, nil
	}
	if capture.Mask == nil {
		return maskcontract.SelectionView{}, maskcontract.NewError(maskcontract.CodeMaskUnavailable, errors.New("selected custom mask snapshot is unavailable"))
	}
	return view, nil
}

func (s *Service) selectionViewForSelection(ctx context.Context, selection maskcontract.Selection) (maskcontract.SelectionView, error) {
	view := maskcontract.SelectionView{Selection: selection, Available: true}
	if selection.MaskID == "" {
		return view, nil
	}
	if maskcontract.IsBuiltinID(selection.MaskID) {
		if _, ok := s.catalog.Get(selection.MaskID); !ok {
			view.Available = false
			view.InactiveReason = "not_compiled"
		}
		return view, nil
	}
	if _, err := s.store.GetCustomMask(ctx, selection.MaskID); err != nil {
		var maskErr *maskcontract.Error
		if errors.As(err, &maskErr) && maskErr != nil && maskErr.Code == maskcontract.CodeNotFound {
			return maskcontract.SelectionView{}, maskcontract.NewError(maskcontract.CodeMaskUnavailable, errors.New("selected custom mask is unavailable"))
		}
		return maskcontract.SelectionView{}, err
	}
	return view, nil
}

func metadataOf(definition maskcontract.Definition) maskcontract.Metadata {
	return maskcontract.Metadata{
		ID: definition.ID, Name: definition.Name, Description: definition.Description,
		Digest: definition.Digest, GenerationID: definition.GenerationID,
		Revision: definition.Revision, BuiltIn: definition.BuiltIn,
	}
}

func cloneDefinition(in maskcontract.Definition) maskcontract.Definition {
	in.Body = string([]byte(in.Body))
	in.Name = string([]byte(in.Name))
	in.Description = string([]byte(in.Description))
	in.Digest = string([]byte(in.Digest))
	in.GenerationID = string([]byte(in.GenerationID))
	in.ID = string([]byte(in.ID))
	return in
}

func cloneSnapshot(in *maskcontract.Snapshot) *maskcontract.Snapshot {
	if in == nil {
		return nil
	}
	out := *in
	out.ID = string([]byte(in.ID))
	out.Name = string([]byte(in.Name))
	out.Body = string([]byte(in.Body))
	out.Digest = string([]byte(in.Digest))
	out.GenerationID = string([]byte(in.GenerationID))
	return &out
}
