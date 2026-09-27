package channelhost

import (
	"context"
	"testing"
	"time"

	"agent-vivy/internal/channelhost/fake"
	"agent-vivy/internal/domain"
	plugin "agent-vivy/sdk/port/channel"
)

// runesChannel is a fake.Channel with a small outbound bound so one reply
// splits into several chunks.
type runesChannel struct {
	*fake.Channel
	runes int
}

// MaxMessageRunes implements plugin.RunesLimiter.
func (c *runesChannel) MaxMessageRunes() int { return c.runes }

// TestReplyThreadsFirstChunkOnly: the host quotes the triggering message
// on the first chunk of a reply only — later chunks are follow-ups in the
// same thread, not replies of their own.
func TestReplyThreadsFirstChunkOnly(t *testing.T) {
	ch := &runesChannel{Channel: fake.New(), runes: 10}
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	host, backend := newTypingHost(t, ch)

	publishHello(t, host, ch, "m-trigger")
	if err := backend.AppendMessage(context.Background(), domain.Message{
		ID: "msg-run-typing", SessionID: ChannelSessionID("fake", "chat-1", ""), RunID: "run-typing",
		Role: domain.RoleAssistant, CreatedAt: time.Now().UnixMilli(),
		Content: "0123456789abcdefghijklmnopqrstuvwxyz", // 36 runes -> 4 chunks of 10
	}); err != nil {
		t.Fatalf("seed assistant message: %v", err)
	}
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: "run-typing", Type: domain.EventRunCompleted,
	})

	deadline := time.Now().Add(2 * time.Second)
	for len(ch.Snapshot()) < 4 {
		if time.Now().After(deadline) {
			t.Fatalf("sends = %d, want 4 chunks", len(ch.Snapshot()))
		}
		time.Sleep(5 * time.Millisecond)
	}
	sent := ch.Snapshot()
	for i, msg := range sent {
		want := ""
		if i == 0 {
			want = "m-trigger"
		}
		if msg.ReplyTo != want {
			t.Fatalf("chunk %d ReplyTo = %q, want %q", i, msg.ReplyTo, want)
		}
	}
	host.StopAll(context.Background())
}
