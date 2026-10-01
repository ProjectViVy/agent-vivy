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

	"agent-vivy/internal/app"
	"agent-vivy/internal/config"
	controlrpc "agent-vivy/internal/rpc"
)

// DefaultQueueCapacity bounds buffered notifications (DN-0: 10_000).
// DefaultPollLimit caps one Poll drain (DN-0: 500).
const (
	DefaultQueueCapacity = 10_000
	DefaultPollLimit     = 500
)

// ABIVersion is the single source of truth for the DIVA C-ABI contract
// version. cmd/vivy-shared/vivy_abi.h must define VIVY_ABI_VERSION to the
// same value (checked by exports_test).
const ABIVersion = 1

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

// Host is the single embedded lifecycle owner.
type Host struct {
	app    *app.App
	peer   *controlrpc.Peer
	cancel context.CancelFunc

	events chan Notification
	gap    atomic.Bool

	mu     sync.Mutex
	closed bool
	close  sync.Once
	err    error
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
	host := &Host{app: a, events: make(chan Notification, capacity)}
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
			return out, nil
		}
	}
	out.Gap = h.gap.Swap(false)
	return out, nil
}

// Close ends the control peer and the app composition, bounded by the app's
// shutdown grace. Safe to call more than once and concurrent with Call/Poll.
func (h *Host) Close() error {
	h.close.Do(func() {
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
		h.cancel()
		if h.peer != nil {
			_ = h.peer.Close()
		}
		h.err = h.app.Close()
	})
	return h.err
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
	}
	return nil, nil
}
