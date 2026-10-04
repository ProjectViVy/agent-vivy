// Package embedded owns one process-resident VIVY composition for a native
// host (agent-diva's thin shell via the C ABI in cmd/vivy-shared). It composes
// the app gateway-less, dials the in-process control plane, and owns the
// lifecycle services App.Run would normally own (interaction sweeper, cron)
// plus the notification queue the embedder drains with Poll.
//
// Ownership rules (DN-0 ABI freeze):
//   - one Host per process; Open callers hold the only pointer
//   - Close is the sole shutdown path and is idempotent; it bounds the whole
//     teardown by the app's shutdownGrace
//   - the notification queue is bounded; overflow drops the oldest event and
//     latches a gap flag the next Poll reports once
//   - the host never blocks RPC dispatch on the queue: a full queue sheds
//     events, it never stalls Peer.NotifyContext
package embedded

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"agent-vivy/internal/app"
	"agent-vivy/internal/config"
	"agent-vivy/internal/embedded/abi"
	controlrpc "agent-vivy/internal/rpc"
)

// DefaultQueueCapacity bounds buffered notifications (DN-0: 10_000).
// DefaultPollLimit caps one Poll drain (DN-0: 500).
const (
	DefaultQueueCapacity = 10_000
	DefaultPollLimit     = 500
)

// ABIVersion re-exports the leaf-owned contract version in
// internal/embedded/abi so existing callers keep their spelling.
const ABIVersion = abi.Version

// ErrClosed is returned by Call/Poll once the host has shut down.
var ErrClosed = errors.New("embedded: host is closed")

// Notification is one server-initiated control-plane message (for example a
// run/event after run/subscribe), exactly as the control peer delivered it.
type Notification struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// PollResult is one drain of the notification queue.
type PollResult struct {
	Events []Notification `json:"events"`
	// Gap is true when events were dropped due to queue overflow since the
	// last reported gap. Sticky: it stays true until a Poll returns it.
	Gap bool `json:"gap"`
}

// Options tunes Open. Zero values take the DN-0 frozen defaults.
type Options struct {
	// QueueCapacity bounds buffered notifications. 0 = DefaultQueueCapacity.
	QueueCapacity int
	// AppOptions are forwarded to app.New. The host always disables the
	// gateway; callers decide WithoutEars from the recipe's channel contract.
	AppOptions []app.AppOption
}

// closePeerJoinGrace bounds joining the control peer's Serve pump during
// teardown; a peer that cannot unwind in time is reported, not waited on
// forever.
const closePeerJoinGrace = 5 * time.Second

// Host is the single embedded lifecycle owner.
type Host struct {
	app    *app.App
	peer   *controlrpc.Peer
	cancel context.CancelFunc

	events chan Notification
	notify chan struct{}
	gap    atomic.Bool

	mu         sync.Mutex
	closed     bool
	closeCh    chan struct{}
	closeOnce  sync.Once
	closeDone  chan struct{}
	closeStage atomic.Value // string: component currently tearing down
	err        error

	// teardownHook runs at the start of teardown while the close stage is
	// "control peer". Test seam: lets deadline tests hold the teardown open.
	teardownHook func()
}

// Open composes the app (gateway-less by construction), starts the lifecycle
// services, and dials the control plane. The host's lifetime is decoupled
// from ctx: Close is the sole shutdown path (the caller's ctx must not
// silently kill an in-flight embedded runtime).
func Open(ctx context.Context, cfg config.Config, o Options) (*Host, error) {
	capacity := o.QueueCapacity
	if capacity <= 0 {
		capacity = DefaultQueueCapacity
	}
	appOpts := append([]app.AppOption{app.WithoutGateway()}, o.AppOptions...)
	a, err := app.New(ctx, cfg, appOpts...)
	if err != nil {
		return nil, fmt.Errorf("embedded: compose app: %w", err)
	}
	host := &Host{
		app:       a,
		events:    make(chan Notification, capacity),
		notify:    make(chan struct{}, 1),
		closeCh:   make(chan struct{}),
		closeDone: make(chan struct{}),
	}
	hostCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	host.cancel = cancel
	peer, err := a.DialControl(hostCtx, host)
	if err != nil {
		cancel()
		_ = a.Close()
		return nil, fmt.Errorf("embedded: dial control: %w", err)
	}
	host.peer = peer
	a.StartEmbeddedServices()
	return host, nil
}

// Call issues one control-plane request and returns the raw result. Errors
// from the peer (including *rpc.Error) are returned unchanged so the
// envelope conversion stays in the ABI layer.
func (h *Host) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if h.isClosed() {
		return nil, ErrClosed
	}
	result, err := h.peer.Call(ctx, method, params)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Poll drains up to limit buffered notifications (≤0 or >DefaultPollLimit
// clamps to DefaultPollLimit). It never blocks: an empty queue returns an
// empty slice immediately.
func (h *Host) Poll(_ context.Context, limit int) (PollResult, error) {
	if h.isClosed() {
		return PollResult{}, ErrClosed
	}
	return h.drain(limit), nil
}

// Next blocks until at least one notification or a sticky gap is pending,
// the host closes, or ctx is done; it then drains up to limit events
// (≤0 or >DefaultPollLimit clamps to DefaultPollLimit). Events still
// buffered at close are delivered before ErrClosed is reported.
func (h *Host) Next(ctx context.Context, limit int) (PollResult, error) {
	for {
		out := h.drain(limit)
		if len(out.Events) > 0 || out.Gap {
			return out, nil
		}
		if h.isClosed() {
			return PollResult{}, ErrClosed
		}
		select {
		case <-ctx.Done():
			return PollResult{}, ctx.Err()
		case <-h.notify:
		case <-h.closeCh:
		}
	}
}

// drain dequeues up to limit buffered notifications without blocking and
// collects the sticky gap flag.
func (h *Host) drain(limit int) PollResult {
	if limit <= 0 || limit > DefaultPollLimit {
		limit = DefaultPollLimit
	}
	out := PollResult{Events: make([]Notification, 0, limit)}
	for len(out.Events) < limit {
		select {
		case n := <-h.events:
			out.Events = append(out.Events, n)
		default:
			out.Gap = h.gap.Swap(false)
			return out
		}
	}
	out.Gap = h.gap.Swap(false)
	return out
}

// Close ends the control peer and the app composition, bounded by the app's
// shutdown grace. Safe to call more than once and concurrent with
// Call/Poll/Next.
func (h *Host) Close() error {
	h.startClose()
	<-h.closeDone
	return h.err
}

// CloseContext is Close with a caller deadline: on ctx expiry it reports
// which component is still unwinding instead of blocking, while teardown
// continues in the background and later Close calls can still observe its
// completion.
func (h *Host) CloseContext(ctx context.Context) error {
	h.startClose()
	select {
	case <-h.closeDone:
		return h.err
	case <-ctx.Done():
		return fmt.Errorf("embedded: close timed out during %s: %w", h.currentCloseStage(), ctx.Err())
	}
}

func (h *Host) startClose() {
	h.closeOnce.Do(func() {
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
		close(h.closeCh)
		h.closeStage.Store("control peer")
		go h.teardown()
	})
}

// teardown runs once on its own goroutine so CloseContext callers can
// abandon the wait without stranding an unbounded shutdown: callers are
// cancelled first (peer close drains pending waiters), the Serve pump is
// joined, and only then the app composition closes its stores.
func (h *Host) teardown() {
	defer close(h.closeDone)
	if h.teardownHook != nil {
		h.teardownHook()
	}
	if h.cancel != nil {
		h.cancel()
	}
	if h.peer != nil {
		_ = h.peer.Close()
		select {
		case <-h.peer.ServeDone():
		case <-time.After(closePeerJoinGrace):
			h.err = errors.Join(h.err, errors.New("embedded: control peer serve loop did not join"))
		}
	}
	h.closeStage.Store("app")
	h.err = errors.Join(h.err, h.app.Close())
}

func (h *Host) currentCloseStage() string {
	if stage, ok := h.closeStage.Load().(string); ok {
		return stage
	}
	return "startup"
}

func (h *Host) isClosed() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closed
}

// Handle is the control-peer notification handler: server-initiated methods
// land in the bounded queue; overflow drops the oldest event and latches the
// gap flag rather than blocking the dispatch path.
func (h *Host) Handle(_ context.Context, _ *controlrpc.Peer, request controlrpc.Request) (any, *controlrpc.Error) {
	n := Notification{Method: request.Method, Params: request.Params}
	select {
	case h.events <- n:
		h.wake()
	default:
		select {
		case <-h.events:
		default:
		}
		select {
		case h.events <- n:
		default:
		}
		h.gap.Store(true)
		h.wake()
	}
	return nil, nil
}

// wake signals a blocked Next that an event or gap landed; it never blocks
// the producer.
func (h *Host) wake() {
	select {
	case h.notify <- struct{}{}:
	default:
	}
}
