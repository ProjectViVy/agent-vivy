package tools

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"agent-vivy/internal/domain"
)

func TestMountedToolsOrderDedupAndNilSafety(t *testing.T) {
	m := NewMountedTools()
	m.Mount("read_file", "write_file")
	m.Mount("read_file", "patch", "")
	want := []string{"read_file", "write_file", "patch"}
	if got := m.Mounted(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Mounted() = %v, want %v", got, want)
	}
	if !m.Has("write_file") || m.Has("absent") {
		t.Fatal("Has membership broken")
	}
	var nilSet *MountedTools
	nilSet.Mount("x")
	if got := nilSet.Mounted(); got != nil {
		t.Fatalf("nil receiver Mounted() = %v, want nil", got)
	}
	if nilSet.Has("x") {
		t.Fatal("nil receiver Has must be false")
	}
}

// skillViewStub implements SkillOperations for one view result; unimplemented
// methods panic if a tool ever calls them.
type skillViewStub struct {
	SkillOperations
	view SkillView
}

func (s *skillViewStub) ViewSkill(context.Context, domain.RunID, string, string) (SkillView, error) {
	return s.view, nil
}

// Viewing a SKILL.md that declares tools mounts them for the run.
func TestSkillViewMountsDeclaredTools(t *testing.T) {
	ops := &skillViewStub{view: SkillView{
		SkillSummary: SkillSummary{Name: "writer", Tools: []string{"write_file", "absent_tool"}},
		Content:      "body", RelativePath: "SKILL.md",
	}}
	mounts := NewMountedTools()
	ctx := WithMountedTools(context.Background(), mounts)
	if _, err := NewSkillView(ops).InvokableRun(ctx, json.RawMessage(`{"name":"writer"}`)); err != nil {
		t.Fatalf("skill_view: %v", err)
	}
	want := []string{"write_file", "absent_tool"}
	if got := mounts.Mounted(); !reflect.DeepEqual(got, want) {
		t.Fatalf("mounts = %v, want %v", got, want)
	}
}

// Viewing a supporting file must not mount anything.
func TestSkillViewSupportingFileDoesNotMount(t *testing.T) {
	ops := &skillViewStub{view: SkillView{
		SkillSummary: SkillSummary{Name: "writer", Tools: []string{"write_file"}},
		Content:      "x", RelativePath: "references/a.md",
	}}
	mounts := NewMountedTools()
	ctx := WithMountedTools(context.Background(), mounts)
	if _, err := NewSkillView(ops).InvokableRun(ctx, json.RawMessage(`{"name":"writer","path":"references/a.md"}`)); err != nil {
		t.Fatalf("skill_view: %v", err)
	}
	if got := mounts.Mounted(); len(got) != 0 {
		t.Fatalf("supporting file mounts = %v, want none", got)
	}
}

// Without a mount registry in the run context, skill_view still succeeds.
func TestSkillViewWithoutMountRegistrySucceeds(t *testing.T) {
	ops := &skillViewStub{view: SkillView{
		SkillSummary: SkillSummary{Name: "writer", Tools: []string{"write_file"}},
		Content:      "body", RelativePath: "SKILL.md",
	}}
	if _, err := NewSkillView(ops).InvokableRun(context.Background(), json.RawMessage(`{"name":"writer"}`)); err != nil {
		t.Fatalf("skill_view without mounts: %v", err)
	}
}
