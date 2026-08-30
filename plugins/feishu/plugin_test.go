package feishu

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
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"agent-vivy/sdk/plugin"
)

// Synthetic test credentials. No real Feishu/Lark app is involved anywhere;
// every network touch in this file lands on a loopback httptest server.
const (
	stubAppIDEnvName     = "VIVY_TEST_FEISHU_APP_ID"
	stubAppSecretEnvName = "VIVY_TEST_FEISHU_APP_SECRET"
	stubAppSecretValue   = "vivy-test-app-secret-value"
	stubTenantToken      = "t-vivy-test-tenant-token"
	stubMessageID        = "om_vivy_test_message"
)

// setCredentials plants the two test credential variables the way the
// Host's environment lookup would see them. The app id is unique per
// test: the SDK's tenant-token cache is a package-level singleton keyed
// by app id, so a unique id keeps tests from sharing cached tokens.
func setCredentials(t *testing.T) string {
	t.Helper()
	appID := fmt.Sprintf("cli-vivy-test-%d", testAppIDSeq.Add(1))
	t.Setenv(stubAppIDEnvName, appID)
	t.Setenv(stubAppSecretEnvName, stubAppSecretValue)
	return appID
}

// testAppIDSeq hands every test its own app id (see setCredentials).
var testAppIDSeq atomic.Int64

// fakeEnv is an in-memory plugin.ChannelEnv: the same surface the kernel
// ChannelHost hands out, with Secret resolving through the process
// environment exactly like the Host does (t.Setenv plants or empties the
// variables). Published inbound envelopes are recorded. appID carries the
// planted app id value for assertions.
type fakeEnv struct {
	appID    string
	settings json.RawMessage
	client   *http.Client

	mu        sync.Mutex
	published []plugin.InboundMessage
}

func (e *fakeEnv) Secret(envKey string) (string, error) {
	v, ok := os.LookupEnv(envKey)
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

// envFor builds a fake env with the standard credential variables planted
// and the given settings JSON.
func envFor(t *testing.T, settings string) *fakeEnv {
	t.Helper()
	return &fakeEnv{
		appID:    setCredentials(t),
		settings: json.RawMessage(settings),
		client:   &http.Client{},
	}
}

// fakeWS is an in-memory wsClient: it records the event handler and the
// factory inputs, fires the ready callback once connected, and blocks in
// Start until the context ends or the run is dropped — the observable
// behavior of the real SDK client with auto-reconnect off. It fails
// closed on demand.
type fakeWS struct {
	mu       sync.Mutex
	onEvent  eventFunc
	onReady  func()
	creds    wsCreds
	domain   string
	starts   int
	closed   bool
	startErr error

	readyOnce sync.Once
	ready     chan struct{}

	dropOnce sync.Once
	dropped  chan struct{}
	dropErr  error
}

func newFakeWS(startErr error) *fakeWS {
	return &fakeWS{startErr: startErr, ready: make(chan struct{}), dropped: make(chan struct{})}
}

func (f *fakeWS) SetOnReady(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onReady = fn
}

func (f *fakeWS) Start(ctx context.Context) error {
	f.mu.Lock()
	f.starts++
	err := f.startErr
	fn := f.onReady
	f.mu.Unlock()
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if fn != nil {
		fn()
	}
	f.readyOnce.Do(func() { close(f.ready) })
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-f.dropped:
		return f.dropErr
	}
}

func (f *fakeWS) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

// cancelRun simulates the event gateway dropping a healthy connection:
// the blocked Start returns with the given error while the plugin's own
// context stays live.
func (f *fakeWS) cancelRun(err error) {
	f.dropOnce.Do(func() {
		f.mu.Lock()
		f.dropErr = err
		f.mu.Unlock()
		close(f.dropped)
	})
}

// setOnEvent wires the event handler the factory receives, mirroring how
// the production factory bakes it into the SDK dispatcher.
func (f *fakeWS) setOnEvent(fn eventFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onEvent = fn
}

func (f *fakeWS) state() (onEvent eventFunc, starts int, closed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.onEvent, f.starts, f.closed
}

func (f *fakeWS) isReady() bool {
	select {
	case <-f.ready:
		return true
	default:
		return false
	}
}

// wsFactorySpy wraps a websocket factory and records what Start passed
// into it.
type wsFactorySpy struct {
	mu         sync.Mutex
	gotOnEvent eventFunc
	gotCreds   wsCreds
	gotDomain  string
	f          func(onEvent eventFunc, creds wsCreds, domain string) wsClient
}

func (s *wsFactorySpy) build(onEvent eventFunc, creds wsCreds, domain string) wsClient {
	s.mu.Lock()
	s.gotOnEvent, s.gotCreds, s.gotDomain = onEvent, creds, domain
	s.mu.Unlock()
	return s.f(onEvent, creds, domain)
}

func (s *wsFactorySpy) snapshot() (eventFunc, wsCreds, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gotOnEvent, s.gotCreds, s.gotDomain
}

// apiFactorySpy wraps the OpenAPI client factory and records its inputs.
type apiFactorySpy struct {
	mu        sync.Mutex
	gotCreds  wsCreds
	gotDomain string
	f         func(creds wsCreds, domain string) *lark.Client
}

func (s *apiFactorySpy) build(creds wsCreds, domain string) *lark.Client {
	s.mu.Lock()
	s.gotCreds, s.gotDomain = creds, domain
	s.mu.Unlock()
	return s.f(creds, domain)
}

func (s *apiFactorySpy) snapshot() (wsCreds, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gotCreds, s.gotDomain
}

// startWithFake runs Start against a fake websocket client and registers
// a Stop cleanup; it returns the plugin and the fake.
func startWithFake(t *testing.T, env *fakeEnv, ws *fakeWS) (*Plugin, *wsFactorySpy) {
	t.Helper()
	spy := &wsFactorySpy{f: func(onEvent eventFunc, _ wsCreds, _ string) wsClient {
		ws.setOnEvent(onEvent)
		return ws
	}}
	p := New().(*Plugin)
	p.newWS = spy.build
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

// withField clones an event deeply enough that matrix rows mutate their
// own copy, never a shared SDK model.
func withField(m *larkim.P2MessageReceiveV1, mutate func(*larkim.P2MessageReceiveV1)) *larkim.P2MessageReceiveV1 {
	if m == nil {
		return nil
	}
	clone := &larkim.P2MessageReceiveV1{Event: &larkim.P2MessageReceiveV1Data{}}
	if m.Event != nil {
		*clone.Event = *m.Event
		if m.Event.Message != nil {
			msg := *m.Event.Message
			clone.Event.Message = &msg
		}
		if m.Event.Sender != nil {
			sender := *m.Event.Sender
			clone.Event.Sender = &sender
			if m.Event.Sender.SenderId != nil {
				ids := *m.Event.Sender.SenderId
				clone.Event.Sender.SenderId = &ids
			}
		}
	}
	mutate(clone)
	return clone
}

func ptr(s string) *string { return &s }

// p2pTextEvent builds a p2p text receive event, the one shape this slice
// publishes.
func p2pTextEvent() *larkim.P2MessageReceiveV1 {
	return &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId:   &larkim.UserId{OpenId: ptr("ou_manager1234")},
				SenderType: ptr("user"),
			},
			Message: &larkim.EventMessage{
				MessageId:   ptr("om_100"),
				ChatId:      ptr("oc_chat_1"),
				ChatType:    ptr("p2p"),
				MessageType: ptr("text"),
				Content:     ptr(`{"text":"hello vivy"}`),
			},
		},
	}
}

// TestDecodeSettings: valid decode, unknown field fails closed, absent or
// malformed settings fail closed, every knob survives.
func TestDecodeSettings(t *testing.T) {
	raw := `{"app_id_env":"A","app_secret_env":"B","encrypt_key":"key","is_lark":true,"open_base_url":"http://127.0.0.1:1"}`
	s, err := DecodeSettings(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("decode valid settings: %v", err)
	}
	if s.AppIDEnv != "A" || s.AppSecretEnv != "B" || s.EncryptKey != "key" || !s.IsLark || s.OpenBaseURL != "http://127.0.0.1:1" {
		t.Fatalf("decoded settings = %+v", s)
	}

	if _, err := DecodeSettings(json.RawMessage(`{"app_id_env":"A","proxy":"p"}`)); err == nil {
		t.Fatal("unknown field must fail closed")
	}

	s, err = DecodeSettings(nil)
	if err != nil || s != (Settings{}) {
		t.Fatalf("absent settings = %+v err=%v, want zero value with no error", s, err)
	}

	if _, err := DecodeSettings(json.RawMessage(`not json`)); err == nil {
		t.Fatal("malformed settings must fail closed")
	}
	if _, err := DecodeSettings(json.RawMessage(`{"app_id_env":"A"} {"app_id_env":"B"}`)); err == nil {
		t.Fatal("trailing json documents must fail closed")
	}
}

// TestDomainFor: the open_base_url override wins, then the is_lark switch,
// then the Feishu default.
func TestDomainFor(t *testing.T) {
	if got := domainFor(Settings{}); got != lark.FeishuBaseUrl {
		t.Fatalf("default domain = %q, want %q", got, lark.FeishuBaseUrl)
	}
	if got := domainFor(Settings{IsLark: true}); got != lark.LarkBaseUrl {
		t.Fatalf("is_lark domain = %q, want %q", got, lark.LarkBaseUrl)
	}
	if got := domainFor(Settings{IsLark: true, OpenBaseURL: "http://127.0.0.1:1"}); got != "http://127.0.0.1:1" {
		t.Fatalf("open_base_url override = %q, want the override", got)
	}
}

// TestNormalizeEvent: exactly one shape publishes — a p2p text message
// from a human that carries a sender, message id, and chat id. Everything
// else is dropped locally (the Host allow-list is the policy layer, not
// this shape filter). Reaction events are separate event types and never
// reach this handler.
func TestNormalizeEvent(t *testing.T) {
	msg, ok := normalizeEvent(p2pTextEvent())
	if !ok {
		t.Fatal("p2p text event must be publishable")
	}
	if msg.Channel != "feishu" || msg.ChatID != "oc_chat_1" || msg.Sender != "feishu:ou_manager1234" || msg.MessageID != "om_100" {
		t.Fatalf("normalized envelope = %+v", msg)
	}
	if msg.ReplyTo != "" || msg.TopicID != "" {
		t.Fatalf("first cut must not thread replies or topics: %+v", msg)
	}
	if len(msg.Parts) != 1 || msg.Parts[0].Kind != plugin.PartText || msg.Parts[0].Text != "hello vivy" {
		t.Fatalf("envelope parts = %+v", msg.Parts)
	}

	// Sender falls back to user_id, then union_id, when open_id is empty.
	noOpen := withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) {
		m.Event.Sender.SenderId.OpenId = nil
		m.Event.Sender.SenderId.UserId = ptr("u_88")
	})
	if msg, ok = normalizeEvent(noOpen); !ok || msg.Sender != "feishu:u_88" {
		t.Fatalf("user_id fallback = %+v ok=%v", msg, ok)
	}
	noOpen = withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) {
		m.Event.Sender.SenderId.OpenId = nil
		m.Event.Sender.SenderId.UserId = nil
		m.Event.Sender.SenderId.UnionId = ptr("un_99")
	})
	if msg, ok = normalizeEvent(noOpen); !ok || msg.Sender != "feishu:un_99" {
		t.Fatalf("union_id fallback = %+v ok=%v", msg, ok)
	}

	cases := map[string]*larkim.P2MessageReceiveV1{
		"nil event":            nil,
		"nil event body":       withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event = nil }),
		"nil message":          withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Message = nil }),
		"group chat":           withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Message.ChatType = ptr("group") }),
		"unknown chat type":    withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Message.ChatType = nil }),
		"non-text message":     withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Message.MessageType = ptr("image") }),
		"post message":         withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Message.MessageType = ptr("post") }),
		"missing message type": withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Message.MessageType = nil }),
		"bot sender":           withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Sender.SenderType = ptr("bot") }),
		"empty text":           withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Message.Content = ptr(`{"text":"  "}`) }),
		"content without text": withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Message.Content = ptr(`{"image_key":"img_1"}`) }),
		"malformed content":    withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Message.Content = ptr(`not json`) }),
		"empty content":        withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Message.Content = nil }),
		"nil sender":           withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Sender = nil }),
		"missing sender id":    withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Sender.SenderId = nil }),
		"all sender ids empty": withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Sender.SenderId = &larkim.UserId{OpenId: ptr(" ")} }),
		"missing message id":   withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Message.MessageId = nil }),
		"missing chat id":      withField(p2pTextEvent(), func(m *larkim.P2MessageReceiveV1) { m.Event.Message.ChatId = nil }),
	}
	for name, event := range cases {
		if msg, ok := normalizeEvent(event); ok {
			t.Fatalf("%s must not be publishable, got %+v", name, msg)
		}
	}
}

// TestStartFailsClosed: settings, credential, and connection problems stop
// the ear before it goes live. All cases run against a fake websocket
// client, so a leaky Start would succeed and fail the test.
func TestStartFailsClosed(t *testing.T) {
	fullSettings := `{"app_id_env":"` + stubAppIDEnvName + `","app_secret_env":"` + stubAppSecretEnvName + `"}`
	cases := map[string]struct {
		settings string
		plantEnv func(t *testing.T)
		startErr error
		deadCtx  bool
	}{
		"absent settings (no env_key names)": {settings: `{}`, plantEnv: func(t *testing.T) { setCredentials(t) }},
		"unknown settings field":             {settings: `{"app_id_env":"A","app_secret_env":"B","wat":1}`, plantEnv: func(t *testing.T) { setCredentials(t) }},
		"missing app_id_env":                 {settings: `{"app_secret_env":"B"}`, plantEnv: func(t *testing.T) { setCredentials(t) }},
		"missing app_secret_env":             {settings: `{"app_id_env":"A"}`, plantEnv: func(t *testing.T) { setCredentials(t) }},
		"same env name for both credentials": {settings: `{"app_id_env":"SAME","app_secret_env":"SAME"}`, plantEnv: func(t *testing.T) { setCredentials(t) }},
		"missing app id variable": {settings: fullSettings, plantEnv: func(t *testing.T) {
			t.Setenv(stubAppSecretEnvName, stubAppSecretValue)
		}},
		"empty app id variable": {settings: fullSettings, plantEnv: func(t *testing.T) {
			t.Setenv(stubAppIDEnvName, "")
			t.Setenv(stubAppSecretEnvName, stubAppSecretValue)
		}},
		"missing app secret variable": {settings: fullSettings, plantEnv: func(t *testing.T) {
			t.Setenv(stubAppIDEnvName, "cli-fail-case-app-id")
		}},
		"empty app secret variable": {settings: fullSettings, plantEnv: func(t *testing.T) {
			t.Setenv(stubAppIDEnvName, "cli-fail-case-app-id")
			t.Setenv(stubAppSecretEnvName, "")
		}},
		"gateway rejects the connection": {
			settings: fullSettings,
			plantEnv: func(t *testing.T) { setCredentials(t) },
			startErr: errors.New("app secret and clientAssertionProvider cannot be nil"),
		},
		"cancelled caller context": {settings: fullSettings, plantEnv: func(t *testing.T) { setCredentials(t) }, deadCtx: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.plantEnv(t)
			env := &fakeEnv{settings: json.RawMessage(tc.settings), client: &http.Client{}}
			ws := newFakeWS(tc.startErr)
			spy := &wsFactorySpy{f: func(eventFunc, wsCreds, string) wsClient { return ws }}
			p := New().(*Plugin)
			p.newWS = spy.build
			ctx := context.Background()
			if tc.deadCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			err := p.Start(ctx, env)
			if err == nil {
				t.Fatal("Start must fail closed")
			}
			p.mu.Lock()
			live := p.current != nil || p.api != nil
			p.mu.Unlock()
			if live {
				t.Fatal("failed Start must not leave a live ear")
			}
		})
	}

	// The healthy env still starts, proving the failures above are each
	// case's own fault.
	startWithFake(t, envFor(t, fullSettings), newFakeWS(nil))
}

// TestStartPassesResolvedCredentials: the factories receive the resolved
// values (never the env names), the encrypt key from plain settings, and
// the configured domain.
func TestStartPassesResolvedCredentials(t *testing.T) {
	appID := setCredentials(t)
	env := &fakeEnv{
		appID:    appID,
		settings: json.RawMessage(`{"app_id_env":"` + stubAppIDEnvName + `","app_secret_env":"` + stubAppSecretEnvName + `","encrypt_key":"stub-encrypt-key","open_base_url":"http://127.0.0.1:1"}`),
		client:   &http.Client{},
	}
	ws := newFakeWS(nil)
	wsSpy := &wsFactorySpy{f: func(eventFunc, wsCreds, string) wsClient { return ws }}
	apiSpy := &apiFactorySpy{f: func(creds wsCreds, domain string) *lark.Client {
		return lark.NewClient(creds.AppID, creds.AppSecret, lark.WithOpenBaseUrl(domain))
	}}
	p := New().(*Plugin)
	p.newWS = wsSpy.build
	p.newAPI = apiSpy.build
	if err := p.Start(context.Background(), env); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = p.Stop(ctx)
	})

	onEvent, creds, domain := wsSpy.snapshot()
	if onEvent == nil {
		t.Fatal("websocket factory must receive the event handler")
	}
	if creds.AppID != appID || creds.AppSecret != stubAppSecretValue {
		t.Fatalf("factory credentials = %+v, want the resolved values", creds)
	}
	if creds.AppSecret == stubAppSecretEnvName || creds.AppID == stubAppIDEnvName {
		t.Fatal("factory must receive credential values, not env names")
	}
	if creds.EncryptKey != "stub-encrypt-key" {
		t.Fatalf("factory encrypt key = %q", creds.EncryptKey)
	}
	if domain != "http://127.0.0.1:1" {
		t.Fatalf("factory domain = %q", domain)
	}
	apiCreds, apiDomain := apiSpy.snapshot()
	if apiCreds != creds || apiDomain != domain {
		t.Fatalf("api factory inputs = %+v %q, want the same credentials and domain", apiCreds, apiDomain)
	}
}

// TestStartStopFullLoop: the whole Start → event → PublishInbound → Send →
// Stop path against the fake websocket client and the real OpenAPI client
// on a loopback stub — Start wires the handler before going live, the
// handler publishes a normalized envelope, Send delivers through the API,
// and Stop sheds the supervisor promptly and idempotently.
func TestStartStopFullLoop(t *testing.T) {
	stub := newLarkStub(t, stubOptions{})
	env := envFor(t, `{"app_id_env":"`+stubAppIDEnvName+`","app_secret_env":"`+stubAppSecretEnvName+`","open_base_url":"`+stub.server.URL+`"}`)
	ws := newFakeWS(nil)
	p, _ := startWithFake(t, env, ws)

	onEvent, starts, _ := ws.state()
	if onEvent == nil {
		t.Fatal("Start must wire the event handler")
	}
	if starts != 1 {
		t.Fatalf("starts = %d, want exactly the one connect from Start", starts)
	}

	if err := onEvent(context.Background(), p2pTextEvent()); err != nil {
		t.Fatalf("event handler: %v", err)
	}
	waitFor(t, "first inbound envelope", func() bool { return len(env.snapshot()) == 1 })
	got := env.snapshot()[0]
	if got.Channel != "feishu" || got.ChatID != "oc_chat_1" || got.Sender != "feishu:ou_manager1234" ||
		got.MessageID != "om_100" || len(got.Parts) != 1 || got.Parts[0].Text != "hello vivy" {
		t.Fatalf("published envelope = %+v", got)
	}

	ids, err := p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "oc_chat_1",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "hello human"}},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(ids) != 1 || ids[0] != stubMessageID {
		t.Fatalf("send ids = %v, want [%s]", ids, stubMessageID)
	}
	if got := len(stub.messageCalls()); got != 1 {
		t.Fatalf("message calls = %d, want 1", got)
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
	if _, starts, closed := ws.state(); !closed || starts != 1 {
		t.Fatalf("after stop: starts=%d closed=%v, want 1 start and closed client", starts, closed)
	}
	// The supervisor exited inside Stop (Stop waits on it), so no redial
	// can follow.
	time.Sleep(50 * time.Millisecond)
	if _, starts, _ := ws.state(); starts != 1 {
		t.Fatalf("starts after stop = %d, want no redial", starts)
	}
}

// TestHandlerFencedAfterStop: the SDK dispatches frames on its own
// goroutines, so an event read before Close can reach the handler after
// Stop returned. The handler itself must fence: nothing published.
func TestHandlerFencedAfterStop(t *testing.T) {
	env := envFor(t, `{"app_id_env":"`+stubAppIDEnvName+`","app_secret_env":"`+stubAppSecretEnvName+`"}`)
	ws := newFakeWS(nil)
	p, _ := startWithFake(t, env, ws)
	onEvent, _, _ := ws.state()

	// Latch stopped exactly the way Stop does, without tearing the fake
	// down: the fence lives in the handler, not in Stop's cleanup.
	p.mu.Lock()
	p.stopped = true
	p.mu.Unlock()

	if err := onEvent(context.Background(), p2pTextEvent()); err != nil {
		t.Fatalf("event handler: %v", err)
	}
	if got := len(env.snapshot()); got != 0 {
		t.Fatalf("published envelopes after stop = %d, want 0", got)
	}
}

// TestStartAfterStopStartsFresh: starting again on the same instance is a
// new ear — Start resets the stopped latch (qq pattern, review L3-F2).
// Pre-fix, the stale latch made the restarted supervisor exit before
// firstErr was delivered, so Start hung until the caller's context ended;
// the bounded restart context proves it returns promptly, and the new ear
// publishes instead of having every event fenced.
func TestStartAfterStopStartsFresh(t *testing.T) {
	env := envFor(t, `{"app_id_env":"`+stubAppIDEnvName+`","app_secret_env":"`+stubAppSecretEnvName+`"}`)
	ws := newFakeWS(nil)
	p, _ := startWithFake(t, env, ws)
	onEvent, _, _ := ws.state()

	// Traffic on the first ear leaves published state behind.
	if err := onEvent(context.Background(), p2pTextEvent()); err != nil {
		t.Fatalf("event handler: %v", err)
	}
	waitFor(t, "first inbound envelope", func() bool { return len(env.snapshot()) == 1 })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// The restart swaps in a fresh fake client; the same plugin instance
	// must publish through it.
	second := newFakeWS(nil)
	p.newWS = func(onEvent eventFunc, _ wsCreds, _ string) wsClient {
		second.setOnEvent(onEvent)
		return second
	}
	startCtx, startCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer startCancel()
	if err := p.Start(startCtx, env); err != nil {
		t.Fatalf("restart: %v", err)
	}
	handler2, starts2, _ := second.state()
	if handler2 == nil {
		t.Fatal("restart must wire the event handler")
	}
	if starts2 != 1 {
		t.Fatalf("restart starts = %d, want exactly the one fresh connect", starts2)
	}
	if err := handler2(context.Background(), p2pTextEvent()); err != nil {
		t.Fatalf("restarted event handler: %v", err)
	}
	waitFor(t, "inbound envelope on the restarted ear", func() bool { return len(env.snapshot()) == 2 })
}

// TestSendNotStartedFailsClosed: Send before Start, and an empty chat id,
// fail closed without touching the network.
func TestSendNotStartedFailsClosed(t *testing.T) {
	p := New().(*Plugin)
	if _, err := p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "oc_chat_1",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "x"}},
	}); err == nil || !strings.Contains(err.Error(), "not started") {
		t.Fatalf("send before start = %v, want a not-started failure", err)
	}

	stub := newLarkStub(t, stubOptions{})
	env := envFor(t, `{"app_id_env":"`+stubAppIDEnvName+`","app_secret_env":"`+stubAppSecretEnvName+`","open_base_url":"`+stub.server.URL+`"}`)
	live, _ := startWithFake(t, env, newFakeWS(nil))
	if _, err := live.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "  ",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "x"}},
	}); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("send with an empty chat id = %v, want a failure", err)
	}
	if got := len(stub.messageCalls()); got != 0 {
		t.Fatalf("message calls = %d, want 0", got)
	}
}

// TestSendPlainText: Send delivers text parts as one im.v1.message Create
// call each — receive_id_type chat_id, msg_type text, content
// {"text":...} — and skips non-text parts.
func TestSendPlainText(t *testing.T) {
	stub := newLarkStub(t, stubOptions{})
	env := envFor(t, `{"app_id_env":"`+stubAppIDEnvName+`","app_secret_env":"`+stubAppSecretEnvName+`","open_base_url":"`+stub.server.URL+`"}`)
	p, _ := startWithFake(t, env, newFakeWS(nil))

	ids, err := p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "oc_chat_1",
		Parts: []plugin.Part{
			{Kind: plugin.PartText, Text: "first"},
			{Kind: plugin.PartMediaRef, MediaRef: "noop:x"}, // skipped: text-only this slice
			{Kind: plugin.PartText, Text: "second"},
		},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(ids) != 2 || ids[0] != stubMessageID || ids[1] != stubMessageID {
		t.Fatalf("send ids = %v, want the stub message id twice", ids)
	}
	calls := stub.messageCalls()
	if len(calls) != 2 {
		t.Fatalf("message calls = %d, want 2", len(calls))
	}
	for i, want := range []string{"first", "second"} {
		call := calls[i]
		if call.path != "/open-apis/im/v1/messages" {
			t.Fatalf("message call %d path = %q", i, call.path)
		}
		if call.receiveIDType != "chat_id" {
			t.Fatalf("message call %d receive_id_type = %q, want chat_id", i, call.receiveIDType)
		}
		if call.receiveID != "oc_chat_1" || call.msgType != "text" || call.text != want {
			t.Fatalf("message call %d body = %+v, want text %q to oc_chat_1", i, call, want)
		}
		if !strings.HasPrefix(call.authorization, "Bearer ") {
			t.Fatalf("message call %d authorization = %q, want a bearer tenant token", i, call.authorization)
		}
	}

	// An envelope with no text parts sends nothing.
	if ids, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "oc_chat_1", Parts: []plugin.Part{{Kind: plugin.PartMediaRef, MediaRef: "noop:x"}}}); err != nil || len(ids) != 0 {
		t.Fatalf("media-only envelope = ids %v err %v, want no send and no error", ids, err)
	}
	if got := len(stub.messageCalls()); got != 2 {
		t.Fatalf("message calls = %d, want still 2", got)
	}

	// Exactly one token exchange backs both sends: the SDK resolves and
	// caches it; this adapter never touches tokens.
	if got := stub.tokenCount(); got != 1 {
		t.Fatalf("token calls = %d, want 1 (SDK-managed and cached)", got)
	}
	token := stub.tokenCalls()[0]
	if token.appID != env.appID || token.appSecret != stubAppSecretValue {
		t.Fatalf("token call body = %+v, want the resolved credentials", token)
	}
}

// TestSendSurfacesAPIError: platform failures surface to the Host as Send
// errors — non-zero platform code on HTTP 200, a rejected tenant token,
// and a dead API endpoint — and no error ever carries the app secret
// (D-010).
func TestSendSurfacesAPIError(t *testing.T) {
	settings := func(t *testing.T, baseURL string) *fakeEnv {
		return envFor(t, `{"app_id_env":"`+stubAppIDEnvName+`","app_secret_env":"`+stubAppSecretEnvName+`","open_base_url":"`+baseURL+`"}`)
	}

	t.Run("non-zero platform code", func(t *testing.T) {
		stub := newLarkStub(t, stubOptions{messageCode: 230013, messageMsg: "bot ability is off"})
		p, _ := startWithFake(t, settings(t, stub.server.URL), newFakeWS(nil))
		_, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "oc_chat_1", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}})
		if err == nil {
			t.Fatal("send must surface the platform error")
		}
		if !strings.Contains(err.Error(), "230013") {
			t.Fatalf("platform error = %v, want code 230013 surfaced", err)
		}
		if strings.Contains(err.Error(), stubAppSecretValue) {
			t.Fatalf("error leaks the app secret: %v", err)
		}
	})

	t.Run("rejected tenant token", func(t *testing.T) {
		stub := newLarkStub(t, stubOptions{tokenCode: 99991661, tokenMsg: "app secret invalid"})
		p, _ := startWithFake(t, settings(t, stub.server.URL), newFakeWS(nil))
		_, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "oc_chat_1", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}})
		if err == nil {
			t.Fatal("send must surface the token error")
		}
		if strings.Contains(err.Error(), stubAppSecretValue) {
			t.Fatalf("token error leaks the app secret: %v", err)
		}
	})

	t.Run("dead endpoint keeps the cause", func(t *testing.T) {
		stub := newLarkStub(t, stubOptions{})
		p, _ := startWithFake(t, settings(t, stub.server.URL), newFakeWS(nil))
		stub.server.Close()
		_, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "oc_chat_1", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}})
		if err == nil {
			t.Fatal("send against a dead endpoint must surface an error")
		}
		if strings.Contains(err.Error(), stubAppSecretValue) {
			t.Fatalf("transport error leaks the app secret: %v", err)
		}
		// The transport cause must stay visible. The SDK wraps dial
		// failures in a DialFailedError that keeps the message but breaks
		// the Unwrap chain, so the text is the contract here.
		if !strings.Contains(err.Error(), "dial tcp") {
			t.Fatalf("dead-endpoint error lost its cause text: %v", err)
		}
	})
}

// --- Real-SDK loopback integration (no Feishu/Lark network) ---

// wsGUID is the WebSocket handshake magic string (RFC 6455 §1.3).
const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// larkStub is a loopback stand-in for the Feishu/Lark platform: it answers
// the websocket bootstrap POST, serves the websocket the SDK dials, and
// exposes the tenant-token and message-send OpenAPI endpoints the SDK
// client calls. The websocket framing is hand-rolled with the standard
// library only — the gorilla/websocket dependency stays with the SDK, not
// this module.
type larkStub struct {
	server *httptest.Server
	wsOpen chan struct{}

	mu           sync.Mutex
	tokenHits    int
	tokenCode    int
	tokenMsg     string
	messageCode  int
	messageMsg   string
	tokens       []tokenCall
	messages     []messageCall
	endpointHits int
}

type tokenCall struct {
	appID     string
	appSecret string
}

type messageCall struct {
	path          string
	receiveIDType string
	receiveID     string
	msgType       string
	text          string
	authorization string
}

type stubOptions struct {
	tokenCode   int // non-zero: the token endpoint answers with this error
	tokenMsg    string
	messageCode int // non-zero: the message endpoint answers with this error
	messageMsg  string
}

func newLarkStub(t *testing.T, opts stubOptions) *larkStub {
	t.Helper()
	stub := &larkStub{
		wsOpen:      make(chan struct{}, 1),
		tokenCode:   opts.tokenCode,
		tokenMsg:    opts.tokenMsg,
		messageCode: opts.messageCode,
		messageMsg:  opts.messageMsg,
	}
	mux := http.NewServeMux()

	// Websocket bootstrap: the SDK POSTs {AppID, AppSecret} and expects
	// the event-gateway endpoint URL (field name "URL").
	mux.HandleFunc("/callback/ws/endpoint", func(w http.ResponseWriter, r *http.Request) {
		stub.mu.Lock()
		stub.endpointHits++
		stub.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]string{"URL": "ws://" + r.Host + "/ws"},
		})
	})

	// The event-gateway websocket: hijack the connection, push one
	// im.message.receive_v1 data frame, read the SDK's ack, then hold the
	// socket open until the client goes away.
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := hijackWebSocket(t, w, r)
		if err != nil {
			t.Errorf("websocket hijack: %v", err)
			return
		}
		defer conn.Close()
		stub.wsOpen <- struct{}{}

		event := map[string]any{
			"schema": "2.0",
			"header": map[string]any{
				"event_id":    "ev-loopback-1",
				"event_type":  "im.message.receive_v1",
				"create_time": fmt.Sprint(time.Now().UnixMilli()),
				"token":       "stub-token",
				"app_id":      "cli-loopback-app-id",
				"tenant_key":  "stub-tenant",
			},
			"event": map[string]any{
				"sender": map[string]any{
					"sender_id":   map[string]any{"open_id": "ou_manager1234"},
					"sender_type": "user",
					"tenant_key":  "stub-tenant",
				},
				"message": map[string]any{
					"message_id":   "om_loopback_1",
					"chat_id":      "oc_loopback",
					"chat_type":    "p2p",
					"message_type": "text",
					"content":      `{"text":"hello vivy"}`,
				},
			},
		}
		eventJSON, _ := json.Marshal(event)
		frameBytes, err := buildEventDataFrame(eventJSON, "frame-1")
		if err != nil {
			t.Errorf("build event frame: %v", err)
			return
		}
		if err := writeWSBinaryFrame(conn, frameBytes); err != nil {
			t.Errorf("write event frame: %v", err)
			return
		}

		// Read frames until the SDK acks the data frame (control frames
		// such as pings come first), asserting the ack as we go.
		for {
			opcode, body, err := readWSFrame(conn)
			if err != nil {
				return // the client went away (Stop or test end)
			}
			if opcode != 0x2 {
				continue
			}
			frame := &larkws.Frame{}
			if err := frame.Unmarshal(body); err != nil ||
				frame.Method != int32(larkws.FrameTypeData) {
				continue
			}
			var ack struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(frame.Payload, &ack); err != nil {
				t.Errorf("decode ack payload: %v", err)
				return
			}
			if ack.Code != http.StatusOK {
				t.Errorf("ack code = %d, want %d", ack.Code, http.StatusOK)
			}
			for {
				if _, _, err := readWSFrame(conn); err != nil {
					return
				}
			}
		}
	})

	// Tenant access token: the SDK exchanges app credentials before the
	// first message call and caches the result.
	mux.HandleFunc("/open-apis/auth/v3/tenant_access_token/internal", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			AppID     string `json:"app_id"`
			AppSecret string `json:"app_secret"`
		}
		_ = json.Unmarshal(body, &req)
		stub.mu.Lock()
		stub.tokenHits++
		stub.tokens = append(stub.tokens, tokenCall{appID: req.AppID, appSecret: req.AppSecret})
		code, msg := stub.tokenCode, stub.tokenMsg
		stub.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if code != 0 {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "msg": msg})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":                0,
			"msg":                 "ok",
			"tenant_access_token": stubTenantToken,
			"expire":              7200,
		})
	})

	// Message send: the reply path of this slice.
	mux.HandleFunc("/open-apis/im/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ReceiveID string `json:"receive_id"`
			MsgType   string `json:"msg_type"`
			Content   string `json:"content"`
		}
		_ = json.Unmarshal(body, &req)
		var textPayload struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal([]byte(req.Content), &textPayload)
		stub.mu.Lock()
		code, msg := stub.messageCode, stub.messageMsg
		stub.messages = append(stub.messages, messageCall{
			path:          r.URL.Path,
			receiveIDType: r.URL.Query().Get("receive_id_type"),
			receiveID:     req.ReceiveID,
			msgType:       req.MsgType,
			text:          textPayload.Text,
			authorization: r.Header.Get("Authorization"),
		})
		stub.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if code != 0 {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "msg": msg})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "success",
			"data": map[string]any{"message_id": stubMessageID},
		})
	})

	stub.server = httptest.NewServer(mux)
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *larkStub) tokenCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenHits
}

func (s *larkStub) tokenCalls() []tokenCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]tokenCall(nil), s.tokens...)
}

func (s *larkStub) messageCalls() []messageCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]messageCall(nil), s.messages...)
}

func (s *larkStub) endpointCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.endpointHits
}

// TestWSLoopbackLifecycle runs the production websocket client and the
// production OpenAPI client against a loopback platform: bootstrap,
// websocket connect, one real event frame through the SDK dispatcher,
// PublishInbound, Send through the OpenAPI endpoint, the SDK's ack, then a
// Stop that closes the socket and leaves no redial behind.
func TestWSLoopbackLifecycle(t *testing.T) {
	stub := newLarkStub(t, stubOptions{})
	env := envFor(t, `{"app_id_env":"`+stubAppIDEnvName+`","app_secret_env":"`+stubAppSecretEnvName+`","open_base_url":"`+stub.server.URL+`"}`)

	p := New().(*Plugin) // production factories: the real SDK clients
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
		case <-stub.wsOpen:
			return true
		default:
			return false
		}
	})
	waitFor(t, "inbound envelope from the event frame", func() bool { return len(env.snapshot()) == 1 })
	got := env.snapshot()[0]
	if got.Channel != "feishu" || got.ChatID != "oc_loopback" || got.Sender != "feishu:ou_manager1234" ||
		got.MessageID != "om_loopback_1" || len(got.Parts) != 1 || got.Parts[0].Text != "hello vivy" {
		t.Fatalf("published envelope = %+v", got)
	}

	ids, err := p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "oc_loopback",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "hello human"}},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(ids) != 1 || ids[0] != stubMessageID {
		t.Fatalf("send ids = %v", ids)
	}
	calls := stub.messageCalls()
	if len(calls) != 1 || calls[0].receiveIDType != "chat_id" || calls[0].receiveID != "oc_loopback" ||
		calls[0].msgType != "text" || calls[0].text != "hello human" {
		t.Fatalf("message calls = %+v", calls)
	}

	// Stop sheds the supervisor promptly; the bootstrap endpoint is never
	// hit again (no redial after stop).
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := stub.endpointCount(); got != 1 {
		t.Fatalf("bootstrap exchanges = %d, want exactly the initial one (no redial after stop)", got)
	}
	time.Sleep(50 * time.Millisecond)
	if got := stub.endpointCount(); got != 1 {
		t.Fatalf("bootstrap exchanges after settle = %d, want still 1", got)
	}
}

// TestRedialAfterDrop: when the event gateway drops a healthy connection
// (and Stop has not run), the supervisor reconnects with a fresh client.
func TestRedialAfterDrop(t *testing.T) {
	first := newFakeWS(nil)
	second := newFakeWS(nil)
	clients := []*fakeWS{first, second}
	var mu sync.Mutex
	attempt := 0
	spy := &wsFactorySpy{f: func(onEvent eventFunc, _ wsCreds, _ string) wsClient {
		mu.Lock()
		defer mu.Unlock()
		c := clients[min(attempt, len(clients)-1)]
		c.setOnEvent(onEvent)
		attempt++
		return c
	}}
	env := envFor(t, `{"app_id_env":"`+stubAppIDEnvName+`","app_secret_env":"`+stubAppSecretEnvName+`"}`)
	p := New().(*Plugin)
	p.newWS = spy.build
	if err := p.Start(context.Background(), env); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = p.Stop(ctx)
	})

	// Drop the first connection the way a dead socket would: the fake's
	// Start returns (its run ended) without Stop having run.
	first.cancelRun(context.Canceled)
	waitFor(t, "redial with a fresh client", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return attempt >= 2 && second.isReady()
	})

	// Stop sheds the loop; no third attempt may follow.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	assertAttempts := func(what string) {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if attempt != 2 {
			t.Fatalf("connection attempts %s = %d, want 2", what, attempt)
		}
	}
	assertAttempts("at stop")
	time.Sleep(50 * time.Millisecond)
	assertAttempts("after settle (no redial after stop)")
}

// --- Frame helpers for the websocket stub (the SDK's own codec) ---

// buildEventDataFrame encodes one data frame carrying an event with the
// SDK's own Frame codec, so the stub speaks exactly the wire format the
// client parses.
func buildEventDataFrame(eventJSON []byte, messageID string) ([]byte, error) {
	var hs larkws.Headers
	hs.Add("type", "event")
	hs.Add("message_id", messageID)
	hs.Add("sum", "1")
	hs.Add("seq", "0")
	frame := &larkws.Frame{
		SeqID:   1,
		LogID:   100,
		Method:  int32(larkws.FrameTypeData),
		Headers: hs,
		Payload: eventJSON,
	}
	return frame.Marshal()
}

// --- Hand-rolled RFC 6455 server helpers (standard library only) ---

// hijackWebSocket performs the RFC 6455 server handshake over a hijacked
// connection and returns it.
func hijackWebSocket(t *testing.T, w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	t.Helper()
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("response writer does not support hijacking")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	h := sha1.New()
	_, _ = io.WriteString(h, r.Header.Get("Sec-WebSocket-Key")+wsGUID)
	accept := base64.StdEncoding.EncodeToString(h.Sum(nil))
	fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept)
	_ = rw.Flush()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	return conn, nil
}

// writeWSBinaryFrame writes one unmasked server-to-client binary frame.
func writeWSBinaryFrame(conn net.Conn, payload []byte) error {
	header := []byte{0x82} // FIN + binary opcode
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
