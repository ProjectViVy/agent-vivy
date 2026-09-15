package channelhost

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"agent-vivy/internal/domain"
	plugin "agent-vivy/sdk/port/channel"
)

// outboundDeliveryTimeout bounds one outbound delivery attempt sequence.
// Send runs on a detached goroutine, so a slow platform call never stalls
// the runtime event loop; the bound keeps a wedged adapter from leaking
// goroutines.
const outboundDeliveryTimeout = 30 * time.Second

// outboundDeliveryAttempts bounds one delivery intent's send attempts,
// including redeliveries carried across restarts through the durable row
// (pending.attempts). Exhaustion marks the row failed instead of retrying
// forever against a dead channel.
const outboundDeliveryAttempts = 3

// outboundDeliveryRetryDelay is the pause between delivery attempts. It is
// a var so lifecycle tests can shrink it.
var outboundDeliveryRetryDelay = 2 * time.Second

// outboundTarget remembers where a channel run's reply must land.
type outboundTarget struct {
	runID       domain.RunID
	sessionID   domain.SessionID
	chatID      string
	topicID     string
	channelName string
	ch          plugin.Channel
	// msgID is the triggering inbound message id (tier-1 reply threading):
	// sendReply quotes it on the first chunk via OutboundMessage.ReplyTo.
	// In-process only — the durable intent row carries no message id, so a
	// restart-recovered redelivery sends unthreaded (persisting it would
	// need a channel_deliveries migration for a cosmetic header).
	msgID string
	// maxRunes is the adapter's outbound text bound (plugin.RunesLimiter,
	// CH-C4-N1); 0 = the adapter declares no limit and gets whole messages.
	maxRunes int
	// createdAtMs is the durable intent's creation stamp, carried through
	// state transitions so the row's arrival order survives updates.
	createdAtMs int64
	// stopTyping closes to end the run's live-surface typing loop; nil when
	// the adapter has no typing face. Closed under mu (terminal handler or
	// StopAll), read by the loop goroutine.
	stopTyping chan struct{}
	// live is the placeholder/reaction half of the run's live surface; nil
	// when the adapter has neither face. Its goroutine settles itself when
	// closeLive fires (terminal handler or StopAll) or the TTL fires.
	live *liveSurface
}

// name returns the durable channel identity even when no live plugin object is
// available. Recovery and persistence must never depend on dereferencing ch.
func (t outboundTarget) name() string {
	if t.channelName != "" {
		return t.channelName
	}
	if t.ch != nil {
		return t.ch.Name()
	}
	return ""
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
	// notes records the per-channel skip/fail reason of the last StartAll
	// ("no config envelope", "disabled", "empty allow_from — start
	// refused", "start failed: <err>"); empty means started. Guarded by mu.
	notes   map[string]string
	targets map[domain.RunID]outboundTarget
	// draining gates delivery-goroutine spawns once StopAll began: a run
	// completing during shutdown leaves its durable intent pending instead
	// of racing the wait. Guarded by mu.
	draining bool
	// deliveryWG tracks in-flight delivery goroutines so StopAll can join
	// them instead of dropping the Send to process exit.
	deliveryWG sync.WaitGroup
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
		notes:   make(map[string]string),
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
	if h.deps.Deliveries == nil {
		return errors.New("channelhost: durable delivery store is required")
	}
	seen := make(map[string]bool)
	h.mu.Lock()
	h.notes = make(map[string]string)
	h.mu.Unlock()
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
			h.setNote(name, "no config envelope")
			h.logger.Info("channelhost: channel compiled-in but not configured; not started", "channel", name)
			continue
		}
		if !envelope.Enabled {
			h.setNote(name, "disabled")
			h.logger.Info("channelhost: channel disabled by config; not started", "channel", name)
			continue
		}
		if len(envelope.AllowFrom) == 0 {
			h.setNote(name, "empty allow_from — start refused")
			h.logger.Error("channelhost: refusing to start channel with empty allow_from", "channel", name)
			continue
		}
		auditSettingsEnvNames(name, envelope, h.logger)
		if err := ch.Start(ctx, h.envFor(ch)); err != nil {
			h.setNote(name, fmt.Sprintf("start failed: %v", err))
			h.logger.Error("channelhost: channel start failed", "channel", name, "err", err)
			continue
		}
		h.setNote(name, "")
		h.mu.Lock()
		h.started = append(h.started, ch)
		h.mu.Unlock()
		h.logger.Info("channelhost: channel started", "channel", name, "allow_from_count", len(envelope.AllowFrom))
	}
	// Reconcile only after the adapters are live: redeliveries resolve the
	// channel by name and need a started Send. Callers run StartAll after
	// runtime restart recovery, so a reconciled run's terminal state is
	// already settled in the journal.
	h.recoverDeliveries(ctx)
	return nil
}

// StopAll stops the started channels in reverse start order. It first
// stops accepting new delivery goroutines and waits — bounded by the
// caller's deadline — for in-flight Sends to finish, so a graceful
// shutdown no longer drops a reply that was already on the wire. An intent
// that does not drain stays pending in the durable store and is
// redelivered on the next start. It is safe to call on a host that never
// started.
func (h *Host) StopAll(ctx context.Context) {
	h.mu.Lock()
	h.draining = true
	started := append([]plugin.Channel(nil), h.started...)
	h.started = nil
	// Live-surface typing loops die with the host: their runs will never
	// see another terminal. A ping already in flight finishes against its
	// 5s bound; the adapter may already be stopping — the loop ends on the
	// error either way. The placeholder/reaction half settles the same way:
	// the surface goroutines wake, delete the placeholder and withdraw the
	// ack against bounded calls (still before the adapters stop), and the
	// deliveryWG wait below joins them.
	for _, t := range h.targets {
		closeTyping(t)
		closeLive(t)
	}
	h.mu.Unlock()
	done := make(chan struct{})
	go func() {
		h.deliveryWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		h.logger.Warn("channelhost: shutdown deadline reached before deliveries drained; open intents stay durable for the next start")
	}
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

// setNote records the StartAll decision note for one channel. It is safe
// to call with an empty note (started).
func (h *Host) setNote(name, note string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.notes == nil {
		h.notes = make(map[string]string)
	}
	h.notes[name] = note
}

// ChannelStatus is the inspect surface of one compiled-in channel: what
// the generation carries, what the effective envelope configures, what
// the process actually started, and why a channel did not start. It is
// the process truth of the last StartAll — settings writes apply on the
// next restart, never mid-process.
type ChannelStatus struct {
	// Name is the compiled-in channel plugin name.
	Name string
	// Capabilities is the optional ABI surface Discover reported.
	Capabilities Capabilities
	// Configured reports a channels.<name> envelope in the effective
	// config (config.yaml merged with the settings overlay at startup).
	Configured bool
	// Enabled is the effective envelope switch; false when unconfigured.
	Enabled bool
	// AllowFrom is the startup-effective envelope's allowed-sender summary.
	// Sender IDs are not secrets (D-010) and already cross channel/get;
	// carrying them here lets the UI detect a pure allow_from edit as
	// pending-restart. Nil when unconfigured.
	AllowFrom []string
	// Started reports a live adapter in this process.
	Started bool
	// TokenEnv is the envelope's declared token_env name; empty when none.
	// The secret value itself never crosses this surface (D-010).
	TokenEnv string
	// TokenEnvSet reports the env variable non-empty at inspect time.
	TokenEnvSet bool
	// Note is the human-readable skip/fail reason of the last StartAll;
	// empty when the channel started.
	Note string
	// Health is the live probe of a started HealthChecker adapter; nil when
	// the channel is not started or the adapter has no Health face. The
	// StartAll note answers "why did it not start"; this answers "is the
	// running ear actually connected" (CH-R-1).
	Health *ChannelHealth
}

// Inspect reports every compiled-in channel in deterministic name order,
// regardless of configuration: a channel that is compiled-in is visible
// even when it has no envelope yet. Secret values are never included —
// only the env NAME and whether it is set (D-010).
func (h *Host) Inspect() []ChannelStatus {
	h.mu.Lock()
	startedNames := make(map[string]bool, len(h.started))
	for _, ch := range h.started {
		startedNames[ch.Name()] = true
	}
	notes := make(map[string]string, len(h.notes))
	for name, note := range h.notes {
		notes[name] = note
	}
	h.mu.Unlock()

	statuses := make([]ChannelStatus, 0, len(h.deps.Channels))
	for _, ch := range h.deps.Channels {
		if ch == nil {
			continue
		}
		name := ch.Name()
		status := ChannelStatus{
			Name:         name,
			Capabilities: Discover(ch),
			Started:      startedNames[name],
			Note:         notes[name],
		}
		if envelope, ok := h.deps.Config[name]; ok {
			status.Configured = true
			status.Enabled = envelope.Enabled
			status.AllowFrom = envelope.AllowFrom
			status.TokenEnv = envelope.TokenEnv
		}
		if status.TokenEnv != "" && h.deps.Credentials != nil {
			status.TokenEnvSet = h.deps.Credentials.IsSet("vivy/"+name, status.TokenEnv)
		}
		if status.Started {
			status.Health = h.probeHealth(ch)
		}
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Name < statuses[j].Name })
	return statuses
}

// Deliver sends an outbound message to the named channel and target chat ID.
// It resolves the channel adapter, verifies that it is currently started,
// splits the content if the adapter declares a runes ceiling (plugin.RunesLimiter),
// and delivers parts sequentially via ch.Send.
func (h *Host) Deliver(ctx context.Context, channelName, chatID, content string) error {
	if channelName == "" {
		return errors.New("channelhost: channel name is required")
	}
	if chatID == "" {
		return errors.New("channelhost: chat ID is required")
	}
	ch := h.channelByName(channelName)
	if ch == nil {
		return fmt.Errorf("channelhost: channel %q not registered", channelName)
	}

	h.mu.Lock()
	started := false
	for _, running := range h.started {
		if running.Name() == channelName {
			started = true
			break
		}
	}
	note := h.notes[channelName]
	h.mu.Unlock()

	if !started {
		if note != "" {
			return fmt.Errorf("channelhost: channel %q is not running (%s)", channelName, note)
		}
		return fmt.Errorf("channelhost: channel %q is not running", channelName)
	}

	maxRunes := 0
	if rl, ok := ch.(plugin.RunesLimiter); ok {
		maxRunes = rl.MaxMessageRunes()
	}

	chunks := splitRunes(content, maxRunes)
	for _, chunk := range chunks {
		if _, err := ch.Send(ctx, plugin.OutboundMessage{
			ChatID: chatID,
			Parts:  []plugin.Part{{Kind: plugin.PartText, Text: chunk}},
		}); err != nil {
			h.logger.Error("channelhost: delivery to channel failed",
				"channel", channelName, "chat_id", chatID, "err", err)
			return fmt.Errorf("channelhost: send to %s (%s): %w", channelName, chatID, err)
		}
	}

	h.logger.Info("channelhost: delivery to channel completed",
		"channel", channelName, "chat_id", chatID, "chunks", len(chunks))
	return nil
}
