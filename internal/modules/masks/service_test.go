package masks

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/maskcontract"
)

type serviceStore struct {
	custom         []maskcontract.Definition
	createdRequest *maskcontract.CreateRequest
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

func (store *serviceStore) CreateCustomMask(_ context.Context, in maskcontract.CreateRequest) (maskcontract.Definition, error) {
	copy := in
	store.createdRequest = &copy
	return maskcontract.Definition{
		ID: "custom/00000000-0000-4000-8000-000000000099", Name: in.Name, Description: in.Description, Body: in.Body,
		Revision: 1, Digest: maskcontract.DefinitionDigest("custom/00000000-0000-4000-8000-000000000099", in.Name, in.Body), Origin: maskcontract.OriginCustom,
		Bundle: in.Bundle, RecommendedSkills: in.RecommendedSkills,
	}, nil
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

type fixtureToolkitViewManager struct{ view bundlecontract.View }

func (manager fixtureToolkitViewManager) CurrentView(context.Context) (bundlecontract.View, func(), error) {
	return manager.view, func() {}, nil
}

func toolkitMaskView(t *testing.T, includeMask, skillAvailable bool) (bundlecontract.View, string, string) {
	return toolkitMaskViewVersion(t, includeMask, skillAvailable, "literal PM guide", 7, strings.Repeat("a", 64), strings.Repeat("b", 64))
}

func toolkitMaskViewVersion(t *testing.T, includeMask, skillAvailable bool, body string, sequence int64, releaseDigest, catalogDigest string) (bundlecontract.View, string, string) {
	t.Helper()
	installID := "11111111-1111-4111-8111-111111111111"
	release := bundlecontract.ReleaseRef{InstallID: installID, BundleID: "acme/experts", ReleaseDigest: releaseDigest, ReleaseSequence: sequence}
	skillID, _ := bundlecontract.SkillID(installID, "prd")
	maskID, _ := bundlecontract.MaskID(installID, "pm")
	components := []bundlecontract.ComponentRecord{{
		Ref:         bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.Skill, LocalID: "prd"},
		ProjectedID: skillID, Release: release, ContentDigest: release.ReleaseDigest, Name: "PRD Author", Description: "Writes PRDs", Body: "write a PRD", Available: skillAvailable,
	}}
	if includeMask {
		components = append(components, bundlecontract.ComponentRecord{
			Ref:         bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.Mask, LocalID: "pm"},
			ProjectedID: maskID, Release: release, ContentDigest: release.ReleaseDigest,
			Name: "Product Manager", Description: "Plans product work", Body: body,
			RecommendedSkills: []bundlecontract.ComponentRef{{InstallID: installID, Kind: bundlecontract.Skill, LocalID: "prd"}}, Available: true,
		})
		devID, _ := bundlecontract.MaskID(installID, "dev")
		components = append(components, bundlecontract.ComponentRecord{
			Ref:         bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.Mask, LocalID: "dev"},
			ProjectedID: devID, Release: release, ContentDigest: release.ReleaseDigest,
			Name: "Developer", Description: "Implements approved plans", Body: "literal developer guide", Available: true,
		})
	}
	view, err := bundlecontract.NewView(bundlecontract.CatalogIdentity{Epoch: sequence, EffectiveDigest: catalogDigest}, []bundlecontract.ReleaseRef{release}, components, nil)
	if err != nil {
		t.Fatal(err)
	}
	return view, maskID, skillID
}

func TestBundleMasksCatalogAndAdmissionCapture(t *testing.T) {
	catalog, err := NewCatalog("test-generation")
	if err != nil {
		t.Fatal(err)
	}
	view, maskID, skillID := toolkitMaskView(t, true, true)
	store := &serviceStore{captures: map[domain.SessionID]maskcontract.Capture{
		"bundle-session": {Selection: maskcontract.Selection{SessionID: "bundle-session", MaskID: maskID, Revision: 4}},
	}, selection: map[domain.SessionID]maskcontract.Selection{"bundle-session": {SessionID: "bundle-session", MaskID: maskID, Revision: 4}}}
	service, err := NewService(store, catalog)
	if err != nil {
		t.Fatal(err)
	}
	service.SetToolkitManager(fixtureToolkitViewManager{view: view})
	page, err := service.ListMasksFromView(context.Background(), maskcontract.ListRequest{Limit: 100}, view)
	if err != nil {
		t.Fatal(err)
	}
	var listed *maskcontract.Metadata
	for i := range page.Items {
		if page.Items[i].ID == maskID {
			listed = &page.Items[i]
		}
	}
	if listed == nil || listed.Origin != maskcontract.OriginBundle || !listed.Available || listed.Bundle == nil || listed.Revision != 7 || len(listed.RecommendedSkills) != 1 {
		t.Fatalf("bundle Mask metadata = %+v", listed)
	}
	if listed.RecommendedSkills[0].SkillID != skillID || !listed.RecommendedSkills[0].Available {
		t.Fatalf("recommendation did not resolve as display metadata: %+v", listed.RecommendedSkills)
	}
	bundleMasks := 0
	for _, item := range page.Items {
		if item.Origin == maskcontract.OriginBundle {
			bundleMasks++
		}
	}
	if bundleMasks != 2 {
		t.Fatalf("bundle Mask catalog contains %d entries, want PM and Dev", bundleMasks)
	}
	definition, err := service.GetMask(context.Background(), maskcontract.GetRequest{ID: maskID})
	if err != nil || definition.Origin != maskcontract.OriginBundle || definition.Body != "literal PM guide" {
		t.Fatalf("bundle Mask definition = %+v, err=%v", definition, err)
	}
	capture, err := service.CaptureFromView(context.Background(), "bundle-session", view)
	if err != nil || capture.Mask == nil {
		t.Fatalf("bundle Mask capture = %+v, err=%v", capture, err)
	}
	if capture.Mask.Body != definition.Body || capture.Mask.Origin != maskcontract.OriginBundle || capture.Mask.Bundle == nil || capture.Mask.Bundle.Release != definition.Bundle.Release {
		t.Fatalf("captured prompt Mask lost immutable provenance: %+v", capture.Mask)
	}
	if _, err := service.SetMaskSelection(context.Background(), maskcontract.SetSelectionRequest{SessionID: "bundle-session", MaskID: maskID, ExpectedRevision: 4, ExpectedCatalogEpoch: 6}); err == nil {
		t.Fatal("selection accepted a stale Toolkit catalog epoch")
	}
	selected, err := service.SetMaskSelection(context.Background(), maskcontract.SetSelectionRequest{SessionID: "bundle-session", MaskID: maskID, ExpectedRevision: 4, ExpectedCatalogEpoch: 7})
	if err != nil || selected.Selection.MaskID != maskID || !selected.Available {
		t.Fatalf("bundle Mask selection = %+v, err=%v", selected, err)
	}
	currentSelection, err := service.GetMaskSelection(context.Background(), maskcontract.SelectionRequest{SessionID: "bundle-session"})
	if err != nil || !currentSelection.Available || currentSelection.Selection.MaskID != maskID {
		t.Fatalf("selected bundle Mask inspection = %+v, err=%v", currentSelection, err)
	}
}

func TestBundleMaskDisabledSelectionRemainsVisibleAndUnavailable(t *testing.T) {
	catalog, err := NewCatalog("test-generation")
	if err != nil {
		t.Fatal(err)
	}
	_, maskID, _ := toolkitMaskView(t, true, true)
	store := &serviceStore{captures: map[domain.SessionID]maskcontract.Capture{
		"bundle-disabled": {Selection: maskcontract.Selection{SessionID: "bundle-disabled", MaskID: maskID, Revision: 2}},
	}, selection: map[domain.SessionID]maskcontract.Selection{}}
	service, err := NewService(store, catalog)
	if err != nil {
		t.Fatal(err)
	}
	disabled, _, _ := toolkitMaskView(t, false, true)
	service.SetToolkitManager(fixtureToolkitViewManager{view: disabled})
	selection, err := service.GetMaskSelection(context.Background(), maskcontract.SelectionRequest{SessionID: "bundle-disabled"})
	if err != nil || selection.Available || selection.InactiveReason != "toolkit_disabled" || selection.Selection.MaskID != maskID {
		t.Fatalf("disabled bundle Mask selection = %+v, err=%v", selection, err)
	}
	if _, err := service.CaptureFromView(context.Background(), "bundle-disabled", disabled); err == nil {
		t.Fatal("disabled current selection was silently replaced")
	}
}

func TestBundleMaskUpdateKeepsSelectionAndChangesOnlyNextCapture(t *testing.T) {
	catalog, err := NewCatalog("test-generation")
	if err != nil {
		t.Fatal(err)
	}
	oldView, maskID, _ := toolkitMaskViewVersion(t, true, true, "old PM guide", 7, strings.Repeat("a", 64), strings.Repeat("b", 64))
	newView, updatedID, _ := toolkitMaskViewVersion(t, true, true, "updated PM guide", 8, strings.Repeat("c", 64), strings.Repeat("d", 64))
	if updatedID != maskID {
		t.Fatalf("stable local Mask ID changed across releases: %q != %q", updatedID, maskID)
	}
	store := &serviceStore{captures: map[domain.SessionID]maskcontract.Capture{
		"mask-update": {Selection: maskcontract.Selection{SessionID: "mask-update", MaskID: maskID, Revision: 9}},
	}}
	service, err := NewService(store, catalog)
	if err != nil {
		t.Fatal(err)
	}
	oldCapture, err := service.CaptureFromView(context.Background(), "mask-update", oldView)
	if err != nil {
		t.Fatal(err)
	}
	newCapture, err := service.CaptureFromView(context.Background(), "mask-update", newView)
	if err != nil {
		t.Fatal(err)
	}
	if oldCapture.Mask == nil || oldCapture.Mask.Body != "old PM guide" || oldCapture.Mask.DefinitionRevision != 7 {
		t.Fatalf("old captured Mask changed after update: %+v", oldCapture.Mask)
	}
	if newCapture.Mask == nil || newCapture.Mask.Body != "updated PM guide" || newCapture.Mask.DefinitionRevision != 8 || newCapture.Selection.MaskID != maskID || newCapture.Selection.Revision != 9 {
		t.Fatalf("next Mask capture did not retain stable selection and new definition: %+v", newCapture)
	}
}

func TestCopyBundleMaskCapturesPinnedSourceAndRecommendationMetadata(t *testing.T) {
	catalog, err := NewCatalog("test-generation")
	if err != nil {
		t.Fatal(err)
	}
	view, maskID, skillID := toolkitMaskView(t, true, true)
	store := &serviceStore{}
	service, err := NewService(store, catalog)
	if err != nil {
		t.Fatal(err)
	}
	service.SetToolkitManager(fixtureToolkitViewManager{view: view})
	request := maskcontract.CopyMaskRequest{
		OperationID: "00000000-0000-4000-8000-000000000098", SourceID: maskID, ExpectedCatalogEpoch: 6,
	}
	if _, err := service.CopyMask(context.Background(), request); err == nil {
		t.Fatal("copy accepted a stale catalog epoch")
	}
	if store.createdRequest != nil {
		t.Fatal("stale copy wrote a custom Mask")
	}
	request.ExpectedCatalogEpoch = 7
	copied, err := service.CopyMask(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if copied.ID != "custom/00000000-0000-4000-8000-000000000099" || copied.Origin != maskcontract.OriginCustom || copied.Body != "literal PM guide" || copied.Bundle == nil || copied.Bundle.LocalID != "pm" {
		t.Fatalf("copied definition = %+v", copied)
	}
	if store.createdRequest == nil || store.createdRequest.OperationID != request.OperationID || len(store.createdRequest.RecommendedSkills) != 1 || store.createdRequest.RecommendedSkills[0].SkillID != skillID {
		t.Fatalf("copy request did not preserve source recommendations: %+v", store.createdRequest)
	}
}

func TestUnavailableBundleSkillRecommendationStaysInformational(t *testing.T) {
	catalog, err := NewCatalog("test-generation")
	if err != nil {
		t.Fatal(err)
	}
	view, maskID, _ := toolkitMaskView(t, true, false)
	service, err := NewService(&serviceStore{}, catalog)
	if err != nil {
		t.Fatal(err)
	}
	service.SetToolkitManager(fixtureToolkitViewManager{view: view})
	definition, err := service.GetMask(context.Background(), maskcontract.GetRequest{ID: maskID})
	if err != nil || len(definition.RecommendedSkills) != 1 || definition.RecommendedSkills[0].Available || definition.RecommendedSkills[0].InactiveReason != "skill_unavailable" {
		t.Fatalf("unavailable recommendation state = %+v/%v", definition.RecommendedSkills, err)
	}
}

func TestBundleMaskDefinitionsAreReadOnlyUntilCopied(t *testing.T) {
	catalog, err := NewCatalog("test-generation")
	if err != nil {
		t.Fatal(err)
	}
	view, maskID, _ := toolkitMaskView(t, true, true)
	service, err := NewService(&serviceStore{}, catalog)
	if err != nil {
		t.Fatal(err)
	}
	service.SetToolkitManager(fixtureToolkitViewManager{view: view})
	_, updateErr := service.UpdateMask(context.Background(), maskcontract.UpdateRequest{ID: maskID, ExpectedRevision: 7, Name: "Edited", Body: "changed"})
	var typed *maskcontract.Error
	if !errors.As(updateErr, &typed) || typed.Code != maskcontract.CodeReadOnlyComponent {
		t.Fatalf("bundle Mask update error = %v, want read_only_component", updateErr)
	}
	_, deleteErr := service.DeleteMask(context.Background(), maskcontract.DeleteRequest{ID: maskID, ExpectedRevision: 7})
	if !errors.As(deleteErr, &typed) || typed.Code != maskcontract.CodeReadOnlyComponent {
		t.Fatalf("bundle Mask delete error = %v, want read_only_component", deleteErr)
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
