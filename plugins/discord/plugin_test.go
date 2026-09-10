package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	plugin "agent-vivy/sdk/port/channel"
)

// Synthetic test credentials. No real Discord bot, gateway or REST
// endpoint is involved anywhere; every network-shaped call in this file
// lands on an in-memory stub. Tests never run in parallel (a few
// package-level tuning vars are swapped and restored).
const (
	stubTokenEnvName = "VIVY_TEST_DISCORD_BOT_TOKEN"
	stubTokenValue   = "vivy-test-bot-token"

	validSettings = `{"token_env":"` + stubTokenEnvName + `"}`
)

// waitFor polls until cond holds or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// --- fakes -----------------------------------------------------------------

// fakeEnv is an in-memory plugin.ChannelEnv: the same surface the kernel
// ChannelHost hands out, with Secret resolving through the process
// environment exactly like the Host does (t.Setenv plants or empties the
// variables). Published inbound envelopes are recorded.
type fakeEnv struct {
	settings json.RawMessage

	mu        sync.Mutex
	published []plugin.InboundMessage
}

func (e *fakeEnv) ModuleID() string     { return "vivy/discord" }
func (e *fakeEnv) Logger() *slog.Logger { return nil }

func (e *fakeEnv) Secret(envKey string) (string, error) {
	v, ok := os.LookupEnv(envKey)
	if !ok || v == "" {
		return "", fmt.Errorf("env variable %q empty or unset", envKey)
	}
	return v, nil
}

func (e *fakeEnv) HTTP() *http.Client { return &http.Client{} }
func (e *fakeEnv) DialTLS(ctx context.Context, network, address string) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, address)
}

func (e *fakeEnv) Settings() json.RawMessage {
	if len(e.settings) == 0 {
		return json.RawMessage("{}")
	}
	return e.settings
}

func (e *fakeEnv) PublishInbound(_ context.Context, msg plugin.InboundMessage) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.published = append(e.published, msg)
	return nil
}

func (e *fakeEnv) Media() plugin.MediaStore { return nil }

func (e *fakeEnv) snapshot() []plugin.InboundMessage {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]plugin.InboundMessage(nil), e.published...)
}

// sentCall records one ChannelMessageSend invocation.
type sentCall struct {
	channelID string
	content   string
}

// fakeSession is an in-memory session. It records the factory inputs and
// the protocol calls, fails on demand, and (for the ear attempts) lets
// the test fire the MESSAGE_CREATE and DISCONNECT handlers the way
// discordgo's goroutines would.
type fakeSession struct {
	// captured factory inputs
	onMessage    messageHandlerFunc
	onDisconnect disconnectHandlerFunc

	openErr    error
	sendErr    error
	sendFailAt int  // 1-based call index that starts failing (0 = never)
	hanging    bool // Open blocks, then self-releases (dialer-timeout stand-in)

	mu         sync.Mutex
	openCalls  int
	closeCalls int
	sent       []sentCall
}

// Open implements session. A hanging Open blocks briefly and fails: the
// real dialer bounds its handshake, and the supervisor cannot Close a
// session mid-Open (discordgo holds the session mutex for the whole
// handshake), so the attempt self-releases exactly like the real thing.
func (f *fakeSession) Open() error {
	f.mu.Lock()
	f.openCalls++
	hanging := f.hanging
	f.mu.Unlock()
	if hanging {
		time.Sleep(150 * time.Millisecond)
		return errors.New("dial timed out")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.openErr
}

// Close implements session.
func (f *fakeSession) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeCalls++
	return nil
}

// ChannelMessageSend implements session: records the call and hands out
// a canned platform message id.
func (f *fakeSession) ChannelMessageSend(channelID, content string, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.mu.Lock()
	f.sent = append(f.sent, sentCall{channelID: channelID, content: content})
	call := len(f.sent)
	err, failAt := f.sendErr, f.sendFailAt
	f.mu.Unlock()
	if err != nil && (failAt == 0 || call >= failAt) {
		return nil, err
	}
	return &discordgo.Message{ID: fmt.Sprintf("sent-%d", call)}, nil
}

// calls snapshots the counters.
func (f *fakeSession) calls() (open, close int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.openCalls, f.closeCalls
}

// sentCalls snapshots the send invocations.
func (f *fakeSession) sentCalls() []sentCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentCall(nil), f.sent...)
}

// sessionFactorySpy wires the plugin's session factory to fakes and
// records every build. Build slot 0 is the never-opened send client
// Start creates; slots 1.. are the supervised gateway attempts (so
// ear(n) is built slot n+1). onBuild runs at build time (on the
// supervisor goroutine) so a test can pre-configure the next attempt.
type sessionFactorySpy struct {
	failAt  int // build index that fails (-1 = none)
	mu      sync.Mutex
	built   []*fakeSession
	onBuild func(n int, f *fakeSession)
}

func (s *sessionFactorySpy) build(token string, onMessage messageHandlerFunc, onDisconnect disconnectHandlerFunc) (session, error) {
	s.mu.Lock()
	n := len(s.built)
	failAt, hook := s.failAt, s.onBuild
	s.built = append(s.built, nil)
	s.mu.Unlock()
	if n == failAt {
		return nil, errors.New("factory exploded")
	}
	f := &fakeSession{onMessage: onMessage, onDisconnect: onDisconnect}
	s.mu.Lock()
	s.built[n] = f
	hook = s.onBuild
	s.mu.Unlock()
	if hook != nil {
		hook(n, f)
	}
	return f, nil
}

func (s *sessionFactorySpy) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.built)
}

func (s *sessionFactorySpy) nth(n int) *fakeSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n < 0 || n >= len(s.built) {
		return nil
	}
	return s.built[n]
}

// --- harness -----------------------------------------------------------------

// harness bundles a freshly wired plugin with its fakes.
type harness struct {
	p   *Plugin
	env *fakeEnv
	spy *sessionFactorySpy
}

func newHarness(t *testing.T, settings string) *harness {
	t.Helper()
	t.Setenv(stubTokenEnvName, stubTokenValue)
	h := &harness{
		env: &fakeEnv{settings: json.RawMessage(settings)},
	}
	h.p = newAdapter()
	h.spy = &sessionFactorySpy{failAt: -1}
	h.p.newSession = h.spy.build
	return h
}

// start runs Start and requires it to succeed; Stop is scheduled as
// cleanup so a failing test never leaks the supervisor.
func (h *harness) start(t *testing.T) {
	t.Helper()
	if err := h.p.Start(context.Background(), h.env); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = h.p.Stop(ctx)
	})
}

// sender is the build slot the send client occupies (index 0: no gateway
// handlers, never opened).
func (h *harness) sender() *fakeSession { return h.spy.nth(0) }

// ear is the n-th supervised gateway attempt (build slot n+1, after the
// send client).
func (h *harness) ear(n int) *fakeSession { return h.spy.nth(n + 1) }

// dispatch feeds one MESSAGE_CREATE event through the ear attempt's
// handler the way discordgo's dispatch goroutine would.
func (h *harness) dispatch(t *testing.T, n int, m *discordgo.MessageCreate) {
	t.Helper()
	f := h.ear(n)
	if f == nil {
		t.Fatalf("no gateway attempt #%d built", n)
	}
	if f.onMessage == nil {
		t.Fatalf("MESSAGE_CREATE handler not wired on attempt #%d", n)
	}
	f.onMessage(nil, m)
}

// drop simulates the gateway dropping a healthy connection: discordgo's
// read loop closes the socket and emits the synthetic DISCONNECT event,
// which is the death signal the supervisor waits on.
func (h *harness) drop(t *testing.T, n int) {
	t.Helper()
	f := h.ear(n)
	if f == nil {
		t.Fatalf("no gateway attempt #%d built", n)
	}
	if f.onDisconnect == nil {
		t.Fatalf("DISCONNECT handler not wired on attempt #%d", n)
	}
	f.onDisconnect(nil, &discordgo.Disconnect{})
}

// message builds a minimal guild text MESSAGE_CREATE payload.
func message(msgID, authorID, content string) *discordgo.MessageCreate {
	return &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        msgID,
		ChannelID: "chan-1",
		GuildID:   "guild-1",
		Content:   content,
		Author:    &discordgo.User{ID: authorID},
		Type:      discordgo.MessageTypeDefault,
	}}
}

// --- settings ----------------------------------------------------------------

func TestDecodeSettings(t *testing.T) {
	t.Run("absent and empty decode to zero", func(t *testing.T) {
		for _, raw := range []json.RawMessage{nil, json.RawMessage(""), json.RawMessage("null"), json.RawMessage("{}")} {
			s, err := DecodeSettings(raw)
			if err != nil {
				t.Fatalf("DecodeSettings(%s): %v", raw, err)
			}
			if s != (Settings{}) {
				t.Fatalf("DecodeSettings(%s) = %+v, want zero", raw, s)
			}
		}
	})
	t.Run("token_env decodes and trims", func(t *testing.T) {
		s, err := DecodeSettings(json.RawMessage(`{"token_env":" A "}`))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if s.TokenEnv != "A" {
			t.Fatalf("token_env = %q, want A", s.TokenEnv)
		}
	})
	t.Run("unknown fields are rejected", func(t *testing.T) {
		_, err := DecodeSettings(json.RawMessage(`{"token_env":"A","gateway_url":"wss://nope"}`))
		if err == nil || !strings.Contains(err.Error(), "decode settings") {
			t.Fatalf("want strict-decode failure, got %v", err)
		}
	})
	t.Run("trailing data is rejected", func(t *testing.T) {
		_, err := DecodeSettings(json.RawMessage(`{"token_env":"A"} {"x":1}`))
		if err == nil || !strings.Contains(err.Error(), "trailing") {
			t.Fatalf("want trailing-data failure, got %v", err)
		}
	})
}

// --- Start fail-closed matrix --------------------------------------------------

func TestStartFailsClosed(t *testing.T) {
	cases := []struct {
		name     string
		settings string
		prepare  func(h *harness)
		wantErr  string
	}{
		{
			name:     "missing token_env",
			settings: `{}`,
			wantErr:  "token_env is required",
		},
		{
			name:     "unknown settings field",
			settings: `{"token_env":"A","nope":1}`,
			wantErr:  "decode settings",
		},
		{
			name:     "token env unset",
			settings: validSettings,
			prepare: func(h *harness) {
				_ = os.Unsetenv(stubTokenEnvName)
			},
			wantErr: "resolve bot token through env",
		},
		{
			name:     "send client factory fails",
			settings: validSettings,
			prepare: func(h *harness) {
				h.spy.failAt = 0
			},
			wantErr: "create send client",
		},
		{
			name:     "first gateway handshake fails",
			settings: validSettings,
			prepare: func(h *harness) {
				h.spy.onBuild = func(n int, f *fakeSession) {
					if n == 1 {
						f.openErr = errors.New("401: Unauthorized")
					}
				}
			},
			wantErr: "gateway handshake",
		},
		{
			name:     "first attempt session factory fails",
			settings: validSettings,
			prepare: func(h *harness) {
				h.spy.failAt = 1
			},
			wantErr: "build session",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.settings)
			if tc.prepare != nil {
				tc.prepare(h)
			}
			err := h.p.Start(context.Background(), h.env)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Start error = %v, want it to contain %q", err, tc.wantErr)
			}
			// A failed Start leaves neither a Send-able client nor a live
			// ear.
			h.p.mu.Lock()
			senderNil := h.p.sender == nil
			currentNil := h.p.current == nil
			h.p.mu.Unlock()
			if !senderNil || !currentNil {
				t.Fatalf("failed Start left sender=%v current=%v, want both nil", senderNil, currentNil)
			}
			if _, err := h.p.Send(context.Background(), plugin.OutboundMessage{ChatID: "chan-1"}); err == nil {
				t.Fatalf("Send after failed Start must fail")
			}
		})
	}
}

func TestStartContextCancelled(t *testing.T) {
	h := newHarness(t, validSettings)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := h.p.Start(ctx, h.env)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("Start with cancelled context = %v, want context canceled", err)
	}
}

func TestStartSuccessWiring(t *testing.T) {
	h := newHarness(t, validSettings)
	h.start(t)

	sender := h.sender()
	if sender == nil {
		t.Fatalf("no send client built")
	}
	if sender.onMessage != nil || sender.onDisconnect != nil {
		t.Fatalf("send client must carry no gateway handlers")
	}
	if open, _ := sender.calls(); open != 0 {
		t.Fatalf("send client opened %d times, want 0 (REST only)", open)
	}
	ear := h.ear(0)
	if ear == nil {
		t.Fatalf("no gateway attempt built")
	}
	if ear.onMessage == nil || ear.onDisconnect == nil {
		t.Fatalf("event handlers not wired into the attempt")
	}
	open, closed := ear.calls()
	if open != 1 || closed != 0 {
		t.Fatalf("attempt calls = (open %d, close %d), want one open, no close", open, closed)
	}
	if len(h.env.snapshot()) != 0 {
		t.Fatalf("no envelope may be published by Start alone")
	}
}

// TestProductionSessionFactory pins the production session factory's
// offline wiring: reconnect off (its loop is unstoppable), voice
// resurrection off (voice is banned), TEXT intents + Message Content,
// and the Bot-scheme token. discordgo.New only wires structs — no
// network.
func TestProductionSessionFactory(t *testing.T) {
	p := newAdapter()
	p.host = &fakeEnv{}
	onMessage := func(*discordgo.Session, *discordgo.MessageCreate) {}
	onDisconnect := func(*discordgo.Session, *discordgo.Disconnect) {}
	s, err := p.newSession(stubTokenValue, onMessage, onDisconnect)
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	real, ok := s.(*discordgo.Session)
	if !ok {
		t.Fatalf("production session = %T, want *discordgo.Session", s)
	}
	if real.ShouldReconnectOnError {
		t.Fatalf("SDK reconnect must be off: v0.29's reconnect loop is endless and survives Close")
	}
	if real.ShouldReconnectVoiceOnSessionError {
		t.Fatalf("voice resurrection must be off: voice is banned in plugins")
	}
	want := discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent
	if real.Identify.Intents != want {
		t.Fatalf("intents = %d, want %d (guild text + DM + message content)", real.Identify.Intents, want)
	}
	if real.Token != "Bot "+stubTokenValue {
		t.Fatalf("token scheme = %q, want the Bot-prefixed value", real.Token)
	}
}

// --- inbound normalization ------------------------------------------------------

func TestNormalizeMessage(t *testing.T) {
	cases := []struct {
		name string
		m    *discordgo.MessageCreate
		want bool
		msg  plugin.InboundMessage
	}{
		{
			name: "guild text message",
			m:    message("m-1", "U1", "hello"),
			want: true,
			msg: plugin.InboundMessage{
				Channel:   "discord",
				ChatID:    "chan-1",
				Sender:    "discord:U1",
				MessageID: "m-1",
				Parts:     []plugin.Part{{Kind: plugin.PartText, Text: "hello"}},
			},
		},
		{
			name: "dm text message (empty guild)",
			m: &discordgo.MessageCreate{Message: &discordgo.Message{
				ID: "m-2", ChannelID: "dm-1", Content: "dm hello",
				Author: &discordgo.User{ID: "U2"}, Type: discordgo.MessageTypeDefault,
			}},
			want: true,
			msg: plugin.InboundMessage{
				Channel:   "discord",
				ChatID:    "dm-1",
				Sender:    "discord:U2",
				MessageID: "m-2",
				Parts:     []plugin.Part{{Kind: plugin.PartText, Text: "dm hello"}},
			},
		},
		{
			name: "reply captures the referenced message id",
			m: &discordgo.MessageCreate{Message: &discordgo.Message{
				ID: "m-3", ChannelID: "chan-1", GuildID: "guild-1", Content: "a reply",
				Author: &discordgo.User{ID: "U1"}, Type: discordgo.MessageTypeReply,
				ReferencedMessage: &discordgo.Message{ID: "orig-9"},
			}},
			want: true,
			msg: plugin.InboundMessage{
				Channel:   "discord",
				ChatID:    "chan-1",
				Sender:    "discord:U1",
				MessageID: "m-3",
				ReplyTo:   "orig-9",
				Parts:     []plugin.Part{{Kind: plugin.PartText, Text: "a reply"}},
			},
		},
		{name: "nil event", m: nil, want: false},
		{name: "nil message", m: &discordgo.MessageCreate{}, want: false},
		{name: "nil author", m: &discordgo.MessageCreate{Message: &discordgo.Message{
			ID: "m", ChannelID: "chan-1", Content: "hi", Type: discordgo.MessageTypeDefault,
		}}, want: false},
		{
			name: "bot author (echo guard)",
			m: &discordgo.MessageCreate{Message: &discordgo.Message{
				ID: "m-4", ChannelID: "chan-1", Content: "beep",
				Author: &discordgo.User{ID: "BOT", Bot: true}, Type: discordgo.MessageTypeDefault,
			}},
			want: false,
		},
		{
			name: "empty author id",
			m:    message("m-5", "  ", "hi"),
			want: false,
		},
		{
			name: "empty content (attachment only)",
			m:    message("m-6", "U1", ""),
			want: false,
		},
		{
			name: "whitespace content",
			m:    message("m-7", "U1", "  \n\t"),
			want: false,
		},
		{
			name: "slash command message type",
			m: &discordgo.MessageCreate{Message: &discordgo.Message{
				ID: "m-8", ChannelID: "chan-1", Content: "/do thing",
				Author: &discordgo.User{ID: "U1"}, Type: discordgo.MessageTypeChatInputCommand,
			}},
			want: false,
		},
		{
			name: "context menu command message type",
			m: &discordgo.MessageCreate{Message: &discordgo.Message{
				ID: "m-9", ChannelID: "chan-1", Content: "translated",
				Author: &discordgo.User{ID: "U1"}, Type: discordgo.MessageTypeContextMenuCommand,
			}},
			want: false,
		},
		{
			name: "system message type",
			m: &discordgo.MessageCreate{Message: &discordgo.Message{
				ID: "m-10", ChannelID: "chan-1", Content: "x joined the guild",
				Author: &discordgo.User{ID: "U1"}, Type: discordgo.MessageTypeGuildMemberJoin,
			}},
			want: false,
		},
		{
			name: "empty message id",
			m:    message("", "U1", "hi"),
			want: false,
		},
		{
			name: "empty channel id",
			m: &discordgo.MessageCreate{Message: &discordgo.Message{
				ID: "m-11", Content: "hi",
				Author: &discordgo.User{ID: "U1"}, Type: discordgo.MessageTypeDefault,
			}},
			want: false,
		},
		{
			name: "reply with empty referenced id stays publishable without ReplyTo",
			m: &discordgo.MessageCreate{Message: &discordgo.Message{
				ID: "m-12", ChannelID: "chan-1", Content: "reply",
				Author: &discordgo.User{ID: "U1"}, Type: discordgo.MessageTypeReply,
				ReferencedMessage: &discordgo.Message{ID: " "},
			}},
			want: true,
			msg: plugin.InboundMessage{
				Channel:   "discord",
				ChatID:    "chan-1",
				Sender:    "discord:U1",
				MessageID: "m-12",
				ReplyTo:   "",
				Parts:     []plugin.Part{{Kind: plugin.PartText, Text: "reply"}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, publishable := normalizeMessage(tc.m)
			if publishable != tc.want {
				t.Fatalf("publishable = %v, want %v", publishable, tc.want)
			}
			if !tc.want {
				return
			}
			if got.Channel != tc.msg.Channel || got.ChatID != tc.msg.ChatID ||
				got.Sender != tc.msg.Sender || got.MessageID != tc.msg.MessageID ||
				got.ReplyTo != tc.msg.ReplyTo || got.TopicID != "" ||
				len(got.Parts) != 1 || got.Parts[0].Text != tc.msg.Parts[0].Text ||
				got.Parts[0].Kind != plugin.PartText {
				t.Fatalf("envelope = %+v, want %+v", got, tc.msg)
			}
		})
	}
}

func TestMessageHandlerPublishesAndFences(t *testing.T) {
	h := newHarness(t, validSettings)
	h.start(t)

	// A publishable shape: one envelope, sender in the allow_from format.
	h.dispatch(t, 0, message("m-1", "U1", "hi"))
	waitFor(t, "first inbound envelope", func() bool { return len(h.env.snapshot()) == 1 })
	if got := h.env.snapshot()[0]; got.Sender != "discord:U1" || got.ChatID != "chan-1" {
		t.Fatalf("envelope = %+v", got)
	}

	// Non-publishable shapes: dropped locally, no envelope.
	h.dispatch(t, 0, message("m-2", "U1", ""))
	h.dispatch(t, 0, message("", "U1", "no id"))
	h.dispatch(t, 0, &discordgo.MessageCreate{Message: &discordgo.Message{
		ID: "m-3", ChannelID: "chan-1", Content: "echo",
		Author: &discordgo.User{ID: "BOT", Bot: true}, Type: discordgo.MessageTypeDefault,
	}})
	if got := len(h.env.snapshot()); got != 1 {
		t.Fatalf("unpublishable shapes changed the envelope count to %d, want 1", got)
	}

	// Late callback after Stop: fenced, nothing published.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	h.dispatch(t, 0, message("m-4", "U1", "after stop"))
	if got := len(h.env.snapshot()); got != 1 {
		t.Fatalf("late callback published, envelope count = %d, want 1", got)
	}
}

// --- Send --------------------------------------------------------------------

func TestSendViaStubbedSession(t *testing.T) {
	h := newHarness(t, validSettings)
	h.start(t)

	// Two text parts become two plain REST sends on the send client
	// (never the ear), addressed to the channel id, content as-is
	// (Discord renders markdown natively; the adapter neither strips nor
	// adds formatting).
	ids, err := h.p.Send(context.Background(), plugin.OutboundMessage{
		ChatID:  "chan-1",
		ReplyTo: "ignored-this-slice",
		Parts: []plugin.Part{
			{Kind: plugin.PartText, Text: "**bold** and _italics_"},
			{Kind: plugin.PartText, Text: "second"},
		},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(ids) != 2 || ids[0] != "sent-1" || ids[1] != "sent-2" {
		t.Fatalf("ids = %v, want [sent-1 sent-2]", ids)
	}
	calls := h.sender().sentCalls()
	if len(calls) != 2 {
		t.Fatalf("send client calls = %d, want 2", len(calls))
	}
	for i, call := range calls {
		if call.channelID != "chan-1" {
			t.Fatalf("call %d channel = %q, want chan-1", i, call.channelID)
		}
	}
	if calls[0].content != "**bold** and _italics_" {
		t.Fatalf("call 0 content = %q, want the markdown passed through as-is", calls[0].content)
	}
	if calls[1].content != "second" {
		t.Fatalf("call 1 content = %q, want second", calls[1].content)
	}
	// The ear gateway session must not be used for sends.
	if ear := h.ear(0); len(ear.sentCalls()) != 0 {
		t.Fatalf("ear session carried %d sends, want 0 (send path is REST-only)", len(ear.sentCalls()))
	}
}

func TestSendFailClosed(t *testing.T) {
	t.Run("empty chat id", func(t *testing.T) {
		h := newHarness(t, validSettings)
		h.start(t)
		_, err := h.p.Send(context.Background(), plugin.OutboundMessage{
			ChatID: "  ",
			Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "hi"}},
		})
		if err == nil || !strings.Contains(err.Error(), "chat id is empty") {
			t.Fatalf("error = %v, want the empty-chat-id failure", err)
		}
	})
	t.Run("not started", func(t *testing.T) {
		h := newHarness(t, validSettings)
		_, err := h.p.Send(context.Background(), plugin.OutboundMessage{
			ChatID: "chan-1",
			Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "hi"}},
		})
		if err == nil || !strings.Contains(err.Error(), "not started") {
			t.Fatalf("error = %v, want the not-started failure", err)
		}
	})
	t.Run("stopped", func(t *testing.T) {
		h := newHarness(t, validSettings)
		h.start(t)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := h.p.Stop(ctx); err != nil {
			t.Fatalf("stop: %v", err)
		}
		if _, err := h.p.Send(context.Background(), plugin.OutboundMessage{
			ChatID: "chan-1",
			Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "hi"}},
		}); err == nil || !strings.Contains(err.Error(), "not started") {
			t.Fatalf("Send after Stop = %v, want the not-started failure", err)
		}
	})
	t.Run("non-text and empty parts are skipped", func(t *testing.T) {
		h := newHarness(t, validSettings)
		h.start(t)
		ids, err := h.p.Send(context.Background(), plugin.OutboundMessage{
			ChatID: "chan-1",
			Parts: []plugin.Part{
				{Kind: plugin.PartMediaRef, MediaRef: "ref"},
				{Kind: plugin.PartText, Text: ""},
				{Kind: plugin.PartStructured, Structured: json.RawMessage(`{}`)},
			},
		})
		if err != nil {
			t.Fatalf("send: %v", err)
		}
		if len(ids) != 0 {
			t.Fatalf("ids = %v, want none", ids)
		}
		if got := len(h.sender().sentCalls()); got != 0 {
			t.Fatalf("send client calls = %d, want 0", got)
		}
	})
	t.Run("REST error surfaces with its cause chain", func(t *testing.T) {
		h := newHarness(t, validSettings)
		h.start(t)
		cause := errors.New("403: Missing Permissions")
		h.sender().sendErr = fmt.Errorf("POST /channels/chan-1/messages: %w", cause)
		// The first part lands, the second hits the platform failure.
		h.sender().sendFailAt = 2
		ids, err := h.p.Send(context.Background(), plugin.OutboundMessage{
			ChatID: "chan-1",
			Parts: []plugin.Part{
				{Kind: plugin.PartText, Text: "first"},
				{Kind: plugin.PartText, Text: "second"},
			},
		})
		if err == nil {
			t.Fatalf("send over a REST failure must fail")
		}
		if !errors.Is(err, cause) {
			t.Fatalf("error = %v, want the platform cause chain preserved", err)
		}
		if !strings.Contains(err.Error(), "chan-1") {
			t.Fatalf("error = %v, want the chat id in the message", err)
		}
		// The ids delivered before the failure are still returned, and
		// only the first part reached the wire.
		if len(ids) != 1 || ids[0] != "sent-1" {
			t.Fatalf("ids = %v, want [sent-1]", ids)
		}
		if got := len(h.sender().sentCalls()); got != 2 {
			t.Fatalf("send client calls = %d, want 2 (the failed attempt included)", got)
		}
	})
}

// --- lifecycle ----------------------------------------------------------------

// shrinkRedialDelay makes the supervised redial loop fast for lifecycle
// tests; restored by cleanup.
func shrinkRedialDelay(t *testing.T) {
	t.Helper()
	old := wsRedialDelay
	wsRedialDelay = 20 * time.Millisecond
	t.Cleanup(func() { wsRedialDelay = old })
}

func TestRedialAfterDrop(t *testing.T) {
	shrinkRedialDelay(t)
	h := newHarness(t, validSettings)
	h.start(t)

	// Traffic through the live ear publishes normally.
	h.dispatch(t, 0, message("m-1", "U1", "hi"))
	waitFor(t, "first inbound envelope", func() bool { return len(h.env.snapshot()) == 1 })

	// The gateway drops the healthy connection (read-loop death →
	// DISCONNECT): the supervisor redials with a fresh session, which
	// re-identifies (no resume — see the package comment).
	h.drop(t, 0)
	waitFor(t, "redial after drop", func() bool {
		ear := h.ear(1)
		if ear == nil {
			return false
		}
		open, _ := ear.calls()
		return open == 1
	})
	if open, _ := h.ear(0).calls(); open != 1 {
		t.Fatalf("first ear opened %d times, want exactly 1", open)
	}

	// The redialed attempt's handler publishes normally.
	h.dispatch(t, 1, message("m-2", "U1", "after redial"))
	waitFor(t, "envelope after redial", func() bool { return len(h.env.snapshot()) == 2 })

	// The send path is untouched by ear redials: the same send client
	// slot serves the whole Start lifetime.
	if _, err := h.p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "chan-1",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "still here"}},
	}); err != nil {
		t.Fatalf("send across a redial: %v", err)
	}
	if got := len(h.sender().sentCalls()); got != 1 {
		t.Fatalf("send client calls = %d, want 1 (same slot across redials)", got)
	}
	if got := h.spy.count(); got != 3 {
		t.Fatalf("build slots = %d, want 3 (send client + two ears)", got)
	}
}

func TestStopIdempotentAndBounded(t *testing.T) {
	h := newHarness(t, validSettings)
	// Stop before Start is a no-op.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("stop before start: %v", err)
	}
	h.start(t)
	ear := h.ear(0)
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("second stop: %v", err)
	}
	// The supervisor — not Stop — closed the live ear, exactly once.
	if _, closed := ear.calls(); closed != 1 {
		t.Fatalf("ear close calls = %d, want exactly 1 (idempotent)", closed)
	}
}

func TestStartAfterStopStartsFresh(t *testing.T) {
	h := newHarness(t, validSettings)
	h.start(t)
	h.dispatch(t, 0, message("m-1", "U1", "hi"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	// Starting again on the same instance is a new ear: fresh send
	// client, fresh session, no carried gateway state.
	if err := h.p.Start(context.Background(), h.env); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if got := h.spy.count(); got != 4 {
		t.Fatalf("build slots after restart = %d, want 4 (two send clients, two ears)", got)
	}
	// Slot indexing after the restart: 0/1 were the first lifetime's send
	// client and ear, 2 is the new send client, 3 is the new ear.
	ear := h.spy.nth(3)
	if ear == nil {
		t.Fatalf("no second ear built")
	}
	open, closed := ear.calls()
	if open != 1 || closed != 0 {
		t.Fatalf("second ear calls = (open %d, close %d), want a fresh open", open, closed)
	}
	if ear.onMessage == nil {
		t.Fatalf("second ear carries no MESSAGE_CREATE handler")
	}
	ear.onMessage(nil, message("m-2", "U2", "fresh ear"))
	waitFor(t, "envelope on the fresh ear", func() bool { return len(h.env.snapshot()) == 2 })
}

func TestStopDuringHangingOpenIsBounded(t *testing.T) {
	h := newHarness(t, validSettings)
	// The first gateway attempt hangs in the handshake (bounded by the
	// real dialer, stand-in here); Stop must still return.
	h.spy.onBuild = func(n int, f *fakeSession) {
		if n == 1 {
			f.hanging = true
		}
	}
	startErr := make(chan error, 1)
	go func() {
		startErr <- h.p.Start(context.Background(), h.env)
	}()
	waitFor(t, "first attempt stuck in the handshake", func() bool {
		ear := h.ear(0)
		if ear == nil {
			return false
		}
		open, _ := ear.calls()
		return open == 1
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("stop during connect: %v", err)
	}
	select {
	case err := <-startErr:
		if err == nil {
			t.Fatalf("Start interrupted by Stop must fail")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Start did not return after Stop")
	}
}

// TestLateDeathSignalAfterStopIsBenign: a DISCONNECT fired after Stop
// (a socket dying late, or the supervisor's own close racing the latch)
// must not panic or resurrect anything.
func TestLateDeathSignalAfterStopIsBenign(t *testing.T) {
	h := newHarness(t, validSettings)
	h.start(t)
	ear := h.ear(0)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	// discordgo dispatches handlers on their own goroutines; fire the
	// signal the way a late dispatch would.
	go ear.onDisconnect(nil, &discordgo.Disconnect{})
	time.Sleep(50 * time.Millisecond)
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("second stop: %v", err)
	}
	if _, err := h.p.Send(context.Background(), plugin.OutboundMessage{ChatID: "chan-1"}); err == nil {
		t.Fatalf("Send after Stop must fail")
	}
}
