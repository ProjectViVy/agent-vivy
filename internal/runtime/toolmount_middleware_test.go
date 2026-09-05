package runtime

import (
	"context"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/tools"
)

func TestFixedVisibleToolNamesAreExact(t *testing.T) {
	want := []string{
		tools.AskUserName,
		tools.ListDirName,
		tools.ReadFileName,
		tools.SearchFilesName,
		tools.SkillsListName,
		tools.SkillViewName,
		tools.WriteFileName,
		tools.PatchName,
		tools.MultiEditName,
		tools.ExecuteName,
		tools.BashName,
	}
	if len(fixedVisibleToolNames) != len(want) {
		t.Fatalf("fixed-visible core size = %d, want %d", len(fixedVisibleToolNames), len(want))
	}
	for _, name := range want {
		if !isFixedVisibleTool(name) {
			t.Errorf("fixed-visible core missing %q", name)
		}
	}
	for _, name := range []string{"echo_info", "commandline", "skill_manage", "tool_search"} {
		if isFixedVisibleTool(name) {
			t.Errorf("%q must remain dynamic/reserved", name)
		}
	}
}

func surfaceInfos(names ...string) []*schema.ToolInfo {
	out := make([]*schema.ToolInfo, 0, len(names))
	for _, name := range names {
		out = append(out, &schema.ToolInfo{Name: name})
	}
	return out
}

func surfaceNames(infos []*schema.ToolInfo) []string {
	out := make([]string, 0, len(infos))
	for _, info := range infos {
		if info != nil {
			out = append(out, info.Name)
		}
	}
	return out
}

func TestMountedToolVisibilityMiddlewareFiltersAndRehydrates(t *testing.T) {
	hiddenInfo := &schema.ToolInfo{Name: "hidden_tool", Desc: "mounted"}
	mw := newMountedToolVisibilityMiddleware(
		map[string]*schema.ToolInfo{"hidden_tool": hiddenInfo},
		[]string{"hidden_tool"},
	).(*mountedToolVisibilityMiddleware)

	state := &adk.ChatModelAgentState{ToolInfos: surfaceInfos("fixed_core", "hidden_tool", "foreign")}
	_, got, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("pre-mount rewrite: %v", err)
	}
	if want := []string{"fixed_core", "foreign"}; !reflect.DeepEqual(surfaceNames(got.ToolInfos), want) {
		t.Fatalf("pre-mount view = %v, want %v", surfaceNames(got.ToolInfos), want)
	}

	// Eino persists the previous filtered state. The hidden ToolInfo is no
	// longer present here, so a correct projection must rehydrate it from its
	// static inventory after the Skill mount.
	mounts := tools.NewMountedTools()
	mounts.Mount("hidden_tool")
	ctx := tools.WithMountedTools(context.Background(), mounts)
	state = &adk.ChatModelAgentState{ToolInfos: surfaceInfos("fixed_core", "foreign")}
	_, got, err = mw.BeforeModelRewriteState(ctx, state, nil)
	if err != nil {
		t.Fatalf("post-mount rewrite: %v", err)
	}
	want := []string{"fixed_core", "foreign", "hidden_tool"}
	if gotNames := surfaceNames(got.ToolInfos); !reflect.DeepEqual(gotNames, want) {
		t.Fatalf("post-mount view = %v, want %v", gotNames, want)
	}
	if got.ToolInfos[2] != hiddenInfo {
		t.Fatal("post-mount view did not use the complete hidden ToolInfo")
	}

	// Re-running the projection must be idempotent.
	_, got, err = mw.BeforeModelRewriteState(ctx, got, nil)
	if err != nil {
		t.Fatalf("repeat post-mount rewrite: %v", err)
	}
	if gotNames := surfaceNames(got.ToolInfos); !reflect.DeepEqual(gotNames, want) {
		t.Fatalf("repeat post-mount view = %v, want %v", gotNames, want)
	}
}

func TestMountedToolVisibilityMiddlewareToleratesNilStateAndInfo(t *testing.T) {
	mw := newMountedToolVisibilityMiddleware(
		map[string]*schema.ToolInfo{"hidden_tool": {Name: "hidden_tool"}},
		[]string{"hidden_tool"},
	).(*mountedToolVisibilityMiddleware)
	state := &adk.ChatModelAgentState{ToolInfos: []*schema.ToolInfo{nil, {Name: "fixed_core"}, {Name: "hidden_tool"}}}
	_, got, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("rewrite state: %v", err)
	}
	if want := []string{"fixed_core"}; !reflect.DeepEqual(surfaceNames(got.ToolInfos), want) {
		t.Fatalf("view = %v, want %v", surfaceNames(got.ToolInfos), want)
	}
	if _, got, err := mw.BeforeModelRewriteState(context.Background(), nil, nil); err != nil || got != nil {
		t.Fatalf("nil state must pass through, got (%+v, %v)", got, err)
	}
}
