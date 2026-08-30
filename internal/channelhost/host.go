package channelhost

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/sdk/plugin"
)

// outboundDeliveryTimeout bounds one outbound assistant delivery. Send
// runs on a detached goroutine, so a slow platform call never stalls the
// runtime event loop; the bound keeps a wedged adapter from leaking
// goroutines.
const outboundDeliveryTimeout = 30 * time.Second

// outboundTarget remembers where a channel run's reply must land.
type outboundTarget struct {
	sessionID domain.SessionID
	chatID    string
	topicID   string
	ch        plugin.Channel
}

// Host owns the started channel adapters and the inbound/outbound
// pipelines. It satisfies runtime.RunHook structurally (OnRunEvent) —
// the package never imports internal/runtime.
type Host struct {
	deps   Deps
	logger *slog.Logger
	client *http.Client
	media  plugin.MediaStore

	mu      sync.Mutex
	started []plugin.Channel // start order; StopAll walks it in reverse
	targets map[domain.RunID]outboundTarget
}

// New builds a Host over its dependencies. Call StartAll once startup
// recovery has settled the run store.
func New(deps Deps) *Host {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &Host{
		deps:    deps,
		logger:  deps.Logger,
		client:  &http.Client{Timeout: 30 * time.Second},
		media:   noopMediaStore{},
		targets: make(map[domain.RunID]outboundTarget),
	}
}

// StartAll starts every configured, enabled channel. The decision per
// channel is fail-closed (VIVY-CHANNEL-PACK.md §7):
//   - no channels.<name> envelope: compiled-in but not configured, skipped;
//   - enabled=false: skipped with a log;
//   - enabled=true with empty allow_from: NOT started, error logged —
//     an ear with no sender allow-list never goes live;
//   - plugin.Start failure: logged, the channel is not recorded as
//     started, the rest of the organism goes on.
//
// StartAll only returns an error for host-level wiring problems (nil
// stores, duplicate names) — those abort the whole startup in app.
func (h *Host) StartAll(ctx context.Context) error {
	if h.deps.Journal == nil || h.deps.Messages == nil || h.deps.Sessions == nil {
		return errors.New("channelhost: journal, messages, and sessions stores are required")
	}
	if h.deps.Run == nil {
		return errors.New("channelhost: run callback is required")
	}
	seen := make(map[string]bool)
	for _, ch := range h.deps.Channels {
		if ch == nil {
			continue
		}
		name := ch.Name()
		if name == "" {
			return errors.New("channelhost: channel plugin with empty name")
		}
		if seen[name] {
			return fmt.Errorf("channelhost: duplicate channel plugin name %q", name)
		}
		seen[name] = true
		envelope, ok := h.deps.Config[name]
		if !ok {
			h.logger.Info("channelhost: channel compiled-in but not configured; not started", "channel", name)
			continue
		}
		if !envelope.Enabled {
			h.logger.Info("channelhost: channel disabled by config; not started", "channel", name)
			continue
		}
		if len(envelope.AllowFrom) == 0 {
			h.logger.Error("channelhost: refusing to start channel with empty allow_from", "channel", name)
			continue
		}
		if err := ch.Start(ctx, h.envFor(ch)); err != nil {
			h.logger.Error("channelhost: channel start failed", "channel", name, "err", err)
			continue
		}
		h.mu.Lock()
		h.started = append(h.started, ch)
		h.mu.Unlock()
		h.logger.Info("channelhost: channel started", "channel", name, "allow_from_count", len(envelope.AllowFrom))
	}
	return nil
}

// StopAll stops the started channels in reverse start order. It is safe
// to call on a host that never started.
func (h *Host) StopAll(ctx context.Context) {
	h.mu.Lock()
	started := append([]plugin.Channel(nil), h.started...)
	h.started = nil
	h.mu.Unlock()
	for i := len(started) - 1; i >= 0; i-- {
		ch := started[i]
		if err := ch.Stop(ctx); err != nil {
			h.logger.Warn("channelhost: channel stop failed", "channel", ch.Name(), "err", err)
		}
	}
}

// Started lists the names of currently started channels, in start order.
func (h *Host) Started() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.started))
	for _, ch := range h.started {
		out = append(out, ch.Name())
	}
	return out
}
