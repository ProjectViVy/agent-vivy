package dingtalk

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"

	"agent-vivy/sdk/plugin"
)

// fakeEnv is an in-memory plugin.ChannelEnv: the same surface the kernel
// ChannelHost hands out, without the kernel. Secrets resolve from a map —
// mirroring the Host's CH-C6/D2 rule that any settings-declared *_env name
// may be resolved; published inbound envelopes are recorded.
type fakeEnv struct {
	settings  json.RawMessage
	secrets   map[string]string
	client    *http.Client
	openAPIHI string // host the factory saw, for assertions

	mu        sync.Mutex
	published []plugin.InboundMessage
}

func (e *fakeEnv) Secret(envKey string) (string, error) {
	v, ok := e.secrets[envKey]
	if !ok || v == "" {
		return "", fmt.Errorf("env variable %q empty or unset", envKey)
	}
	return v, nil
}

func (e *fakeEnv) HTTP() *http.Client { return e.client }

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

// fakeStream is an in-memory streamClient: it records the registered
// handler, counts successful Starts, and fails closed on demand.
type fakeStream struct {
	mu       sync.Mutex
	handler  chatbot.IChatBotMessageHandler
	starts   int
	closed   bool
	startErr error
	started  chan struct{}
	once     sync.Once
}

func newFakeStream(startErr error) *fakeStream {
	return &fakeStream{startErr: startErr, started: make(chan struct{})}
}

func (f *fakeStream) RegisterChatBotCallbackRouter(h chatbot.IChatBotMessageHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handler = h
}

func (f *fakeStream) Start(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.startErr != nil {
		return f.startErr
	}
	f.starts++
	f.once.Do(func() { close(f.started) })
	return nil
}

func (f *fakeStream) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

func (f *fakeStream) state() (handler chatbot.IChatBotMessageHandler, starts int, closed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.handler, f.starts, f.closed
}

// factorySpy wraps a factory and records what Start passed to it.
type factorySpy struct {
	gotCreds streamCreds
	gotHost  string
	f        func(creds streamCreds, openAPIHost string) streamClient
}

func (s *factorySpy) build(creds streamCreds, openAPIHost string) streamClient {
	s.gotCreds = creds
	s.gotHost = openAPIHost
	return s.f(creds, openAPIHost)
}

// botCredentials are synthetic: no real DingTalk app is involved anywhere.
const (
	stubClientID     = "ding-vivy-test-app-key"
	stubClientSecret = "ding-vivy-test-app-secret-value"
)

// envFor builds a fake env with resolvable credentials and the given
// settings JSON.
func envFor(t *testing.T, settings string) *fakeEnv {
	t.Helper()
	return &fakeEnv{
		settings: json.RawMessage(settings),
		secrets: map[string]string{
			stubClientID:     "app-key-value",
			stubClientSecret: "app-secret-value",
		},
		client: &http.Client{},
	}
}

// startWithFake runs Start against a fake stream client and registers a
// Stop cleanup; it returns the plugin and the fake.
func startWithFake(t *testing.T, env *fakeEnv, stream *fakeStream) (*Plugin, *factorySpy) {
	t.Helper()
	spy := &factorySpy{f: func(streamCreds, string) streamClient { return stream }}
	p := New().(*Plugin)
	p.newClient = spy.build
	if err := p.Start(context.Background(), env); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = p.Stop(ctx)
	})
	return p, spy
}

// waitFor polls until cond holds or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s never happened", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// textCallback builds a single-chat text callback model.
func textCallback() *chatbot.BotCallbackDataModel {
	return &chatbot.BotCallbackDataModel{
		ConversationId:   "cid-20:1",
		ConversationType: "1",
		SenderStaffId:    "manager1234",
		SenderNick:       "Manager",
		MsgId:            "msg-100",
		SessionWebhook:   "https://oapi.dingtalk.com/robot/send?access_token=sekret",
		Text:             chatbot.BotCallbackDataTextModel{Content: "hello vivy"},
		Msgtype:          "text",
	}
}

// webhookStub is a loopback stand-in for the DingTalk sessionWebhook
// robot endpoint. It records every POST and answers with a configurable
// ack.
type webhookStub struct {
	server *httptest.Server
	status int
	ack    map[string]any

	mu    sync.Mutex
	calls []webhookCall
}

type webhookCall struct {
	url        string
	msgType    string
	content    string
	rawBody    []byte
	httpClient string
}

func newWebhookStub(t *testing.T, status int, ack map[string]any) *webhookStub {
	t.Helper()
	stub := &webhookStub{status: status, ack: ack}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var decoded struct {
			MsgType string `json:"msgtype"`
			Text    struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		_ = json.Unmarshal(body, &decoded)
		stub.mu.Lock()
		stub.calls = append(stub.calls, webhookCall{
			url:        r.URL.String(),
			msgType:    decoded.MsgType,
			content:    decoded.Text.Content,
			rawBody:    body,
			httpClient: r.Header.Get("Content-Type"),
		})
		stub.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(stub.status)
		_ = json.NewEncoder(w).Encode(stub.ack)
	})
	stub.server = httptest.NewServer(mux)
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *webhookStub) callsSnapshot() []webhookCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]webhookCall(nil), s.calls...)
}

// TestDecodeSettings: valid decode, unknown field fails closed, absent or
// malformed settings fail closed, both env_key names survive.
func TestDecodeSettings(t *testing.T) {
	s, err := DecodeSettings(json.RawMessage(`{"client_id_env":"A","client_secret_env":"B","open_api_host":"http://127.0.0.1:1"}`))
	if err != nil {
		t.Fatalf("decode valid settings: %v", err)
	}
	if s.ClientIDEnv != "A" || s.ClientSecretEnv != "B" || s.OpenAPIHost != "http://127.0.0.1:1" {
		t.Fatalf("decoded settings = %+v", s)
	}

	if _, err := DecodeSettings(json.RawMessage(`{"client_id_env":"A","proxy":"p"}`)); err == nil {
		t.Fatal("unknown field must fail closed")
	}

	s, err = DecodeSettings(nil)
	if err != nil || s != (Settings{}) {
		t.Fatalf("absent settings = %+v err=%v, want zero value with no error", s, err)
	}

	if _, err := DecodeSettings(json.RawMessage(`not json`)); err == nil {
		t.Fatal("malformed settings must fail closed")
	}
	if _, err := DecodeSettings(json.RawMessage(`{"client_id_env":"A"} {"client_id_env":"B"}`)); err == nil {
		t.Fatal("trailing json documents must fail closed")
	}
}

// TestNormalizeCallback: exactly one shape publishes — a single-chat text
// message from a human that is not the bot and carries a message id.
// Everything else is dropped locally (the Host allow-list is the policy
// layer, not this shape filter).
func TestNormalizeCallback(t *testing.T) {
	msg, ok := normalizeCallback(textCallback())
	if !ok {
		t.Fatal("single-chat text callback must be publishable")
	}
	if msg.Channel != "dingtalk" || msg.ChatID != "cid-20:1" || msg.Sender != "dingtalk:manager1234" || msg.MessageID != "msg-100" {
		t.Fatalf("normalized envelope = %+v", msg)
	}
	if msg.ReplyTo != "" || msg.TopicID != "" {
		t.Fatalf("first cut must not thread replies or topics: %+v", msg)
	}
	if len(msg.Parts) != 1 || msg.Parts[0].Kind != plugin.PartText || msg.Parts[0].Text != "hello vivy" {
		t.Fatalf("envelope parts = %+v", msg.Parts)
	}

	// Sender falls back to SenderId when SenderStaffId is empty.
	noStaff := textCallback()
	noStaff.SenderStaffId = ""
	noStaff.SenderId = "sender-uid-9"
	msg, ok = normalizeCallback(noStaff)
	if !ok || msg.Sender != "dingtalk:sender-uid-9" {
		t.Fatalf("senderId fallback = %+v ok=%v", msg, ok)
	}

	// Single-chat conversation falls back to the sender when the
	// conversation id is missing.
	noConversation := textCallback()
	noConversation.ConversationId = ""
	msg, ok = normalizeCallback(noConversation)
	if !ok || msg.ChatID != "manager1234" {
		t.Fatalf("conversation fallback = %+v ok=%v", msg, ok)
	}

	// Text falls back to content.content when the typed text field is
	// empty.
	contentOnly := textCallback()
	contentOnly.Text.Content = ""
	contentOnly.Content = map[string]any{"content": "from content map"}
	msg, ok = normalizeCallback(contentOnly)
	if !ok || msg.Parts[0].Text != "from content map" {
		t.Fatalf("content map fallback = %+v ok=%v", msg, ok)
	}

	cases := map[string]*chatbot.BotCallbackDataModel{
		"nil callback":     nil,
		"group chat":       withField(textCallback(), func(m *chatbot.BotCallbackDataModel) { m.ConversationType = "2" }),
		"unknown type":     withField(textCallback(), func(m *chatbot.BotCallbackDataModel) { m.ConversationType = "" }),
		"empty text":       withField(textCallback(), func(m *chatbot.BotCallbackDataModel) { m.Text.Content = " " }),
		"non-text payload": withField(textCallback(), func(m *chatbot.BotCallbackDataModel) { m.Text.Content = ""; m.Content = "a plain string" }),
		"bot self echo": withField(textCallback(), func(m *chatbot.BotCallbackDataModel) {
			m.ChatbotUserId = "sender-uid-9"
			m.SenderId = "sender-uid-9"
			m.SenderStaffId = ""
		}),
		"missing sender": withField(textCallback(), func(m *chatbot.BotCallbackDataModel) { m.SenderStaffId = ""; m.SenderId = "" }),
		"missing msg id": withField(textCallback(), func(m *chatbot.BotCallbackDataModel) { m.MsgId = "" }),
	}
	for name, data := range cases {
		if msg, ok := normalizeCallback(data); ok {
			t.Fatalf("%s must not be publishable, got %+v", name, msg)
		}
	}
}

// withField clones a callback and applies a mutation, keeping the matrix
// rows independent.
func withField(m *chatbot.BotCallbackDataModel, mutate func(*chatbot.BotCallbackDataModel)) *chatbot.BotCallbackDataModel {
	clone := *m
	mutate(&clone)
	return &clone
}

// TestStartFailsClosed: settings, credential, and connection problems stop
// the ear before it goes live. All cases run against a fake stream client,
// so a leaky Start would succeed and fail the test.
func TestStartFailsClosed(t *testing.T) {
	newEnv := func(settings string, secrets map[string]string) *fakeEnv {
		e := envFor(t, settings)
		if secrets != nil {
			e.secrets = secrets
		}
		return e
	}
	fullSettings := `{"client_id_env":"ding-vivy-test-app-key","client_secret_env":"ding-vivy-test-app-secret-value"}`
	cases := map[string]struct {
		env      *fakeEnv
		startErr error
	}{
		"absent settings (no env_key names)": {env: newEnv(`{}`, nil)},
		"unknown settings field":             {env: newEnv(`{"client_id_env":"A","client_secret_env":"B","wat":1}`, nil)},
		"missing client_id_env":              {env: newEnv(`{"client_secret_env":"B"}`, nil)},
		"missing client_secret_env":          {env: newEnv(`{"client_id_env":"A"}`, nil)},
		"same env name for both credentials": {env: newEnv(`{"client_id_env":"SAME","client_secret_env":"SAME"}`, nil)},
		"missing app key variable": {
			env: newEnv(fullSettings, map[string]string{stubClientSecret: "s"}),
		},
		"missing app secret variable": {
			env: newEnv(fullSettings, map[string]string{stubClientID: "k"}),
		},
		"empty app key variable": {
			env: newEnv(fullSettings, map[string]string{stubClientID: "", stubClientSecret: "s"}),
		},
		"gateway rejects the connection": {
			env:      newEnv(fullSettings, nil),
			startErr: errors.New("AppCredentialConfigEmpty"),
		},
	}
	for name, tc := range cases {
		stream := newFakeStream(tc.startErr)
		spy := &factorySpy{f: func(streamCreds, string) streamClient { return stream }}
		p := New().(*Plugin)
		p.newClient = spy.build
		err := p.Start(context.Background(), tc.env)
		if err == nil {
			t.Fatalf("%s: Start must fail closed", name)
		}
		p.mu.Lock()
		live := p.stream != nil || p.cancel != nil
		p.mu.Unlock()
		if live {
			t.Fatalf("%s: failed Start must not leave a live ear", name)
		}
	}

	// The healthy env still starts, proving the failures above are each
	// case's own fault.
	startWithFake(t, envFor(t, fullSettings), newFakeStream(nil))
}

// TestStartPassesResolvedCredentials: the factory receives the resolved
// values (never the env names) and the configured gateway host.
func TestStartPassesResolvedCredentials(t *testing.T) {
	env := envFor(t, `{"client_id_env":"ding-vivy-test-app-key","client_secret_env":"ding-vivy-test-app-secret-value","open_api_host":"http://127.0.0.1:1"}`)
	stream := newFakeStream(nil)
	_, spy := startWithFake(t, env, stream)
	if spy.gotCreds.ClientID != "app-key-value" || spy.gotCreds.ClientSecret != "app-secret-value" {
		t.Fatalf("factory credentials = %+v", spy.gotCreds)
	}
	if spy.gotHost != "http://127.0.0.1:1" {
		t.Fatalf("factory open api host = %q", spy.gotHost)
	}
}

// TestStartStopFullLoop: the whole Start → callback → PublishInbound →
// Send → Stop path against the fake stream client — Start registers the
// callback before going live, the callback publishes a normalized
// envelope and stores its session webhook, Send delivers through the
// webhook, and Stop sheds the supervisor promptly and idempotently.
func TestStartStopFullLoop(t *testing.T) {
	webhook := newWebhookStub(t, http.StatusOK, map[string]any{"errcode": 0, "errmsg": "ok"})
	env := envFor(t, `{"client_id_env":"ding-vivy-test-app-key","client_secret_env":"ding-vivy-test-app-secret-value"}`)
	stream := newFakeStream(nil)
	p, _ := startWithFake(t, env, stream)

	handler, starts, _ := stream.state()
	if handler == nil {
		t.Fatal("Start must register the chatbot callback")
	}
	if starts != 1 {
		t.Fatalf("starts = %d, want exactly the one connect from Start", starts)
	}

	data := textCallback()
	data.SessionWebhook = webhook.server.URL + "/robot/send?access_token=sekret"
	if _, err := handler(context.Background(), data); err != nil {
		t.Fatalf("callback: %v", err)
	}
	waitFor(t, "first inbound envelope", func() bool { return len(env.snapshot()) == 1 })
	got := env.snapshot()[0]
	if got.Channel != "dingtalk" || got.ChatID != "cid-20:1" || got.Sender != "dingtalk:manager1234" ||
		len(got.Parts) != 1 || got.Parts[0].Text != "hello vivy" {
		t.Fatalf("published envelope = %+v", got)
	}

	// Send rides the stored session webhook.
	ids, err := p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "cid-20:1",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "hello human"}},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(ids) != 1 || ids[0] != "cid-20:1" {
		t.Fatalf("send ids = %v, want [cid-20:1]", ids)
	}
	calls := webhook.callsSnapshot()
	if len(calls) != 1 {
		t.Fatalf("webhook calls = %d, want 1", len(calls))
	}
	if !strings.Contains(calls[0].url, "access_token=sekret") {
		t.Fatalf("webhook call url = %q, want the stored session webhook", calls[0].url)
	}

	// Stop ends the loop promptly and is idempotent.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("second stop: %v", err)
	}
	if err := (New().(*Plugin)).Stop(context.Background()); err != nil {
		t.Fatalf("stop without start: %v", err)
	}
	if _, starts, closed := stream.state(); !closed || starts != 1 {
		t.Fatalf("after stop: starts=%d closed=%v, want 1 start and closed client", starts, closed)
	}
	// The supervisor exited inside Stop (Stop waits on it), so no redial
	// can follow.
	time.Sleep(50 * time.Millisecond)
	if _, starts, _ := stream.state(); starts != 1 {
		t.Fatalf("starts after stop = %d, want no redial", starts)
	}
}

// TestHandlerFencedAfterStop: the SDK dispatches frames on its own
// goroutines, so a callback read before Close can reach the handler after
// Stop returned. The handler itself must fence: nothing remembered,
// nothing published.
func TestHandlerFencedAfterStop(t *testing.T) {
	env := envFor(t, `{"client_id_env":"ding-vivy-test-app-key","client_secret_env":"ding-vivy-test-app-secret-value"}`)
	stream := newFakeStream(nil)
	p, _ := startWithFake(t, env, stream)
	handler, _, _ := stream.state()

	// Latch stopped exactly the way Stop does, without tearing the fake
	// down: the fence lives in the handler, not in Stop's cleanup.
	p.mu.Lock()
	p.stopped = true
	p.mu.Unlock()

	if _, err := handler(context.Background(), textCallback()); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if got := len(env.snapshot()); got != 0 {
		t.Fatalf("published envelopes after stop = %d, want 0", got)
	}
	if _, ok := p.webhookFor("cid-20:1"); ok {
		t.Fatal("session webhook must not be remembered after stop")
	}
}

// TestStartAfterStopStartsFresh: starting again on the same instance is a
// new ear — Start resets the stopped latch, so the restarted ear publishes
// instead of having every callback fenced by the previous ear's Stop (qq
// pattern, review L3-F2).
func TestStartAfterStopStartsFresh(t *testing.T) {
	env := envFor(t, `{"client_id_env":"ding-vivy-test-app-key","client_secret_env":"ding-vivy-test-app-secret-value"}`)
	stream := newFakeStream(nil)
	p, _ := startWithFake(t, env, stream)
	handler, _, _ := stream.state()

	// Traffic on the first ear leaves published state behind.
	if _, err := handler(context.Background(), textCallback()); err != nil {
		t.Fatalf("callback: %v", err)
	}
	waitFor(t, "first inbound envelope", func() bool { return len(env.snapshot()) == 1 })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// The restart swaps in a fresh fake client; the same plugin instance
	// must publish through it (no carried fence, no carried webhook drop).
	second := newFakeStream(nil)
	p.newClient = func(streamCreds, string) streamClient { return second }
	if err := p.Start(context.Background(), env); err != nil {
		t.Fatalf("restart: %v", err)
	}
	handler2, _, _ := second.state()
	if handler2 == nil {
		t.Fatal("restart must register the chatbot callback")
	}
	if _, err := handler2(context.Background(), textCallback()); err != nil {
		t.Fatalf("restarted callback: %v", err)
	}
	waitFor(t, "inbound envelope on the restarted ear", func() bool { return len(env.snapshot()) == 2 })
}

// TestWebhookLatestWins: the latest session webhook per conversation is
// the one Send uses.
func TestWebhookLatestWins(t *testing.T) {
	first := newWebhookStub(t, http.StatusOK, map[string]any{"errcode": 0})
	second := newWebhookStub(t, http.StatusOK, map[string]any{"errcode": 0})
	env := envFor(t, `{"client_id_env":"ding-vivy-test-app-key","client_secret_env":"ding-vivy-test-app-secret-value"}`)
	stream := newFakeStream(nil)
	p, _ := startWithFake(t, env, stream)

	handler, _, _ := stream.state()
	for _, webhook := range []*webhookStub{first, second} {
		data := textCallback()
		data.SessionWebhook = webhook.server.URL + "/robot/send"
		if _, err := handler(context.Background(), data); err != nil {
			t.Fatalf("callback: %v", err)
		}
	}

	if _, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "cid-20:1", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if got := len(first.callsSnapshot()); got != 0 {
		t.Fatalf("stale webhook called %d times, want 0", got)
	}
	if got := len(second.callsSnapshot()); got != 1 {
		t.Fatalf("latest webhook called %d times, want 1", got)
	}
}

// TestSendPlainText: Send delivers text parts as one sessionWebhook POST
// each, with the DingTalk robot text payload shape, and skips non-text
// parts.
func TestSendPlainText(t *testing.T) {
	webhook := newWebhookStub(t, http.StatusOK, map[string]any{"errcode": 0, "errmsg": "ok"})
	env := envFor(t, `{"client_id_env":"ding-vivy-test-app-key","client_secret_env":"ding-vivy-test-app-secret-value"}`)
	stream := newFakeStream(nil)
	p, _ := startWithFake(t, env, stream)

	// Send for a chat that never messaged the bot fails closed.
	if _, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "unknown", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}}); err == nil {
		t.Fatal("send without a stored session webhook must fail closed")
	} else if !strings.Contains(err.Error(), "no session webhook") {
		t.Fatalf("unknown chat error = %v", err)
	}

	handler, _, _ := stream.state()
	data := textCallback()
	data.SessionWebhook = webhook.server.URL + "/robot/send?access_token=sekret"
	if _, err := handler(context.Background(), data); err != nil {
		t.Fatalf("callback: %v", err)
	}

	ids, err := p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "cid-20:1",
		Parts: []plugin.Part{
			{Kind: plugin.PartText, Text: "first"},
			{Kind: plugin.PartMediaRef, MediaRef: "noop:x"}, // skipped: text-only this slice
			{Kind: plugin.PartText, Text: "second"},
		},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(ids) != 2 || ids[0] != "cid-20:1" || ids[1] != "cid-20:1" {
		t.Fatalf("send ids = %v, want the chat id twice", ids)
	}
	calls := webhook.callsSnapshot()
	if len(calls) != 2 {
		t.Fatalf("webhook calls = %d, want 2", len(calls))
	}
	for i, want := range []string{"first", "second"} {
		call := calls[i]
		if call.msgType != "text" || call.content != want {
			t.Fatalf("webhook call %d = %+v, want text %q in robot text shape", i, call, want)
		}
		if call.httpClient != "application/json" {
			t.Fatalf("webhook call %d content type = %q", i, call.httpClient)
		}
		if !strings.Contains(call.url, "access_token=sekret") {
			t.Fatalf("webhook call %d url = %q, want the stored session webhook", i, call.url)
		}
	}

	// An envelope with no text parts sends nothing.
	if ids, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "cid-20:1", Parts: []plugin.Part{{Kind: plugin.PartMediaRef, MediaRef: "noop:x"}}}); err != nil || len(ids) != 0 {
		t.Fatalf("media-only envelope = ids %v err %v, want no send and no error", ids, err)
	}
	if got := len(webhook.callsSnapshot()); got != 2 {
		t.Fatalf("webhook calls = %d, want still 2", got)
	}
}

// TestSendSurfacesWebhookError: platform failures surface to the Host as
// Send errors — non-zero errcode on HTTP 200, non-200 status, and a dead
// webhook — and transport errors never carry the session webhook's token
// (D-010).
func TestSendSurfacesWebhookError(t *testing.T) {
	erroring := newWebhookStub(t, http.StatusOK, map[string]any{"errcode": 310000, "errmsg": "sign not match"})
	failing := newWebhookStub(t, http.StatusInternalServerError, map[string]any{"errcode": 0})

	env := envFor(t, `{"client_id_env":"ding-vivy-test-app-key","client_secret_env":"ding-vivy-test-app-secret-value"}`)
	stream := newFakeStream(nil)
	p, _ := startWithFake(t, env, stream)
	handler, _, _ := stream.state()

	stubs := map[string]*webhookStub{"errcode": erroring, "status": failing}
	for name, webhook := range stubs {
		data := textCallback()
		data.SessionWebhook = webhook.server.URL + "/robot/send?access_token=sekret"
		if _, err := handler(context.Background(), data); err != nil {
			t.Fatalf("%s: callback: %v", name, err)
		}
		if _, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "cid-20:1", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}}); err == nil {
			t.Fatalf("%s: send must surface the webhook failure", name)
		} else if strings.Contains(err.Error(), "sekret") {
			t.Fatalf("%s: error leaks the session webhook token: %v", name, err)
		}
	}

	// errcode content reaches the error message.
	data := textCallback()
	data.SessionWebhook = erroring.server.URL + "/robot/send?access_token=sekret"
	if _, err := handler(context.Background(), data); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if _, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "cid-20:1", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}}); err == nil || !strings.Contains(err.Error(), "310000") {
		t.Fatalf("errcode error = %v, want errcode 310000 surfaced", err)
	}

	// Malformed session webhooks: request-construction and transport
	// errors must carry neither the token nor the raw URL, but keep the
	// cause. A non-numeric port fails at URL parse time, where the
	// redaction fallback drops the URL entirely.
	parseBroken := textCallback()
	parseBroken.SessionWebhook = "http://127.0.0.1:bad/robot/send?access_token=sekret-parse"
	if _, err := handler(context.Background(), parseBroken); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if _, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "cid-20:1", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}}); err == nil {
		t.Fatal("send against a malformed webhook must surface an error")
	} else if strings.Contains(err.Error(), "sekret-parse") || strings.Contains(err.Error(), "robot/send") {
		t.Fatalf("malformed-webhook error leaks the session webhook URL: %v", err)
	} else if !strings.Contains(err.Error(), "invalid port") {
		t.Fatalf("malformed-webhook error lost its cause: %v", err)
	}

	// An out-of-range numeric port parses fine and fails at dial time; the
	// transport path keeps the (now tokenless) host and path.
	dialBroken := textCallback()
	dialBroken.SessionWebhook = "http://127.0.0.1:70000/robot/send?access_token=sekret-dial"
	if _, err := handler(context.Background(), dialBroken); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if _, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "cid-20:1", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}}); err == nil {
		t.Fatal("send against an undialable webhook must surface an error")
	} else if strings.Contains(err.Error(), "sekret-dial") {
		t.Fatalf("transport error leaks the session webhook token: %v", err)
	}

	// A dead webhook: the transport error keeps its cause but loses the
	// tokenized query string.
	dead := textCallback()
	dead.SessionWebhook = erroring.server.URL + "/robot/send?access_token=sekret-session-token"
	erroring.server.Close()
	if _, err := handler(context.Background(), dead); err != nil {
		t.Fatalf("callback: %v", err)
	}
	if _, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "cid-20:1", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}}); err == nil {
		t.Fatal("send against a dead webhook must surface an error")
	} else if strings.Contains(err.Error(), "sekret-session-token") {
		t.Fatalf("transport error leaks the session webhook token: %v", err)
	}
}

// --- Real-SDK loopback integration (no DingTalk network) ---

// wsGUID is the WebSocket handshake magic string (RFC 6455 §1.3).
const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// streamStub is a loopback stand-in for the DingTalk OpenAPI gateway: it
// answers the connection-ticket POST and then serves the websocket the
// SDK dials. Framing is hand-rolled with the standard library only — the
// gorilla/websocket dependency stays with the SDK, not this module.
type streamStub struct {
	server     *httptest.Server
	wsOpened   chan struct{}
	acked      chan payload.DataFrameResponse
	robotPosts chan webhookCall
	tickets    sync.Mutex
	ticketHits int
	wsOnce     sync.Once
}

func newStreamStub(t *testing.T) *streamStub {
	t.Helper()
	stub := &streamStub{
		wsOpened:   make(chan struct{}),
		acked:      make(chan payload.DataFrameResponse, 1),
		robotPosts: make(chan webhookCall, 8),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0/gateway/connections/open", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		stub.tickets.Lock()
		stub.ticketHits++
		stub.tickets.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"endpoint": "ws://" + r.Host + "/ws",
			"ticket":   "ticket-1",
		})
	})
	mux.HandleFunc("/robot/send", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var decoded struct {
			MsgType string `json:"msgtype"`
			Text    struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		_ = json.Unmarshal(body, &decoded)
		stub.robotPosts <- webhookCall{url: r.URL.String(), msgType: decoded.MsgType, content: decoded.Text.Content}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "errmsg": "ok"})
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn := hijackWebSocket(t, w, r)
		defer conn.Close()
		stub.wsOnce.Do(func() { close(stub.wsOpened) })

		// One chatbot callback frame, as the DingTalk gateway would push
		// after a user messages the robot in a single chat.
		botMsg, _ := json.Marshal(map[string]any{
			"conversationId":   "cid-loopback",
			"conversationType": "1",
			"senderStaffId":    "manager1234",
			"msgId":            "msg-200",
			"sessionWebhook":   stub.server.URL + "/robot/send?access_token=sekret-loopback",
			"text":             map[string]string{"content": "hello vivy"},
			"msgtype":          "text",
			"createAt":         time.Now().UnixMilli(),
		})
		frame, _ := json.Marshal(map[string]any{
			"specVersion": "1.0",
			"type":        "CALLBACK",
			"time":        time.Now().UnixMilli(),
			"headers": map[string]string{
				"topic":       payload.BotMessageCallbackTopic,
				"messageId":   "frame-1",
				"contentType": "application/json",
			},
			"data": string(botMsg),
		})
		if err := writeWSTextFrame(conn, frame); err != nil {
			t.Errorf("write data frame: %v", err)
			return
		}

		// The SDK acks every data frame; capture the ack to prove the
		// whole codec round-trip worked.
		opcode, body, err := readWSFrame(conn)
		if err != nil {
			t.Errorf("read ack frame: %v", err)
			return
		}
		if opcode != 0x1 {
			t.Errorf("ack opcode = %#x, want a text frame", opcode)
			return
		}
		ack, err := payload.DecodeDataFrameResponse(body)
		if err != nil {
			t.Errorf("decode ack: %v", err)
			return
		}
		select {
		case stub.acked <- *ack:
		default:
		}

		// Hold the socket open until the client goes away.
		for {
			if _, _, err := readWSFrame(conn); err != nil {
				return
			}
		}
	})
	stub.server = httptest.NewServer(mux)
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *streamStub) ticketCount() int {
	s.tickets.Lock()
	defer s.tickets.Unlock()
	return s.ticketHits
}

// hijackWebSocket performs the RFC 6455 server handshake over a hijacked
// connection and returns it.
func hijackWebSocket(t *testing.T, w http.ResponseWriter, r *http.Request) net.Conn {
	t.Helper()
	hj, ok := w.(http.Hijacker)
	if !ok {
		t.Fatal("response writer does not support hijacking")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		t.Fatalf("hijack: %v", err)
	}
	h := sha1.New()
	_, _ = io.WriteString(h, r.Header.Get("Sec-WebSocket-Key")+wsGUID)
	accept := base64.StdEncoding.EncodeToString(h.Sum(nil))
	fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept)
	_ = rw.Flush()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	return conn
}

// writeWSTextFrame writes one unmasked server-to-client text frame.
func writeWSTextFrame(conn net.Conn, payload []byte) error {
	header := []byte{0x81} // FIN + text opcode
	n := len(payload)
	switch {
	case n < 126:
		header = append(header, byte(n))
	case n < 1<<16:
		header = append(header, 126, byte(n>>8), byte(n))
	default:
		return errors.New("frame too large for the stub")
	}
	_, err := conn.Write(append(header, payload...))
	return err
}

// readWSFrame reads one client-to-server frame and unmasks it.
func readWSFrame(conn net.Conn) (opcode byte, body []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(conn, h[:]); err != nil {
		return 0, nil, err
	}
	opcode = h[0] & 0x0f
	length := int(h[1] & 0x7f)
	switch length {
	case 126:
		var ext [2]byte
		if _, err = io.ReadFull(conn, ext[:]); err != nil {
			return 0, nil, err
		}
		length = int(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err = io.ReadFull(conn, ext[:]); err != nil {
			return 0, nil, err
		}
		length = int(binary.BigEndian.Uint64(ext[:]))
	}
	var mask [4]byte
	if _, err = io.ReadFull(conn, mask[:]); err != nil {
		return 0, nil, err
	}
	body = make([]byte, length)
	if _, err = io.ReadFull(conn, body); err != nil {
		return 0, nil, err
	}
	for i := range body {
		body[i] ^= mask[i%4]
	}
	return opcode, body, nil
}

// TestStreamLoopbackLifecycle runs the production stream client against a
// loopback gateway: ticket exchange, websocket connect, one real callback
// frame through the SDK codec, PublishInbound, Send through the session
// webhook, then a Stop that closes the socket and leaves no redial behind.
func TestStreamLoopbackLifecycle(t *testing.T) {
	stub := newStreamStub(t)
	env := envFor(t, fmt.Sprintf(`{"client_id_env":"ding-vivy-test-app-key","client_secret_env":"ding-vivy-test-app-secret-value","open_api_host":%q}`, stub.server.URL))

	p := New().(*Plugin) // production factory: the real SDK client
	if err := p.Start(context.Background(), env); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = p.Stop(ctx)
	})

	waitFor(t, "websocket open", func() bool {
		select {
		case <-stub.wsOpened:
			return true
		default:
			return false
		}
	})
	waitFor(t, "inbound envelope from the callback frame", func() bool { return len(env.snapshot()) == 1 })
	got := env.snapshot()[0]
	if got.Channel != "dingtalk" || got.ChatID != "cid-loopback" || got.Sender != "dingtalk:manager1234" ||
		got.MessageID != "msg-200" || len(got.Parts) != 1 || got.Parts[0].Text != "hello vivy" {
		t.Fatalf("published envelope = %+v", got)
	}

	ids, err := p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "cid-loopback",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "hello human"}},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(ids) != 1 || ids[0] != "cid-loopback" {
		t.Fatalf("send ids = %v", ids)
	}
	select {
	case call := <-stub.robotPosts:
		if call.msgType != "text" || call.content != "hello human" {
			t.Fatalf("robot post = %+v", call)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("session webhook was never called")
	}

	// The SDK acked the data frame: the codec round-trip completed.
	select {
	case ack := <-stub.acked:
		if ack.Code != payload.DataFrameResponseStatusCodeKOK {
			t.Fatalf("ack code = %d, want %d", ack.Code, payload.DataFrameResponseStatusCodeKOK)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("data frame was never acked")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := stub.ticketCount(); got != 1 {
		t.Fatalf("ticket exchanges = %d, want exactly the initial one (no redial after stop)", got)
	}
}
