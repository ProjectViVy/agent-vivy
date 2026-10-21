package runtime

import (
	"agent-vivy/internal/domain"
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

// WarmingFactory and SteeringFactory remain lazy metadata until #93. Core
// owns the single working warmer and queue; no Host is constructed by this seam.
type WarmingFactory func(MaintenanceHost) ModelSettlement

type ModelSettlement interface {
	Settled(context.Context, ObservedModelInput) error
}

// These opaque handles can only be populated by the bound runtime Host. A
// later adapter cannot supply a different prefix, Run, model or credential.
type ObservedModelInput struct{ input modelCallInput }
type ReusablePrefix struct {
	runID           domain.RunID
	callID          string
	messages        []*schema.AgenticMessage
	tools           []*schema.ToolInfo
	estimatedTokens int
}

type MaintenanceResult struct {
	CallID, Status, Reason                                         string
	PromptTokens, CompletionTokens, CachedTokens, CacheWriteTokens int
}
type MaintenanceDiagnostic struct {
	MaintenanceResult
	EstimatedPrefixTokens int
}

type MaintenanceHost interface {
	ReusablePrefix(ObservedModelInput) (ReusablePrefix, error)
	PolicyInput(ReusablePrefix) loopexp.WarmingInput
	GenerateMaintenance(context.Context, ReusablePrefix, loopexp.WarmingRequest) (MaintenanceResult, error)
	Diagnostic(context.Context, MaintenanceDiagnostic)
}

type SteeringFactory func(SteeringHost) SteeringController

type SteeringController interface {
	Steer(context.Context, domain.QueuedTurn) (domain.QueuedTurn, error)
	SetMode(context.Context, loopexp.QueueMode) error
	OnBoundary(context.Context) (handled bool, err error)
}

type SteeringSnapshot struct {
	Pending     []domain.QueuedTurn
	Mode        loopexp.QueueMode
	Eligibility loopexp.SteeringEligibility
}

type HistoryModifier func(context.Context, []*schema.AgenticMessage) []*schema.AgenticMessage

type SteeringHost interface {
	Snapshot(context.Context, domain.QueuedTurn) (SteeringSnapshot, error)
	Enqueue(context.Context, domain.QueuedTurn, loopexp.QueueTrack) (domain.QueuedTurn, error)
	SetMode(context.Context, loopexp.QueueMode) error
	RequestCancelAfterModel(context.Context, string) (domain.QueuedTurn, error)
	ContinueBoundary(context.Context, []string, HistoryModifier) error
}
