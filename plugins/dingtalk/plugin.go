// Package dingtalk is a seam-channel adapter (VIVY-CHANNEL-PACK.md §9):
// single-chat text in / text out over the DingTalk Stream gateway. It is
// the dingtalk instance of the shape pinned by plugins/telegram — same
// file layout, same Start/Stop/Send skeleton.
//
// First-cut scope (what this adapter deliberately does NOT do): no group
// chats, no cards or interactive media, no webhook-mode bot, no markdown
// or actionCard message suites. Single-chat TEXT only.
//
// Transport: an OUTBOUND websocket to the DingTalk Stream gateway
// (manifest transport "poll", grant channel.poll). The adapter never opens
// a listen socket.
//
// Policy boundaries:
//   - allow_from is enforced by the kernel ChannelHost at dispatch; the
//     adapter never filters senders itself.
//   - secrets travel only through ChannelEnv.Secret (env_key names, never
//     values in config); the app key and app secret each get their own
//     settings-declared *_env name (CH-C6/D2).
//   - the sessionWebhook (DingTalk's per-message reply URL) is plugin-side
//     runtime state keyed by conversation. It never enters kernel config,
//     the config envelope, or a log line, and it does not survive a
//     restart — a Send for a chat that has not messaged the bot since the
//     process started fails closed.
package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"

	plugin "agent-vivy/sdk/port/channel"
)

// ChannelName is the platform name carried by every inbound envelope and
// the config envelope key this adapter consumes (channels.dingtalk).
const ChannelName = "dingtalk"

// senderPrefix is the allow_from entry format for a DingTalk sender
// (contract §11): "dingtalk:<senderStaffId or senderId>". The Host matches
// InboundMessage.Sender against allow_from exactly.
const senderPrefix = ChannelName + ":"

// singleChatType is the DingTalk conversationType for one-to-one robot
// chats; group chats report "2" and are ignored this slice.
const singleChatType = "1"

// streamRedialDelay is how often the supervisor offers the stream client a
// redial while it is disconnected. The SDK's own auto-reconnect is
// disabled (it redials forever on a background context, which would keep
// the ear alive after Stop); this loop is the context-aware replacement.
// A var so lifecycle tests can shrink it (qq's shrinkRedialDelay pattern).
var streamRedialDelay = 3 * time.Second

// webhookMaxBodyBytes bounds one sessionWebhook reply body read.
const webhookMaxBodyBytes = 64 << 10

// streamClient is the focused DingTalk Stream surface this adapter drives.
// It exists so tests can stand in for the real websocket client (CH-C6/D1);
// production uses a minimal protocol client whose HTTP and websocket paths
// are both injected by ChannelEnv.
type streamClient interface {
	// RegisterChatBotCallbackRouter wires the chatbot callback handler.
	RegisterChatBotCallbackRouter(handler chatbot.IChatBotMessageHandler)
	// Start connects (or, with an existing healthy connection, is a
	// no-op). Credential and gateway failures surface here.
	Start(ctx context.Context) error
	// Close drops the connection, if any.
	Close()
}

// streamCreds carries the resolved DingTalk app credentials between Start
// and the client factory. Values live only in memory (D-010).
type streamCreds struct {
	ClientID     string
	ClientSecret string
}

// Plugin is the transport implementation bound by the v1 ChannelProvider.
type Plugin struct {
	// mu guards the mutable fields below. Callbacks arrive on SDK
	// goroutines while Stop may run on any other goroutine.
	mu   sync.Mutex
	host plugin.ChannelEnv
	// webhooks maps conversation id → latest sessionWebhook URL. This is
	// the only reply path DingTalk gives a robot message; it is runtime
	// state only (never persisted, never logged, empty after restart).
	webhooks map[string]string
	// stream is the live stream client; nil before Start and after Stop.
	stream streamClient
	// newClient is the stream client factory; New pins the production
	// constructor and tests swap it. Never mutated after New.
	newClient func(creds streamCreds, openAPIHost string) streamClient
	// http is the Host's outbound client captured at Start (Send posts
	// sessionWebhook replies through it).
	http *http.Client
	// cancel stops the supervisor loop; nil until Start.
	cancel context.CancelFunc
	// done is closed when the supervisor goroutine exited; nil until
	// Start. Stop and supervise communicate only through this channel and
	// the mutex, never by re-reading the fields after setup.
	done chan struct{}
	// stopped latches once Stop ran; the supervisor checks it before
	// every redial so a redial can never resurrect a stopped ear.
	stopped bool
	// logger is the kernel log face captured at Start (the optional
	// plugin.ChannelLogger face on the env, CH-C6-N1): the supervisor
	// reports failed redials and reconnects through it instead of staying
	// silent. Written once by Start before the supervisor goroutine exists
	// and never mutated afterwards, so the loop reads it without the
	// mutex; nil (env without the face) keeps the loop silent.
	logger *slog.Logger
}

func newAdapter() *Plugin {
	p := &Plugin{webhooks: make(map[string]string)}
	p.newClient = func(creds streamCreds, openAPIHost string) streamClient {
		p.mu.Lock()
		host := p.host
		p.mu.Unlock()
		return newGovernedStreamClient(host, creds, openAPIHost)
	}
	return p
}

// Start implements plugin.Channel. Fail-closed order: settings must decode
// and declare both env_key names, both credentials must resolve through
// the Host-pinned Secret, and the stream client must authenticate and
// connect (the gateway round-trip happens inside Start) before any ear
// goes live.
//
// The passed ctx stays the parent of the supervisor loop: cancelling it
// (or calling Stop) takes the ear down.
func (p *Plugin) Start(ctx context.Context, env plugin.ChannelEnv) error {
	p.mu.Lock()
	// A new Start is a new ear: a Stop that ran before this Start must
	// not deafen it (the callback fence reads this latch). (Stop remains
	// idempotent within an ear's lifetime.)
	p.stopped = false
	p.mu.Unlock()
	settings, err := DecodeSettings(env.Settings())
	if err != nil {
		return err
	}
	if settings.ClientIDEnv == "" {
		return errors.New("dingtalk: settings.client_id_env is required " +
			"(channels.dingtalk.settings.client_id_env must name the app key variable)")
	}
	if settings.ClientSecretEnv == "" {
		return errors.New("dingtalk: settings.client_secret_env is required " +
			"(channels.dingtalk.settings.client_secret_env must name the app secret variable)")
	}
	if settings.ClientIDEnv == settings.ClientSecretEnv {
		return errors.New("dingtalk: settings.client_id_env and client_secret_env must name different variables")
	}
	clientID, err := env.Secret(settings.ClientIDEnv)
	if err != nil {
		return fmt.Errorf("dingtalk: resolve client id through env %q: %w", settings.ClientIDEnv, err)
	}
	clientSecret, err := env.Secret(settings.ClientSecretEnv)
	if err != nil {
		return fmt.Errorf("dingtalk: resolve client secret through env %q: %w", settings.ClientSecretEnv, err)
	}
	p.mu.Lock()
	p.host = env
	p.mu.Unlock()

	stream := p.newClient(streamCreds{ClientID: clientID, ClientSecret: clientSecret}, settings.OpenAPIHost)
	runCtx, cancel := context.WithCancel(ctx)
	// The callback is registered before the first dial so no message can
	// arrive between connect and register.
	stream.RegisterChatBotCallbackRouter(p.onChatBotMessage(env))
	// Start performs the gateway ticket exchange (which is what
	// rejects bad credentials) and the websocket handshake, then returns;
	// its read loop runs on SDK goroutines. Any failure here — auth,
	// gateway unreachable, handshake refused — fails Start closed.
	if err := stream.Start(runCtx); err != nil {
		cancel()
		return fmt.Errorf("dingtalk: connect stream: %w", err)
	}

	done := make(chan struct{})
	p.mu.Lock()
	p.http = env.HTTP()
	// The optional log face (CH-C6-N1): capture before the supervisor
	// goroutine starts, per the field's set-once contract.
	if lc, ok := env.(plugin.ChannelLogger); ok {
		p.logger = lc.Logger()
	}
	p.stream = stream
	p.cancel = cancel
	p.done = done
	p.mu.Unlock()
	go p.supervise(runCtx, done)
	return nil
}

// supervise keeps the stream client connected until the context is
// cancelled. The protocol client's auto-reconnect is deliberately absent;
// this loop is the context-aware owner: while the client holds a connection,
// Start is a cheap no-op, so the tick only re-runs the gateway exchange
// and handshake once the connection is actually gone. That happens on
// graceful gateway disconnect frames (the protocol reader closes
// the conn, so the next tick redials) — but NOT on silent network death
// (NAT timeout, read stall): the SDK never notices, Start keeps being a
// no-op, and the ear stays deaf until process restart (CH-C6-N3). Failed
// redials stay visible — every failed attempt is warned through the
// kernel log face (CH-C6-N1) and the recovery is logged — while Stop ends
// the loop.
func (p *Plugin) supervise(ctx context.Context, done chan struct{}) {
	defer close(done)
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(streamRedialDelay):
		}
		p.mu.Lock()
		stream, stopped := p.stream, p.stopped
		p.mu.Unlock()
		if stopped || stream == nil {
			return
		}
		if err := stream.Start(ctx); err != nil {
			failures++
			if p.logger != nil {
				p.logger.Warn("dingtalk: stream redial failed; will retry",
					"failures", failures, "err", err)
			}
			continue
		}
		if failures > 0 && p.logger != nil {
			p.logger.Info("dingtalk: stream reconnected", "failed_attempts", failures)
		}
		failures = 0
		// A redial that raced Stop must not leave an orphan socket behind.
		p.mu.Lock()
		stopped = p.stopped
		p.mu.Unlock()
		if stopped {
			stream.Close()
			return
		}
	}
}

// onChatBotMessage builds the chatbot callback handler for one Start
// lifetime: classify the callback, remember its sessionWebhook for
// replies, and publish the normalized envelope through the only
// world→kernel path. Everything the adapter cannot reply to or does not
// serve this slice is dropped locally (the Host allow-list is the policy
// layer, not this shape filter).
func (p *Plugin) onChatBotMessage(env plugin.ChannelEnv) chatbot.IChatBotMessageHandler {
	return func(ctx context.Context, data *chatbot.BotCallbackDataModel) ([]byte, error) {
		// Fence late callbacks: the SDK dispatches every frame on its own
		// goroutine with a background context, so a frame read before
		// Close can still reach this handler after Stop has returned. A
		// stopped ear must not remember webhooks or publish anything.
		p.mu.Lock()
		stopped := p.stopped
		p.mu.Unlock()
		if stopped {
			return nil, nil
		}
		if data == nil {
			return nil, nil
		}
		msg, publishable := normalizeCallback(data)
		if !publishable {
			return nil, nil
		}
		// The sessionWebhook is DingTalk's per-message reply URL: runtime
		// state keyed by conversation, latest wins. It never travels
		// further than this map (not into config, envelope, or logs).
		p.rememberWebhook(msg.ChatID, data.SessionWebhook)
		// PublishInbound is synchronous (journal + run start). A dispatch
		// failure must not kill the stream; the Host's structured logs own
		// the audit trail, so the adapter drops and continues.
		_ = env.PublishInbound(ctx, msg)
		// nil response = handled asynchronously; DingTalk needs no reply
		// payload for a robot message.
		return nil, nil
	}
}

// rememberWebhook stores the latest sessionWebhook for a conversation.
// Empty keys or URLs are ignored: an empty URL stored today would surface
// as a confusing Send failure tomorrow.
func (p *Plugin) rememberWebhook(chatID, sessionWebhook string) {
	if chatID == "" || sessionWebhook == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.webhooks[chatID] = sessionWebhook
}

// webhookFor returns the stored sessionWebhook for a conversation.
func (p *Plugin) webhookFor(chatID string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	w, ok := p.webhooks[chatID]
	return w, ok
}

// Stop implements plugin.Channel: latch stopped, cancel the supervisor,
// close the websocket, and wait (as long as the given context allows) for
// the supervisor goroutine to exit. It must return even when the context
// is already cancelled, and it is idempotent (a second Stop is a no-op).
func (p *Plugin) Stop(ctx context.Context) error {
	p.mu.Lock()
	p.stopped = true
	stream, cancel, done := p.stream, p.cancel, p.done
	p.stream, p.cancel, p.done, p.host = nil, nil, nil, nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if stream != nil {
		stream.Close()
		if waiter, ok := stream.(interface{ Wait(context.Context) error }); ok {
			_ = waiter.Wait(ctx)
		}
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
		}
	}
	return nil
}

// Send implements plugin.Channel: deliver each text part as one
// sessionWebhook POST and return one id per delivered part. DingTalk's
// robot webhook reply carries no platform message id ({errcode,errmsg}
// only), so the conversation id stands in as the per-part id.
//
// OutboundMessage.ReplyTo and TopicID are ignored this slice. Non-text
// parts are skipped (media is a later slice); an envelope with no text
// parts sends nothing and returns no ids.
//
// Unknown ChatID fails closed with a documented error: sessionWebhooks are
// minted per inbound message, held only in this process, and do not
// survive a restart — a chat must message the bot again before Vivy can
// reply to it.
func (p *Plugin) Send(ctx context.Context, msg plugin.OutboundMessage) ([]string, error) {
	sessionWebhook, ok := p.webhookFor(msg.ChatID)
	if !ok {
		return nil, fmt.Errorf("dingtalk: no session webhook stored for chat %q; "+
			"the chat must message the bot first (session webhooks are runtime state "+
			"and do not survive a restart)", msg.ChatID)
	}
	if p.http == nil {
		return nil, errors.New("dingtalk: channel not started")
	}
	var ids []string
	for _, part := range msg.Parts {
		if part.Kind != plugin.PartText {
			continue // this slice: text out only
		}
		if part.Text == "" {
			continue
		}
		if err := replyText(ctx, p.http, sessionWebhook, part.Text); err != nil {
			return ids, fmt.Errorf("dingtalk: send message to chat %q: %w", msg.ChatID, err)
		}
		ids = append(ids, msg.ChatID)
	}
	return ids, nil
}

// replyText posts one plain-text reply to a sessionWebhook. The payload is
// the DingTalk robot webhook text contract
// ({"msgtype":"text","text":{"content":...}}) — the shape the Stream SDK's
// own chatbot.SimpleReplyText sends. The reply body must decode as the
// robot ack {"errcode":<int>,"errmsg":<string>}; a non-zero errcode is an
// error even on HTTP 200.
func replyText(ctx context.Context, httpClient *http.Client, sessionWebhook, text string) error {
	body, err := json.Marshal(map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": text},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sessionWebhook, bytes.NewReader(body))
	if err != nil {
		// A parse failure wraps the raw webhook URL (token included) in
		// the error; redact it like any transport failure.
		return redactWebhookURLError(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return redactWebhookURLError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook http status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, webhookMaxBodyBytes))
	if err != nil {
		return fmt.Errorf("read webhook reply: %w", err)
	}
	var ack struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(raw, &ack); err != nil {
		return fmt.Errorf("decode webhook reply: %w", err)
	}
	if ack.ErrCode != 0 {
		return fmt.Errorf("webhook errcode %d: %s", ack.ErrCode, ack.ErrMsg)
	}
	return nil
}

// redactWebhookURLError rebuilds a request-construction or transport error
// so the sessionWebhook URL's query (which carries a per-session token)
// never reaches an error chain (D-010), while the underlying cause (parse
// failure, dial failure, timeout) is preserved. Errors that are not
// *url.Error cannot carry the URL and pass through untouched.
func redactWebhookURLError(err error) error {
	var ue *url.Error
	if !errors.As(err, &ue) {
		return err
	}
	u, perr := url.Parse(ue.URL)
	if perr != nil {
		return fmt.Errorf("%s session webhook: %w", ue.Op, ue.Err)
	}
	u.RawQuery = ""
	u.Fragment = ""
	return &url.Error{Op: ue.Op, URL: u.String(), Err: ue.Err}
}

// normalizeCallback maps one DingTalk chatbot callback to a kernel inbound
// envelope. It accepts exactly one shape this slice — a single-chat
// (conversationType "1") TEXT message from a human other than the bot —
// and reports everything else as not publishable: group chats, cards and
// non-text payloads (their text.content is empty; media in is a later
// slice), empty messages, and envelopes the Host dispatch would drop
// anyway (missing sender or message id).
//
// The bot-self guard compares the sender against ChatbotUserId: if the
// platform ever echoes the bot's own outgoing message back through the
// callback, the envelope is dropped instead of looping into a new turn.
func normalizeCallback(data *chatbot.BotCallbackDataModel) (plugin.InboundMessage, bool) {
	if data == nil {
		return plugin.InboundMessage{}, false
	}
	if data.ConversationType != singleChatType {
		return plugin.InboundMessage{}, false
	}
	if chatbotUserID := strings.TrimSpace(data.ChatbotUserId); chatbotUserID != "" &&
		(strings.TrimSpace(data.SenderStaffId) == chatbotUserID || strings.TrimSpace(data.SenderId) == chatbotUserID) {
		return plugin.InboundMessage{}, false
	}
	content := strings.TrimSpace(data.Text.Content)
	if content == "" {
		// Some clients fill the free-form content object instead of the
		// typed text field; accept a plain string content.content like the
		// picoclaw classifier does.
		if contentMap, ok := data.Content.(map[string]any); ok {
			if s, ok := contentMap["content"].(string); ok {
				content = strings.TrimSpace(s)
			}
		}
	}
	if content == "" {
		// Pictures, audio, cards — no text part to publish.
		return plugin.InboundMessage{}, false
	}
	sender := strings.TrimSpace(data.SenderStaffId)
	if sender == "" {
		sender = strings.TrimSpace(data.SenderId)
	}
	if sender == "" {
		// No sender id: allow_from could never match this envelope.
		return plugin.InboundMessage{}, false
	}
	messageID := strings.TrimSpace(data.MsgId)
	if messageID == "" {
		// The Host dispatch drops envelopes without a message id; do not
		// publish what cannot be journaled.
		return plugin.InboundMessage{}, false
	}
	chatID := strings.TrimSpace(data.ConversationId)
	if chatID == "" {
		// Single-chat fallback: the conversation IS the sender (picoclaw's
		// choice), which keeps webhook keying and Send addressing
		// consistent.
		chatID = sender
	}
	return plugin.InboundMessage{
		Channel:   ChannelName,
		ChatID:    chatID,
		Sender:    senderPrefix + sender,
		MessageID: messageID,
		// ReplyTo/TopicID stay empty this slice (single chat, no reply
		// threading, no forum topics).
		ReplyTo: "",
		TopicID: "",
		Parts:   []plugin.Part{{Kind: plugin.PartText, Text: content}},
	}, true
}
