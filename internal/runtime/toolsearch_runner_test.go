package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// toolSurfaceScriptModel is a deterministic ToolCallingChatModel that records
// the ToolInfo list actually passed to each model generation. It intentionally
// drives the real Eino Runner rather than calling middleware hooks directly.
type toolSurfaceScriptModel struct {
	mu       sync.Mutex
	script   []*schema.Message
	calls    int
	surfaces [][]string
}

func (m *toolSurfaceScriptModel) Generate(_ context.Context, _ []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.next(opts)
}

func (m *toolSurfaceScriptModel) Stream(_ context.Context, _ []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.next(opts)
	if err != nil {
		return nil, err
	}
	reader, writer := schema.Pipe[*schema.Message](1)
	go func() {
		defer writer.Close()
		writer.Send(msg, nil)
	}()
	return reader, nil
}

func (m *toolSurfaceScriptModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *toolSurfaceScriptModel) next(opts []model.Option) (*schema.Message, error) {
	options := model.GetCommonOptions(nil, opts...)
	names := make([]string, 0, len(options.Tools))
	for _, info := range options.Tools {
		if info != nil {
			names = append(names, info.Name)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.surfaces = append(m.surfaces, append([]string(nil), names...))
	if m.calls >= len(m.script) {
		return nil, fmt.Errorf("surface script exhausted at model call %d", m.calls)
	}
	msg := m.script[m.calls]
	m.calls++
	return msg, nil
}

func (m *toolSurfaceScriptModel) snapshotSurfaces() [][]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]string, len(m.surfaces))
	for i, surface := range m.surfaces {
		out[i] = append([]string(nil), surface...)
	}
	return out
}

type runnerProbeTool struct {
	name  string
	mu    sync.Mutex
	calls int
}

func (t *runnerProbeTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: t.name, Description: "deterministic dynamic probe", Readonly: true}
}

func (t *runnerProbeTool) InvokableRun(context.Context, json.RawMessage) (string, error) {
	t.mu.Lock()
	t.calls++
	t.mu.Unlock()
	return "dynamic probe complete", nil
}

func (t *runnerProbeTool) callCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}

func drainSurfaceRunner(t *testing.T, eng *Engine, ctx context.Context) {
	t.Helper()
	it := eng.Query(ctx, "exercise the configured tools")
	for {
		event, ok := it.Next()
		if !ok {
			return
		}
		if event.Err != nil {
			t.Fatalf("runner event error: %v", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil || event.Output.MessageOutput.MessageStream == nil {
			continue
		}
		stream := event.Output.MessageOutput.MessageStream
		for {
			_, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("runner stream error: %v", err)
			}
		}
	}
}

func runDynamicToolSearchScenario(t *testing.T, active []tools.Tool, selected []string, dynamic *runnerProbeTool) *toolSurfaceScriptModel {
	t.Helper()
	model := &toolSurfaceScriptModel{script: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "search-1",
			Function: schema.FunctionCall{Name: officialToolSearchName, Arguments: `{"query":"select:dynamic_probe"}`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "dynamic-1",
			Function: schema.FunctionCall{Name: dynamic.name, Arguments: `{}`},
		}}),
		schema.AssistantMessage("done", nil),
	}}
	eng, err := NewEngine(context.Background(), model, active, EngineConfig{StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	drainSurfaceRunner(t, eng, withSelectedTools(context.Background(), selected))
	return model
}

func TestEngineRunnerUsesOfficialToolSearchForActiveDynamicTools(t *testing.T) {
	dynamic := &runnerProbeTool{name: "dynamic_probe"}
	model := runDynamicToolSearchScenario(t,
		[]tools.Tool{tools.NewListDir(nil), dynamic},
		[]string{tools.ListDirName, dynamic.name}, dynamic)

	want := [][]string{
		{tools.ListDirName, officialToolSearchName},
		{tools.ListDirName, officialToolSearchName, dynamic.name},
		{tools.ListDirName, officialToolSearchName, dynamic.name},
	}
	if got := model.snapshotSurfaces(); !reflect.DeepEqual(got, want) {
		t.Fatalf("model ToolInfo surfaces = %v, want %v", got, want)
	}
	if got := dynamic.callCount(); got != 1 {
		t.Fatalf("dynamic tool calls = %d, want one real execution", got)
	}
}

func TestEngineRunnerUsesOfficialToolSearchWithoutFixedTools(t *testing.T) {
	dynamic := &runnerProbeTool{name: "dynamic_probe"}
	model := runDynamicToolSearchScenario(t, []tools.Tool{dynamic}, []string{dynamic.name}, dynamic)

	want := [][]string{
		{officialToolSearchName},
		{officialToolSearchName, dynamic.name},
		{officialToolSearchName, dynamic.name},
	}
	if got := model.snapshotSurfaces(); !reflect.DeepEqual(got, want) {
		t.Fatalf("model ToolInfo surfaces without fixed core = %v, want %v", got, want)
	}
	if got := dynamic.callCount(); got != 1 {
		t.Fatalf("dynamic tool calls without fixed core = %d, want one real execution", got)
	}
}

func TestEngineRunnerMountsHiddenToolInfoAfterTheFirstModelCall(t *testing.T) {
	model := &toolSurfaceScriptModel{script: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "view-1",
			Function: schema.FunctionCall{Name: tools.SkillViewName, Arguments: `{"name":"writer"}`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "mounted-1",
			Function: schema.FunctionCall{Name: tools.EchoInfoName, Arguments: `{"text":"mounted"}`},
		}}),
		schema.AssistantMessage("done", nil),
	}}
	eng, err := NewEngine(context.Background(), model, []tools.Tool{tools.NewSkillView(mountSkillOps{})}, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
		HiddenTools: []tools.Tool{tools.NewEchoInfo()},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	mounts := tools.NewMountedTools()
	ctx := tools.WithMountedTools(withSelectedTools(context.Background(), []string{tools.SkillViewName}), mounts)
	drainSurfaceRunner(t, eng, ctx)

	want := [][]string{
		{tools.SkillViewName},
		{tools.SkillViewName, tools.EchoInfoName},
		{tools.SkillViewName, tools.EchoInfoName},
	}
	if got := model.snapshotSurfaces(); !reflect.DeepEqual(got, want) {
		t.Fatalf("model ToolInfo surfaces after mount = %v, want %v", got, want)
	}
	if !mounts.Has(tools.EchoInfoName) {
		t.Fatal("skill_view did not mount the declared hidden tool")
	}
}

func TestEngineRunnerWithoutDynamicToolsDoesNotInstallSearch(t *testing.T) {
	model := &toolSurfaceScriptModel{script: []*schema.Message{schema.AssistantMessage("done", nil)}}
	eng, err := NewEngine(context.Background(), model, []tools.Tool{tools.NewListDir(nil)}, EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
		HiddenTools: []tools.Tool{tools.NewEchoInfo()},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	drainSurfaceRunner(t, eng, context.Background())
	if got, want := model.snapshotSurfaces(), [][]string{{tools.ListDirName}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("no-dynamic ToolInfo surface = %v, want %v", got, want)
	}
}

type reservedTool struct{}

func (reservedTool) Spec() domain.ToolSpec {
	return domain.ToolSpec{Name: officialToolSearchName, Description: "conflicting tool", Readonly: true}
}

func (reservedTool) InvokableRun(context.Context, json.RawMessage) (string, error) { return "", nil }

func TestEngineRejectsReservedToolSearchName(t *testing.T) {
	_, err := NewEngine(context.Background(), &toolSurfaceScriptModel{script: []*schema.Message{{}}}, []tools.Tool{reservedTool{}}, EngineConfig{})
	if err == nil || !strings.Contains(err.Error(), officialToolSearchName) {
		t.Fatalf("reserved tool name error = %v", err)
	}
}
