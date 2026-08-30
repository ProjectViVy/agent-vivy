// Package feishu is a seam-channel adapter (VIVY-CHANNEL-PACK.md §9):
// p2p (single-chat) text in / text out over the Feishu/Lark websocket long
// connection. It is the feishu instance of the shape pinned by
// plugins/telegram and carried on by plugins/dingtalk — same file layout,
// same Start/Stop/Send skeleton, same supervised-redial lifecycle.
//
// First-cut scope (what this adapter deliberately does NOT do): no group
// chats, no rich-text (post) or interactive-card messages, no media, no
// reactions, no reply threading or topics, no webhook-mode event
// subscription. P2P TEXT only.
//
// Transport: an OUTBOUND websocket to the Feishu/Lark event gateway
// (manifest transport "poll", grant channel.poll). The adapter never opens
// a listen socket; the docs describe a webhook URL, but the long-connection
// SDK never binds one (contract §14.3).
//
// Policy boundaries:
//   - allow_from is enforced by the kernel ChannelHost at dispatch; the
//     adapter never filters senders itself.
//   - secrets travel only through ChannelEnv.Secret (env_key names, never
//     values in config); the app id and app secret each get their own
//     settings-declared *_env name (CH-C6/D2).
//   - encrypt_key is a plain settings value per contract §14.3 (the Host
//     never decodes it); it is handed to the SDK's event dispatcher, whose
//     websocket path pushes events unencrypted.
package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"agent-vivy/sdk/plugin"
)

// ChannelName is the platform name carried by every inbound envelope and
// the config envelope key this adapter consumes (channels.feishu).
const ChannelName = "feishu"

// senderPrefix is the allow_from entry format for a Feishu sender
// (contract §11): "feishu:<open_id>". The Host matches
// InboundMessage.Sender against allow_from exactly.
const senderPrefix = ChannelName + ":"

// p2pChatType is the Feishu chat_type for one-to-one (single) chats; group
// chats report "group" and are ignored this slice.
const p2pChatType = "p2p"

// wsRedialDelay is how often the supervisor offers the event gateway a
// fresh websocket after a healthy connection died. The SDK's own
// auto-reconnect is disabled (its retry loop would keep redialing under a
// dead context); this loop is the context-aware replacement.
const wsRedialDelay = 3 * time.Second

// wsCreds carries the credentials the two SDK clients need between Start
// and the client factories. AppID/AppSecret are values resolved through
// ChannelEnv.Secret; EncryptKey is a plain settings value (contract
// §14.3). Everything here lives only in memory (D-010).
type wsCreds struct {
	AppID      string
	AppSecret  string
	EncryptKey string
}

// eventFunc is the callback the event dispatcher invokes for one
// im.message.receive_v1 event. It exists so the websocket client factory
// can be swapped in tests.
type eventFunc func(context.Context, *larkim.P2MessageReceiveV1) error

// wsClient is the slice of the Feishu websocket SDK this adapter drives.
// It exists so tests can stand in for the real websocket client; the
// production factory returns the SDK's *larkws.Client with its own
// auto-reconnect disabled.
//
// Start semantics (v3 SDK, auto-reconnect off): it connects, fires the
// onReady callback once the connection is live, then blocks while the
// connection stays healthy. It returns an error when the initial connect
// fails (bootstrap, handshake, or credentials) or when a live connection
// dies; it returns after cancellation/Close once all SDK workers drained.
type wsClient interface {
	// SetOnReady registers the callback fired when the connection goes
	// live. It must be called before Start.
	SetOnReady(func())
	// Start connects and blocks as described above.
	Start(ctx context.Context) error
	// Close drops the connection, if any, and stops the client.
	Close()
}

// Plugin is the feishu channel adapter. It implements both plugin.Plugin
// (so Register() can carry it) and plugin.Channel (so the kernel
// ChannelHost can start it); the compile-time assertions below pin that.
type Plugin struct {
	// mu guards the mutable fields below. Events arrive on SDK goroutines
	// while Stop may run on any other goroutine.
	mu sync.Mutex
	// creds and domain are the resolved Start inputs; set once by Start.
	creds wsCreds
	// domain is the platform base URL (feishu/lark/open_base_url).
	domain string
	// onEvent is the env-bound event handler handed to the websocket
	// factory; set once by Start.
	onEvent eventFunc
	// newWS is the websocket client factory; New pins the production
	// constructor and tests swap it. Never mutated after New.
	newWS func(onEvent eventFunc, creds wsCreds, domain string) wsClient
	// newAPI is the OpenAPI client factory; New pins the production
	// constructor and tests swap it. Never mutated after New.
	newAPI func(creds wsCreds, domain string) *lark.Client
	// current is the live websocket client of the running supervisor
	// attempt; Stop closes it. Nil before Start, between redials, and
	// after Stop.
	current wsClient
	// api is the OpenAPI client used by Send; non-nil once Start
	// succeeded, nil after Stop.
	api *lark.Client
	// cancel stops the supervisor loop; nil until Start.
	cancel context.CancelFunc
	// done is closed when the supervisor goroutine exited; nil until
	// Start. Stop and supervise communicate only through this channel and
	// the mutex, never by re-reading the fields after setup.
	done chan struct{}
	// stopped latches once Stop ran; the supervisor checks it before every
	// redial and the event handler before every publish, so neither a
	// redial nor a late callback can resurrect a stopped ear.
	stopped bool
}

// Compile-time assertions: a seam-channel plugin IS a Channel and a
// Plugin.
var (
	_ plugin.Plugin  = (*Plugin)(nil)
	_ plugin.Channel = (*Plugin)(nil)
)

// New is the pack-generated entry point (Register calls feishu.New()).
func New() plugin.Plugin {
	p := &Plugin{}
	p.newWS = func(onEvent eventFunc, creds wsCreds, domain string) wsClient {
		// The dispatcher is built per client; its verification token stays
		// empty because the websocket path never answers a URL challenge.
		// encrypt_key is carried for the dispatcher contract; the websocket
		// push itself is plaintext.
		dispatcher := larkdispatcher.NewEventDispatcher("", creds.EncryptKey).
			OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
				return onEvent(ctx, event)
			})
		opts := []larkws.ClientOption{
			larkws.WithEventHandler(dispatcher),
			larkws.WithDomain(domain),
			// Auto-reconnect stays off: the SDK's retry loop would keep
			// redialing under a cancelled context and could outlive Stop.
			// p.supervise is the context-aware replacement, one fresh
			// client per attempt.
			larkws.WithAutoReconnect(false),
		}
		return larkws.NewClient(creds.AppID, creds.AppSecret, opts...)
	}
	p.newAPI = func(creds wsCreds, domain string) *lark.Client {
		// The SDK manages the tenant_access_token from app id + app secret;
		// this adapter never touches tokens.
		return lark.NewClient(creds.AppID, creds.AppSecret, lark.WithOpenBaseUrl(domain))
	}
	return p
}

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return ChannelName }

// Seam implements plugin.Plugin: the channel seam, never the tool table.
func (p *Plugin) Seam() plugin.Seam { return plugin.SeamChannel }

// Grants implements plugin.Plugin. channel.poll covers the outbound
// websocket long connection; secret.read covers the app id and app secret
// resolution.
func (p *Plugin) Grants() []plugin.Grant {
	return []plugin.Grant{plugin.GrantChannelPoll, plugin.GrantSecretRead}
}

// Tools implements plugin.Plugin: channel plugins carry no tools.
func (p *Plugin) Tools() []plugin.Tool { return nil }

// Start implements plugin.Channel. Fail-closed order: settings must decode
// and declare both env_key names, both credentials must resolve through
// the Host-pinned Secret, and the first websocket connection must go live
// (onReady fires) before any ear is reported started. A rejected
// credential, an unreachable gateway, or a cancelled context fails Start.
//
// The passed ctx stays the parent of the supervisor loop: cancelling it
// (or calling Stop) takes the ear down.
func (p *Plugin) Start(ctx context.Context, env plugin.ChannelEnv) error {
	p.mu.Lock()
	// A new Start is a new ear: a Stop that ran before this Start must
	// neither fence the new callbacks nor make the new supervisor exit
	// before firstErr is delivered (a stale latch would hang Start here).
	// (Stop remains idempotent within an ear's lifetime.)
	p.stopped = false
	p.mu.Unlock()
	settings, err := DecodeSettings(env.Settings())
	if err != nil {
		return err
	}
	if settings.AppIDEnv == "" {
		return errors.New("feishu: settings.app_id_env is required " +
			"(channels.feishu.settings.app_id_env must name the app id variable)")
	}
	if settings.AppSecretEnv == "" {
		return errors.New("feishu: settings.app_secret_env is required " +
			"(channels.feishu.settings.app_secret_env must name the app secret variable)")
	}
	if settings.AppIDEnv == settings.AppSecretEnv {
		return errors.New("feishu: settings.app_id_env and app_secret_env must name different variables")
	}
	appID, err := env.Secret(settings.AppIDEnv)
	if err != nil {
		return fmt.Errorf("feishu: resolve app id through env %q: %w", settings.AppIDEnv, err)
	}
	appSecret, err := env.Secret(settings.AppSecretEnv)
	if err != nil {
		return fmt.Errorf("feishu: resolve app secret through env %q: %w", settings.AppSecretEnv, err)
	}

	p.mu.Lock()
	p.creds = wsCreds{AppID: appID, AppSecret: appSecret, EncryptKey: settings.EncryptKey}
	p.domain = domainFor(settings)
	p.onEvent = p.messageHandler(env)
	creds, domain := p.creds, p.domain
	p.mu.Unlock()

	// The OpenAPI client is created up front; its first Send resolves the
	// tenant_access_token through the SDK (never in this adapter).
	api := p.newAPI(creds, domain)
	p.mu.Lock()
	p.api = api
	p.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	firstErr := make(chan error, 1)
	p.mu.Lock()
	p.cancel = cancel
	p.done = done
	p.mu.Unlock()
	go p.supervise(runCtx, done, firstErr)

	// The supervisor reports the first connection attempt: nil once the
	// connection is live, the failure otherwise. Waiting here keeps Start
	// fail-closed — a deaf ear is never reported as started, and a failed
	// Start leaves neither a live websocket nor a Send-able API client.
	select {
	case err := <-firstErr:
		if err != nil {
			cancel()
			p.mu.Lock()
			p.api = nil
			p.mu.Unlock()
			return fmt.Errorf("feishu: connect: %w", err)
		}
	case <-ctx.Done():
		cancel()
		p.mu.Lock()
		p.api = nil
		p.mu.Unlock()
		return fmt.Errorf("feishu: connect: %w", ctx.Err())
	}
	// Stop may have raced the first connect; a stopped ear is not a
	// started ear.
	p.mu.Lock()
	stopped := p.stopped
	p.mu.Unlock()
	if stopped {
		cancel()
		p.mu.Lock()
		p.api = nil
		p.mu.Unlock()
		return fmt.Errorf("feishu: channel stopped while connecting")
	}
	return nil
}

// domainFor resolves the platform base URL from the settings: an explicit
// open_base_url override wins, otherwise the is_lark switch picks between
// Feishu and international Lark.
func domainFor(s Settings) string {
	if s.OpenBaseURL != "" {
		return s.OpenBaseURL
	}
	if s.IsLark {
		return lark.LarkBaseUrl
	}
	return lark.FeishuBaseUrl
}

// messageHandler builds the event handler for one Start lifetime: fence
// late callbacks, classify the event, and publish the normalized envelope
// through the only world→kernel path. Everything the adapter does not
// serve this slice is dropped locally (the Host allow-list is the policy
// layer, not this shape filter).
func (p *Plugin) messageHandler(env plugin.ChannelEnv) eventFunc {
	return func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		// Fence late events: the SDK dispatches every frame on its own
		// goroutine, so a frame read before Close can still reach this
		// handler after Stop has returned. A stopped ear must not publish
		// anything.
		p.mu.Lock()
		stopped := p.stopped
		p.mu.Unlock()
		if stopped {
			return nil
		}
		msg, publishable := normalizeEvent(event)
		if !publishable {
			return nil
		}
		// PublishInbound is synchronous (journal + run start). A dispatch
		// failure must not kill the stream; the Host's structured logs own
		// the audit trail, so the adapter drops and continues. Returning
		// nil lets the SDK ack the frame as handled.
		_ = env.PublishInbound(ctx, msg)
		return nil
	}
}

// supervise keeps an event-gateway websocket connected until the context
// is cancelled. The SDK's auto-reconnect is disabled on purpose; this loop
// is the context-aware replacement: one fresh client per attempt (the v3
// SDK client is single-use — a stopped run is terminal), Start blocks
// while the connection is healthy, and a returned Start means the attempt
// is over. The first attempt's outcome is reported to Start through
// firstErr; later failures stay silent — the next attempt retries, and
// Stop ends the loop.
func (p *Plugin) supervise(ctx context.Context, done chan struct{}, firstErr chan<- error) {
	defer close(done)

	first := true
	for {
		if !p.shouldContinue(ctx) {
			return
		}

		client := p.buildClient()
		ready := make(chan struct{})
		client.SetOnReady(sync.OnceFunc(func() { close(ready) }))

		startErr := make(chan error, 1)
		go func() { startErr <- client.Start(ctx) }()

		// Wait for the connection to go live, the attempt to fail, or the
		// context (or a Stop) to end the run.
		select {
		case err := <-startErr:
			p.retire(client)
			if first {
				// Start returned before onReady: the initial connect
				// failed (credentials, gateway, handshake) or the run was
				// closed mid-dial. Both end the first attempt; the error
				// (even a cancellation) is what Start reports.
				firstErr <- err
				return
			}
			if !p.pause(ctx) {
				return
			}
			continue
		case <-ready:
		case <-ctx.Done():
			client.Close()
			p.retire(client)
			return
		}

		// A connect that raced Stop must not leave an orphan socket behind.
		if !p.shouldContinue(ctx) {
			client.Close()
			p.retire(client)
			return
		}

		if first {
			firstErr <- nil
			first = false
		}

		// Hold the connection until it dies or the context is cancelled.
		select {
		case <-startErr:
			// The connection died (with auto-reconnect off, Start returns
			// the failure). Close defensively and schedule a redial.
			client.Close()
			p.retire(client)
		case <-ctx.Done():
			client.Close()
			p.retire(client)
			return
		}
		if !p.pause(ctx) {
			return
		}
	}
}

// shouldContinue reports whether the supervisor may start (or keep
// running) another connection attempt.
func (p *Plugin) shouldContinue(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.stopped
}

// buildClient creates the next websocket client through the factory and
// records it as the current one so Stop can close it.
func (p *Plugin) buildClient() wsClient {
	p.mu.Lock()
	onEvent, creds, domain := p.onEvent, p.creds, p.domain
	p.mu.Unlock()
	client := p.newWS(onEvent, creds, domain)
	p.mu.Lock()
	p.current = client
	p.mu.Unlock()
	return client
}

// retire drops the client from the current slot (when it is still the
// one stored there) so Stop never closes an already-discarded client.
func (p *Plugin) retire(client wsClient) {
	p.mu.Lock()
	if p.current == client {
		p.current = nil
	}
	p.mu.Unlock()
}

// pause waits out the redial delay, reporting false when the context (or
// a Stop) ended the wait early.
func (p *Plugin) pause(ctx context.Context) bool {
	timer := time.NewTimer(wsRedialDelay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// Stop implements plugin.Channel: latch stopped, cancel the supervisor,
// close the live websocket and the OpenAPI client slot, and wait (as long
// as the given context allows) for the supervisor goroutine to exit. It
// must return even when the context is already cancelled, and it is
// idempotent (a second Stop is a no-op).
func (p *Plugin) Stop(ctx context.Context) error {
	p.mu.Lock()
	p.stopped = true
	cancel, done, current := p.cancel, p.done, p.current
	p.cancel, p.done, p.current, p.api = nil, nil, nil, nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if current != nil {
		current.Close()
	}
	// The OpenAPI client holds no long-lived sockets of its own (HTTP
	// per-request connections), so dropping the reference is enough.
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
		}
	}
	return nil
}

// Send implements plugin.Channel: deliver each text part as one
// im.v1.message Create call (receive_id_type chat_id, msg_type text) and
// return the platform message ids produced.
//
// OutboundMessage.ReplyTo and TopicID are ignored this slice — p2p text
// has no reply threading or forum topics in the first cut. Non-text parts
// are skipped (media is a later slice); an envelope with no text parts
// sends nothing and returns no ids. A platform success without a message
// id contributes no id.
//
// The SDK resolves the tenant_access_token from the app credentials on
// the first call; this adapter never handles tokens.
func (p *Plugin) Send(ctx context.Context, msg plugin.OutboundMessage) ([]string, error) {
	p.mu.Lock()
	api := p.api
	p.mu.Unlock()
	if api == nil {
		return nil, errors.New("feishu: channel not started")
	}
	if strings.TrimSpace(msg.ChatID) == "" {
		return nil, errors.New("feishu: outbound chat id is empty")
	}
	var ids []string
	for _, part := range msg.Parts {
		if part.Kind != plugin.PartText {
			continue // this slice: text out only
		}
		if part.Text == "" {
			continue
		}
		messageID, err := sendText(ctx, api, msg.ChatID, part.Text)
		if err != nil {
			return ids, fmt.Errorf("feishu: send message to chat %q: %w", msg.ChatID, err)
		}
		if messageID != "" {
			ids = append(ids, messageID)
		}
	}
	return ids, nil
}

// sendText posts one plain-text message to a chat through the SDK
// messaging API. The content payload is the Feishu text message contract
// ({"text":...}); the receive_id_type is chat_id. A non-zero platform
// code is an error even on HTTP 200.
func sendText(ctx context.Context, api *lark.Client, chatID, text string) (string, error) {
	content, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return "", err
	}
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.CreateMessageV1ReceiveIDTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(larkim.MsgTypeText).
			Content(string(content)).
			Build()).
		Build()
	resp, err := api.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("api error (code=%d msg=%s)", resp.Code, resp.Msg)
	}
	if resp.Data != nil && resp.Data.MessageId != nil {
		return *resp.Data.MessageId, nil
	}
	return "", nil
}

// normalizeEvent maps one im.message.receive_v1 event to a kernel inbound
// envelope. It accepts exactly one shape this slice — a p2p (single-chat)
// TEXT message from a human sender — and reports everything else as not
// publishable: group chats, non-text message types (post, image, audio,
// interactive cards, ...), bot senders (the platform's or another bot's
// echo), unparseable or empty text, and envelopes the Host dispatch would
// drop anyway (missing sender, message id, or chat id).
//
// Reaction events are separate event types (im.message.reaction.*), so
// they never reach this handler; the dispatcher registers
// im.message.receive_v1 only.
func normalizeEvent(event *larkim.P2MessageReceiveV1) (plugin.InboundMessage, bool) {
	if event == nil || event.Event == nil {
		return plugin.InboundMessage{}, false
	}
	message, sender := event.Event.Message, event.Event.Sender
	if message == nil {
		return plugin.InboundMessage{}, false
	}
	if strings.TrimSpace(str(message.ChatType)) != p2pChatType {
		return plugin.InboundMessage{}, false
	}
	if str(message.MessageType) != larkim.MsgTypeText {
		// Pictures, rich text, cards, audio — no text part to publish
		// (media in is a later slice).
		return plugin.InboundMessage{}, false
	}
	if sender != nil && strings.TrimSpace(str(sender.SenderType)) == "bot" {
		// The bot's own echo (or another bot's message) must not loop back
		// in as a new turn.
		return plugin.InboundMessage{}, false
	}
	content := textFromContent(str(message.Content))
	if content == "" {
		return plugin.InboundMessage{}, false
	}
	senderID := extractSenderID(sender)
	if senderID == "" {
		// No sender id: allow_from could never match this envelope.
		return plugin.InboundMessage{}, false
	}
	messageID := strings.TrimSpace(str(message.MessageId))
	if messageID == "" {
		// The Host dispatch drops envelopes without a message id; do not
		// publish what cannot be journaled.
		return plugin.InboundMessage{}, false
	}
	chatID := strings.TrimSpace(str(message.ChatId))
	if chatID == "" {
		// The p2p chat id is the Send addressing key; without it Vivy
		// could not reply.
		return plugin.InboundMessage{}, false
	}
	return plugin.InboundMessage{
		Channel:   ChannelName,
		ChatID:    chatID,
		Sender:    senderPrefix + senderID,
		MessageID: messageID,
		// ReplyTo/TopicID stay empty this slice (p2p text, no reply
		// threading, no forum topics).
		ReplyTo: "",
		TopicID: "",
		Parts:   []plugin.Part{{Kind: plugin.PartText, Text: content}},
	}, true
}

// extractSenderID picks the sender identity for the allow_from entry:
// open_id first (universally available to apps), then user_id, then
// union_id.
func extractSenderID(sender *larkim.EventSender) string {
	if sender == nil || sender.SenderId == nil {
		return ""
	}
	for _, id := range []*string{sender.SenderId.OpenId, sender.SenderId.UserId, sender.SenderId.UnionId} {
		if v := strings.TrimSpace(str(id)); v != "" {
			return v
		}
	}
	return ""
}

// textFromContent parses the text message content JSON ({"text":"..."})
// minimally: the text field, nothing else. A payload that does not decode
// to a string text field yields no content.
func textFromContent(raw string) string {
	if raw == "" {
		return ""
	}
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Text)
}

// str safely dereferences a *string field of the SDK event model.
func str(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
