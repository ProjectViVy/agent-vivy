package channelhost

// Live-surface lifecycle coverage for the placeholder/reaction half
// (contract §1/§12, 2026-09-15): an accepted turn announces itself, any
// terminal (or StopAll, or the TTL) deletes the placeholder and withdraws
// the ack, and none of it ever enters the delivery ledger.

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/channelhost/fake"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	plugin "agent-vivy/sdk/port/channel"
)

// liveChannel is a fake.Channel with counting placeholder / delete /
// reaction faces. Placeholder deletes are the only deletes it counts, so
// assertions read the live-surface cleanup directly.
type liveChannel struct {
	*fake.Channel
	placeholders atomic.Int64
	reacts       atomic.Int64
	removes      atomic.Int64
	phDeletes    atomic.Int64
}

// Placeholder implements plugin.Placeholder for the stub.
func (c *liveChannel) Placeholder(_ context.Context, _ string) (string, error) {
	return fmt.Sprintf("ph-%d", c.placeholders.Add(1)), nil
}

// DeleteMessage implements plugin.MessageDeleter for the stub.
func (c *liveChannel) DeleteMessage(_ context.Context, _ string, messageID string) error {
	if strings.HasPrefix(messageID, "ph-") {
		c.phDeletes.Add(1)
	}
	return nil
}

// React implements plugin.ReactionSender for the stub.
func (c *liveChannel) React(_ context.Context, _ string, _ string, _ string) (string, error) {
	return fmt.Sprintf("re-%d", c.reacts.Add(1)), nil
}

// RemoveReaction implements plugin.ReactionRemover for the stub.
func (c *liveChannel) RemoveReaction(_ context.Context, _ string, _ string, _ string) error {
	c.removes.Add(1)
	return nil
}

// newLiveHost wires a started host over a real sqlite backend with the
// live-capable stub channel (reuses the typing harness wiring).
func newLiveHost(t *testing.T, ch plugin.Channel) (*Host, *sqlite.Backend) {
	t.Helper()
	return newTypingHost(t, ch)
}

// shrinkLiveTTL fast-forwards the leak backstop for the test and restores
// the production constant on cleanup.
func shrinkLiveTTL(t *testing.T, d time.Duration) {
	t.Helper()
	old := liveTTL
	liveTTL = d
	t.Cleanup(func() { liveTTL = old })
}

// waitForLive polls until cond holds; the live goroutines are asynchronous
// by design.
func waitForLive(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s never happened", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestLivePlaceholderAndAckDeletedAtCompleted: the accepted turn gets a
// placeholder and an ack reaction; the completed terminal deletes and
// withdraws them, and the reply is still a fresh delivery. The ledger
// carries exactly the one reply intent — never the live surface.
func TestLivePlaceholderAndAckDeletedAtCompleted(t *testing.T) {
	ch := &liveChannel{Channel: fake.New()}
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host, backend := newLiveHost(t, ch)

	publishHello(t, host, ch, "m-1")
	waitForLive(t, "placeholder send", func() bool { return ch.placeholders.Load() == 1 })
	waitForLive(t, "ack reaction", func() bool { return ch.reacts.Load() == 1 })

	// The live surface rides no ledger row: exactly the one armed reply
	// intent exists.
	if got := len(openIntent(t, backend)); got != 1 {
		t.Fatalf("open intents = %d, want exactly the armed reply intent", got)
	}

	seedReply(t, backend, ChannelSessionID("fake", "chat-1", ""), "run-typing")
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: "run-typing", Type: domain.EventRunCompleted,
	})
	waitForLive(t, "placeholder delete", func() bool { return ch.phDeletes.Load() == 1 })
	waitForLive(t, "ack withdraw", func() bool { return ch.removes.Load() == 1 })
	waitForLive(t, "reply delivery", func() bool { return len(ch.Snapshot()) == 1 })
	if got := len(openIntent(t, backend)); got != 0 {
		t.Fatalf("open intents = %d, want none after delivery", got)
	}
	host.StopAll(context.Background())
}

// TestLiveSettlesAtFailed: a failed run deletes the placeholder and
// withdraws the ack without delivering anything.
func TestLiveSettlesAtFailed(t *testing.T) {
	ch := &liveChannel{Channel: fake.New()}
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host, backend := newLiveHost(t, ch)

	publishHello(t, host, ch, "m-1")
	waitForLive(t, "placeholder send", func() bool { return ch.placeholders.Load() == 1 })

	journalTerminal(t, backend, "run-typing", domain.EventRunFailed)
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: "run-typing", Type: domain.EventRunFailed,
	})
	waitForLive(t, "placeholder delete", func() bool { return ch.phDeletes.Load() == 1 })
	waitForLive(t, "ack withdraw", func() bool { return ch.removes.Load() == 1 })
	if got := len(ch.Snapshot()); got != 0 {
		t.Fatalf("sends = %d, want none for a failed run", got)
	}
	host.StopAll(context.Background())
}

// TestLiveSettlesAtCancelled: same convergence on cancellation.
func TestLiveSettlesAtCancelled(t *testing.T) {
	ch := &liveChannel{Channel: fake.New()}
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host, backend := newLiveHost(t, ch)

	publishHello(t, host, ch, "m-1")
	waitForLive(t, "placeholder send", func() bool { return ch.placeholders.Load() == 1 })

	journalTerminal(t, backend, "run-typing", domain.EventRunCancelled)
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: "run-typing", Type: domain.EventRunCancelled,
	})
	waitForLive(t, "placeholder delete", func() bool { return ch.phDeletes.Load() == 1 })
	host.StopAll(context.Background())
}

// TestLiveSurvivesApprovalWait: an approval-required event is not a
// terminal — the placeholder and ack stay through the wait, then the
// completed terminal settles them.
func TestLiveSurvivesApprovalWait(t *testing.T) {
	ch := &liveChannel{Channel: fake.New()}
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host, backend := newLiveHost(t, ch)

	publishHello(t, host, ch, "m-1")
	waitForLive(t, "placeholder send", func() bool { return ch.placeholders.Load() == 1 })

	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: "run-typing", Type: domain.EventToolApprovalRequired,
		Payload: []byte(`{"approval_id":"ap_1"}`),
	})
	time.Sleep(150 * time.Millisecond)
	if got := ch.phDeletes.Load(); got != 0 {
		t.Fatalf("placeholder deletes after approval-required = %d, want none", got)
	}
	if got := ch.removes.Load(); got != 0 {
		t.Fatalf("ack withdrawals after approval-required = %d, want none", got)
	}

	seedReply(t, backend, ChannelSessionID("fake", "chat-1", ""), "run-typing")
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: "run-typing", Type: domain.EventRunCompleted,
	})
	waitForLive(t, "placeholder delete", func() bool { return ch.phDeletes.Load() == 1 })
	host.StopAll(context.Background())
}

// TestLiveStopAllSweeps: shutdown deletes every live placeholder and
// withdraws every ack even though no run reached a terminal.
func TestLiveStopAllSweeps(t *testing.T) {
	ch := &liveChannel{Channel: fake.New()}
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host, _ := newLiveHost(t, ch)

	publishHello(t, host, ch, "m-1")
	waitForLive(t, "placeholder send", func() bool { return ch.placeholders.Load() == 1 })

	host.StopAll(context.Background())
	waitForLive(t, "placeholder delete", func() bool { return ch.phDeletes.Load() == 1 })
	waitForLive(t, "ack withdraw", func() bool { return ch.removes.Load() == 1 })
}

// TestLiveTTLEndsTheOrphan: a run that never reaches a terminal still has
// its placeholder deleted and its ack withdrawn when the TTL fires.
func TestLiveTTLEndsTheOrphan(t *testing.T) {
	shrinkLiveTTL(t, 60*time.Millisecond)
	ch := &liveChannel{Channel: fake.New()}
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host, _ := newLiveHost(t, ch)

	publishHello(t, host, ch, "m-1")
	waitForLive(t, "placeholder send", func() bool { return ch.placeholders.Load() == 1 })

	waitForLive(t, "TTL placeholder delete", func() bool { return ch.phDeletes.Load() == 1 })
	waitForLive(t, "TTL ack withdraw", func() bool { return ch.removes.Load() == 1 })
	host.StopAll(context.Background())
}

// TestNoLiveFacesStartNothing: a channel without the faces gets no live
// entry at all; delivery works as before.
func TestNoLiveFacesStartNothing(t *testing.T) {
	ch := fake.New()
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host, backend := newLiveHost(t, ch)

	publishHello(t, host, ch, "m-1")

	host.mu.Lock()
	target, tracked := host.targets["run-typing"]
	host.mu.Unlock()
	if !tracked {
		t.Fatal("run target never registered")
	}
	if target.live != nil {
		t.Fatal("a channel without live faces must not get a live entry")
	}

	seedReply(t, backend, ChannelSessionID("fake", "chat-1", ""), "run-typing")
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: "run-typing", Type: domain.EventRunCompleted,
	})
	waitForLive(t, "reply delivery", func() bool { return len(ch.Snapshot()) == 1 })
	host.StopAll(context.Background())
}
