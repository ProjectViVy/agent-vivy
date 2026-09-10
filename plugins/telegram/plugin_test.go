package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mymmrac/telego"

	plugin "agent-vivy/sdk/port/channel"
)

// stubToken matches telego's token format regexp but is synthetic: the
// stub authenticates every token, so the value carries no secret.
var stubToken = "123456:" + strings.Repeat("A", 35)

// fakeEnv is an in-memory plugin.ChannelEnv: the same surface the kernel
// ChannelHost hands out, without the kernel. Secrets resolve from a map
// with the same pinning discipline the Host applies (only the declared
// token_env resolves); published inbound envelopes are recorded.
type fakeEnv struct {
	settings json.RawMessage
	secrets  map[string]string
	tokenEnv string // declared token_env; Secret is pinned to it
	client   *http.Client

	mu              sync.Mutex
	published       []plugin.InboundMessage
	publishFailures int // when > 0, the next publish fails and decrements this
	failedPublishes int // observable count of failed publishes
}

func (e *fakeEnv) ModuleID() string     { return "vivy/telegram" }
func (e *fakeEnv) Logger() *slog.Logger { return nil }

func (e *fakeEnv) Secret(envKey string) (string, error) {
	if e.tokenEnv == "" {
		return "", errors.New("no token_env declared")
	}
	if envKey != e.tokenEnv {
		return "", fmt.Errorf("env_key %q does not match the declared token_env", envKey)
	}
	v, ok := e.secrets[envKey]
	if !ok || v == "" {
		return "", errors.New("env variable empty or unset")
	}
	return v, nil
}

func (e *fakeEnv) HTTP() *http.Client { return e.client }
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
	if e.publishFailures > 0 {
		e.publishFailures--
		e.failedPublishes++
		return errors.New("journal down")
	}
	e.published = append(e.published, msg)
	return nil
}

func (e *fakeEnv) Media() plugin.MediaStore { return nil }

func (e *fakeEnv) snapshot() []plugin.InboundMessage {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]plugin.InboundMessage(nil), e.published...)
}

func (e *fakeEnv) failureCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.failedPublishes
}

func (e *fakeEnv) setFailures(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.publishFailures = n
}

// telegramStub is a loopback stand-in for the Telegram Bot API serving the
// three calls the adapter makes this slice: getMe (authentication),
// getUpdates (polling), and sendMessage (outbound text). All routing is by
// path suffix because the bot token is embedded in the URL path.
type telegramStub struct {
	server  *httptest.Server
	updates chan []map[string]any // one-shot update batches for getUpdates

	sends   chan sendMessageCall
	sendSeq atomic.Int64
}

type sendMessageCall struct {
	// ChatID stays raw: telego serializes a numeric ChatID as a JSON
	// number, a username as a string.
	ChatID    json.RawMessage `json:"chat_id"`
	Text      string          `json:"text"`
	ParseMode string          `json:"parse_mode"`
}

func newTelegramStub(t *testing.T) *telegramStub {
	t.Helper()
	stub := &telegramStub{
		updates: make(chan []map[string]any, 8),
		sends:   make(chan sendMessageCall, 16),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			writeTelegramOK(w, map[string]any{"id": 42, "is_bot": true, "username": "vivy_test_bot"})
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			select {
			case batch := <-stub.updates:
				writeTelegramOK(w, batch)
			default:
				// No batch queued: hold the request like a long poll, then
				// answer with an empty batch so the loop keeps running.
				select {
				case <-time.After(2 * time.Second):
					writeTelegramOK(w, []any{})
				case <-t.Context().Done():
				}
			}
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			body, _ := io.ReadAll(r.Body)
			var call sendMessageCall
			if err := json.Unmarshal(body, &call); err != nil {
				writeTelegramError(w, "bad request")
				return
			}
			stub.sends <- call
			writeTelegramOK(w, map[string]any{
				"message_id": stub.sendSeq.Add(1),
				"chat":       map[string]any{"id": 1, "type": "private"},
			})
		default:
			http.NotFound(w, r)
		}
	})
	stub.server = httptest.NewServer(mux)
	t.Cleanup(stub.server.Close)
	return stub
}

// pushUpdates queues one update batch; the next getUpdates call returns it.
func (s *telegramStub) pushUpdates(batch []map[string]any) { s.updates <- batch }

// waitForSend reads one recorded sendMessage call.
func (s *telegramStub) waitForSend(t *testing.T) sendMessageCall {
	t.Helper()
	select {
	case call := <-s.sends:
		return call
	case <-time.After(5 * time.Second):
		t.Fatal("sendMessage was never called")
		return sendMessageCall{}
	}
}

func writeTelegramOK(w http.ResponseWriter, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
}

func writeTelegramError(w http.ResponseWriter, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 400, "description": description})
}

// textUpdate builds one raw private-chat text update as the stub serves it.
func textUpdate(updateID int64, text string) map[string]any {
	return map[string]any{
		"update_id": updateID,
		"message": map[string]any{
			"message_id": int(updateID) + 48, // deterministic message ids 49, 50, ...
			"from":       map[string]any{"id": 123456, "is_bot": false, "first_name": "Human"},
			"chat":       map[string]any{"id": 123456, "type": "private"},
			"text":       text,
		},
	}
}

// stubSettings are the settings JSON for an env pointed at the stub.
func stubSettings(stub *telegramStub) string {
	return fmt.Sprintf(`{"token_env":"TELEGRAM_BOT_TOKEN","base_url":%q}`, stub.server.URL)
}

// envForStub builds a fake env pointed at the stub with declared settings
// and a resolvable token. settings must embed the stub base_url (use
// stubSettings) or a deliberate override.
func envForStub(settings string) *fakeEnv {
	return &fakeEnv{
		settings: json.RawMessage(settings),
		secrets:  map[string]string{"TELEGRAM_BOT_TOKEN": stubToken},
		tokenEnv: "TELEGRAM_BOT_TOKEN",
		client:   &http.Client{},
	}
}

// startForTest runs Start and registers a Stop cleanup.
func startForTest(t *testing.T, env *fakeEnv) *Plugin {
	t.Helper()
	p := newAdapter()
	if err := p.Start(context.Background(), env); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = p.Stop(ctx)
	})
	return p
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

// TestDecodeSettings: valid decode, unknown field fails closed, absent or
// malformed settings fail closed.
func TestDecodeSettings(t *testing.T) {
	s, err := DecodeSettings(json.RawMessage(`{"token_env":"TELEGRAM_BOT_TOKEN","base_url":"http://127.0.0.1:8081","proxy":"http://127.0.0.1:7890"}`))
	if err != nil {
		t.Fatalf("decode valid settings: %v", err)
	}
	if s.TokenEnv != "TELEGRAM_BOT_TOKEN" || s.BaseURL != "http://127.0.0.1:8081" || s.Proxy != "http://127.0.0.1:7890" {
		t.Fatalf("decoded settings = %+v", s)
	}

	if _, err := DecodeSettings(json.RawMessage(`{"token_env":"X","parse_mode":"html"}`)); err == nil {
		t.Fatal("unknown field must fail closed")
	}

	s, err = DecodeSettings(nil)
	if err != nil || s != (Settings{}) {
		t.Fatalf("absent settings = %+v err=%v, want zero value with no error", s, err)
	}

	if _, err := DecodeSettings(json.RawMessage(`not json`)); err == nil {
		t.Fatal("malformed settings must fail closed")
	}

	// Trailing data is a config mistake, not data.
	if _, err := DecodeSettings(json.RawMessage(`{"token_env":"A"} {"token_env":"B"}`)); err == nil {
		t.Fatal("trailing json documents must fail closed")
	}
}

// TestNormalizeUpdate: exactly one shape publishes — a private text message
// from a human. Everything else is dropped locally (the Host allow-list is
// the policy layer, not this shape filter).
func TestNormalizeUpdate(t *testing.T) {
	privateText := telego.Update{Message: &telego.Message{
		MessageID: 55,
		From:      &telego.User{ID: 123456, IsBot: false, FirstName: "Human"},
		Chat:      telego.Chat{ID: 123456, Type: "private"},
		Text:      "hello vivy",
	}}
	msg, ok := normalizeUpdate(privateText)
	if !ok {
		t.Fatal("private text message must be publishable")
	}
	if msg.Channel != "telegram" || msg.ChatID != "123456" || msg.Sender != "telegram:123456" || msg.MessageID != "55" {
		t.Fatalf("normalized envelope = %+v", msg)
	}
	if msg.ReplyTo != "" || msg.TopicID != "" {
		t.Fatalf("first cut must not thread replies or topics: %+v", msg)
	}
	if len(msg.Parts) != 1 || msg.Parts[0].Kind != plugin.PartText || msg.Parts[0].Text != "hello vivy" {
		t.Fatalf("envelope parts = %+v", msg.Parts)
	}

	cases := map[string]telego.Update{
		"edited message": {EditedMessage: &telego.Message{
			MessageID: 55, From: &telego.User{ID: 1}, Chat: telego.Chat{ID: 1, Type: "private"}, Text: "edited",
		}},
		"channel post": {ChannelPost: &telego.Message{
			MessageID: 56, From: &telego.User{ID: 1}, Chat: telego.Chat{ID: -100, Type: "channel"}, Text: "post",
		}},
		"no message at all": {},
		"group chat": {Message: &telego.Message{
			MessageID: 57, From: &telego.User{ID: 123456}, Chat: telego.Chat{ID: -999, Type: "group"}, Text: "group",
		}},
		"supergroup chat": {Message: &telego.Message{
			MessageID: 58, From: &telego.User{ID: 123456}, Chat: telego.Chat{ID: -999, Type: "supergroup"}, Text: "sg",
		}},
		"missing sender": {Message: &telego.Message{
			MessageID: 59, Chat: telego.Chat{ID: 123456, Type: "private"}, Text: "who sent this",
		}},
		"sender is a bot (echo loop guard)": {Message: &telego.Message{
			MessageID: 60, From: &telego.User{ID: 42, IsBot: true}, Chat: telego.Chat{ID: 42, Type: "private"}, Text: "echo loop",
		}},
		"message on behalf of a chat": {Message: &telego.Message{
			MessageID: 61, From: &telego.User{ID: 123456}, SenderChat: &telego.Chat{ID: 777, Type: "private"},
			Chat: telego.Chat{ID: 123456, Type: "private"}, Text: "not a human sender",
		}},
		"photo without text": {Message: &telego.Message{
			MessageID: 62, From: &telego.User{ID: 123456}, Chat: telego.Chat{ID: 123456, Type: "private"},
			Photo: []telego.PhotoSize{{FileID: "f"}},
		}},
		"empty text": {Message: &telego.Message{
			MessageID: 63, From: &telego.User{ID: 123456}, Chat: telego.Chat{ID: 123456, Type: "private"}, Text: "",
		}},
	}
	for name, upd := range cases {
		if msg, ok := normalizeUpdate(upd); ok {
			t.Fatalf("%s must not be publishable, got %+v", name, msg)
		}
	}
}

// TestStartFailsClosed: settings, secret, and authentication problems stop
// the ear before it goes live. All cases run against a live loopback stub,
// so a leaky Start would succeed and fail the test.
func TestStartFailsClosed(t *testing.T) {
	stub := newTelegramStub(t)
	cases := map[string]*fakeEnv{
		"absent settings (no token_env)": {settings: json.RawMessage(`{}`)},
		"unknown settings field":         {settings: json.RawMessage(`{"token_env":"T","proxy":"p","wat":1}`)},
		"missing token variable": {
			settings: json.RawMessage(`{"token_env":"MISSING_TOKEN"}`),
			tokenEnv: "MISSING_TOKEN",
			secrets:  map[string]string{},
		},
		"token_env mismatch with declaration": {
			settings: json.RawMessage(`{"token_env":"OTHER_NAME"}`),
			tokenEnv: "DECLARED_NAME",
			secrets:  map[string]string{"OTHER_NAME": "tok"},
		},
		"bad token format": {
			settings: json.RawMessage(`{"token_env":"TELEGRAM_BOT_TOKEN"}`),
			tokenEnv: "TELEGRAM_BOT_TOKEN",
			secrets:  map[string]string{"TELEGRAM_BOT_TOKEN": "not-a-token"},
		},
		"unreachable api server": {
			settings: json.RawMessage(`{"token_env":"TELEGRAM_BOT_TOKEN","base_url":"http://127.0.0.1:1"}`),
			tokenEnv: "TELEGRAM_BOT_TOKEN",
			secrets:  map[string]string{"TELEGRAM_BOT_TOKEN": stubToken},
		},
	}
	for name, env := range cases {
		env.client = &http.Client{Timeout: 2 * time.Second}
		p := newAdapter()
		err := p.Start(context.Background(), env)
		if err == nil {
			t.Fatalf("%s: Start must fail closed", name)
		}
		if p.bot != nil || p.cancel != nil {
			t.Fatalf("%s: failed Start must not leave a live bot", name)
		}
	}
	// The healthy env still starts against the stub, proving the failures
	// above are each case's own fault.
	startForTest(t, envForStub(stubSettings(stub)))
}

// TestStartStopFullLoop: the whole Start → PublishInbound → Stop path
// against the loopback stub — Start authenticates, the poll loop publishes
// a normalized private text envelope, one failed dispatch does not kill
// the ear, and Stop sheds the poll goroutine promptly.
func TestStartStopFullLoop(t *testing.T) {
	stub := newTelegramStub(t)
	env := envForStub(stubSettings(stub))

	p := startForTest(t, env)

	stub.pushUpdates([]map[string]any{textUpdate(7, "hello vivy")})
	waitFor(t, "first inbound envelope", func() bool { return len(env.snapshot()) == 1 })
	got := env.snapshot()[0]
	if got.Channel != "telegram" || got.Sender != "telegram:123456" || got.ChatID != "123456" ||
		len(got.Parts) != 1 || got.Parts[0].Text != "hello vivy" {
		t.Fatalf("published envelope = %+v", got)
	}

	// A dispatch failure must not kill the ear: the failed envelope is
	// dropped, and the next update still lands.
	env.setFailures(1)
	stub.pushUpdates([]map[string]any{textUpdate(8, "during outage")})
	waitFor(t, "failed publish", func() bool { return env.failureCount() == 1 })
	stub.pushUpdates([]map[string]any{textUpdate(9, "after outage")})
	waitFor(t, "second inbound envelope", func() bool { return len(env.snapshot()) == 2 })

	// Stop ends the loop promptly and is idempotent.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("second stop: %v", err)
	}
	if err := (newAdapter()).Stop(context.Background()); err != nil {
		t.Fatalf("stop without start: %v", err)
	}

	// After the poll goroutine has exited, further updates are never
	// consumed: the ear is really down.
	stub.pushUpdates([]map[string]any{textUpdate(10, "too late")})
	time.Sleep(100 * time.Millisecond)
	if got := len(env.snapshot()); got != 2 {
		t.Fatalf("published envelopes after stop = %d, want 2", got)
	}
}

// TestSendPlainText: Send delivers text parts as plain sendMessage calls
// (no parse_mode) to the numeric chat and returns the platform ids.
func TestSendPlainText(t *testing.T) {
	stub := newTelegramStub(t)
	env := envForStub(stubSettings(stub))

	// Send before Start fails closed on the missing bot handle.
	if _, err := (newAdapter()).Send(context.Background(), plugin.OutboundMessage{ChatID: "1", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}}); err == nil {
		t.Fatal("Send before Start must fail")
	}

	p := startForTest(t, env)

	ids, err := p.Send(context.Background(), plugin.OutboundMessage{
		ChatID: "123456",
		Parts: []plugin.Part{
			{Kind: plugin.PartText, Text: "first"},
			{Kind: plugin.PartMediaRef, MediaRef: "noop:x"}, // skipped: text-only this slice
			{Kind: plugin.PartText, Text: "second"},
		},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(ids) != 2 || ids[0] != "1" || ids[1] != "2" {
		t.Fatalf("send ids = %v, want [1 2]", ids)
	}

	first := stub.waitForSend(t)
	second := stub.waitForSend(t)
	for _, call := range []sendMessageCall{first, second} {
		if string(call.ChatID) != "123456" || call.ParseMode != "" {
			t.Fatalf("sendMessage call = %+v, want numeric chat id and no parse_mode", call)
		}
	}
	if first.Text != "first" || second.Text != "second" {
		t.Fatalf("sendMessage texts = %q, %q", first.Text, second.Text)
	}

	// A non-numeric chat id fails closed with a clear error.
	if _, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "@not-numeric", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}}); err == nil {
		t.Fatal("non-numeric chat id must fail")
	}

	// An envelope with no text parts sends nothing.
	if ids, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "123456", Parts: []plugin.Part{{Kind: plugin.PartMediaRef, MediaRef: "noop:x"}}}); err != nil || len(ids) != 0 {
		t.Fatalf("media-only envelope = ids %v err %v, want no send and no error", ids, err)
	}
}

// TestSendIsIdempotentAfterStop: Stop does not nil the bot, so a delivery
// racing Stop either succeeds or fails at the API/context level — never
// with a nil-pointer panic (CH-C3-N2, adapter side).
func TestSendIsIdempotentAfterStop(t *testing.T) {
	stub := newTelegramStub(t)
	env := envForStub(stubSettings(stub))
	p := newAdapter()
	if err := p.Start(context.Background(), env); err != nil {
		t.Fatalf("start: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	// The call may succeed (loopback race) or fail with a context error;
	// the contract is "no panic, no deadlock".
	_, _ = p.Send(context.Background(), plugin.OutboundMessage{ChatID: "123456", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "after stop"}}})
}

// TestSendSurfacesAPIError: a platform error surfaces to the Host as a
// Send error (the Host logs it; the adapter has no logger of its own).
func TestSendSurfacesAPIError(t *testing.T) {
	stub := newTelegramStub(t)
	env := envForStub(stubSettings(stub))

	p := newAdapter()
	if err := p.Start(context.Background(), env); err != nil {
		t.Fatalf("start: %v", err)
	}
	// The platform goes away after Start; Send must surface that instead
	// of silently pretending the delivery happened.
	stub.server.Close()
	if _, err := p.Send(context.Background(), plugin.OutboundMessage{ChatID: "123456", Parts: []plugin.Part{{Kind: plugin.PartText, Text: "x"}}}); err == nil {
		t.Fatal("send against a dead platform must surface an error")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
}
