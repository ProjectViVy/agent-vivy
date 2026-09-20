package channelhost

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"agent-vivy/internal/channelhost/fake"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	plugin "agent-vivy/sdk/port/channel"
)

// typingChannel is a fake.Channel with a counting typing face.
type typingChannel struct {
	*fake.Channel
	pings atomic.Int64
}

// Typing implements plugin.Typing for the stub.
func (c *typingChannel) Typing(_ context.Context, _ string) error {
	c.pings.Add(1)
	return nil
}

// typingRuns hands out one fixed run id and returns immediately: the run
// stays live because only the test sends its terminal event.
func typingRuns(_ context.Context, _ domain.SessionID, _ string, _ []domain.Attachment, _ *domain.Provenance) (domain.RunID, error) {
	return domain.RunID("run-typing"), nil
}

// newTypingHost wires a started host over a real sqlite backend.
func newTypingHost(t *testing.T, ch plugin.Channel) (*Host, *sqlite.Backend) {
	t.Helper()
	backend := openBackend(t)
	host := New(Deps{
		Journal:    backend,
		Messages:   backend,
		Sessions:   backend,
		Deliveries: backend,
		Run:        typingRuns,
		Channels:   []plugin.Channel{ch},
		Config:     config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:     testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	return host, backend
}

// shrinkTypingTempo fast-forwards the loop cadence for the test and
// restores the production constants on cleanup.
func shrinkTypingTempo(t *testing.T) {
	t.Helper()
	oldInterval, oldMax := typingInterval, typingMaxDuration
	typingInterval, typingMaxDuration = 5*time.Millisecond, 10*time.Second
	t.Cleanup(func() { typingInterval, typingMaxDuration = oldInterval, oldMax })
}

// waitPings polls until the ping counter reaches want.
func waitPings(t *testing.T, c *typingChannel, want int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if got := c.pings.Load(); got >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("typing pings = %d, want >= %d", c.pings.Load(), want)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// pingsSettled polls until the ping count stops growing across a full
// interval window and returns the final count.
func pingsSettled(t *testing.T, c *typingChannel) int64 {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	last := c.pings.Load()
	for {
		time.Sleep(30 * time.Millisecond)
		now := c.pings.Load()
		if now == last {
			return now
		}
		last = now
		if time.Now().After(deadline) {
			t.Fatalf("typing pings never settled (still at %d)", now)
		}
	}
}

// TestTypingRunsForTheTurnAndStopsAtCompleted: the indicator starts with
// the accepted run, keeps resending while the run is live, and the
// completed terminal — not the cap, not a timeout — ends it. The reply
// delivery is unaffected.
func TestTypingRunsForTheTurnAndStopsAtCompleted(t *testing.T) {
	shrinkTypingTempo(t)
	ch := &typingChannel{Channel: fake.New()}
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host, backend := newTypingHost(t, ch)

	publishHello(t, host, ch, "m-1")
	waitPings(t, ch, 2) // the immediate ping plus at least one resend

	seedReply(t, backend, ChannelSessionID("fake", "chat-1", ""), "run-typing")
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: "run-typing", Type: domain.EventRunCompleted,
	})
	settled := pingsSettled(t, ch)
	if settled < 2 {
		t.Fatalf("typing pings settled at %d, want >= 2 before the terminal", settled)
	}
	host.StopAll(context.Background())

	snapshot := ch.Snapshot()
	if len(snapshot) != 1 || len(snapshot[0].Parts) != 1 || snapshot[0].Parts[0].Text != "channel reply" {
		t.Fatalf("reply = %+v, want one 'channel reply' part", snapshot)
	}
}

// TestTypingStopsAtFailedTerminal: a failed run settles the intent without
// a reply and still ends the indicator.
func TestTypingStopsAtFailedTerminal(t *testing.T) {
	shrinkTypingTempo(t)
	ch := &typingChannel{Channel: fake.New()}
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host, backend := newTypingHost(t, ch)

	publishHello(t, host, ch, "m-1")
	waitPings(t, ch, 2)

	journalTerminal(t, backend, "run-typing", domain.EventRunFailed)
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: "run-typing", Type: domain.EventRunFailed,
	})
	pingsSettled(t, ch)
	host.StopAll(context.Background())

	if got := len(ch.Snapshot()); got != 0 {
		t.Fatalf("sends = %d, want none for a failed run", got)
	}
}

// TestTypingStopsAtStopAll: shutdown ends every live indicator even though
// the run never reached a terminal.
func TestTypingStopsAtStopAll(t *testing.T) {
	shrinkTypingTempo(t)
	ch := &typingChannel{Channel: fake.New()}
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host, _ := newTypingHost(t, ch)

	publishHello(t, host, ch, "m-1")
	waitPings(t, ch, 2)

	host.StopAll(context.Background())
	if settled := pingsSettled(t, ch); settled < 2 {
		t.Fatalf("typing pings settled at %d, want >= 2 before StopAll", settled)
	}
}

// TestTypingCapEndsTheLoop: the 5-minute cap (shrunk here) ends the loop
// on its own, with no terminal and no StopAll.
func TestTypingCapEndsTheLoop(t *testing.T) {
	shrinkTypingTempo(t)
	typingMaxDuration = 40 * time.Millisecond
	ch := &typingChannel{Channel: fake.New()}
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host, _ := newTypingHost(t, ch)

	publishHello(t, host, ch, "m-1")
	waitPings(t, ch, 1)
	if settled := pingsSettled(t, ch); settled < 1 {
		t.Fatalf("typing pings settled at %d, want >= 1", settled)
	}
	host.StopAll(context.Background())
}

// TestNoTypingFaceStartsNothing: a channel without plugin.Typing gets no
// stop channel on its target and no loop goroutine; delivery works as
// before.
func TestNoTypingFaceStartsNothing(t *testing.T) {
	shrinkTypingTempo(t)
	ch := fake.New()
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host, backend := newTypingHost(t, ch)

	publishHello(t, host, ch, "m-1")

	host.mu.Lock()
	target, tracked := host.targets["run-typing"]
	host.mu.Unlock()
	if !tracked {
		t.Fatal("run target never registered")
	}
	if target.stopTyping != nil {
		t.Fatal("a channel without a typing face must not get a stop channel")
	}

	seedReply(t, backend, ChannelSessionID("fake", "chat-1", ""), "run-typing")
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: "run-typing", Type: domain.EventRunCompleted,
	})
	deadline := time.Now().Add(2 * time.Second)
	for len(ch.Snapshot()) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("reply never delivered; sent = %+v", ch.Snapshot())
		}
		time.Sleep(5 * time.Millisecond)
	}
	host.StopAll(context.Background())
}
