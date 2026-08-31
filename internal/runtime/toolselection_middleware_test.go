package runtime

import (
	"context"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/tools"
)

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

// The surface middleware owns the model's view: active tools stay, hidden
// tools stay hidden until mounted, and foreign names injected by other
// middleware (the Eino skill tool) are never hidden.
func TestToolSurfaceMiddlewareFiltersViewModel(t *testing.T) {
	mw := newToolSurfaceMiddleware(
		[]string{"skill_view", "read_file"},
		[]string{"skill_view", "read_file", "write_file", "patch"},
	).(*toolSurfaceMiddleware)

	state := &adk.ChatModelAgentState{ToolInfos: surfaceInfos("skill_view", "read_file", "write_file", "patch", "some_foreign_skill_tool")}
	_, got, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("rewrite state: %v", err)
	}
	want := []string{"skill_view", "read_file", "some_foreign_skill_tool"}
	if gotNames := surfaceNames(got.ToolInfos); !reflect.DeepEqual(gotNames, want) {
		t.Fatalf("pre-mount view = %v, want %v", gotNames, want)
	}

	mounts := tools.NewMountedTools()
	mounts.Mount("patch", "write_file")
	ctx := tools.WithMountedTools(context.Background(), mounts)
	state = &adk.ChatModelAgentState{ToolInfos: surfaceInfos("skill_view", "read_file", "write_file", "patch")}
	_, got, err = mw.BeforeModelRewriteState(ctx, state, nil)
	if err != nil {
		t.Fatalf("rewrite state after mount: %v", err)
	}
	want = []string{"skill_view", "read_file", "write_file", "patch"}
	if gotNames := surfaceNames(got.ToolInfos); !reflect.DeepEqual(gotNames, want) {
		t.Fatalf("post-mount view = %v, want %v", gotNames, want)
	}
}

func TestToolSurfaceMiddlewareToleratesMissingMountsAndNilInfos(t *testing.T) {
	mw := newToolSurfaceMiddleware([]string{"echo_info"}, []string{"echo_info", "hidden_tool"})
	state := &adk.ChatModelAgentState{ToolInfos: []*schema.ToolInfo{nil, {Name: "echo_info"}, {Name: "hidden_tool"}}}
	_, got, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("rewrite state: %v", err)
	}
	if want := []string{"echo_info"}; !reflect.DeepEqual(surfaceNames(got.ToolInfos), want) {
		t.Fatalf("view = %v, want %v", surfaceNames(got.ToolInfos), want)
	}
	if _, got, err := mw.BeforeModelRewriteState(context.Background(), nil, nil); err != nil || got != nil {
		t.Fatalf("nil state must pass through, got (%+v, %v)", got, err)
	}
}
