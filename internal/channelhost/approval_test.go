package channelhost

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/channelhost/fake"
	"agent-vivy/internal/config"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage/sqlite"
	plugin "agent-vivy/sdk/port/channel"
)

// approvalDecision records one forwarded DecideApproval call.
type approvalDecision struct {
	approvalID string
	decision   string
	actor      string
}

type decisionRecorder struct {
	mu    sync.Mutex
	calls []approvalDecision
}

func (d *decisionRecorder) decide(_ context.Context, approvalID, decision, actor string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, approvalDecision{approvalID, decision, actor})
	return nil
}

func (d *decisionRecorder) snapshot() []approvalDecision {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]approvalDecision(nil), d.calls...)
}

// newApprovalHost wires a host with the HITL surface enabled and a real
// sqlite store behind Approvals/Runs, like the app assembly does.
func newApprovalHost(t *testing.T, backend *sqlite.Backend, runs *runRecorder, ch plugin.Channel, decisions *decisionRecorder) *Host {
	t.Helper()
	host := New(Deps{
		Journal:        backend,
		Messages:       backend,
		Sessions:       backend,
		Deliveries:     backend,
		Run:            runs.run,
		Approvals:      backend,
		Runs:           backend,
		DecideApproval: decisions.decide,
		Channels:       []plugin.Channel{ch},
		Config:         config.Channels{"fake": {Enabled: true, AllowFrom: []string{"alice"}}},
		Logger:         testLogger(),
	})
	if err := host.StartAll(context.Background()); err != nil {
		t.Fatalf("start all: %v", err)
	}
	return host
}

// seedRunAndApproval writes one run row and one pending approval for it —
// the durable shape a suspended run leaves behind.
func seedRunAndApproval(t *testing.T, backend *sqlite.Backend, runID domain.RunID, sessionID domain.SessionID, approvalID, toolName string) {
	t.Helper()
	if err := backend.CreateRun(context.Background(), domain.Run{
		ID: runID, SessionID: sessionID, Status: domain.RunActive, CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := backend.CreateApproval(context.Background(), domain.Approval{
		ID: approvalID, RunID: runID, ToolCallID: "call-1", ToolName: toolName,
		Decision: domain.ApprovalPending, ExpiresAt: time.Now().Add(10 * time.Minute).UnixMilli(),
		CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("create approval: %v", err)
	}
}

// waitSentText polls the fake channel's snapshot until a send whose text
// satisfies want arrives.
func waitSentText(t *testing.T, ch *fake.Channel, what string, want func(string) bool) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		for _, msg := range ch.Snapshot() {
			if len(msg.Parts) > 0 && want(msg.Parts[0].Text) {
				return msg.Parts[0].Text
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s never arrived; sent = %+v", what, ch.Snapshot())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestApprovalRequiredNotifiesWithoutConsumingTarget: the suspension
// notification reaches the originating chat, and the later terminal still
// delivers the reply — the notification consumed nothing.
func TestApprovalRequiredNotifiesWithoutConsumingTarget(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	ch := fake.New()
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	decisions := &decisionRecorder{}
	host := newApprovalHost(t, backend, runs, ch, decisions)

	publishHello(t, host, ch, "m-approval-1")
	call := runs.snapshot()[0]

	payload, err := json.Marshal(approvalNotifyPayload{ApprovalID: "apr_1234567890abcdef", ToolName: "execute"})
	if err != nil {
		t.Fatal(err)
	}
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: call.runID, Type: domain.EventToolApprovalRequired,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1, Payload: payload,
	})
	text := waitSentText(t, ch, "approval notification", func(s string) bool {
		return strings.Contains(s, "等待人工审批") && strings.Contains(s, "execute")
	})
	if !strings.Contains(text, "/approve apr_12345678") || !strings.Contains(text, string(call.sessionID)) {
		t.Fatalf("notification text = %q, want command hint and session pointer", text)
	}

	// The terminal still delivers: the target was read, not consumed.
	seedReply(t, backend, call.sessionID, call.runID)
	host.OnRunEvent(context.Background(), domain.RunEvent{
		RunID: call.runID, Type: domain.EventRunCompleted,
		CreatedAt: time.Now().UnixMilli(), PayloadVersion: 1,
	})
	waitSentText(t, ch, "assistant reply", func(s string) bool { return s == "channel reply" })
	waitOpenState(t, backend, call.runID, "")
}

// TestApprovalCommandDecidesWithinSession: /pending lists only this chat's
// session approvals, /approve forwards the decision attributed to the
// channel sender, and command messages open no run and no delivery intent.
func TestApprovalCommandDecidesWithinSession(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	ch := fake.New()
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	decisions := &decisionRecorder{}
	host := newApprovalHost(t, backend, runs, ch, decisions)

	sessionID := ChannelSessionID("fake", "chat-1", "")
	seedRunAndApproval(t, backend, "run-approval-target", sessionID, "apr_1234567890abcdef", "execute")

	// /pending lists the pending approval.
	env := host.envFor(ch)
	publish := func(text, messageID string) {
		if err := env.PublishInbound(context.Background(), plugin.InboundMessage{
			Channel: "fake", ChatID: "chat-1", Sender: "alice", MessageID: messageID,
			Parts: []plugin.Part{{Kind: plugin.PartText, Text: text}},
		}); err != nil {
			t.Fatalf("publish %q: %v", text, err)
		}
	}
	publish("/pending", "m-cmd-1")
	listText := waitSentText(t, ch, "pending list", func(s string) bool {
		return strings.Contains(s, "待审批")
	})
	if !strings.Contains(listText, "apr_12345678") || !strings.Contains(listText, "execute") {
		t.Fatalf("pending list = %q, want the approval id and tool", listText)
	}

	// /approve with a prefix decides it, attributed to the channel sender.
	publish("/approve apr_12345678", "m-cmd-2")
	waitSentText(t, ch, "approval reply", func(s string) bool {
		return strings.Contains(s, "已批准") && strings.Contains(s, "execute")
	})
	calls := decisions.snapshot()
	if len(calls) != 1 || calls[0].approvalID != "apr_1234567890abcdef" ||
		calls[0].decision != domain.ApprovalApproved || calls[0].actor != "channel:fake:alice" {
		t.Fatalf("decide calls = %+v, want one attributed approved decision", calls)
	}

	// Commands are journaled but open no run and record no intent.
	if got := runs.snapshot(); len(got) != 0 {
		t.Fatalf("commands opened runs: %+v", got)
	}
	if open := openIntent(t, backend); len(open) != 0 {
		t.Fatalf("commands recorded delivery intents: %+v", open)
	}
}

// TestApprovalCommandScopedToSession: a pending approval of another chat's
// session is invisible — deciding it through this chat fails closed without
// reaching the kernel.
func TestApprovalCommandScopedToSession(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	ch := fake.New()
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	decisions := &decisionRecorder{}
	host := newApprovalHost(t, backend, runs, ch, decisions)

	otherSession := ChannelSessionID("fake", "chat-999", "")
	seedRunAndApproval(t, backend, "run-approval-other", otherSession, "apr_ffff000011112222", "execute")

	env := host.envFor(ch)
	if err := env.PublishInbound(context.Background(), plugin.InboundMessage{
		Channel: "fake", ChatID: "chat-1", Sender: "alice", MessageID: "m-cmd-3",
		Parts: []plugin.Part{{Kind: plugin.PartText, Text: "/approve apr_ffff000011112222"}},
	}); err != nil {
		t.Fatal(err)
	}
	waitSentText(t, ch, "scope rejection", func(s string) bool {
		return strings.Contains(s, "没有待审批") || strings.Contains(s, "没有匹配")
	})
	if calls := decisions.snapshot(); len(calls) != 0 {
		t.Fatalf("cross-session decision forwarded: %+v", calls)
	}
}

// TestNonCommandSlashTextOpensRun: only the three exact tokens are commands;
// any other "/" text is an ordinary turn.
func TestNonCommandSlashTextOpensRun(t *testing.T) {
	backend := openBackend(t)
	runs := &runRecorder{messages: backend}
	ch := fake.New()
	ch.Publish = func(context.Context, plugin.ChannelEnv) error { return nil }
	decisions := &decisionRecorder{}
	host := newApprovalHost(t, backend, runs, ch, decisions)

	env := host.envFor(ch)
	if err := env.PublishInbound(context.Background(), plugin.InboundMessage{
		Channel: "fake", ChatID: "chat-1", Sender: "alice", MessageID: "m-cmd-4",
		Parts: []plugin.Part{{Kind: plugin.PartText, Text: "/tmp shows a path"}},
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(runs.snapshot()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("slash-prefixed text never opened a run")
		}
		time.Sleep(5 * time.Millisecond)
	}
	call := runs.snapshot()[0]
	if call.text != "/tmp shows a path" {
		t.Fatalf("run text = %q", call.text)
	}
	if calls := decisions.snapshot(); len(calls) != 0 {
		t.Fatalf("non-command text decided something: %+v", calls)
	}
}
