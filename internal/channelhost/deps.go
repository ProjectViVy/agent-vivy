// Package channelhost is the kernel owner of Channel Modules. It starts and
// stops adapters over the focused v1 Channel Port and maps each
// (channel, chat[, topic]) to a deterministic session, journals inbound
// turns as channel.inbound events, and delivers the assistant reply of
// the opened run back to the originating chat.
//
// Layering rules (VIVY-CHANNEL-PACK.md §7):
//   - the package never imports github.com/cloudwego/eino or
//     internal/runtime; the app injects a Run callback from
//     *runtime.Service.RunWithOptions instead.
//   - adapters only ever see plugin.ChannelEnv; they never touch the
//     Journal, storage, or config directly.
package channelhost

import (
	"context"
	"log/slog"

	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	plugin "agent-vivy/sdk/port/channel"
)

// RunFunc opens one run for an inbound turn. The app injects it from
// *runtime.Service.RunWithOptions; the host never holds *runtime.Service,
// and the runtime package is never imported here.
type RunFunc func(ctx context.Context, sessionID domain.SessionID, text string, prov *domain.Provenance) (domain.RunID, error)

// DecideApprovalFunc settles one approval attributed to the named actor.
// The app injects it from *runtime.Service.DecideApprovalAsActor; the host
// only forwards decisions that passed its own scoping (allow-listed sender,
// originating session), and every decision still runs through the kernel's
// single DecideApproval path (contract §12).
type DecideApprovalFunc func(ctx context.Context, approvalID, decision, actor string) error

type CredentialResolver interface {
	Resolve(moduleID, ref string) (string, error)
	IsSet(moduleID, ref string) bool
}

// Deps wires the host. Journal, Messages, Sessions and Deliveries are the
// organism's durable stores; Channels is the generated Assembly's Channel
// set; Config is the kernel-owned channels envelope.
type Deps struct {
	Journal  storage.Journal
	Messages storage.MessageStore
	Sessions storage.SessionStore
	// Deliveries persists the durable outbound reply intents (CH-C3-N1).
	// Nil Deps.Deliveries makes StartAll fail closed: an ear that can lose
	// replies to a restart must not go live.
	Deliveries storage.ChannelDeliveryStore
	// Run starts one run per accepted inbound turn. Nil Deps.Run makes
	// StartAll fail closed.
	Run         RunFunc
	Channels    []plugin.Channel
	Config      config.Channels
	Credentials CredentialResolver
	// Approvals, Runs, and DecideApproval enable the HITL channel surface
	// (contract §12): a pending-approval notification to the originating
	// chat plus the session-scoped /approve, /deny, and /pending commands.
	// All three are optional; if any is nil the notification and the
	// commands are disabled with a start note, never half-enabled.
	Approvals      storage.ApprovalStore
	Runs           storage.RunStore
	DecideApproval DecideApprovalFunc
	// Logger receives structured host logs. Inbound content is never
	// logged; sender ids and chat ids are identifiers, not content.
	Logger *slog.Logger
}
