package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
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

func (e *fakeEnv) reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.published = nil
}

// sentCall records one ChannelMessageSend invocation.
type sentCall struct {
	channelID string
	content   string
	// reference is the reply-threading message id ("" for plain sends).
	reference string
	// fileNames lists the multipart file names of a complex send (media).
	fileNames []string
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

	mu          sync.Mutex
	openCalls   int
	closeCalls  int
	sent        []sentCall
	typingCalls []string
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

// ChannelMessageSendComplex implements session: records the call including
// the reply reference.
func (f *fakeSession) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	ref := ""
	if data.Reference != nil {
		ref = data.Reference.MessageID
	}
	var fileNames []string
	for _, file := range data.Files {
		fileNames = append(fileNames, file.Name)
	}
	f.mu.Lock()
	f.sent = append(f.sent, sentCall{channelID: channelID, content: data.Content, reference: ref, fileNames: fileNames})
	call := len(f.sent)
	err, failAt := f.sendErr, f.sendFailAt
	f.mu.Unlock()
	if err != nil && (failAt == 0 || call >= failAt) {
		return nil, err
	}
	return &discordgo.Message{ID: fmt.Sprintf("sent-%d", call)}, nil
}

// ChannelTyping implements session: records the indicator ping target.
func (f *fakeSession) ChannelTyping(channelID string, _ ...discordgo.RequestOption) error {
	f.mu.Lock()
	f.typingCalls = append(f.typingCalls, channelID)
	f.mu.Unlock()
	return nil
}

// typingSnapshot returns the recorded typing target channel ids.
func (f *fakeSession) typingSnapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.typingCalls...)
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
	// A DM-shaped message: the group (guild) trigger path has its own
	// builder below, since mention-only gating applies there.
	return &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        msgID,
		ChannelID: "chan-1",
		Content:   content,
		Author:    &discordgo.User{ID: authorID},
		Type:      discordgo.MessageTypeDefault,
	}}
}

// groupMessage builds one guild text-channel message; mentioned records
// whether the bot itself is among the Mentions (and renders the markup
// into the content like Discord does).
func groupMessage(msgID, authorID, content string, mentioned bool) *discordgo.MessageCreate {
	m := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID:        msgID,
		ChannelID: "chan-1",
		GuildID:   "guild-1",
		Content:   content,
		Author:    &discordgo.User{ID: authorID},
		Type:      discordgo.MessageTypeDefault,
	}}
	if mentioned {
		m.Message.Content = "<@bot-1> " + content
		m.Message.Mentions = []*discordgo.User{{ID: "bot-1"}}
	}
	return m
}

// gatewaySession is a minimal live-session stand-in carrying the READY
// state the group trigger reads the bot identity from.
func gatewaySession(botUserID string) *discordgo.Session {
	return &discordgo.Session{State: &discordgo.State{Ready: discordgo.Ready{User: &discordgo.User{ID: botUserID}}}}
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
				ID: "m-3", ChannelID: "chan-1", Content: "a reply",
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
		{
			name: "guild message with the bot mention publishes stripped",
			m: &discordgo.MessageCreate{Message: &discordgo.Message{
				ID: "m-13", ChannelID: "chan-1", GuildID: "guild-1",
				Content: "<@bot-1> what is up", Type: discordgo.MessageTypeDefault,
				Author:   &discordgo.User{ID: "U1"},
				Mentions: []*discordgo.User{{ID: "bot-1"}},
			}},
			want: true,
			msg: plugin.InboundMessage{
				Channel:   "discord",
				ChatID:    "chan-1",
				Sender:    "discord:U1",
				MessageID: "m-13",
				Parts:     []plugin.Part{{Kind: plugin.PartText, Text: "what is up"}},
			},
		},
		{
			name: "guild message without the bot mention drops",
			m: &discordgo.MessageCreate{Message: &discordgo.Message{
				ID: "m-14", ChannelID: "chan-1", GuildID: "guild-1",
				Content: "just chatting", Type: discordgo.MessageTypeDefault,
				Author:   &discordgo.User{ID: "U1"},
				Mentions: []*discordgo.User{{ID: "U2"}},
			}},
			want: false,
		},
		{
			name: "guild message mentioning another user drops",
			m: &discordgo.MessageCreate{Message: &discordgo.Message{
				ID: "m-15", ChannelID: "chan-1", GuildID: "guild-1",
				Content: "hey <@U2> look", Type: discordgo.MessageTypeDefault,
				Author:   &discordgo.User{ID: "U1"},
				Mentions: []*discordgo.User{{ID: "U2"}},
			}},
			want: false,
		},
		{
			name: "guild bare mention leaves nothing to publish",
			m: &discordgo.MessageCreate{Message: &discordgo.Message{
				ID: "m-16", ChannelID: "chan-1", GuildID: "guild-1",
				Content: "<@!bot-1>", Type: discordgo.MessageTypeDefault,
				Author:   &discordgo.User{ID: "U1"},
				Mentions: []*discordgo.User{{ID: "bot-1"}},
			}},
			want: false,
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
			got, _, publishable := normalizeMessage(tc.m, "bot-1")
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

// TestHealthClassifiesRedialingGateway (CH-R-1): a dropped gateway session
// surfaces as a temporary HealthError while the loop redials, and Health
// returns to nil once the replacement session opens.
func TestHealthClassifiesRedialingGateway(t *testing.T) {
	old := wsRedialDelay
	wsRedialDelay = 20 * time.Millisecond
	t.Cleanup(func() { wsRedialDelay = old })

	h := newHarness(t, validSettings)
	h.start(t)

	h.drop(t, 0)
	var healthErr *plugin.HealthError
	waitFor(t, "temporary health while redialing", func() bool {
		err := h.p.Health(context.Background())
		return errors.As(err, &healthErr) && healthErr.Class == plugin.ClassTemporary
	})
	waitFor(t, "healthy after reconnect", func() bool {
		return h.p.Health(context.Background()) == nil
	})
}

// --- Typing ------------------------------------------------------------------

// TestTypingPingsTheSendClient: plugin.Typing rides the never-opened REST
// send client — one ChannelTyping ping per call; the Host owns the resend
// cadence and the stop. Typing before Start fails closed like Send.
func TestTypingPingsTheSendClient(t *testing.T) {
	h := newHarness(t, validSettings)
	h.start(t)

	if err := h.p.Typing(context.Background(), "chan-1"); err != nil {
		t.Fatalf("typing: %v", err)
	}
	calls := h.sender().typingSnapshot()
	if len(calls) != 1 || calls[0] != "chan-1" {
		t.Fatalf("typing calls = %v, want [chan-1]", calls)
	}

	if err := newAdapter().Typing(context.Background(), "chan-1"); err == nil {
		t.Fatal("typing before start must fail closed")
	}
}

// TestSendThreadsViaMessageReference: a non-empty ReplyTo routes the send
// through the complex send with a MessageReference (the host quotes only
// the first chunk; the adapter threads whatever envelope it is given); an
// envelope without ReplyTo travels as a plain send.
func TestSendThreadsViaMessageReference(t *testing.T) {
	h := newHarness(t, validSettings)
	h.start(t)

	if _, err := h.p.Send(context.Background(), plugin.OutboundMessage{
		ChatID:  "chan-1",
		ReplyTo: "msg-9",
		Parts:   []plugin.Part{{Kind: plugin.PartText, Text: "threaded"}},
	}); err != nil {
		t.Fatalf("threaded send: %v", err)
	}
	if _, err := h.p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "chan-1",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "plain"}},
	}); err != nil {
		t.Fatalf("plain send: %v", err)
	}
	calls := h.sender().sentCalls()
	if len(calls) != 2 {
		t.Fatalf("send calls = %d, want 2", len(calls))
	}
	if calls[0].reference != "msg-9" {
		t.Fatalf("call 0 reference = %q, want msg-9", calls[0].reference)
	}
	if calls[1].reference != "" || calls[1].content != "plain" {
		t.Fatalf("call 1 = %+v, want a plain unthreaded send", calls[1])
	}
}

// --- group trigger (tier-1, mention-only) --------------------------------------

// dispatchFromSession feeds one MESSAGE_CREATE through the ear attempt's
// handler with a live-shaped session (the group trigger reads the bot
// identity from the session's READY state).
func (h *harness) dispatchFromSession(t *testing.T, n int, s *discordgo.Session, m *discordgo.MessageCreate) {
	t.Helper()
	f := h.ear(n)
	if f == nil {
		t.Fatalf("no gateway attempt #%d built", n)
	}
	f.onMessage(s, m)
}

// TestGroupMentionOnlyGatesGuildMessages: a guild message mentioning the
// bot publishes with the mention markup stripped (the identity comes from
// the session's READY state); without the mention nothing publishes; a DM
// skips the gate entirely.
func TestGroupMentionOnlyGatesGuildMessages(t *testing.T) {
	h := newHarness(t, validSettings)
	h.start(t)

	h.dispatchFromSession(t, 0, gatewaySession("bot-1"), groupMessage("g-1", "U1", "what is up", true))
	waitFor(t, "guild mention envelope", func() bool { return len(h.env.snapshot()) == 1 })
	got := h.env.snapshot()[0]
	if got.Parts[0].Text != "what is up" || got.Sender != "discord:U1" || got.ChatID != "chan-1" {
		t.Fatalf("guild envelope = %+v, want the stripped text from U1", got)
	}

	h.dispatchFromSession(t, 0, gatewaySession("bot-1"), groupMessage("g-2", "U1", "chatting", false))
	if got := len(h.env.snapshot()); got != 1 {
		t.Fatalf("envelopes = %d, want still 1 (unmentioned guild message must not publish)", got)
	}

	// A DM skips the gate (the nil-session dispatch stays legal there).
	h.dispatch(t, 0, message("d-1", "U1", "dm hello"))
	waitFor(t, "dm envelope", func() bool { return len(h.env.snapshot()) == 2 })
}

// stubImageBytes is a JPEG-magic payload; the adapter does not sniff
// (the Host does), but real magic bytes keep the fixture honest.
var stubImageBytes = append([]byte{0xff, 0xd8, 0xff, 0xe0}, bytes.Repeat([]byte{0x00}, 32)...)

// TestNormalizeMessageImagePreScreen: image attachments return as download
// refs (DM and guild alike), non-image attachments survive as [file: name]
// annotations, and a captionless image message is a valid turn.
func TestNormalizeMessageImagePreScreen(t *testing.T) {
	withMedia := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID: "m-img-1", ChannelID: "chan-1", Content: "look",
		Author: &discordgo.User{ID: "U1"}, Type: discordgo.MessageTypeDefault,
		Attachments: []*discordgo.MessageAttachment{
			{ID: "a1", URL: "https://cdn.example.com/pic.jpg", Filename: "pic.jpg", ContentType: "image/jpeg"},
			{ID: "a2", URL: "https://cdn.example.com/notes.txt", Filename: "notes.txt", ContentType: "text/plain"},
		},
	}}
	msg, refs, publishable := normalizeMessage(withMedia, "bot-1")
	if !publishable {
		t.Fatal("text + media message must be publishable")
	}
	if len(refs) != 1 || refs[0].url != "https://cdn.example.com/pic.jpg" ||
		refs[0].name != "pic.jpg" || refs[0].contentType != "image/jpeg" {
		t.Fatalf("image refs = %+v", refs)
	}
	if len(msg.Parts) != 2 || msg.Parts[0].Text != "look" || msg.Parts[1].Text != "[file: notes.txt]" {
		t.Fatalf("parts = %+v, want text then the file annotation", msg.Parts)
	}

	captionless := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID: "m-img-2", ChannelID: "chan-1",
		Author: &discordgo.User{ID: "U1"}, Type: discordgo.MessageTypeDefault,
		Attachments: []*discordgo.MessageAttachment{
			{ID: "a1", URL: "https://cdn.example.com/pic.png", Filename: "pic.png", ContentType: "image/png"},
		},
	}}
	msg, refs, publishable = normalizeMessage(captionless, "bot-1")
	if !publishable || len(refs) != 1 || len(msg.Parts) != 0 {
		t.Fatalf("captionless image: publishable=%v refs=%d parts=%+v", publishable, len(refs), msg.Parts)
	}

	// An image-class attachment by extension alone (no content type) is
	// still pre-screened.
	byExt := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID: "m-img-3", ChannelID: "chan-1",
		Author: &discordgo.User{ID: "U1"}, Type: discordgo.MessageTypeDefault,
		Attachments: []*discordgo.MessageAttachment{
			{ID: "a1", URL: "https://cdn.example.com/pic", Filename: "photo.webp"},
		},
	}}
	if _, refs, publishable := normalizeMessage(byExt, "bot-1"); !publishable || len(refs) != 1 {
		t.Fatalf("extension-only image: publishable=%v refs=%d", publishable, len(refs))
	}

	// A lone non-image attachment still publishes its annotation.
	junk := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID: "m-img-4", ChannelID: "chan-1",
		Author: &discordgo.User{ID: "U1"}, Type: discordgo.MessageTypeDefault,
		Attachments: []*discordgo.MessageAttachment{
			{ID: "a1", URL: "https://cdn.example.com/data.bin", Filename: "data.bin", ContentType: "application/octet-stream"},
		},
	}}
	if _, _, publishable := normalizeMessage(junk, "bot-1"); !publishable {
		t.Fatal("a lone non-image attachment must publish its [file: name] annotation")
	}
}

// TestInboundImageDownloadsBounded: the handler downloads image attachments
// through the governed transport with the shared bound — a good image
// lands as annotation plus media part, an oversize or failing one keeps
// only the annotation.
func TestInboundImageDownloadsBounded(t *testing.T) {
	oversize := append([]byte{0x89, 'P', 'N', 'G'}, bytes.Repeat([]byte{0x00}, 5<<20)...) // > 5 MiB
	var mode atomic.Value                                                                 // "ok" | "oversize" | "missing"
	mode.Store("ok")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch mode.Load() {
		case "ok":
			_, _ = w.Write(stubImageBytes)
		case "oversize":
			_, _ = w.Write(oversize)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	h := newHarness(t, validSettings)
	h.start(t)

	withImage := &discordgo.MessageCreate{Message: &discordgo.Message{
		ID: "m-dl-1", ChannelID: "chan-1", Content: "look",
		Author: &discordgo.User{ID: "U1"}, Type: discordgo.MessageTypeDefault,
		Attachments: []*discordgo.MessageAttachment{
			{ID: "a1", URL: srv.URL + "/pic.jpg", Filename: "pic.jpg", ContentType: "image/jpeg"},
		},
	}}

	// A good image arrives as annotation + bounded media part.
	h.dispatch(t, 0, withImage)
	waitFor(t, "downloaded envelope", func() bool { return len(h.env.snapshot()) == 1 })
	env1 := h.env.snapshot()[0]
	if len(env1.Parts) != 3 ||
		env1.Parts[0].Text != "look" ||
		env1.Parts[1].Text != "[image: pic.jpg]" ||
		env1.Parts[2].Kind != plugin.PartMedia ||
		string(env1.Parts[2].Media.Data) != string(stubImageBytes) ||
		env1.Parts[2].Media.MimeType != "image/jpeg" {
		t.Fatalf("envelope parts = %+v", env1.Parts)
	}

	// An over-bound image keeps the text and the annotation, drops bytes.
	h.env.reset()
	mode.Store("oversize")
	h.dispatch(t, 0, withImage)
	waitFor(t, "oversize envelope", func() bool { return len(h.env.snapshot()) == 1 })
	env2 := h.env.snapshot()[0]
	if len(env2.Parts) != 2 || env2.Parts[0].Text != "look" || env2.Parts[1].Text != "[image: pic.jpg]" {
		t.Fatalf("oversize envelope parts = %+v", env2.Parts)
	}

	// A failing download keeps the text and the annotation too.
	h.env.reset()
	mode.Store("missing")
	h.dispatch(t, 0, withImage)
	waitFor(t, "failed-download envelope", func() bool { return len(h.env.snapshot()) == 1 })
	env3 := h.env.snapshot()[0]
	if len(env3.Parts) != 2 || env3.Parts[0].Text != "look" || env3.Parts[1].Text != "[image: pic.jpg]" {
		t.Fatalf("failed-download envelope parts = %+v", env3.Parts)
	}
}

// TestSendMediaOneComplexSend: the batch of media parts leaves as ONE
// complex send with multipart files and no content (the reply text already
// went out through Send); a failed batch fails whole.
func TestSendMediaOneComplexSend(t *testing.T) {
	h := newHarness(t, validSettings)
	h.start(t)

	ids, err := h.p.SendMedia(context.Background(), "chan-9", []plugin.Part{
		{Kind: plugin.PartMedia, Media: plugin.Media{Name: "a.png", MimeType: "image/png", Data: []byte("png-bytes")}},
		{Kind: plugin.PartMedia, Media: plugin.Media{Name: "", MimeType: "image/jpeg"}}, // empty: skipped
		{Kind: plugin.PartText, Text: "not media"},                                      // ignored
		{Kind: plugin.PartMedia, Media: plugin.Media{Name: "b.jpg", MimeType: "image/jpeg", Data: []byte("jpg-bytes")}},
	})
	if err != nil {
		t.Fatalf("send media: %v", err)
	}
	if len(ids) != 1 || ids[0] != "sent-1" {
		t.Fatalf("ids = %v, want one complex send", ids)
	}
	calls := h.sender().sentCalls()
	if len(calls) != 1 {
		t.Fatalf("send client calls = %d, want 1", len(calls))
	}
	call := calls[0]
	if call.channelID != "chan-9" || call.content != "" {
		t.Fatalf("call = %+v, want empty content to chan-9", call)
	}
	if len(call.fileNames) != 2 || call.fileNames[0] != "a.png" || call.fileNames[1] != "b.jpg" {
		t.Fatalf("file names = %v, want both images", call.fileNames)
	}
	if ear := h.ear(0); len(ear.sentCalls()) != 0 {
		t.Fatalf("ear session carried media, want send-client only")
	}
}

// TestSendMediaFailClosed: not started fails closed; a send failure
// surfaces so the Host burns the delivery attempt.
func TestSendMediaFailClosed(t *testing.T) {
	p := newAdapter()
	if _, err := p.SendMedia(context.Background(), "chan-1", []plugin.Part{
		{Kind: plugin.PartMedia, Media: plugin.Media{Name: "a.png", Data: []byte("x")}},
	}); err == nil {
		t.Fatal("send media before start must fail closed")
	}

	h := newHarness(t, validSettings)
	h.start(t)
	h.sender().sendErr = errors.New("413 payload too large")
	if _, err := h.p.SendMedia(context.Background(), "chan-1", []plugin.Part{
		{Kind: plugin.PartMedia, Media: plugin.Media{Name: "a.png", MimeType: "image/png", Data: []byte("x")}},
	}); err == nil {
		t.Fatal("a failed complex send must surface")
	}
}
