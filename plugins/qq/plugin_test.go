package qq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/constant"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/errs"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/log"
	"github.com/tencent-connect/botgo/openapi/options"
	"golang.org/x/oauth2"

	plugin "agent-vivy/sdk/port/channel"
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
	logger   *slog.Logger
	client   *http.Client

	mu        sync.Mutex
	published []plugin.InboundMessage
}

func (e *fakeEnv) ModuleID() string { return "vivy/qq" }

func (e *fakeEnv) Secret(envKey string) (string, error) {
	v, ok := os.LookupEnv(envKey)
	if !ok || v == "" {
		return "", fmt.Errorf("env variable %q empty or unset", envKey)
	}
	return v, nil
}

func (e *fakeEnv) HTTP() *http.Client {
	if e.client != nil {
		return e.client
	}
	return &http.Client{}
}
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

// Logger implements the optional plugin.ChannelLogger face; nil (unset)
// keeps the supervisor silent, mirroring an env without the face.
func (e *fakeEnv) Logger() *slog.Logger { return e.logger }

func (e *fakeEnv) snapshot() []plugin.InboundMessage {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]plugin.InboundMessage(nil), e.published...)
}

type testRoundTripper func(*http.Request) (*http.Response, error)

func (fn testRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestGovernedTokenSourceRedactsEchoedSecret(t *testing.T) {
	const secret = "qq-secret-must-not-leak"
	env := &fakeEnv{client: &http.Client{Transport: testRoundTripper(func(*http.Request) (*http.Response, error) {
		body := `{"code":401,"message":"invalid ` + secret + `"}`
		return &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	source := newGovernedQQTokenSource(context.Background(), env, "app-id", secret)
	_, err := source.Token()
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "invalid") {
		t.Fatalf("Token() error = %v, want redacted endpoint failure", err)
	}
}

func TestGovernedTokenSourceHonorsLifetimeCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	env := &fakeEnv{client: &http.Client{Transport: testRoundTripper(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}}
	_, err := newGovernedQQTokenSource(ctx, env, "app-id", "secret").Token()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Token() error = %v, want context.Canceled", err)
	}
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

	mu         sync.Mutex
	c2cCalls   []c2cCall
	groupCalls []groupCall
}

type c2cCall struct {
	userID string
	msg    dto.APIMessage
}

type groupCall struct {
	groupOpenID string
	msg         dto.APIMessage
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

// PostGroupMessage records the group post like its C2C sibling.
func (a *fakeAPI) PostGroupMessage(_ context.Context, groupOpenID string, msg dto.APIMessage, _ ...options.Option) (*dto.Message, error) {
	a.mu.Lock()
	a.groupCalls = append(a.groupCalls, groupCall{groupOpenID: groupOpenID, msg: msg})
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
	onGroup   groupATMessageHandler
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

func (s *wsFactorySpy) build(onC2C event.C2CMessageEventHandler, onGroup groupATMessageHandler, onReady event.ReadyHandler,
	gatewayURL string, _ oauth2.TokenSource, resumeID string, resumeSeq uint32) wsClient {
	f := &fakeWS{
		url:        gatewayURL,
		resumeID:   resumeID,
		resumeSeq:  resumeSeq,
		onC2C:      onC2C,
		onGroup:    onGroup,
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
	h.p = newAdapter()
	h.p.newTokenSource = func(context.Context, string, string) oauth2.TokenSource { return h.ts }
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
		s, err := DecodeSettings(json.RawMessage(`{"app_id_env":" A ","app_secret_env":" B ","sandbox":true,"markdown":true}`))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if s.AppIDEnv != "A" || s.AppSecretEnv != "B" || !s.Sandbox || !s.Markdown {
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
			got, _, publishable := normalizeC2C(tc.data)
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

	p := newAdapter()
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
	// failMarkdown rejects msg_type 2 posts with the platform's markdown
	// rejection while text posts succeed (the fallback stand-in).
	failMarkdown bool
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
		if lb.failMarkdown && bytes.Contains(body, []byte(`"msg_type":2`)) {
			code, bodyText = http.StatusBadRequest, `{"code":11253,"message":"markdown not allowed","trace_id":"t-1"}`
		}
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

// rejectMarkdown turns on the markdown rejection mode.
func (lb *loopbackServer) rejectMarkdown() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.failMarkdown = true
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
	waitFor(t, "redial after an unresumable close", func() bool {
		if h.spy.count() < 3 {
			return false
		}
		connect, identify, resume, _, _ := h.spy.nth(2).calls()
		return connect == 1 && identify == 1 && resume == 0
	})
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

// --- supervised redial visibility (CH-C6-N1) -----------------------------------

// logBuffer is a mutex-guarded bytes.Buffer: the supervisor logs from its
// own goroutine while the test polls the output.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// TestRedialFailuresLogged (CH-C6-N1): after the first live attempt dies,
// a refused dial and the eventual recovery are logged through the env's
// ChannelLogger face instead of staying silent.
func TestRedialFailuresLogged(t *testing.T) {
	shrinkRedialDelay(t)
	var buf logBuffer
	h := newHarness(t, validSettings)
	h.env.logger = slog.New(slog.NewTextHandler(&buf, nil))
	// Attempt #1 (the first redial after the drop) dials refused; every
	// later attempt dials normally, so the ear recovers.
	h.spy.onBuild = func(n int, f *fakeWS) {
		if n == 1 {
			f.connectErr = errors.New("refused")
		}
	}
	h.start(t)

	h.spy.nth(0).drop(errors.New("connection reset"))
	waitFor(t, "session-death warning", func() bool {
		return strings.Contains(buf.String(), "stage=session")
	})
	// The dial stage is logged as stage="dial gateway" (TextHandler
	// quotes the space), so assert on the error text instead.
	waitFor(t, "refused-dial warning", func() bool {
		return strings.Contains(buf.String(), "err=refused")
	})
	waitFor(t, "reconnect info line", func() bool {
		return strings.Contains(buf.String(), "gateway reconnected")
	})
	if !strings.Contains(buf.String(), "failed_attempts=2") {
		t.Fatalf("reconnect line lacks the attempt count: %s", buf.String())
	}
}

// TestGiveUpLogged (CH-C6-N1): the terminal cannot-identify give-up — the
// ear stays started but deaf — errors through the log face.
func TestGiveUpLogged(t *testing.T) {
	shrinkRedialDelay(t)
	var buf logBuffer
	h := newHarness(t, validSettings)
	h.env.logger = slog.New(slog.NewTextHandler(&buf, nil))
	h.start(t)

	h.spy.nth(0).drop(errs.New(errs.CodeConnCloseCantIdentify, "bot delisted"))
	waitFor(t, "give-up error line", func() bool {
		return strings.Contains(buf.String(), "ear stays deaf until the channel restarts")
	})
}

// TestHealthClassifiesRedialingGateway (CH-R-1): a broken session is
// temporary while the loop redials, and Health returns to nil after the
// gateway reconnects.
func TestHealthClassifiesRedialingGateway(t *testing.T) {
	shrinkRedialDelay(t)
	h := newHarness(t, validSettings)
	// Attempt #1 (the first redial after the drop) dials refused; every
	// later attempt dials normally, so the ear recovers.
	h.spy.onBuild = func(n int, f *fakeWS) {
		if n == 1 {
			f.connectErr = errors.New("refused")
		}
	}
	h.start(t)

	h.spy.nth(0).drop(errors.New("connection reset"))
	var healthErr *plugin.HealthError
	waitFor(t, "temporary health while redialing", func() bool {
		err := h.p.Health(context.Background())
		return errors.As(err, &healthErr) && healthErr.Class == plugin.ClassTemporary
	})
	waitFor(t, "healthy after reconnect", func() bool {
		return h.p.Health(context.Background()) == nil
	})
}

// TestHealthDeadAfterGiveUp (CH-R-1): the terminal cannot-identify close
// surfaces through Health as a dead classification — the ear cannot recover
// on its own.
func TestHealthDeadAfterGiveUp(t *testing.T) {
	shrinkRedialDelay(t)
	h := newHarness(t, validSettings)
	h.start(t)

	h.spy.nth(0).drop(errs.New(errs.CodeConnCloseCantIdentify, "bot delisted"))
	var healthErr *plugin.HealthError
	waitFor(t, "dead health after give-up", func() bool {
		err := h.p.Health(context.Background())
		return errors.As(err, &healthErr) && healthErr.Class == plugin.ClassDead
	})
}

// TestTypingInputNotifyLoopback: plugin.Typing sends one InputNotify
// (msg_type 6) anchored to the open passive window's msg_id. Without a
// window there is nothing to anchor to — no request, no error (typing is
// best-effort). InputNotify carries no msg_seq.
func TestTypingInputNotifyLoopback(t *testing.T) {
	lb := newLoopback(t)
	h := newRealAPIHarness(t, validSettings)
	h.start(t)

	if err := h.p.Typing(context.Background(), "OPENID-NOWINDOW"); err != nil {
		t.Fatalf("typing without a window: %v", err)
	}
	if got := len(lb.sent()); got != 0 {
		t.Fatalf("c2c posts without a window = %d, want 0", got)
	}

	h.dispatch(t, 7, c2cEvent("in-1", "OPENID1", "hello"))
	if err := h.p.Typing(context.Background(), "OPENID1"); err != nil {
		t.Fatalf("typing: %v", err)
	}
	sent := lb.sent()
	if len(sent) != 1 {
		t.Fatalf("c2c posts = %d, want 1", len(sent))
	}
	if want := "/v2/users/OPENID1/messages"; sent[0].path != want {
		t.Fatalf("post path = %s, want %s", sent[0].path, want)
	}
	var body struct {
		MsgType     *int   `json:"msg_type"`
		MsgID       string `json:"msg_id"`
		InputNotify *struct {
			InputType   int   `json:"input_type"`
			InputSecond int32 `json:"input_second"`
		} `json:"input_notify"`
	}
	if err := json.Unmarshal(sent[0].body, &body); err != nil {
		t.Fatalf("body decode: %v (%s)", err, sent[0].body)
	}
	if body.MsgType == nil || *body.MsgType != 6 {
		t.Fatalf("msg_type = %v, want 6 (input notify)", body.MsgType)
	}
	if body.MsgID != "in-1" {
		t.Fatalf("msg_id = %q, want the passive window in-1", body.MsgID)
	}
	if body.InputNotify == nil || body.InputNotify.InputType != 1 || body.InputNotify.InputSecond != 10 {
		t.Fatalf("input_notify = %+v, want input_type 1 for 10s", body.InputNotify)
	}
}

// TestSendMarkdownFlagWithFallback: with settings.markdown the reply goes
// out as msg_type 2 native markdown anchored to the passive window; a
// platform rejection (most robots lack the markdown permission) retries
// the same chunk as plain text — burning one seq on the way, which the
// platform's (msg_id, msg_seq) dedup treats as a normal gap.
func TestSendMarkdownFlagWithFallback(t *testing.T) {
	lb := newLoopback(t)
	lb.rejectMarkdown()
	markdownSettings := `{"app_id_env":"` + stubAppIDEnvName + `","app_secret_env":"` + stubAppSecretEnvName + `","markdown":true}`
	h := newRealAPIHarness(t, markdownSettings)
	h.start(t)

	h.dispatch(t, 7, c2cEvent("in-1", "OPENID1", "hello"))
	ids, err := h.p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "OPENID1",
		Parts:  []plugin.Part{{Kind: plugin.PartText, Text: "**bold**"}},
	})
	if err != nil || len(ids) != 1 {
		t.Fatalf("send with fallback = ids %v err %v, want one delivered id", ids, err)
	}
	sent := lb.sent()
	if len(sent) != 2 {
		t.Fatalf("c2c posts = %d, want the markdown attempt plus the text fallback", len(sent))
	}

	var mdBody struct {
		MsgType  *int `json:"msg_type"`
		Markdown *struct {
			Content string `json:"content"`
		} `json:"markdown"`
		MsgID  string `json:"msg_id"`
		MsgSeq int    `json:"msg_seq"`
	}
	if err := json.Unmarshal(sent[0].body, &mdBody); err != nil {
		t.Fatalf("markdown body decode: %v (%s)", err, sent[0].body)
	}
	if mdBody.MsgType == nil || *mdBody.MsgType != 2 {
		t.Fatalf("markdown msg_type = %v, want 2", mdBody.MsgType)
	}
	if mdBody.Markdown == nil || mdBody.Markdown.Content != "**bold**" {
		t.Fatalf("markdown body = %+v, want the native markdown content", mdBody.Markdown)
	}
	if mdBody.MsgID != "in-1" || mdBody.MsgSeq != 1 {
		t.Fatalf("markdown window = %q seq %d, want in-1 seq 1", mdBody.MsgID, mdBody.MsgSeq)
	}

	var txtBody struct {
		Content string `json:"content"`
		MsgSeq  int    `json:"msg_seq"`
	}
	if err := json.Unmarshal(sent[1].body, &txtBody); err != nil {
		t.Fatalf("fallback body decode: %v (%s)", err, sent[1].body)
	}
	if txtBody.Content != "**bold**" {
		t.Fatalf("fallback content = %q, want the raw markdown as plain text", txtBody.Content)
	}
	if txtBody.MsgSeq != 2 {
		t.Fatalf("fallback seq = %d, want 2 (the rejected markdown burned one)", txtBody.MsgSeq)
	}
}

// TestSendThreadsToReplyTo: the reply anchors to the ReplyTo msg_id — not
// whatever inbound replaced the window state in the meantime. Two windows
// opened in sequence; threading to the older one must still quote it.
func TestSendThreadsToReplyTo(t *testing.T) {
	lb := newLoopback(t)
	h := newRealAPIHarness(t, validSettings)
	h.start(t)

	h.dispatch(t, 7, c2cEvent("in-1", "OPENID1", "first"))
	h.dispatch(t, 8, c2cEvent("in-2", "OPENID1", "second"))

	if _, err := h.p.Send(context.Background(), plugin.OutboundMessage{
		ChatID:  "OPENID1",
		ReplyTo: "in-1",
		Parts:   []plugin.Part{{Kind: plugin.PartText, Text: "threaded"}},
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
	sent := lb.sent()
	if len(sent) != 1 {
		t.Fatalf("c2c posts = %d, want 1", len(sent))
	}
	var body struct {
		MsgID  string `json:"msg_id"`
		MsgSeq int    `json:"msg_seq"`
	}
	if err := json.Unmarshal(sent[0].body, &body); err != nil {
		t.Fatalf("body decode: %v (%s)", err, sent[0].body)
	}
	if body.MsgID != "in-1" {
		t.Fatalf("msg_id = %q, want the threaded in-1 (not the window's in-2)", body.MsgID)
	}
	if body.MsgSeq != 1 {
		t.Fatalf("msg_seq = %d, want 1", body.MsgSeq)
	}
}

// --- group trigger (tier-1, mention-only) --------------------------------------

// wsGroupPayload builds the ws payload the group handler reads the resume
// sequence from.
func wsGroupPayload(seq uint32) *dto.WSPayload {
	return &dto.WSPayload{WSPayloadBase: dto.WSPayloadBase{Seq: seq}}
}

// TestGroupATMessageRoundTrip: a group AT event publishes an envelope
// keyed by the group_openid with the member openid sender, opens the
// group's passive window, and the reply routes through the group endpoint
// anchored to the event's msg_id. Group AT messages are mention-only by
// construction (the platform strips the @bot prefix from content).
func TestGroupATMessageRoundTrip(t *testing.T) {
	h := newHarness(t, validSettings)
	h.start(t)

	if err := h.p.onGroup(wsGroupPayload(9), &groupATMessage{
		ID:          "gm-1",
		Content:     "what is up",
		GroupOpenID: "GROUPOPEN1",
		Author:      groupAuthor("MEMBER1"),
	}); err != nil {
		t.Fatalf("group dispatch: %v", err)
	}
	waitFor(t, "group envelope", func() bool { return len(h.env.snapshot()) == 1 })
	got := h.env.snapshot()[0]
	if got.ChatID != "GROUPOPEN1" || got.Sender != "qq:user_MEMBER1" || got.MessageID != "gm-1" || got.Parts[0].Text != "what is up" {
		t.Fatalf("group envelope = %+v", got)
	}

	ids, err := h.p.Send(context.Background(), plugin.OutboundMessage{
		ChatID:  "GROUPOPEN1",
		ReplyTo: "gm-1",
		Parts:   []plugin.Part{{Kind: plugin.PartText, Text: "group reply"}},
	})
	if err != nil || len(ids) != 1 {
		t.Fatalf("group send = ids %v err %v", ids, err)
	}

	h.p.mu.Lock()
	api := h.p.api.(*fakeAPI)
	h.p.mu.Unlock()
	if got := api.c2cCount(); got != 0 {
		t.Fatalf("c2c posts = %d, want 0 (the reply must route to the group endpoint)", got)
	}
	if len(api.groupCalls) != 1 || api.groupCalls[0].groupOpenID != "GROUPOPEN1" {
		t.Fatalf("group calls = %+v, want one post to GROUPOPEN1", api.groupCalls)
	}
	body := api.groupCalls[0].msg.(*dto.MessageToCreate)
	if body.Content != "group reply" || body.MsgID != "gm-1" || body.MsgSeq != 1 {
		t.Fatalf("group body = %+v, want the reply anchored to gm-1 seq 1", body)
	}
}

// TestNormalizeGroupShapeFilter: missing group address, sender, message
// id, or content each drop the event locally.
func TestNormalizeGroupShapeFilter(t *testing.T) {
	valid := &groupATMessage{ID: "gm-9", Content: "hi", GroupOpenID: "GROUP1", Author: groupAuthor("MEMBER1")}
	if msg, _, ok := normalizeGroup(valid); !ok || msg.ChatID != "GROUP1" || msg.Sender != "qq:user_MEMBER1" {
		t.Fatalf("valid group event = %+v ok=%v", msg, ok)
	}
	for name, data := range map[string]*groupATMessage{
		"nil":              nil,
		"no content":       {ID: "gm-1", GroupOpenID: "G", Author: groupAuthor("M")},
		"no group openid":  {ID: "gm-1", Content: "hi", Author: groupAuthor("M")},
		"no member openid": {ID: "gm-1", Content: "hi", GroupOpenID: "G"},
		"no message id":    {Content: "hi", GroupOpenID: "G", Author: groupAuthor("M")},
	} {
		if _, _, ok := normalizeGroup(data); ok {
			t.Fatalf("%s must not be publishable", name)
		}
	}
}

// groupAuthor builds the anonymous author member of a groupATMessage.
func groupAuthor(memberOpenID string) struct {
	MemberOpenID string `json:"member_openid"`
} {
	return struct {
		MemberOpenID string `json:"member_openid"`
	}{MemberOpenID: memberOpenID}
}

// stubQQImageBytes is a JPEG-magic payload; the adapter does not sniff
// (the Host does), but real magic bytes keep the fixture honest.
var stubQQImageBytes = append([]byte{0xff, 0xd8, 0xff, 0xe0}, bytes.Repeat([]byte{0x00}, 32)...)

// TestNormalizeC2CImagePreScreen: image attachments return as download
// refs, other attachments survive as [file: name] annotations, and an
// image-only message is a valid turn.
func TestNormalizeC2CImagePreScreen(t *testing.T) {
	data := c2cEvent("m-img-1", "U1", "look")
	data.Attachments = []*dto.MessageAttachment{
		{URL: "https://multimedia.qq.com/pic.jpg", FileName: "pic.jpg", ContentType: "image/jpeg"},
		{URL: "https://multimedia.qq.com/notes.txt", FileName: "notes.txt", ContentType: "text/plain"},
	}
	msg, refs, publishable := normalizeC2C(data)
	if !publishable {
		t.Fatal("text + media message must be publishable")
	}
	if len(refs) != 1 || refs[0].url != "https://multimedia.qq.com/pic.jpg" || refs[0].name != "pic.jpg" {
		t.Fatalf("image refs = %+v", refs)
	}
	if len(msg.Parts) != 2 || msg.Parts[0].Text != "look" || msg.Parts[1].Text != "[file: notes.txt]" {
		t.Fatalf("parts = %+v, want text then the file annotation", msg.Parts)
	}

	imageOnly := c2cEvent("m-img-2", "U1", "")
	imageOnly.Attachments = []*dto.MessageAttachment{
		{URL: "https://multimedia.qq.com/pic.png", FileName: "pic.png", ContentType: "image/png"},
	}
	if msg, refs, ok := normalizeC2C(imageOnly); !ok || len(refs) != 1 || len(msg.Parts) != 0 {
		t.Fatalf("image-only: ok=%v refs=%d parts=%+v", ok, len(refs), msg.Parts)
	}
}

// TestInboundImageDownloadsWithAuthHeaders: the handler downloads image
// attachments through the governed transport carrying X-Union-Appid and
// the bearer token; a failed download keeps the text and the annotation.
func TestInboundImageDownloadsWithAuthHeaders(t *testing.T) {
	var gotAuth, gotAppid atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		gotAppid.Store(r.Header.Get("X-Union-Appid"))
		_, _ = w.Write(stubQQImageBytes)
	}))
	t.Cleanup(srv.Close)

	h := newHarness(t, validSettings)
	h.start(t)
	data := c2cEvent("m-dl-1", "U1", "look")
	data.Attachments = []*dto.MessageAttachment{
		{URL: srv.URL + "/pic.jpg", FileName: "pic.jpg", ContentType: "image/jpeg"},
	}
	h.dispatch(t, 1, data)
	waitFor(t, "downloaded envelope", func() bool { return len(h.env.snapshot()) == 1 })

	env1 := h.env.snapshot()[0]
	if len(env1.Parts) != 3 ||
		env1.Parts[1].Text != "[image: pic.jpg]" ||
		env1.Parts[2].Kind != plugin.PartMedia ||
		string(env1.Parts[2].Media.Data) != string(stubQQImageBytes) {
		t.Fatalf("envelope parts = %+v", env1.Parts)
	}
	if auth, _ := gotAuth.Load().(string); auth != "QQBot "+stubAccessToken {
		t.Fatalf("Authorization = %q, want the bearer token", auth)
	}
	if appid, _ := gotAppid.Load().(string); appid == "" {
		t.Fatal("X-Union-Appid header missing")
	}
}

// TestGroupImagePreScreenAndDownload: group AT messages carry the same
// media pipeline behind the mention gate.
func TestGroupImagePreScreenAndDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(stubQQImageBytes)
	}))
	t.Cleanup(srv.Close)

	h := newHarness(t, validSettings)
	h.start(t)
	data := &groupATMessage{
		ID: "gm-img-1", Content: "look", GroupOpenID: "GROUP1",
		Author: groupAuthor("MEMBER1"),
		Attachments: []*groupAttachment{
			{URL: srv.URL + "/pic.jpg", FileName: "pic.jpg", ContentType: "image/jpeg"},
		},
	}
	f := h.spy.nth(0)
	if f.onGroup == nil {
		t.Fatal("group handler not registered")
	}
	if err := f.onGroup(wsPayload(1), data); err != nil {
		t.Fatalf("dispatch group event: %v", err)
	}
	waitFor(t, "group media envelope", func() bool { return len(h.env.snapshot()) == 1 })
	env1 := h.env.snapshot()[0]
	if env1.ChatID != "GROUP1" || len(env1.Parts) != 3 ||
		env1.Parts[2].Kind != plugin.PartMedia ||
		string(env1.Parts[2].Media.Data) != string(stubQQImageBytes) {
		t.Fatalf("group envelope = %+v", env1)
	}
}
