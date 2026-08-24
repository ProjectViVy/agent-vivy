package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-vivy/internal/domain"
)

// AskUserName is the control-flow tool that suspends a run for a direct
// user answer. It is not an approval and never authorizes an effect.
const AskUserName = "ask_user"

type askUser struct{}

// NewAskUser returns the Vivy-owned user-question control tool.
func NewAskUser() Tool { return askUser{} }

func (askUser) Spec() domain.ToolSpec {
	return domain.ToolSpec{
		Name:        AskUserName,
		Description: "Asks the user one clarifying question and waits for their answer.",
		Readonly:    true,
		Keywords:    []string{"ask", "clarify", "question"},
		Interaction: domain.ToolInteractionQuestion,
		Params: map[string]domain.ToolParam{
			"question": {Desc: "The question to show the user.", Required: true},
		},
	}
}

func (askUser) InvokableRun(_ context.Context, _ json.RawMessage) (string, error) {
	return "", fmt.Errorf("tools: ask_user is a runtime interaction and cannot execute directly")
}
