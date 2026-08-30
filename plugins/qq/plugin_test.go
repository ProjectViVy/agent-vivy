package qq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/constant"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/errs"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/log"
	"github.com/tencent-connect/botgo/openapi/options"
	"golang.org/x/oauth2"

	"agent-vivy/sdk/plugin"
)

// Synthetic test credentials. No real QQ open-platform app is involved
// anywhere; every network touch in this file lands on a loopback
// httptest server or an in-memory stub. Tests never run in parallel (a
// few package-level tuning vars are swapped and restored).
const (
	stubAppIDEnvName     = "VIVY_TEST_QQ_APP_ID"
	stubAppSecretEnvName = "VIVY_TEST_QQ_APP_SECRET"
	stubAppIDValue       = "vivy-test-app-id"
	stubAppSecretValue   = "vivy-test-app-secret-value"
	stubAccessToken      = "stub-access-token"

	validSettings = `{"app_id_env":"` + stubAppIDEnvName + `","app_secret_env":"` + stubAppSecretEnvName + `"}`
	stubGateway   = "wss://stub-gateway.invalid"
)

// plantCredentials sets the two test credential variables the way the
// Host's environment lookup would see them.
func plantCredentials(t *testing.T) {
	t.Helper()
	t.Setenv(stubAppIDEnvName, stubAppIDValue)
	t.Setenv(stubAppSecretEnvName, stubAppSecretValue)
}

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

// botgoOpenAPI and botgoSandbox are the production OpenAPI constructors,
// reachable for the Send loopback test (New wires them into p.newAPI; the
// generic harness swaps that factory out).
func botgoOpenAPI(appID string, ts oauth2.TokenSource) qqAPI { return botgo.NewOpenAPI(appID, ts) }

func botgoSandbox(appID string, ts oauth2.TokenSource) qqAPI {
	return botgo.NewSandboxOpenAPI(appID, ts)
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

func (e *fakeEnv) Secret(envKey string) (string, error) {
	v, ok := os.LookupEnv(envKey)
	if !ok || v == "" {
		return "", fmt.Errorf("env variable %q empty or unset", envKey)
	}
	return v, nil
}

func (e *fakeEnv) HTTP() *http.Client { return &http.Client{} }

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

// fakeTokenSource stands in for the SDK's QQBot token source: it counts
// calls and either fails or hands out a static token. No network.
type fakeTokenSource struct {
	err error

	mu    sync.Mutex
	calls int
}

func (f *fakeTokenSource) Token() (*oauth2.Token, error) {
	f.mu.Lock()
	f.calls++
	err := f.err
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken: stubAccessToken,
		TokenType:   "QQBot",
		Expiry:      time.Now().Add(time.Hour),
	}, nil
}

func (f *fakeTokenSource) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// fakeAPI stands in for botgo's OpenAPI client: the gateway discovery
// result is canned and PostC2CMessage calls are recorded. Send-over-HTTP
// is covered separately against the REAL SDK client on a loopback server.
type fakeAPI struct {
	wsURL string
	wsErr error

	mu       sync.Mutex
	c2cCalls []c2cCall
}

type c2cCall struct {
	userID string
	msg    dto.APIMessage
}

func (a *fakeAPI) WS(context.Context, map[string]string, string) (*dto.WebsocketAP, error) {
	if a.wsErr != nil {
		return nil, a.wsErr
	}
	return &dto.WebsocketAP{
		URL:               a.wsURL,
		Shards:            1,
		SessionStartLimit: dto.SessionStartLimit{Total: 1, Remaining: 1, ResetAfter: 1, MaxConcurrency: 1},
	}, nil
}

func (a *fakeAPI) PostC2CMessage(_ context.Context, userID string, msg dto.APIMessage, _ ...options.Option) (*dto.Message, error) {
	a.mu.Lock()
	a.c2cCalls = append(a.c2cCalls, c2cCall{userID: userID, msg: msg})
	a.mu.Unlock()
	return &dto.Message{ID: "sent-via-stub"}, nil
}

func (a *fakeAPI) c2cCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.c2cCalls)
}

// fakeWS is an in-memory wsClient. It records the factory inputs and the
// protocol calls, fires the READY callback once "connected" (like the
// real client after identify), and blocks in Listening until the test
// drops the connection or Close was called. It fails closed on demand.
type fakeWS struct {
	// captured factory inputs
	url       string
	resumeID  string
	resumeSeq uint32
	onC2C     event.C2CMessageEventHandler
	onReady   event.ReadyHandler

	// assignedID is the gateway-assigned session id handed out at READY.
	assignedID string

	connectErr error
	authErr    error
	dropErr    error
	hanging    bool // Connect blocks until Close
	muteReady  bool // never fire READY (gateway never answers the handshake)

	mu            sync.Mutex
	connectCalls  int
	identifyCalls int
	resumeCalls   int
	listenCalls   int
	closeCalls    int

	session *dto.Session
	stop    chan struct{}
	dropped chan struct{}
}

// wsFactorySpy wires the plugin's websocket factory to fakes and records
// every build. onBuild runs at build time (on the supervisor goroutine)
// so a test can pre-configure the next attempt.
type wsFactorySpy struct {
	mu      sync.Mutex
	built   []*fakeWS
	onBuild func(n int, f *fakeWS)
}

func (s *wsFactorySpy) build(onC2C event.C2CMessageEventHandler, onReady event.ReadyHandler,
	gatewayURL string, _ oauth2.TokenSource, resumeID string, resumeSeq uint32) wsClient {
	f := &fakeWS{
		url:        gatewayURL,
		resumeID:   resumeID,
		resumeSeq:  resumeSeq,
		onC2C:      onC2C,
		onReady:    onReady,
		assignedID: "",
		session:    &dto.Session{URL: gatewayURL, ID: resumeID, LastSeq: resumeSeq},
		stop:       make(chan struct{}),
		dropped:    make(chan struct{}),
	}
	s.mu.Lock()
	f.assignedID = fmt.Sprintf("sess-%d", len(s.built)+1)
	s.built = append(s.built, f)
	hook := s.onBuild
	s.mu.Unlock()
	if hook != nil {
		hook(len(s.built)-1, f)
	}
	return f
}

func (s *wsFactorySpy) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.built)
}

func (s *wsFactorySpy) nth(n int) *fakeWS {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n < 0 || n >= len(s.built) {
		return nil
	}
	return s.built[n]
}

// Connect implements wsClient. A hanging Connect blocks until Close or a
// short self-release (the real dialer's handshake timeout) — the
// supervisor owns every Close, so a Stop during a hanging dial is bounded
// by the timeout, exactly like the real client.
func (f *fakeWS) Connect() error {
	f.mu.Lock()
	f.connectCalls++
	hanging := f.hanging
	f.mu.Unlock()
	if hanging {
		select {
		case <-f.stop:
			return errors.New("dial aborted by close")
		case <-time.After(150 * time.Millisecond):
			return errors.New("dial timed out")
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connectErr
}

// Identify implements wsClient.
func (f *fakeWS) Identify() error {
	f.mu.Lock()
	f.identifyCalls++
	defer f.mu.Unlock()
	return f.authErr
}

// Resume implements wsClient.
func (f *fakeWS) Resume() error {
	f.mu.Lock()
	f.resumeCalls++
	defer f.mu.Unlock()
	return f.authErr
}

// Session implements wsClient: the pointer is stable across the attempt,
// exactly like the real client's internal session.
func (f *fakeWS) Session() *dto.Session { return f.session }

// Listening implements wsClient: fires READY once (assigning the
// gateway-assigned session id first, same goroutine, like the real
// client), then blocks until the test drops the connection or Close ran.
func (f *fakeWS) Listening() error {
	f.mu.Lock()
	f.listenCalls++
	dropErr := f.dropErr
	preDropped := f.isDroppedLocked()
	onReady := f.onReady
	f.mu.Unlock()
	if preDropped {
		return dropErr
	}
	if !f.muteReady && onReady != nil && f.session != nil {
		// Simulate the gateway's READY dispatch: the session id is
		// assigned, then the handler fires, same goroutine.
		go func() {
			f.mu.Lock()
			f.session.ID = f.assignedID
			payload := &dto.WSPayload{Session: f.session}
			f.mu.Unlock()
			onReady(payload, &dto.WSReadyData{})
		}()
	}
	select {
	case <-f.stop:
		return errors.New("client closed")
	case <-f.dropped:
		// Read the error at return time: drop() may race the select.
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.dropErr
	}
}

// Close implements wsClient.
func (f *fakeWS) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeCalls++
	select {
	case <-f.stop:
	default:
		close(f.stop)
	}
}

// drop simulates the gateway dropping a healthy connection: the blocked
// Listening returns with the given error while the plugin's context stays
// live. Safe to call before Listening runs (pre-dropped).
func (f *fakeWS) drop(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dropErr = err
	select {
	case <-f.dropped:
	default:
		close(f.dropped)
	}
}

func (f *fakeWS) isDroppedLocked() bool {
	select {
	case <-f.dropped:
		return true
	default:
		return false
	}
}

// calls snapshots the protocol call counters.
func (f *fakeWS) calls() (connect, identify, resume, listen, close int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connectCalls, f.identifyCalls, f.resumeCalls, f.listenCalls, f.closeCalls
}

// --- harness ----------------------------------------------------------------

// harness bundles a freshly wired plugin with its fakes.
type harness struct {
	p    *Plugin
	env  *fakeEnv
	spy  *wsFactorySpy
	api  *fakeAPI
	ts   *fakeTokenSource
	sand bool // sandbox flag the api factory received
}

func newHarness(t *testing.T, settings string) *harness {
	t.Helper()
	plantCredentials(t)
	h := &harness{
		env: &fakeEnv{settings: json.RawMessage(settings)},
		api: &fakeAPI{wsURL: stubGateway},
		ts:  &fakeTokenSource{},
	}
	h.p = New().(*Plugin)
	h.p.newTokenSource = func(appID, appSecret string) oauth2.TokenSource { return h.ts }
	h.p.newAPI = func(appID string, ts oauth2.TokenSource, sandbox bool) qqAPI {
		h.sand = sandbox
		return h.api
	}
	h.spy = &wsFactorySpy{}
	h.p.newWS = h.spy.build
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

// dispatch feeds one C2C event through the handler of the n-th attempt
// (default: the first).
func (h *harness) dispatch(t *testing.T, seq uint32, data *dto.WSC2CMessageData) {
	t.Helper()
	h.dispatchOn(t, 0, seq, data)
}

func (h *harness) dispatchOn(t *testing.T, n int, seq uint32, data *dto.WSC2CMessageData) {
	t.Helper()
	f := h.spy.nth(n)
	if f == nil {
		t.Fatalf("no websocket attempt #%d built", n)
	}
	if f.onC2C == nil {
		t.Fatalf("C2C handler not wired on attempt #%d", n)
	}
	if err := f.onC2C(wsPayload(seq), data); err != nil {
		t.Fatalf("dispatch c2c event: %v", err)
	}
}

// wsPayload builds a dispatch payload with a gateway sequence number.
func wsPayload(seq uint32) *dto.WSPayload {
	return &dto.WSPayload{WSPayloadBase: dto.WSPayloadBase{Seq: seq}}
}

// c2cEvent builds a minimal C2C_MESSAGE_CREATE payload (botgo's dto.Message
// shape: author.id is the per-app user openid).
func c2cEvent(msgID, senderID, content string) *dto.WSC2CMessageData {
	return &dto.WSC2CMessageData{
		ID:      msgID,
		Content: content,
		Author:  &dto.User{ID: senderID},
	}
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
	t.Run("fields decode and trim", func(t *testing.T) {
		s, err := DecodeSettings(json.RawMessage(`{"app_id_env":" A ","app_secret_env":" B ","sandbox":true}`))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if s.AppIDEnv != "A" || s.AppSecretEnv != "B" || !s.Sandbox {
			t.Fatalf("settings = %+v", s)
		}
	})
	t.Run("unknown fields are rejected", func(t *testing.T) {
		_, err := DecodeSettings(json.RawMessage(`{"app_id_env":"A","app_secret_env":"B","encrypt_key":"nope"}`))
		if err == nil || !strings.Contains(err.Error(), "decode settings") {
			t.Fatalf("want strict-decode failure, got %v", err)
		}
	})
	t.Run("trailing data is rejected", func(t *testing.T) {
		_, err := DecodeSettings(json.RawMessage(`{"app_id_env":"A","app_secret_env":"B"} {"x":1}`))
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
			name:     "missing app_id_env",
			settings: `{"app_secret_env":"X"}`,
			wantErr:  "app_id_env is required",
		},
		{
			name:     "missing app_secret_env",
			settings: `{"app_id_env":"X"}`,
			wantErr:  "app_secret_env is required",
		},
		{
			name:     "unknown settings field",
			settings: `{"app_id_env":"A","app_secret_env":"B","nope":1}`,
			wantErr:  "decode settings",
		},
		{
			name:     "same env name for both credentials",
			settings: `{"app_id_env":"` + stubAppIDEnvName + `","app_secret_env":"` + stubAppIDEnvName + `"}`,
			wantErr:  "must name different variables",
		},
		{
			name:     "app id env unset",
			settings: validSettings,
			prepare: func(h *harness) {
				_ = os.Unsetenv(stubAppIDEnvName)
			},
			wantErr: "resolve app id through env",
		},
		{
			name:     "app secret env unset",
			settings: validSettings,
			prepare: func(h *harness) {
				_ = os.Unsetenv(stubAppSecretEnvName)
			},
			wantErr: "resolve app secret through env",
		},
		{
			name:     "token endpoint rejects the credentials",
			settings: validSettings,
			prepare: func(h *harness) {
				h.ts.err = errors.New("invalid appid/secret")
			},
			wantErr: "resolve access token",
		},
		{
			name:     "gateway discovery fails",
			settings: validSettings,
			prepare: func(h *harness) {
				h.api.wsErr = errors.New("gateway down")
			},
			wantErr: "fetch websocket gateway",
		},
		{
			name:     "gateway returns an empty url",
			settings: validSettings,
			prepare: func(h *harness) {
				h.api.wsURL = "  "
			},
			wantErr: "empty url",
		},
		{
			name:     "gateway dial fails",
			settings: validSettings,
			prepare: func(h *harness) {
				h.spy.onBuild = func(n int, f *fakeWS) {
					if n == 0 {
						f.connectErr = errors.New("refused")
					}
				}
			},
			wantErr: "dial gateway",
		},
		{
			name:     "authenticate fails",
			settings: validSettings,
			prepare: func(h *harness) {
				h.spy.onBuild = func(n int, f *fakeWS) {
					if n == 0 {
						f.authErr = errors.New("write failed")
					}
				}
			},
			wantErr: "authenticate",
		},
		{
			name:     "gateway rejects the session before READY",
			settings: validSettings,
			prepare: func(h *harness) {
				h.spy.onBuild = func(n int, f *fakeWS) {
					if n == 0 {
						f.drop(errs.New(errs.WSCodeBackendAuthenticationFail, "auth failed"))
					}
				}
			},
			wantErr: "gateway rejected the session",
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
			// websocket.
			h.p.mu.Lock()
			apiNil := h.p.api == nil
			wsNil := h.p.ws == nil
			h.p.mu.Unlock()
			if !apiNil || !wsNil {
				t.Fatalf("failed Start left api=%v ws=%v, want both nil", apiNil, wsNil)
			}
			if _, err := h.p.Send(context.Background(), plugin.OutboundMessage{ChatID: "x"}); err == nil {
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

	if got := h.ts.callCount(); got < 1 {
		t.Fatalf("token source calls = %d, want at least 1 (eager fetch)", got)
	}
	if h.sand {
		t.Fatalf("api factory got sandbox=true, want false")
	}
	f := h.spy.nth(0)
	if f == nil {
		t.Fatalf("no websocket attempt built")
	}
	if f.url != stubGateway {
		t.Fatalf("attempt url = %q, want %q", f.url, stubGateway)
	}
	connect, identify, resume, listen, closed := f.calls()
	if connect != 1 || identify != 1 || resume != 0 || listen != 1 || closed != 0 {
		t.Fatalf("attempt calls = (connect %d, identify %d, resume %d, listen %d, close %d)",
			connect, identify, resume, listen, closed)
	}
	if f.onC2C == nil || f.onReady == nil {
		t.Fatalf("event handlers not wired into the attempt")
	}
	if len(h.env.snapshot()) != 0 {
		t.Fatalf("no envelope may be published by Start alone")
	}
}

func TestStartSandboxFlag(t *testing.T) {
	h := newHarness(t, `{"app_id_env":"`+stubAppIDEnvName+`","app_secret_env":"`+stubAppSecretEnvName+`","sandbox":true}`)
	h.start(t)
	if !h.sand {
		t.Fatalf("api factory got sandbox=false, want true")
	}
}

// --- inbound normalization ------------------------------------------------------

func TestNormalizeC2C(t *testing.T) {
	cases := []struct {
		name string
		data *dto.WSC2CMessageData
		want bool
		msg  plugin.InboundMessage
	}{
		{
			name: "text message",
			data: c2cEvent("m-1", "OPENID1", "hello"),
			want: true,
			msg: plugin.InboundMessage{
				Channel:   "qq",
				ChatID:    "OPENID1",
				Sender:    "qq:user_OPENID1",
				MessageID: "m-1",
				Parts:     []plugin.Part{{Kind: plugin.PartText, Text: "hello"}},
			},
		},
		{name: "nil event", data: nil, want: false},
		{name: "nil author", data: &dto.WSC2CMessageData{ID: "m", Content: "hi"}, want: false},
		{name: "empty author id", data: c2cEvent("m", "  ", "hi"), want: false},
		{name: "empty message id", data: c2cEvent("", "U", "hi"), want: false},
		{name: "empty content (media only)", data: c2cEvent("m", "U", ""), want: false},
		{name: "whitespace content", data: c2cEvent("m", "U", "   \n\t"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, publishable := normalizeC2C(tc.data)
			if publishable != tc.want {
				t.Fatalf("publishable = %v, want %v", publishable, tc.want)
			}
			if !tc.want {
				return
			}
			if got.Channel != tc.msg.Channel || got.ChatID != tc.msg.ChatID ||
				got.Sender != tc.msg.Sender || got.MessageID != tc.msg.MessageID ||
				len(got.Parts) != 1 || got.Parts[0].Text != tc.msg.Parts[0].Text ||
				got.Parts[0].Kind != plugin.PartText {
				t.Fatalf("envelope = %+v, want %+v", got, tc.msg)
			}
		})
	}
}

func TestC2CHandlerPublishesAndFences(t *testing.T) {
	h := newHarness(t, validSettings)
	h.start(t)

	// First event: published, window remembered, sequence captured.
	h.dispatch(t, 42, c2cEvent("m-1", "U1", "hi"))
	waitFor(t, "first inbound envelope", func() bool { return len(h.env.snapshot()) == 1 })
	if got := h.env.snapshot()[0]; got.Sender != "qq:user_U1" || got.ChatID != "U1" {
		t.Fatalf("envelope = %+v", got)
	}
	h.p.mu.Lock()
	resumeSeq := h.p.resumeSeq
	window := h.p.chats["U1"]
	h.p.mu.Unlock()
	if resumeSeq != 42 {
		t.Fatalf("resume seq = %d, want 42", resumeSeq)
	}
	if window == nil || window.msgID != "m-1" {
		t.Fatalf("passive window = %+v, want msg id m-1", window)
	}

	// Duplicate delivery of the same msg_id: dropped, window untouched.
	h.dispatch(t, 43, c2cEvent("m-1", "U1", "hi"))
	if got := len(h.env.snapshot()); got != 1 {
		t.Fatalf("duplicate delivery published %d envelopes, want still 1", got)
	}
	h.p.mu.Lock()
	seqAfterDup := h.p.chats["U1"].seq
	h.p.mu.Unlock()
	if seqAfterDup != 0 {
		t.Fatalf("duplicate delivery must not touch the window, seq = %d", seqAfterDup)
	}

	// New message from the same chat: window advances.
	h.dispatch(t, 44, c2cEvent("m-2", "U1", "again"))
	waitFor(t, "second inbound envelope", func() bool { return len(h.env.snapshot()) == 2 })
	h.p.mu.Lock()
	window = h.p.chats["U1"]
	h.p.mu.Unlock()
	if window.msgID != "m-2" {
		t.Fatalf("window after second message = %+v, want m-2", window)
	}

	// Non-publishable shapes: dropped locally.
	h.dispatch(t, 45, c2cEvent("m-3", "U1", ""))
	h.dispatch(t, 46, c2cEvent("", "U1", "no id"))
	h.dispatch(t, 47, c2cEvent("m-4", "", "no sender"))
	if got := len(h.env.snapshot()); got != 2 {
		t.Fatalf("unpublishable shapes changed the envelope count to %d, want 2", got)
	}

	// Late callback after Stop: fenced, nothing published, window untouched.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	h.dispatch(t, 48, c2cEvent("m-5", "U1", "after stop"))
	if got := len(h.env.snapshot()); got != 2 {
		t.Fatalf("late callback published, envelope count = %d, want 2", got)
	}
}

func TestRememberSeen(t *testing.T) {
	oldTTL, oldCap := dedupTTL, dedupMaxEntries
	t.Cleanup(func() { dedupTTL, dedupMaxEntries = oldTTL, oldCap })
	// A cap of 1 keeps the eviction deterministic regardless of the
	// platform clock granularity (ties would make "oldest" ambiguous).
	dedupTTL, dedupMaxEntries = 20*time.Millisecond, 1

	p := New().(*Plugin)
	p.seen = make(map[string]time.Time)

	if !p.rememberSeen("a") {
		t.Fatalf("first a must be new")
	}
	if p.rememberSeen("a") {
		t.Fatalf("second a must be a duplicate")
	}
	time.Sleep(40 * time.Millisecond)
	if !p.rememberSeen("a") {
		t.Fatalf("expired a must be new again")
	}

	// Cap eviction: inserting into a full fence evicts the oldest entry.
	dedupTTL = time.Hour
	if !p.rememberSeen("b") { // evicts the expired-window a
		t.Fatalf("b must be new")
	}
	if p.rememberSeen("b") {
		t.Fatalf("second b must be a duplicate")
	}
	if !p.rememberSeen("a") { // evicted by b's insert
		t.Fatalf("evicted a must be new again")
	}
	if !p.rememberSeen("b") { // evicted by a's re-insert
		t.Fatalf("evicted b must be new again")
	}
	if len(p.seen) != dedupMaxEntries {
		t.Fatalf("seen size = %d, want bounded at %d", len(p.seen), dedupMaxEntries)
	}
}

// --- Send --------------------------------------------------------------------

// loopbackServer is a loopback stub of the official OpenAPI endpoints,
// served to the REAL botgo resty client by swapping the SDK's base-domain
// package var (the only override the SDK offers; tests never run in
// parallel).
type loopbackServer struct {
	srv *httptest.Server

	mu       sync.Mutex
	requests []recordedRequest
	c2cCode  int // status the /v2/users/... handler answers with
	c2cBody  string
}

type recordedRequest struct {
	method string
	path   string
	auth   string
	appid  string
	body   []byte
}

func newLoopback(t *testing.T) *loopbackServer {
	t.Helper()
	lb := &loopbackServer{c2cCode: http.StatusOK, c2cBody: `{"id":"sent-1"}`}
	mux := http.NewServeMux()
	mux.HandleFunc("/gateway/bot", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"url":"wss://stub.invalid","shards":1,`+
			`"session_start_limit":{"total":1,"remaining":1,"reset_after":1,"max_concurrency":1}}`)
	})
	mux.HandleFunc("/v2/users/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		lb.mu.Lock()
		lb.requests = append(lb.requests, recordedRequest{
			method: r.Method,
			path:   r.URL.Path,
			auth:   r.Header.Get("Authorization"),
			appid:  r.Header.Get("X-Union-Appid"),
			body:   body,
		})
		code, bodyText := lb.c2cCode, lb.c2cBody
		lb.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = io.WriteString(w, bodyText)
	})
	lb.srv = httptest.NewServer(mux)
	t.Cleanup(lb.srv.Close)

	old := constant.APIDomain
	constant.APIDomain = lb.srv.URL
	t.Cleanup(func() { constant.APIDomain = old })
	return lb
}

func (lb *loopbackServer) sent() []recordedRequest {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return append([]recordedRequest(nil), lb.requests...)
}

func (lb *loopbackServer) setC2C(code int, body string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.c2cCode, lb.c2cBody = code, body
}

// newRealAPIHarness wires a plugin with the PRODUCTION OpenAPI factory
// (real botgo client over the loopback domain) and a fake websocket
// factory, so Start exercises the real gateway discovery over loopback.
func newRealAPIHarness(t *testing.T, settings string) *harness {
	h := newHarness(t, settings)
	h.p.newAPI = func(appID string, ts oauth2.TokenSource, sandbox bool) qqAPI {
		h.sand = sandbox
		if sandbox {
			return botgoSandbox(appID, ts)
		}
		return botgoOpenAPI(appID, ts)
	}
	return h
}

func TestSendPassiveReplyLoopback(t *testing.T) {
	lb := newLoopback(t)
	h := newRealAPIHarness(t, validSettings)
	h.start(t)

	// Open the passive-reply window the way an inbound C2C event would.
	h.dispatch(t, 7, c2cEvent("in-1", "OPENID1", "hello"))

	ids, err := h.p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "OPENID1",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "hi"}, {Kind: plugin.PartText, Text: "there"}},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(ids) != 2 || ids[0] != "sent-1" || ids[1] != "sent-1" {
		t.Fatalf("ids = %v, want [sent-1 sent-1]", ids)
	}

	sent := lb.sent()
	if len(sent) != 2 {
		t.Fatalf("c2c posts = %d, want 2", len(sent))
	}
	for i, req := range sent {
		if req.method != http.MethodPost {
			t.Fatalf("post %d method = %s, want POST", i, req.method)
		}
		if want := "/v2/users/OPENID1/messages"; req.path != want {
			t.Fatalf("post %d path = %s, want %s", i, req.path, want)
		}
		if req.auth != "QQBot "+stubAccessToken {
			t.Fatalf("post %d authorization = %q, want the QQBot-scheme access token", i, req.auth)
		}
		if req.appid != stubAppIDValue {
			t.Fatalf("post %d X-Union-Appid = %q, want %q", i, req.appid, stubAppIDValue)
		}
		var body struct {
			Content string `json:"content"`
			MsgType *int   `json:"msg_type"`
			MsgID   string `json:"msg_id"`
			MsgSeq  int    `json:"msg_seq"`
		}
		if err := json.Unmarshal(req.body, &body); err != nil {
			t.Fatalf("post %d body decode: %v (%s)", i, err, req.body)
		}
		// botgo tags MsgType with omitempty, so plain text (0) may be
		// absent — the platform reads a missing msg_type as 0 (text).
		if body.MsgType != nil && *body.MsgType != 0 {
			t.Fatalf("post %d msg_type = %d, want 0 (or omitted)", i, *body.MsgType)
		}
		if body.MsgID != "in-1" {
			t.Fatalf("post %d msg_id = %q, want the passive window in-1", i, body.MsgID)
		}
		if wantSeq := i + 1; body.MsgSeq != wantSeq {
			t.Fatalf("post %d msg_seq = %d, want %d", i, body.MsgSeq, wantSeq)
		}
	}
	if !strings.Contains(string(sent[0].body), `"content":"hi"`) {
		t.Fatalf("post 0 body = %s, want content hi", sent[0].body)
	}

	// A platform failure (non-2xx with the {code,message} body) is an
	// error even though the SDK client is shared; the raw body rides in
	// the error chain.
	lb.setC2C(http.StatusBadRequest, `{"code":11244,"message":"token expired","trace_id":"t-1"}`)
	_, err = h.p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "OPENID1",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "again"}},
	})
	if err == nil {
		t.Fatalf("send over a 400 answer must fail")
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "token expired") {
		t.Fatalf("error = %v, want status 400 and the platform body", err)
	}

	// A 200 with no message id contributes no id and no error.
	lb.setC2C(http.StatusOK, `{}`)
	ids, err = h.p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "OPENID1",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "quiet"}},
	})
	if err != nil {
		t.Fatalf("send over an id-less 200: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("ids = %v, want none", ids)
	}
}

func TestSendFailClosed(t *testing.T) {
	t.Run("unknown chat has no passive window", func(t *testing.T) {
		h := newHarness(t, validSettings)
		h.start(t)
		_, err := h.p.Send(context.Background(), plugin.OutboundMessage{
			ChatID: "NEVER-MESSAGED",
			Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "hi"}},
		})
		if err == nil || !strings.Contains(err.Error(), "no passive reply window") {
			t.Fatalf("error = %v, want the passive-window failure", err)
		}
		if h.api.c2cCount() != 0 {
			t.Fatalf("no C2C post may leave the process for an unknown chat")
		}
	})
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
			ChatID: "U1",
			Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "hi"}},
		})
		if err == nil || !strings.Contains(err.Error(), "not started") {
			t.Fatalf("error = %v, want the not-started failure", err)
		}
	})
	t.Run("non-text and empty parts are skipped", func(t *testing.T) {
		h := newHarness(t, validSettings)
		h.start(t)
		h.dispatch(t, 1, c2cEvent("m-1", "U1", "hi"))
		ids, err := h.p.Send(context.Background(), plugin.OutboundMessage{
			ChatID: "U1",
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
		if h.api.c2cCount() != 0 {
			t.Fatalf("c2c posts = %d, want 0", h.api.c2cCount())
		}
	})
	t.Run("stub api records the passive reply contract", func(t *testing.T) {
		h := newHarness(t, validSettings)
		h.start(t)
		h.dispatch(t, 1, c2cEvent("m-1", "U1", "hi"))
		ids, err := h.p.Send(context.Background(), plugin.OutboundMessage{
			ChatID: "U1",
			Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "reply"}, {Kind: plugin.PartText, Text: "second"}},
		})
		if err != nil {
			t.Fatalf("send: %v", err)
		}
		if len(ids) != 2 || ids[0] != "sent-via-stub" || ids[1] != "sent-via-stub" {
			t.Fatalf("ids = %v", ids)
		}
		h.api.mu.Lock()
		calls := append([]c2cCall(nil), h.api.c2cCalls...)
		h.api.mu.Unlock()
		if len(calls) != 2 {
			t.Fatalf("c2c calls = %d, want 2", len(calls))
		}
		for i, call := range calls {
			if call.userID != "U1" {
				t.Fatalf("call %d user = %q, want U1", i, call.userID)
			}
			m, ok := call.msg.(*dto.MessageToCreate)
			if !ok {
				t.Fatalf("call %d payload type = %T, want *dto.MessageToCreate", i, call.msg)
			}
			if m.MsgType != dto.TextMsg || m.Content == "" || m.MsgID != "m-1" || m.MsgSeq != uint32(i+1) {
				t.Fatalf("call %d payload = %+v, want text msg_id m-1 msg_seq %d", i, m, i+1)
			}
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

func TestRedialResumeAndGiveUp(t *testing.T) {
	shrinkRedialDelay(t)
	h := newHarness(t, validSettings)
	h.start(t)

	first := h.spy.nth(0)
	// Traffic on the live connection captures the resume sequence.
	h.dispatch(t, 42, c2cEvent("m-1", "U1", "hi"))

	// The gateway drops the healthy connection with a resumable close:
	// the supervisor redials and RESUMES with the captured state.
	first.drop(errs.New(errs.WSCodeBackendUnknownError, "connection reset"))
	waitFor(t, "redial with resume state", func() bool { return h.spy.count() >= 2 })
	second := h.spy.nth(1)
	if second.resumeID != first.assignedID || second.resumeSeq != 42 {
		t.Fatalf("second attempt resume state = (%q, %d), want (%q, 42)",
			second.resumeID, second.resumeSeq, first.assignedID)
	}
	connect, identify, resume, _, _ := second.calls()
	if connect != 1 || resume != 1 || identify != 0 {
		t.Fatalf("second attempt calls = (connect %d, identify %d, resume %d), want resume only",
			connect, identify, resume)
	}

	// An unresumable close (invalid session): the next attempt re-identifies.
	second.drop(errs.New(errs.CodeConnCloseCantResume, "invalid session"))
	waitFor(t, "redial after an unresumable close", func() bool { return h.spy.count() >= 3 })
	third := h.spy.nth(2)
	if third.resumeID != "" || third.resumeSeq != 0 {
		t.Fatalf("third attempt resume state = (%q, %d), want fresh", third.resumeID, third.resumeSeq)
	}
	connect, identify, resume, _, _ = third.calls()
	if connect != 1 || identify != 1 || resume != 0 {
		t.Fatalf("third attempt calls = (connect %d, identify %d, resume %d), want identify only",
			connect, identify, resume)
	}

	// A close the gateway classifies as cannot-identify (bot banned or
	// delisted) ends the loop: re-identifying can never succeed.
	third.drop(errs.New(errs.CodeConnCloseCantIdentify, "bot banned"))
	time.Sleep(300 * time.Millisecond) // several redial delays must pass
	if got := h.spy.count(); got != 3 {
		t.Fatalf("attempts after cannot-identify = %d, want the loop to stop at 3", got)
	}

	// Stop on a stopped loop is still bounded and idempotent.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("second stop: %v", err)
	}
}

func TestRedialAfterDropKeepsPublishing(t *testing.T) {
	shrinkRedialDelay(t)
	h := newHarness(t, validSettings)
	h.start(t)
	first := h.spy.nth(0)

	first.drop(errs.New(errs.WSCodeBackendUnknownError, "connection reset"))
	waitFor(t, "redial", func() bool { return h.spy.count() >= 2 })
	second := h.spy.nth(1)
	if second.onC2C == nil {
		t.Fatalf("redialed attempt carries no C2C handler")
	}
	// The redialed attempt's handler publishes normally.
	if err := second.onC2C(wsPayload(9), c2cEvent("m-9", "U2", "after redial")); err != nil {
		t.Fatalf("dispatch after redial: %v", err)
	}
	waitFor(t, "envelope after redial", func() bool { return len(h.env.snapshot()) == 1 })
}

func TestStopIdempotent(t *testing.T) {
	h := newHarness(t, validSettings)
	// Stop before Start is a no-op.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("stop before start: %v", err)
	}
	h.start(t)
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("second stop: %v", err)
	}
	f := h.spy.nth(0)
	if _, _, _, _, closed := f.calls(); closed != 1 {
		t.Fatalf("close calls = %d, want exactly 1 (idempotent)", closed)
	}
	// A stopped ear cannot send.
	if _, err := h.p.Send(context.Background(), plugin.OutboundMessage{ChatID: "U1"}); err == nil {
		t.Fatalf("Send after Stop must fail")
	}
}

func TestStartAfterStopStartsFresh(t *testing.T) {
	h := newHarness(t, validSettings)
	h.start(t)
	// Traffic on the first ear leaves resume state behind.
	h.dispatch(t, 5, c2cEvent("m-1", "U1", "hi"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	// Starting again on the same instance is a new ear: no carried resume
	// state, fresh identify, runtime windows rebuilt empty.
	if err := h.p.Start(context.Background(), h.env); err != nil {
		t.Fatalf("restart: %v", err)
	}
	second := h.spy.nth(1)
	if second == nil {
		t.Fatalf("no second attempt built")
	}
	if second.resumeID != "" || second.resumeSeq != 0 {
		t.Fatalf("restart carried resume state (%q, %d), want fresh", second.resumeID, second.resumeSeq)
	}
	connect, identify, resume, listen, _ := second.calls()
	if connect != 1 || identify != 1 || resume != 0 || listen != 1 {
		t.Fatalf("second ear calls = (connect %d, identify %d, resume %d, listen %d), want a fresh identify",
			connect, identify, resume, listen)
	}
	h.p.mu.Lock()
	windows := len(h.p.chats)
	h.p.mu.Unlock()
	if windows != 0 {
		t.Fatalf("restart kept %d runtime windows, want 0", windows)
	}
}

func TestStopDuringConnectIsBounded(t *testing.T) {
	h := newHarness(t, validSettings)
	// The gateway dial hangs forever; Stop must still return.
	h.spy.onBuild = func(n int, f *fakeWS) {
		if n == 0 {
			f.hanging = true
		}
	}
	startErr := make(chan error, 1)
	go func() {
		startErr <- h.p.Start(context.Background(), h.env)
	}()
	time.Sleep(50 * time.Millisecond) // let Start reach the hanging dial
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

// TestNewInstallsQuietLogger: New must mute botgo's default console
// logger (it prints websocket frames and HTTP bodies at INFO, including
// the identify payload's access token and message content — D-010).
func TestNewInstallsQuietLogger(t *testing.T) {
	_ = New()
	if _, ok := log.DefaultLogger.(quietLogger); !ok {
		t.Fatalf("botgo logger after New = %T, want the quiet logger", log.DefaultLogger)
	}
}

// TestStopDuringFirstHandshakeReturns: a Stop landing while the FIRST
// attempt sits in the READY wait (a muted, never-answered handshake)
// must make Start return within a bounded wait — the supervisor reports
// the interrupted first attempt to Start instead of exiting silently.
func TestStopDuringFirstHandshakeReturns(t *testing.T) {
	h := newHarness(t, validSettings)
	h.spy.onBuild = func(n int, f *fakeWS) {
		if n == 0 {
			f.muteReady = true // the gateway never answers the handshake
		}
	}
	startErr := make(chan error, 1)
	go func() {
		startErr <- h.p.Start(context.Background(), h.env)
	}()
	// Let Start reach the READY wait: connected, authenticated, listening,
	// no READY.
	waitFor(t, "first attempt stuck in the ready wait", func() bool {
		f := h.spy.nth(0)
		if f == nil {
			return false
		}
		_, _, _, listen, _ := f.calls()
		return listen == 1
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.p.Stop(ctx); err != nil {
		t.Fatalf("stop during handshake: %v", err)
	}
	select {
	case err := <-startErr:
		if err == nil {
			t.Fatalf("Start interrupted by Stop must return an error")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Start did not return after Stop")
	}
}
