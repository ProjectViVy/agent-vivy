package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	einoskill "github.com/cloudwego/eino/adk/middlewares/skill"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/skillhost"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/skillsource"
)

func toolkitSkillTestView(t *testing.T, body, digest string) (bundlecontract.View, string) {
	t.Helper()
	installID := "11111111-1111-4111-8111-111111111111"
	release := bundlecontract.ReleaseRef{
		InstallID: installID, BundleID: "acme/planning", ReleaseDigest: digest, ReleaseSequence: 3,
	}
	id, err := bundlecontract.SkillID(installID, "planner")
	if err != nil {
		t.Fatal(err)
	}
	maskID, err := bundlecontract.MaskID(installID, "pm")
	if err != nil {
		t.Fatal(err)
	}
	view, err := bundlecontract.NewView(bundlecontract.CatalogIdentity{Epoch: 3}, []bundlecontract.ReleaseRef{release}, []bundlecontract.ComponentRecord{{
		Ref:         bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.Skill, LocalID: "planner"},
		ProjectedID: id, Release: release, ContentDigest: digest, DisplayRef: "acme/planning:planner",
		Name: "Planning Expert", Description: "Creates delivery plans", Body: body, Resources: []string{"references/guide.md"}, Available: true,
	}, {
		Ref:         bundlecontract.ComponentRef{InstallID: installID, Kind: bundlecontract.Mask, LocalID: "pm"},
		ProjectedID: maskID, Release: release, ContentDigest: digest, Name: "PM", Body: "PM guidance",
		RecommendedSkills: []bundlecontract.ComponentRef{{InstallID: installID, Kind: bundlecontract.Skill, LocalID: "planner"}}, Available: true,
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return view, id
}

func TestBundleSkillSourceCapturedEpoch(t *testing.T) {
	oldView, id := toolkitSkillTestView(t, "old captured instructions", strings.Repeat("a", 64))
	newView, newID := toolkitSkillTestView(t, "new catalog instructions", strings.Repeat("b", 64))
	if newID != id {
		t.Fatalf("projection identity changed across releases: %q != %q", newID, id)
	}
	source := NewBundleSkillSource()

	ctx := withBundleView(context.Background(), oldView)
	listed, err := source.List(ctx, skillsource.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != id || listed[0].Metadata["origin"] != BundleSkillOrigin {
		t.Fatalf("captured Skill listing = %+v", listed)
	}
	got, err := source.Get(ctx, skillsource.Request{}, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "old captured instructions" || got.SourceHash != strings.Repeat("a", 64) {
		t.Fatalf("source resolved non-captured content: %+v", got)
	}

	newGot, err := source.Get(withBundleView(context.Background(), newView), skillsource.Request{}, id)
	if err != nil || newGot.Content != "new catalog instructions" {
		t.Fatalf("new captured Skill = %+v, err=%v", newGot, err)
	}
}

func TestHostedBundleSkillLookupUsesStableID(t *testing.T) {
	view, id := toolkitSkillTestView(t, "same Host and explicit invocation", strings.Repeat("c", 64))
	backend, err := NewHostedSkillBackend(nil, NewBundleSkillSource())
	if err != nil {
		t.Fatal(err)
	}
	ctx := withBundleView(context.Background(), view)
	listed, err := backend.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Name != id {
		t.Fatalf("Eino lookup IDs = %+v", listed)
	}
	resolved, err := backend.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Content != "same Host and explicit invocation" || resolved.BaseDirectory != "" {
		t.Fatalf("resolved Toolkit Skill = %+v", resolved)
	}
}

func TestBundleSkillCollision(t *testing.T) {
	view, id := toolkitSkillTestView(t, "bundle skill", strings.Repeat("d", 64))
	localRoot := t.TempDir()
	localDir := filepath.Join(localRoot, id)
	if err := os.MkdirAll(localDir, 0700); err != nil {
		t.Fatal(err)
	}
	localText := "---\nname: " + id + "\ndescription: local collision\n---\nlocal body"
	if err := os.WriteFile(filepath.Join(localDir, "SKILL.md"), []byte(localText), 0600); err != nil {
		t.Fatal(err)
	}
	local, err := NewEinoSkillBackend(localRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	backend, err := NewHostedSkillBackend(local, NewBundleSkillSource())
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.List(withBundleView(context.Background(), view))
	if !errors.Is(err, skillhost.ErrDuplicateSkillID) {
		t.Fatalf("duplicate projected ID error = %v", err)
	}
}

func TestBundleSkillSourceRequiresCapturedView(t *testing.T) {
	if _, err := NewBundleSkillSource().List(context.Background(), skillsource.Request{}); err == nil {
		t.Fatal("listing without a captured View succeeded")
	}
	if _, err := NewBundleSkillSource().Get(context.Background(), skillsource.Request{}, "b_missing"); err == nil {
		t.Fatal("lookup without a captured View succeeded")
	}
}

func TestBundleMaskRecommendationDoesNotGrantSkillActivation(t *testing.T) {
	view, id := toolkitSkillTestView(t, "must remain inactive", strings.Repeat("f", 64))
	ops := newBundleSkillOperations(nil, nil, skillhost.Config{ActiveIDs: []string{}})
	ctx := withBundleView(context.Background(), view)
	items, err := ops.ListSkills(ctx, "run-2")
	if err != nil || len(items) != 0 {
		t.Fatalf("inactive Host projected Toolkit Skill: %+v, err=%v", items, err)
	}
	if _, err := ops.ViewSkill(ctx, "run-2", id, ""); !errors.Is(err, skillhost.ErrSkillInactive) {
		t.Fatalf("inactive Host view error = %v", err)
	}
}

func TestBundleSkillMutationsDenied(t *testing.T) {
	view, id := toolkitSkillTestView(t, "read only body", strings.Repeat("e", 64))
	ops := NewBundleSkillOperations(nil, nil)
	ctx := withBundleView(context.Background(), view)
	items, err := ops.ListSkills(ctx, "run-1")
	if err != nil || len(items) != 1 {
		t.Fatalf("toolkit skills = %+v, err=%v", items, err)
	}
	if items[0].Name != id || items[0].DisplayName != "Planning Expert" || items[0].Origin != BundleSkillOrigin || items[0].Source != "acme/planning:planner" {
		t.Fatalf("skill source projection = %+v", items[0])
	}
	item, err := ops.ViewSkill(ctx, "run-1", id, "")
	if err != nil || item.Content != "read only body" || item.RelativePath != "SKILL.md" {
		t.Fatalf("Skill view = %+v, err=%v", item, err)
	}
	if _, err := ops.ManageSkill(ctx, "run-1", tools.SkillManageRequest{Action: "delete", SkillName: id}); err == nil || !strings.Contains(err.Error(), "read_only_component") {
		t.Fatalf("Toolkit Skill mutation error = %v", err)
	}
	if _, err := ops.PrepareSkillProposal(ctx, "run-1", tools.SkillManageRequest{Action: "edit", SkillName: id}); err == nil || !strings.Contains(err.Error(), "read_only_component") {
		t.Fatalf("Toolkit Skill proposal error = %v", err)
	}
	if _, err := ops.SetSkillEnabled(ctx, id, false, "hash"); err == nil || !strings.Contains(err.Error(), "read_only_component") {
		t.Fatalf("Toolkit Skill enable mutation error = %v", err)
	}
	if _, err := ops.ApplySkillRevision(ctx, "run-1", id); err == nil || !strings.Contains(err.Error(), "read_only_component") {
		t.Fatalf("Toolkit Skill revision mutation error = %v", err)
	}
}

var _ einoskill.Backend = (*HostedSkillBackend)(nil)
