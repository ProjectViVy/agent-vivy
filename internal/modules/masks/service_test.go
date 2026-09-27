package masks

import (
	"context"
	"errors"
	"sort"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/maskcontract"
)

type serviceStore struct {
	custom         []maskcontract.Definition
	captures       map[domain.SessionID]maskcontract.Capture
	selection      map[domain.SessionID]maskcontract.Selection
	postSetCapture *maskcontract.Capture
}

func (store *serviceStore) ListCustomMasks(_ context.Context, in maskcontract.ListRequest) (maskcontract.Page, error) {
	items := make([]maskcontract.Metadata, 0, len(store.custom))
	for _, definition := range store.custom {
		if definition.ID <= in.AfterID {
			continue
		}
		items = append(items, metadataOf(definition))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return maskcontract.Page{Items: items}, nil
}

func (store *serviceStore) GetCustomMask(_ context.Context, id string) (maskcontract.Definition, error) {
	for _, definition := range store.custom {
		if definition.ID == id {
			return definition, nil
		}
	}
	return maskcontract.Definition{}, maskcontract.NewError(maskcontract.CodeNotFound, errors.New("missing"))
}

func (store *serviceStore) CreateCustomMask(context.Context, maskcontract.CreateRequest) (maskcontract.Definition, error) {
	return maskcontract.Definition{}, maskcontract.NewError(maskcontract.CodeUnavailable, errors.New("fixture"))
}

func (store *serviceStore) UpdateCustomMask(context.Context, maskcontract.UpdateRequest) (maskcontract.Definition, error) {
	return maskcontract.Definition{}, maskcontract.NewError(maskcontract.CodeUnavailable, errors.New("fixture"))
}

func (store *serviceStore) DeleteCustomMask(context.Context, maskcontract.DeleteRequest) (maskcontract.DeleteResult, error) {
	return maskcontract.DeleteResult{}, maskcontract.NewError(maskcontract.CodeUnavailable, errors.New("fixture"))
}

func (store *serviceStore) ReadMaskCapture(_ context.Context, id domain.SessionID) (maskcontract.Capture, error) {
	capture, ok := store.captures[id]
	if !ok {
		return maskcontract.Capture{}, maskcontract.NewError(maskcontract.CodeNotFound, errors.New("missing session"))
	}
	return capture, nil
}

func (store *serviceStore) SetMaskSelection(_ context.Context, in maskcontract.SetSelectionRequest) (maskcontract.Selection, error) {
	current := store.selection[in.SessionID]
	if current.Revision != in.ExpectedRevision {
		return maskcontract.Selection{}, maskcontract.NewRevisionError(maskcontract.CodeRevisionConflict, current.Revision, nil)
	}
	next := maskcontract.Selection{SessionID: in.SessionID, MaskID: in.MaskID, Revision: current.Revision + 1}
	store.selection[in.SessionID] = next
	if store.postSetCapture != nil {
		store.captures[in.SessionID] = *store.postSetCapture
	} else {
		store.captures[in.SessionID] = maskcontract.Capture{Selection: next}
	}
	return next, nil
}

func customFixture(id string) maskcontract.Definition {
	return maskcontract.Definition{
		ID: id, Name: "Custom", Description: "fixture", Body: "custom body",
		Revision: 1, Digest: maskcontract.DefinitionDigest(id, "Custom", "custom body"),
	}
}

func TestServiceListMergesBuiltinsAndCustoms(t *testing.T) {
	catalog, err := NewCatalog("test-generation")
	if err != nil {
		t.Fatal(err)
	}
	store := &serviceStore{custom: []maskcontract.Definition{customFixture("custom/00000000-0000-4000-8000-000000000001")}}
	service, err := NewService(store, catalog)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.ListMasks(context.Background(), maskcontract.ListRequest{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.Items[0].ID != maskcontract.BuiltinProgrammerID || first.Items[1].ID != maskcontract.BuiltinResearcherID {
		t.Fatalf("first page = %#v", first.Items)
	}
	if first.NextAfterID != maskcontract.BuiltinResearcherID {
		t.Fatalf("first cursor = %q", first.NextAfterID)
	}
	second, err := service.ListMasks(context.Background(), maskcontract.ListRequest{AfterID: first.NextAfterID, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 2 || second.Items[0].ID != maskcontract.BuiltinWriterID || second.Items[1].ID != store.custom[0].ID {
		t.Fatalf("second page = %#v", second.Items)
	}
	if second.Items[1].Digest == "" || second.Items[1].BuiltIn {
		t.Fatalf("custom metadata lost identity: %#v", second.Items[1])
	}
	if second.Items[0].ID == second.Items[1].ID {
		t.Fatal("merged page contains duplicate IDs")
	}
}

func TestServiceCaptureResolvesBuiltinsAndOwnsCopies(t *testing.T) {
	catalog, err := NewCatalog("test-generation")
	if err != nil {
		t.Fatal(err)
	}
	custom := customFixture("custom/00000000-0000-4000-8000-000000000001")
	store := &serviceStore{
		custom: []maskcontract.Definition{custom},
	}
	store.captures = map[domain.SessionID]maskcontract.Capture{
		"builtin-session": {Selection: maskcontract.Selection{SessionID: "builtin-session", MaskID: maskcontract.BuiltinProgrammerID, Revision: 3}},
		"custom-session":  {Selection: maskcontract.Selection{SessionID: "custom-session", MaskID: custom.ID, Revision: 2}, Mask: &maskcontract.Snapshot{ID: custom.ID, Name: custom.Name, Body: custom.Body, Digest: custom.Digest, DefinitionRevision: custom.Revision, SelectionRevision: 2}},
		"empty-session":   {Selection: maskcontract.Selection{SessionID: "empty-session", Revision: 4}},
	}
	service, err := NewService(store, catalog)
	if err != nil {
		t.Fatal(err)
	}
	builtin, err := service.Capture(context.Background(), "builtin-session")
	if err != nil || builtin.Mask == nil {
		t.Fatalf("builtin capture = %#v, err=%v", builtin, err)
	}
	builtin.Mask.Body = "mutated"
	again, err := service.Capture(context.Background(), "builtin-session")
	if err != nil || again.Mask == nil || again.Mask.Body == "mutated" {
		t.Fatalf("catalog snapshot was not owned: %#v, err=%v", again, err)
	}
	customCapture, err := service.Capture(context.Background(), "custom-session")
	if err != nil || customCapture.Mask == nil {
		t.Fatalf("custom capture = %#v, err=%v", customCapture, err)
	}
	customCapture.Mask.Body = "mutated"
	if store.captures["custom-session"].Mask.Body == "mutated" {
		t.Fatal("custom capture leaked store-owned snapshot")
	}
	empty, err := service.Capture(context.Background(), "empty-session")
	if err != nil || empty.Mask != nil || empty.Selection.MaskID != "" {
		t.Fatalf("empty capture = %#v, err=%v", empty, err)
	}
}

func TestServiceSelectionUsesResolvedCaptureAndRejectsReservedLookup(t *testing.T) {
	catalog, err := NewCatalog("test-generation")
	if err != nil {
		t.Fatal(err)
	}
	custom := customFixture("custom/00000000-0000-4000-8000-000000000001")
	store := &serviceStore{captures: map[domain.SessionID]maskcontract.Capture{"session-1": {Selection: maskcontract.Selection{SessionID: "session-1"}}}, selection: map[domain.SessionID]maskcontract.Selection{}}
	service, err := NewService(store, catalog)
	if err != nil {
		t.Fatal(err)
	}
	view, err := service.SetMaskSelection(context.Background(), maskcontract.SetSelectionRequest{SessionID: "session-1", MaskID: maskcontract.BuiltinWriterID, ExpectedRevision: 0})
	if err != nil {
		t.Fatal(err)
	}
	if view.Selection.MaskID != maskcontract.BuiltinWriterID || view.Selection.Revision != 1 || !view.Available {
		t.Fatalf("selection view = %#v", view)
	}
	store.postSetCapture = &maskcontract.Capture{Selection: maskcontract.Selection{SessionID: "session-1", MaskID: maskcontract.BuiltinProgrammerID, Revision: 9}}
	committed, err := service.SetMaskSelection(context.Background(), maskcontract.SetSelectionRequest{SessionID: "session-1", MaskID: maskcontract.BuiltinResearcherID, ExpectedRevision: 1})
	if err != nil || committed.Selection.MaskID != maskcontract.BuiltinResearcherID || committed.Selection.Revision != 2 {
		t.Fatalf("set response did not preserve committed CAS result: %#v, err=%v", committed, err)
	}
	store.captures["old-generation"] = maskcontract.Capture{Selection: maskcontract.Selection{SessionID: "old-generation", MaskID: maskcontract.BuiltinWriterID, Revision: 2}}
	oldGeneration, err := NewService(store, Catalog{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := oldGeneration.Capture(context.Background(), "old-generation"); err == nil {
		t.Fatal("capture from an omitted built-in generation was accepted")
	} else {
		var maskErr *maskcontract.Error
		if !errors.As(err, &maskErr) || maskErr.Code != maskcontract.CodeMaskUnavailable {
			t.Fatalf("old-generation capture error = %v, want mask_unavailable", err)
		}
	}
	inactive, err := oldGeneration.GetMaskSelection(context.Background(), maskcontract.SelectionRequest{SessionID: "old-generation"})
	if err != nil || inactive.Available || inactive.InactiveReason != "not_compiled" || inactive.Selection.MaskID != maskcontract.BuiltinWriterID {
		t.Fatalf("inactive built-in selection = %#v, err=%v", inactive, err)
	}
	store.captures["missing-custom"] = maskcontract.Capture{Selection: maskcontract.Selection{SessionID: "missing-custom", MaskID: custom.ID, Revision: 1}}
	if _, err := service.GetMaskSelection(context.Background(), maskcontract.SelectionRequest{SessionID: "missing-custom"}); err == nil {
		t.Fatal("custom selection without a capture snapshot was projected as inactive")
	} else {
		var maskErr *maskcontract.Error
		if !errors.As(err, &maskErr) || maskErr.Code != maskcontract.CodeMaskUnavailable {
			t.Fatalf("missing custom capture error = %v, want mask_unavailable", err)
		}
	}
	store.captures["capture-missing-custom"] = maskcontract.Capture{Selection: maskcontract.Selection{SessionID: "capture-missing-custom", MaskID: custom.ID, Revision: 1}}
	if _, err := service.Capture(context.Background(), "capture-missing-custom"); err == nil {
		t.Fatal("custom capture without a definition was accepted")
	} else {
		var maskErr *maskcontract.Error
		if !errors.As(err, &maskErr) || maskErr.Code != maskcontract.CodeMaskUnavailable {
			t.Fatalf("missing custom capture error = %v, want mask_unavailable", err)
		}
	}
	if _, err := service.GetMask(context.Background(), maskcontract.GetRequest{ID: "builtin/unknown"}); err == nil {
		t.Fatalf("reserved lookup error = %v", err)
	} else {
		var maskErr *maskcontract.Error
		if !errors.As(err, &maskErr) || maskErr.Code != maskcontract.CodeInvalidMask {
			t.Fatalf("reserved lookup error = %v, want invalid_mask", err)
		}
	}
}
