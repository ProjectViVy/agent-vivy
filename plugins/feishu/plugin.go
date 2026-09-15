// Package feishu is a seam-channel adapter (VIVY-CHANNEL-PACK.md §9):
// p2p (single-chat) text in / text out over the Feishu/Lark websocket long
// connection. It is the feishu instance of the shape pinned by
// plugins/telegram and carried on by plugins/dingtalk — same file layout,
// same Start/Stop/Send skeleton, same supervised-redial lifecycle.
//
// First-cut scope (what this adapter deliberately does NOT do): no
// rich-text (post) messages, no reply threading or topics, no webhook-mode
// event subscription. Since the first cut: group chats (mention-only,
// tier-1), image media, outbound cards (every text part as a schema-2.0
// markdown card with the 11310 plain-text fallback), the interaction faces
// — edit (card Patch), delete, the "Thinking…" card placeholder, and the
// ack reaction (ack_emojis pool, idempotent withdrawal) — contract §1/§12.
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	plugin "agent-vivy/sdk/port/channel"
)

// ChannelName is the platform name carried by every inbound envelope and
// the config envelope key this adapter consumes (channels.feishu).
const ChannelName = "feishu"

// senderPrefix is the allow_from entry format for a Feishu sender
// (contract §11): "feishu:<open_id>". The Host matches
// InboundMessage.Sender against allow_from exactly.
const senderPrefix = ChannelName + ":"

// p2pChatType is the Feishu chat_type for one-to-one (single) chats; group
// chats report "group" and publish only under the tier-1 mention-only
// gate.
const p2pChatType = "p2p"

// groupChatType is the Feishu chat_type for group chats.
const groupChatType = "group"

// reMentionPlaceholder matches a leftover @_user_N mention placeholder the
// mentions array did not key.
var reMentionPlaceholder = regexp.MustCompile(`@_user_\d+`)

// maxInboundImageBytes mirrors the Host's shared attachment bound
// (internal/attachment, §1 media ruling): 5 MiB per image. The read is
// capped one byte over so an over-limit payload is detected, rejected,
// and never truncated into the turn.
const maxInboundImageBytes = 5 << 20

// imageRef is one pre-screened inbound image: the message id (the
// MessageResource.Get key) and the image key both APIs address.
type imageRef struct {
	messageID string
	imageKey  string
}

// envWarn logs through the Host logger when one is available; the surface
// is advisory and must never panic on a nil logger (test doubles).
func envWarn(env plugin.ChannelEnv, msg string, args ...any) {
	if logger := env.Logger(); logger != nil {
		logger.Warn(msg, args...)
	}
}

// imageKeyFromContent parses the image message content JSON
// ({"image_key":"img_v2_..."}) minimally: the image_key field, nothing
// else. A payload that does not decode to a non-empty string yields "".
func imageKeyFromContent(raw string) string {
	if raw == "" {
		return ""
	}
	var payload struct {
		ImageKey string `json:"image_key"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.ImageKey)
}

// wsRedialDelay is how often the supervisor offers the event gateway a
// fresh websocket after a healthy connection died. The SDK's own
// auto-reconnect is disabled (its retry loop would keep redialing under a
// dead context); this loop is the context-aware replacement. A var, like
// the sibling adapters, so lifecycle tests can shrink it.
var wsRedialDelay = 3 * time.Second

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

// Plugin is the transport implementation bound by the v1 ChannelProvider.
type Plugin struct {
	// mu guards the mutable fields below. Events arrive on SDK goroutines
	// while Stop may run on any other goroutine.
	mu   sync.Mutex
	host plugin.ChannelEnv
	// creds and domain are the resolved Start inputs; set once by Start.
	creds wsCreds
	// domain is the platform base URL (feishu/lark/open_base_url).
	domain string
	// settings are the decoded Start settings; the reaction ack reads the
	// emoji pool from them. Set once by Start, never swapped afterwards.
	settings Settings
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
	// health is the live gateway state for the HealthChecker face (CH-R-1):
	// nil while a connection is live, a temporary HealthError while
	// redialing. Guarded by mu; written only by the supervisor loop, read
	// through Health.
	health error
}

// Compile-time assertions: a seam-channel plugin IS a Channel and a
// Plugin.
func newAdapter() *Plugin {
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
		p.mu.Lock()
		host := p.host
		p.mu.Unlock()
		if host != nil {
			dialer := *websocket.DefaultDialer
			dialer.Proxy = nil
			dialer.NetDialContext = func(context.Context, string, string) (net.Conn, error) { return nil, plugin.ErrDenied }
			dialer.NetDialTLSContext = host.DialTLS
			opts = append(opts, larkws.WithHttpClient(host.HTTP()), larkws.WithWebSocketDialer(&dialer))
		}
		return larkws.NewClient(creds.AppID, creds.AppSecret, opts...)
	}
	p.newAPI = func(creds wsCreds, domain string) *lark.Client {
		// The SDK manages the tenant_access_token from app id + app secret;
		// this adapter never touches tokens.
		p.mu.Lock()
		host := p.host
		p.mu.Unlock()
		opts := []lark.ClientOptionFunc{lark.WithOpenBaseUrl(domain)}
		if host != nil {
			opts = append(opts, lark.WithHttpClient(host.HTTP()))
		}
		return lark.NewClient(creds.AppID, creds.AppSecret, opts...)
	}
	return p
}

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
	p.host = env
	p.creds = wsCreds{AppID: appID, AppSecret: appSecret, EncryptKey: settings.EncryptKey}
	p.domain = domainFor(settings)
	p.settings = settings
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
		msg, refs, publishable := normalizeEvent(event)
		if !publishable {
			return nil
		}
		// Image downloads are best-effort (§12): each success appends the
		// annotation plus the media part; a failure keeps the annotation so
		// the sender's image never vanishes without a trace.
		for _, ref := range refs {
			ann := plugin.Part{Kind: plugin.PartText, Text: "[image: " + ref.imageKey + "]"}
			if media, ok := p.downloadImage(ctx, env, ref); ok {
				msg.Parts = append(msg.Parts, ann, plugin.Part{Kind: plugin.PartMedia, Media: media})
			} else {
				msg.Parts = append(msg.Parts, ann)
			}
		}
		if len(msg.Parts) == 0 {
			// Image-only message whose download failed — nothing to publish.
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
// firstErr — including an exit that lost a race with Stop mid-connect (a
// silent exit would hang Start forever); later failures stay silent — the
// next attempt retries, and Stop ends the loop.
func (p *Plugin) supervise(ctx context.Context, done chan struct{}, firstErr chan<- error) {
	defer close(done)

	// report hands the FIRST attempt's outcome to Start, exactly once, and
	// is a no-op afterwards. Every exit path calls it: Start has no other
	// wakeup while the parent context is still live, so a silent
	// first-attempt exit (a Stop landing mid-connect, say) would hang Start
	// until the caller's context ends.
	reported := false
	report := func(err error) {
		if reported {
			return
		}
		reported = true
		firstErr <- err
	}
	// stopOutcome is the outcome of an exit that lost a race with Stop (or
	// the run context) before the gateway answered anything.
	stopOutcome := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return errors.New("channel stopped while connecting")
	}

	first := true
	for {
		if !p.shouldContinue(ctx) {
			report(stopOutcome())
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
				report(err)
				return
			}
			// A later failed attempt is a redial: the classified health
			// (CH-R-1) keeps the broken link visible to the Host inspect
			// surface, exactly like the CH-C6-N1 log line would.
			p.mu.Lock()
			p.health = &plugin.HealthError{Class: plugin.ClassTemporary, Err: fmt.Errorf("gateway redial failed: %w", err)}
			p.mu.Unlock()
			if !p.pause(ctx) {
				return
			}
			continue
		case <-ready:
			p.mu.Lock()
			p.health = nil
			p.mu.Unlock()
		case <-ctx.Done():
			client.Close()
			p.retire(client)
			report(stopOutcome())
			return
		}

		// A connect that raced Stop must not leave an orphan socket behind.
		if !p.shouldContinue(ctx) {
			client.Close()
			p.retire(client)
			report(stopOutcome())
			return
		}

		if first {
			report(nil)
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
	p.cancel, p.done, p.current, p.api, p.host = nil, nil, nil, nil, nil
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

// Health implements plugin.HealthChecker (CH-R-1): read-only gateway state,
// no network I/O. A live websocket is healthy; a broken link is temporary —
// the supervise loop owns recovery, so no feishu condition is classified
// dead this generation (a failed first connect never starts the ear).
func (p *Plugin) Health(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.health
}

// Send implements plugin.Channel: deliver each text part as one
// im.v1.message Create call (receive_id_type chat_id) and return the
// platform message ids produced. Every text part goes out as a schema-2.0
// markdown interactive card (contract §1, 2026-09-15); a platform 11310
// (card content limit) falls back to a plain text message, and every other
// non-zero code keeps the failure semantics.
//
// OutboundMessage.ReplyTo and TopicID are ignored — p2p/group text has no
// outbound quote semantics (tier-1 ruling). Non-text parts are skipped; an
// envelope with no text parts sends nothing and returns no ids. A platform
// success without a message id contributes no id.
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
		messageID, err := sendCardOrText(ctx, api, msg.ChatID, part.Text)
		if err != nil {
			return ids, fmt.Errorf("feishu: send message to chat %q: %w", msg.ChatID, err)
		}
		if messageID != "" {
			ids = append(ids, messageID)
		}
	}
	return ids, nil
}

// SendMedia implements plugin.MediaSender (§1 outbound media) as the
// two-step Feishu contract: each image uploads through im.v1.image Create
// (image_type "message") and the returned key leaves as one im.v1.message
// Create with msg_type image. The reply's text has already gone out
// through Send — Feishu image messages carry no caption this batch. A
// failure fails the whole batch; the Host's ledger retries it with a
// fresh upload (at-least-once).
func (p *Plugin) SendMedia(ctx context.Context, chatID string, parts []plugin.Part) ([]string, error) {
	p.mu.Lock()
	api := p.api
	p.mu.Unlock()
	if api == nil {
		return nil, errors.New("feishu: channel not started")
	}
	if strings.TrimSpace(chatID) == "" {
		return nil, errors.New("feishu: outbound chat id is empty")
	}
	var ids []string
	for _, part := range parts {
		if part.Kind != plugin.PartMedia || len(part.Media.Data) == 0 {
			continue
		}
		key, err := uploadImage(ctx, api, part.Media)
		if err != nil {
			return ids, fmt.Errorf("feishu: upload image %q: %w", part.Media.Name, err)
		}
		messageID, err := sendImage(ctx, api, chatID, key)
		if err != nil {
			return ids, fmt.Errorf("feishu: send image to chat %q: %w", chatID, err)
		}
		if messageID != "" {
			ids = append(ids, messageID)
		}
	}
	return ids, nil
}

// uploadImage runs step one: the image upload, typed "message" per the
// im contract. A non-zero platform code is an error even on HTTP 200.
func uploadImage(ctx context.Context, api *lark.Client, m plugin.Media) (string, error) {
	req := larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType("message").
			Image(bytes.NewReader(m.Data)).
			Build()).
		Build()
	resp, err := api.Im.V1.Image.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("api error (code=%d msg=%s)", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.ImageKey == nil || *resp.Data.ImageKey == "" {
		return "", errors.New("upload succeeded but no image key came back")
	}
	return *resp.Data.ImageKey, nil
}

// sendImage posts step two: the image message carrying the uploaded key.
func sendImage(ctx context.Context, api *lark.Client, chatID, imageKey string) (string, error) {
	content, err := json.Marshal(map[string]string{"image_key": imageKey})
	if err != nil {
		return "", err
	}
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.CreateMessageV1ReceiveIDTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(larkim.MsgTypeImage).
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

// feishuAPIError is a non-zero platform code on an otherwise-successful
// HTTP call. The typed code lets callers branch on known codes (the 11310
// card-limit fallback) without parsing error strings.
type feishuAPIError struct {
	Code int
	Msg  string
}

func (e *feishuAPIError) Error() string {
	return fmt.Sprintf("api error (code=%d msg=%s)", e.Code, e.Msg)
}

// cardLimitCode is the Feishu code for rejected interactive-card content
// (schema/size limits). The reply is then redelivered as plain text so a
// formatting ceiling degrades the rendering, never the reply.
const cardLimitCode = 11310

// sendCardOrText delivers one text part as a markdown card, falling back
// to plain text exactly on the card-limit code.
func sendCardOrText(ctx context.Context, api *lark.Client, chatID, text string) (string, error) {
	messageID, err := sendCard(ctx, api, chatID, text)
	if err == nil {
		return messageID, nil
	}
	var apiErr *feishuAPIError
	if errors.As(err, &apiErr) && apiErr.Code == cardLimitCode {
		return sendText(ctx, api, chatID, text)
	}
	return "", err
}

// buildMarkdownCard renders one schema-2.0 interactive card with a single
// markdown element — the minimal shape that lets Feishu render model
// markdown natively (CommonMark per the card 2.0 spec).
func buildMarkdownCard(content string) (string, error) {
	card := map[string]any{
		"schema": "2.0",
		"body": map[string]any{
			"elements": []map[string]any{
				{"tag": "markdown", "content": content},
			},
		},
	}
	data, err := json.Marshal(card)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// sendCard posts one interactive markdown-card message through the SDK
// messaging API. A non-zero platform code comes back as *feishuAPIError
// even on HTTP 200.
func sendCard(ctx context.Context, api *lark.Client, chatID, text string) (string, error) {
	content, err := buildMarkdownCard(text)
	if err != nil {
		return "", err
	}
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.CreateMessageV1ReceiveIDTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(larkim.MsgTypeInteractive).
			Content(content).
			Build()).
		Build()
	resp, err := api.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", &feishuAPIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data != nil && resp.Data.MessageId != nil {
		return *resp.Data.MessageId, nil
	}
	return "", nil
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

// placeholderText is the fixed live-surface copy (contract §12); the
// feishu placeholder is a card so the terminal edit path stays uniform
// (Patch only updates card content).
const placeholderText = "Thinking…"

// editContentOf flattens an edit payload to one text body: an edit targets
// a single sent message, so the payload's text parts join.
func editContentOf(parts []plugin.Part) string {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Kind == plugin.PartText && part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// EditMessage implements plugin.MessageEditor: Feishu edits a message by
// Patching new content onto it, and Patch only accepts interactive-card
// content — which is why feishu outbound is card-first. The replacement
// body goes through the same card build as Send; no plain-text fallback
// exists for Patch (the message being edited is already a card).
func (p *Plugin) EditMessage(ctx context.Context, chatID, messageID string, msg plugin.OutboundMessage) error {
	p.mu.Lock()
	api := p.api
	p.mu.Unlock()
	if api == nil {
		return errors.New("feishu: channel not started")
	}
	text := editContentOf(msg.Parts)
	if text == "" {
		return errors.New("feishu: edit payload has no text")
	}
	content, err := buildMarkdownCard(text)
	if err != nil {
		return fmt.Errorf("feishu: edit card build: %w", err)
	}
	req := larkim.NewPatchMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewPatchMessageReqBodyBuilder().Content(content).Build()).
		Build()
	resp, err := api.Im.V1.Message.Patch(ctx, req)
	if err != nil {
		return fmt.Errorf("feishu: edit message %s in chat %q: %w", messageID, chatID, err)
	}
	if !resp.Success() {
		return fmt.Errorf("feishu: edit message %s: %w", messageID, &feishuAPIError{Code: resp.Code, Msg: resp.Msg})
	}
	return nil
}

// DeleteMessage implements plugin.MessageDeleter: remove one sent message.
// Deleting an already-deleted message surfaces as a platform error — the
// Host settles each live-surface message exactly once, so a repeat is a
// caller bug worth seeing.
func (p *Plugin) DeleteMessage(ctx context.Context, chatID, messageID string) error {
	p.mu.Lock()
	api := p.api
	p.mu.Unlock()
	if api == nil {
		return errors.New("feishu: channel not started")
	}
	req := larkim.NewDeleteMessageReqBuilder().MessageId(messageID).Build()
	resp, err := api.Im.V1.Message.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("feishu: delete message %s in chat %q: %w", messageID, chatID, err)
	}
	if !resp.Success() {
		return fmt.Errorf("feishu: delete message %s: %w", messageID, &feishuAPIError{Code: resp.Code, Msg: resp.Msg})
	}
	return nil
}

// Placeholder implements plugin.Placeholder: send the fixed "Thinking…"
// copy as a card and return its message id so the Host can delete it at
// the turn's terminal.
func (p *Plugin) Placeholder(ctx context.Context, chatID string) (string, error) {
	p.mu.Lock()
	api := p.api
	p.mu.Unlock()
	if api == nil {
		return "", errors.New("feishu: channel not started")
	}
	messageID, err := sendCard(ctx, api, chatID, placeholderText)
	if err != nil {
		return "", fmt.Errorf("feishu: send placeholder to chat %q: %w", chatID, err)
	}
	return messageID, nil
}

// React implements plugin.ReactionSender: add one ack emoji to a message
// and return the platform reaction id the withdrawal needs. The emoji is
// drawn randomly from the ack pool (settings.ack_emojis, default
// THUMBSUP); the requested-emoji parameter is ignored because the
// vocabulary is platform-specific — an empty pool (an explicit empty
// ack_emojis list) disables the ack with a reported error.
func (p *Plugin) React(ctx context.Context, chatID, messageID, _ string) (string, error) {
	p.mu.Lock()
	api, settings := p.api, p.settings
	p.mu.Unlock()
	if api == nil {
		return "", errors.New("feishu: channel not started")
	}
	pool := settings.ackEmojiPool()
	if len(pool) == 0 {
		return "", errors.New("feishu: reaction ack disabled (ack_emojis is empty)")
	}
	emoji := pool[rand.Intn(len(pool))]
	req := larkim.NewCreateMessageReactionReqBuilder().
		MessageId(messageID).
		Body(larkim.NewCreateMessageReactionReqBodyBuilder().
			ReactionType(larkim.NewEmojiBuilder().EmojiType(emoji).Build()).
			Build()).
		Build()
	resp, err := api.Im.V1.MessageReaction.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("feishu: react %s to message %s in chat %q: %w", emoji, messageID, chatID, err)
	}
	if !resp.Success() {
		return "", fmt.Errorf("feishu: react %s to message %s: %w", emoji, messageID, &feishuAPIError{Code: resp.Code, Msg: resp.Msg})
	}
	if resp.Data != nil && resp.Data.ReactionId != nil {
		return *resp.Data.ReactionId, nil
	}
	return "", nil
}

// RemoveReaction implements plugin.ReactionRemover: withdraw one reaction
// by the id React returned. Errors surface — the Host logs them
// best-effort, and a double withdrawal is a caller bug worth seeing.
func (p *Plugin) RemoveReaction(ctx context.Context, chatID, messageID, reactionID string) error {
	p.mu.Lock()
	api := p.api
	p.mu.Unlock()
	if api == nil {
		return errors.New("feishu: channel not started")
	}
	req := larkim.NewDeleteMessageReactionReqBuilder().
		MessageId(messageID).
		ReactionId(reactionID).
		Build()
	resp, err := api.Im.V1.MessageReaction.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("feishu: remove reaction %s from message %s in chat %q: %w", reactionID, messageID, chatID, err)
	}
	if !resp.Success() {
		return fmt.Errorf("feishu: remove reaction %s from message %s: %w", reactionID, messageID, &feishuAPIError{Code: resp.Code, Msg: resp.Msg})
	}
	return nil
}

// normalizeEvent maps one im.message.receive_v1 event to a kernel inbound
// envelope. P2p (single-chat) TEXT messages from a human sender publish as
// before. Group chats follow the tier-1 mention-only ruling: a group text
// message publishes only when a mention entry typed "bot" is present (the
// event model does not carry our own open_id, so the check is by mention
// type — a mention of ANOTHER bot would also trigger; recorded in the
// batch log), and the @_user_N placeholders are stripped from the text.
// Everything else reports as not publishable: other chat types, non-text
// message types (post, image, audio, interactive cards, ...), bot senders
// (the platform's or another bot's echo), unmentioned group messages,
// unparseable or empty text, and envelopes the Host dispatch would drop
// anyway (missing sender, message id, or chat id).
//
// Reaction events are separate event types (im.message.reaction.*), so
// they never reach this handler; the dispatcher registers
// im.message.receive_v1 only.
func normalizeEvent(event *larkim.P2MessageReceiveV1) (plugin.InboundMessage, []*imageRef, bool) {
	if event == nil || event.Event == nil {
		return plugin.InboundMessage{}, nil, false
	}
	message, sender := event.Event.Message, event.Event.Sender
	if message == nil {
		return plugin.InboundMessage{}, nil, false
	}
	chatType := strings.TrimSpace(str(message.ChatType))
	isGroup := chatType == groupChatType
	if chatType != p2pChatType && !isGroup {
		return plugin.InboundMessage{}, nil, false
	}
	if sender != nil && strings.TrimSpace(str(sender.SenderType)) == "bot" {
		// The bot's own echo (or another bot's message) must not loop back
		// in as a new turn.
		return plugin.InboundMessage{}, nil, false
	}
	var refs []*imageRef
	var content string
	switch messageType := strings.TrimSpace(str(message.MessageType)); messageType {
	case larkim.MsgTypeText:
		content = textFromContent(str(message.Content))
	case larkim.MsgTypeImage:
		// An image message: the content carries the image key; the bytes
		// are fetched by the handler through the dual-API download.
		if key := imageKeyFromContent(str(message.Content)); key != "" {
			refs = append(refs, &imageRef{
				messageID: strings.TrimSpace(str(message.MessageId)),
				imageKey:  key,
			})
		}
	default:
		// Rich text, cards, audio, video, media — not served (the §12
		// ruling scopes inbound media to images).
		return plugin.InboundMessage{}, nil, false
	}
	if content == "" && len(refs) == 0 {
		return plugin.InboundMessage{}, nil, false
	}
	if isGroup {
		if !botMentioned(message.Mentions) {
			// Mention-only: group chatter that never @-addresses a bot
			// never becomes a turn.
			return plugin.InboundMessage{}, nil, false
		}
		content = strings.TrimSpace(stripMentionPlaceholders(content, message.Mentions))
		if content == "" && len(refs) == 0 {
			// A bare "@vivy" ping has nothing left to answer.
			return plugin.InboundMessage{}, nil, false
		}
	}
	senderID := extractSenderID(sender)
	if senderID == "" {
		// No sender id: allow_from could never match this envelope.
		return plugin.InboundMessage{}, nil, false
	}
	messageID := strings.TrimSpace(str(message.MessageId))
	if messageID == "" {
		// The Host dispatch drops envelopes without a message id; do not
		// publish what cannot be journaled.
		return plugin.InboundMessage{}, nil, false
	}
	chatID := strings.TrimSpace(str(message.ChatId))
	if chatID == "" {
		// The chat id is the Send addressing key; without it Vivy could
		// not reply.
		return plugin.InboundMessage{}, nil, false
	}
	var parts []plugin.Part
	if content != "" {
		parts = append(parts, plugin.Part{Kind: plugin.PartText, Text: content})
	}
	return plugin.InboundMessage{
		Channel:   ChannelName,
		ChatID:    chatID,
		Sender:    senderPrefix + senderID,
		MessageID: messageID,
		// ReplyTo/TopicID stay empty (no reply threading this batch, no
		// forum topics).
		ReplyTo: "",
		TopicID: "",
		Parts:   parts,
	}, refs, true
}

// downloadImage fetches one inbound image through the SDK client: the
// message-resource API first (scoped to the message that carried it); on
// a transport error, a non-success envelope, or an empty body it falls
// back to the image API — picoclaw's dual-API semantics, rewritten on the
// pinned SDK. The read is bounded by maxInboundImageBytes; every failure
// drops only the image part. Logs carry keys and byte counts, never
// tokens or raw transport errors.
func (p *Plugin) downloadImage(ctx context.Context, env plugin.ChannelEnv, ref *imageRef) (plugin.Media, bool) {
	p.mu.Lock()
	api := p.api
	p.mu.Unlock()
	if api == nil || ref == nil || ref.imageKey == "" {
		return plugin.Media{}, false
	}
	read := func(file io.Reader) ([]byte, bool) {
		if file == nil {
			return nil, false
		}
		data, err := io.ReadAll(io.LimitReader(file, maxInboundImageBytes+1))
		if err != nil || len(data) == 0 || len(data) > maxInboundImageBytes {
			return nil, false
		}
		return data, true
	}
	if ref.messageID != "" {
		resp, err := api.Im.MessageResource.Get(ctx, larkim.NewGetMessageResourceReqBuilder().
			MessageId(ref.messageID).FileKey(ref.imageKey).Type("image").Build())
		switch {
		case err == nil && resp.Success():
			if data, ok := read(resp.File); ok {
				return plugin.Media{Name: ref.imageKey, MimeType: "image/jpeg", Data: data}, true
			}
			envWarn(env, "feishu: message-resource image body empty or over the bound; falling back to the image API",
				"message_id", ref.messageID, "image_key", ref.imageKey)
		case err != nil:
			envWarn(env, "feishu: message-resource image fetch failed; falling back to the image API",
				"message_id", ref.messageID, "image_key", ref.imageKey)
		default:
			envWarn(env, "feishu: message-resource image not successful; falling back to the image API",
				"message_id", ref.messageID, "image_key", ref.imageKey, "code", resp.Code)
		}
	}
	resp, err := api.Im.Image.Get(ctx, larkim.NewGetImageReqBuilder().ImageKey(ref.imageKey).Build())
	if err != nil {
		envWarn(env, "feishu: image API fetch failed; dropping the image part", "image_key", ref.imageKey)
		return plugin.Media{}, false
	}
	if !resp.Success() {
		envWarn(env, "feishu: image API not successful; dropping the image part",
			"image_key", ref.imageKey, "code", resp.Code)
		return plugin.Media{}, false
	}
	data, ok := read(resp.File)
	if !ok {
		envWarn(env, "feishu: image API body empty or over the bound; dropping the image part",
			"image_key", ref.imageKey)
		return plugin.Media{}, false
	}
	// The MIME claim is provisional: the Host sniffs the actual bytes
	// against the shared whitelist before the part reaches the turn.
	return plugin.Media{Name: ref.imageKey, MimeType: "image/jpeg", Data: data}, true
}

// botMentioned reports whether any mention entry is of type "bot" — the
// closest identity signal the event model carries (the payload names the
// mentioned party by key and open_id, but the adapter cannot learn its own
// open_id from the pinned SDK).
func botMentioned(mentions []*larkim.MentionEvent) bool {
	for _, m := range mentions {
		if m != nil && strings.TrimSpace(str(m.MentionedType)) == "bot" {
			return true
		}
	}
	return false
}

// stripMentionPlaceholders removes every @_user_N token the mentions array
// keys, plus any leftover placeholder the payload referenced, so the
// markup never reaches the model as literal text.
func stripMentionPlaceholders(content string, mentions []*larkim.MentionEvent) string {
	for _, m := range mentions {
		if m != nil && m.Key != nil && *m.Key != "" {
			content = strings.ReplaceAll(content, *m.Key, "")
		}
	}
	return reMentionPlaceholder.ReplaceAllString(content, "")
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
