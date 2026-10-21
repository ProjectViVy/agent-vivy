package runtime

import (
	"agent-vivy/sdk/port/loopexp"
	"context"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// NudgeFactory is a sealed, lazy composition slot. D04 does not invoke it:
// core remains the sole coaching owner until the ordinary plugin cutover.
type NudgeFactory func() NudgeExtension

type NudgeExtension interface {
	Middleware() adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage]
	BindLeg(context.Context, NudgeLegHost) context.Context
}

// NudgeLegHost binds the current Run and leg; policy never chooses an owner.
type NudgeLegHost interface {
	Outcomes(callIDs []string) ([]loopexp.ToolSample, error)
	Schedule(context.Context, loopexp.NudgeNotice) error
}
