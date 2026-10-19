package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/tools"
)

// ScriptedModel replays a fixed script of agentic assistant messages, one per
// Generate/Stream call. It is exported so packages outside runtime (the
// integration tests, which must not import eino themselves, D-007) can drive
// interrupt/resume flows deterministically. Test-only: production paths never
// use it.
type ScriptedModel struct {
	mu     sync.Mutex
	calls  int
	script []*schema.AgenticMessage
}

var _ model.AgenticModel = (*ScriptedModel)(nil)

// NewScriptedModel replays script in order; exhausting it fails the run.
func NewScriptedModel(script ...*schema.AgenticMessage) *ScriptedModel {
	return &ScriptedModel{script: script}
}

// ScriptedText builds the scripted agentic message for a plain assistant
// answer. Exported for out-of-package integration tests.
func ScriptedText(text string) *schema.AgenticMessage {
	return agenticAssistantTextMessage(text)
}

// ScriptedCall builds one scripted function tool call.
func ScriptedCall(callID, name, args string) *schema.FunctionToolCall {
	return &schema.FunctionToolCall{CallID: callID, Name: name, Arguments: args}
}

// ScriptedToolCalls builds the scripted agentic message that requests calls.
func ScriptedToolCalls(calls ...*schema.FunctionToolCall) *schema.AgenticMessage {
	return agenticAssistantToolCallMessage(calls)
}

// AgenticMessageText exposes the block text projection to out-of-package test
// suites, which must not import eino themselves (D-007). Product code uses the
// package-internal projection directly.
func AgenticMessageText(msg *schema.AgenticMessage) string {
	return agenticMessageText(msg)
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
		agenticAssistantToolCallMessage([]*schema.FunctionToolCall{{
			CallID:    ApprovalFlowCallID,
			Name:      tools.WriteNoteName,
			Arguments: `{"content":"buy milk"}`,
		}}),
		agenticAssistantTextMessage("Done: the note has been handled."),
	)
}

// NewQuestionFlowModel requests one user answer and then closes with a
// deterministic assistant reply after resume.
func NewQuestionFlowModel() *ScriptedModel {
	return NewScriptedModel(
		agenticAssistantToolCallMessage([]*schema.FunctionToolCall{{
			CallID:    QuestionFlowCallID,
			Name:      tools.AskUserName,
			Arguments: `{"question":"Which color should I use?"}`,
		}}),
		agenticAssistantTextMessage("Thanks, I will use your choice."),
	)
}

// Generate returns the next scripted message.
func (m *ScriptedModel) Generate(_ context.Context, _ []*schema.AgenticMessage, _ ...model.Option) (*schema.AgenticMessage, error) {
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
func (m *ScriptedModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	r, w := schema.Pipe[*schema.AgenticMessage](1)
	go func() {
		defer w.Close()
		w.Send(msg, nil)
	}()
	return r, nil
}
