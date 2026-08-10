package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/tools"
)

// ScriptedModel replays a fixed script of assistant messages, one per
// Generate/Stream call. It is exported so packages outside runtime (the
// httpapi integration tests, which must not import eino themselves,
// D-007) can drive interrupt/resume flows deterministically. Test-only:
// production paths never use it (mirrors the provider.NewMock precedent).
type ScriptedModel struct {
	mu     sync.Mutex
	calls  int
	script []*schema.Message
}

var _ model.ToolCallingChatModel = (*ScriptedModel)(nil)

// NewScriptedModel replays script in order; exhausting it fails the run.
func NewScriptedModel(script ...*schema.Message) *ScriptedModel {
	return &ScriptedModel{script: script}
}

// ApprovalFlowCallID is the tool call id the approval flow script uses;
// tests assert it end-to-end (tool.requested -> approval -> resume).
const ApprovalFlowCallID = "call-note-1"

// QuestionFlowCallID is the scripted ask_user call id used by H4 tests.
const QuestionFlowCallID = "call-question-1"

// NewApprovalFlowModel returns the scripted model for the write_note
// approval flow (spike V3 shape): the first turn requests the effectful
// tool call, and the resumed turn closes with a plain assistant reply.
func NewApprovalFlowModel() *ScriptedModel {
	return NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       ApprovalFlowCallID,
			Function: schema.FunctionCall{Name: tools.WriteNoteName, Arguments: `{"content":"buy milk"}`},
		}}),
		schema.AssistantMessage("Done: the note has been handled.", nil),
	)
}

// NewQuestionFlowModel requests one user answer and then closes with a
// deterministic assistant reply after resume.
func NewQuestionFlowModel() *ScriptedModel {
	return NewScriptedModel(
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:       QuestionFlowCallID,
			Function: schema.FunctionCall{Name: tools.AskUserName, Arguments: `{"question":"Which color should I use?"}`},
		}}),
		schema.AssistantMessage("Thanks, I will use your choice.", nil),
	)
}

// Generate returns the next scripted message.
func (m *ScriptedModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := m.calls
	m.calls++
	if i >= len(m.script) {
		return nil, fmt.Errorf("runtime: scripted model exhausted at call %d", i)
	}
	return m.script[i], nil
}

// Stream delivers the next scripted message as a single chunk so tool
// calls stay whole (streaming engines accumulate them from chunks).
func (m *ScriptedModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	r, w := schema.Pipe[*schema.Message](1)
	go func() {
		defer w.Close()
		w.Send(msg, nil)
	}()
	return r, nil
}

// WithTools returns the model unchanged: the script decides when tool
// calls happen.
func (m *ScriptedModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
