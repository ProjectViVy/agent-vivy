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

// Deps wires the host. Journal, Messages and Sessions are the organism's
// durable stores; Channels is the generated Assembly's Channel set;
// Config is the kernel-owned channels envelope.
type Deps struct {
	Journal  storage.Journal
	Messages storage.MessageStore
	Sessions storage.SessionStore
	// Run starts one run per accepted inbound turn. Nil Deps.Run makes
	// StartAll fail closed.
	Run      RunFunc
	Channels []plugin.Channel
	Config   config.Channels
	// Logger receives structured host logs. Inbound content is never
	// logged; sender ids and chat ids are identifiers, not content.
	Logger *slog.Logger
}
