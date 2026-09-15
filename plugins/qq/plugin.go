// Package qq is a seam-channel adapter (VIVY-CHANNEL-PACK.md §9): the QQ
// OFFICIAL open-platform bot, single-chat (C2C) text in / passive text
// reply out, over the official websocket gateway.
//
// THIS IS NOT A PERSONAL-ACCOUNT BOT. The adapter speaks ONLY the official
// QQ open platform (https://q.qq.com) websocket gateway and official v2
// reply API with an app id + app secret issued in the q.qq.com console.
// Personal QQ accounts, OneBot, NapCat, go-cqhttp, reverse-websocket
// servers and any second helper process are out of scope by design: the
// adapter never opens a listen socket (manifest transport "poll", grant
// channel.poll — the WS long connection counts as transport poll,
// contract §14.3) and never drives a personal client protocol.
//
// First-cut scope (what this adapter deliberately does NOT do): no group
// chats (see below), no guild/频道 events, no rich media, no markdown or
// ark cards, no voice. C2C plain TEXT only.
//
// Why no GROUP_AT_MESSAGE_CREATE: the group @-mention event carries the
// group address under the official field `group_openid`, but the pinned
// botgo v0.2.1 payload struct (dto.Message) decodes a `group_id` field
// that real v2 group payloads never send — the group address would be
// silently empty (the picoclaw reference has the same defect). Handling
// groups "cleanly" would mean re-parsing raw websocket frames behind the
// SDK's back, so this cut documents groups as out of scope instead.
//
// Transport: an OUTBOUND websocket to the official event gateway, with
// the plugin's own supervised redial loop (same lifecycle shape as
// plugins/dingtalk and plugins/feishu). botgo's built-in session manager
// is NOT used: its local manager blocks forever in an internal reconnect
// loop that is not context-aware and has no stop — it would keep the ear
// alive (and redialing) after Stop. This adapter drives the documented
// websocket protocol through a Host-governed client, one fresh client per supervised
// attempt, so a stopped ear never resurrects and no goroutine leaks.
//
// Policy boundaries:
//   - allow_from is enforced by the kernel ChannelHost at dispatch; the
//     adapter never filters senders itself.
//   - secrets travel only through ChannelEnv.Secret (env_key names, never
//     values in config); the app id and app secret each get their own
//     settings-declared *_env name (CH-C6/D2).
//   - the passive-reply window (the inbound msg_id QQ requires on every
//     reply) is plugin-side runtime state keyed by chat. It never enters
//     kernel config, the config envelope, or a log line, and it does not
//     survive a restart — a Send for a chat that has not messaged the bot
//     since the process started fails closed.
package qq

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi/options"
	"github.com/tencent-connect/botgo/sessions/manager"
	"golang.org/x/oauth2"

	plugin "agent-vivy/sdk/port/channel"
)

// ChannelName is the platform name carried by every inbound envelope and
// the config envelope key this adapter consumes (channels.qq).
const ChannelName = "qq"

// senderPrefix is the allow_from entry format for a QQ sender (contract
// §11): "qq:user_<openid>". The open-platform identifies users by
// per-application openids (Author.ID on C2C events); the user_ marker
// keeps the allow_from entry self-describing. The Host matches
// InboundMessage.Sender against allow_from exactly.
const (
	senderPrefix   = ChannelName + ":"
	senderIDPrefix = "user_"
)

// wsRedialDelay is how often the supervisor offers the event gateway a
// fresh websocket after a connection died. botgo's own session manager
// reconnect loop is not used (see the package comment); this loop is the
// context-aware replacement. A var so lifecycle tests can shrink it;
// production value here. Tests never run in parallel.
var wsRedialDelay = 3 * time.Second

// Duplicate-event fence bounds (picoclaw's semantics, folded into the hot
// path — no janitor goroutine to stop). QQ redelivers the same msg_id for
// reachability (the official C2C event docs warn about it); a redelivered
// message must not start a second run. Vars so the tests can shrink the
// bounds; production values below. Tests never run in parallel.
var (
	dedupTTL        = 5 * time.Minute
	dedupMaxEntries = 10000
)

// wsClient is the websocket protocol slice this adapter drives. It exists so
// tests can stand in for the real gateway client; production builds a
// Host-governed client over one botgo dto.Session.
//
// Attempt semantics (botgo client): Connect dials the gateway URL,
// Identify (fresh session) or Resume (carried session id) authenticates,
// and Listening blocks while the connection stays healthy — it returns
// when the connection dies (close frame, read failure, protocol error) or
// Close was called. Heartbeats and the built-in opcode handling live
// inside Listening.
type wsClient interface {
	Connect() error
	Identify() error
	Resume() error
	Session() *dto.Session
	Listening() error
	Close()
}

// qqAPI is the OpenAPI slice this adapter drives. It exists so Start
// (gateway discovery) and Send (C2C and group replies) can be tested
// against a loopback stub. Production routes it through ChannelEnv.HTTP
// and shares the Host-governed oauth2.TokenSource.
type qqAPI interface {
	// WS fetches the websocket gateway URL and session limits.
	WS(ctx context.Context, params map[string]string, body string) (*dto.WebsocketAP, error)
	// PostC2CMessage posts one message to a C2C user
	// (POST /v2/users/{user_id}/messages).
	PostC2CMessage(ctx context.Context, userID string, msg dto.APIMessage, opt ...options.Option) (*dto.Message, error)
	// PostGroupMessage posts one message to a group
	// (POST /v2/groups/{group_openid}/messages).
	PostGroupMessage(ctx context.Context, groupOpenID string, msg dto.APIMessage, opt ...options.Option) (*dto.Message, error)
	// PostC2CMediaUpload uploads one rich-media file for a C2C user
	// (POST /v2/users/{user_id}/files) and returns its file_info handle.
	PostC2CMediaUpload(ctx context.Context, userID string, upload qqMediaUpload) (string, error)
	// PostGroupMediaUpload uploads for a group
	// (POST /v2/groups/{group_openid}/files).
	PostGroupMediaUpload(ctx context.Context, groupOpenID string, upload qqMediaUpload) (string, error)
	// PostC2CRichMedia posts one msg_type=7 rich-media reply carrying an
	// uploaded file handle (POST /v2/users/{user_id}/messages).
	PostC2CRichMedia(ctx context.Context, userID string, msg qqRichMediaMessage) (*dto.Message, error)
	// PostGroupRichMedia posts one to a group.
	PostGroupRichMedia(ctx context.Context, groupOpenID string, msg qqRichMediaMessage) (*dto.Message, error)
}

// groupATMessageHandler is the GROUP_AT_MESSAGE_CREATE callback shape.
// botgo v0.2.1 exports no group handler type (its own dispatcher decodes
// the defective dto.Message, whose group_id field real v2 group payloads
// never send), so the plugin owns the type and decodes the event with its
// own struct below.
type groupATMessageHandler func(payload *dto.WSPayload, data *groupATMessage) error

// groupATMessage decodes the official v2 GROUP_AT_MESSAGE_CREATE payload
// (bot.q.qq.com/wiki, event group_at_message_create): the fields verified
// against the official field table are group_openid, author.member_openid,
// content (already stripped of the @bot prefix by the platform), and id
// (the passive-reply anchor). Decoded locally on purpose: botgo v0.2.1's
// dto.Message cannot address a group.
type groupATMessage struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	GroupOpenID string `json:"group_openid"`
	Author      struct {
		MemberOpenID string `json:"member_openid"`
	} `json:"author"`
	// Attachments rides the official payload's attachment array (same
	// shape as C2C): image-class entries are downloaded, the rest survive
	// as text annotations (§12 media ruling).
	Attachments []*groupAttachment `json:"attachments"`
}

// groupAttachment is one attachment of a GROUP_AT_MESSAGE_CREATE payload,
// decoded locally for the same reason as groupATMessage itself.
type groupAttachment struct {
	URL         string `json:"url"`
	FileName    string `json:"filename"`
	ContentType string `json:"content_type"`
}

// chatState is the plugin-side per-chat runtime state: the passive-reply
// window. QQ's v2 reply API has no "send to chat" call for this bot class
// — every reply must carry the msg_id of the inbound message it answers
// (60-minute C2C passive window, 5 minutes for groups; 4 replies per
// msg_id), and replies to the same msg_id need distinct msg_seq values.
// This is runtime state only (never persisted, never logged, empty after
// restart).
type chatState struct {
	msgID string // latest inbound msg_id (the passive window)
	seq   uint32 // next msg_seq to hand out for that msg_id (starts at 1)
	group bool   // true = the window came from a group AT event (route sends to the group endpoint)
}

// Plugin is the transport implementation bound by the v1 ChannelProvider.
type Plugin struct {
	// mu guards the mutable fields below. Events arrive on SDK goroutines
	// while Send may run on Host goroutines and Stop on any other.
	mu   sync.Mutex
	host plugin.ChannelEnv
	// chats maps ChatID (the C2C user openid) to the passive-reply window.
	chats map[string]*chatState
	// seen is the duplicate-event fence: msg_id → first-seen time.
	seen map[string]time.Time
	// api is the OpenAPI client used by Start (gateway discovery) and
	// Send; non-nil once Start succeeded, nil after Stop or a failed Start.
	api qqAPI
	// markdown is the decoded settings.markdown flag (tier-1 text loop):
	// outbound parts go out as QQ native markdown, degrading to plain text
	// when the platform rejects the formatted body.
	markdown bool
	// tokenSource is the access-token source both SDK clients share.
	tokenSource oauth2.TokenSource
	// appID is the resolved open-platform app id; inbound attachment
	// downloads present it as X-Union-Appid alongside the bearer token.
	appID string
	// gatewayURL is the websocket gateway URL fetched once per Start;
	// redials reuse it exactly like botgo's own session manager does.
	gatewayURL string
	// runCtx is the lifetime context of the started ear; event handlers
	// hand it to PublishInbound so a dispatch cannot outlive Stop.
	runCtx context.Context
	// onC2C, onGroup, and onReady are the event handlers handed to the ws
	// factory; set once by Start.
	onC2C   event.C2CMessageEventHandler
	onGroup groupATMessageHandler
	onReady event.ReadyHandler
	// newWS is the websocket client factory; New pins the production
	// constructor and tests swap it. Never mutated after New.
	newWS func(onC2C event.C2CMessageEventHandler, onGroup groupATMessageHandler, onReady event.ReadyHandler,
		gatewayURL string, ts oauth2.TokenSource, resumeID string, resumeSeq uint32) wsClient
	// newAPI is the OpenAPI client factory; New pins the production
	// constructor and tests swap it. Never mutated after New.
	newAPI func(appID string, ts oauth2.TokenSource, sandbox bool) qqAPI
	// newTokenSource is the access-token source factory; New pins the
	// production constructor and tests swap it. Never mutated after New.
	newTokenSource func(context.Context, string, string) oauth2.TokenSource
	// ws is the live websocket client of the running supervisor attempt;
	// Stop closes it. Nil before Start, between redials, and after Stop.
	ws wsClient
	// currentSession is the dto.Session of the live attempt. The READY
	// handler matches the payload's session against it, so a READY from a
	// dying previous connection can never mark a new attempt live.
	currentSession *dto.Session
	// readyCh receives one signal when the live attempt's handshake is
	// accepted (READY event). Recreated per attempt.
	readyCh chan struct{}
	// resumeID/resumeSeq carry the gateway resume state captured through
	// this adapter's own handlers (session id from READY, last dispatched
	// sequence from event payloads) — never by reading the SDK client's
	// internal session after a connection died, which would race the
	// SDK's drain goroutines.
	resumeID  string
	resumeSeq uint32
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
	// logger is the kernel log face captured at Start (the optional
	// plugin.ChannelLogger face on the env, CH-C6-N1): failed redials,
	// terminal give-ups, and reconnects surface through it instead of
	// staying silent. Written once by Start before the supervisor
	// goroutine exists and never mutated afterwards, so the loop reads it
	// without the mutex; nil (env without the face) keeps the loop silent.
	logger *slog.Logger
	// health is the live gateway state for the HealthChecker face (CH-R-1):
	// nil while a session is live, a temporary HealthError while redialing,
	// dead once the gateway gave up on the bot. Guarded by mu; written only
	// by the supervisor loop, read through Health.
	health error
}

// Compile-time assertions: a seam-channel plugin IS a Channel and a
// Plugin.
func newAdapter() *Plugin {
	p := &Plugin{}
	p.newTokenSource = func(ctx context.Context, appID, appSecret string) oauth2.TokenSource {
		p.mu.Lock()
		host := p.host
		p.mu.Unlock()
		return newGovernedQQTokenSource(ctx, host, appID, appSecret)
	}
	p.newAPI = func(appID string, ts oauth2.TokenSource, sandbox bool) qqAPI {
		p.mu.Lock()
		host := p.host
		p.mu.Unlock()
		return newGovernedQQAPI(host, appID, ts, sandbox)
	}
	p.newWS = func(onC2C event.C2CMessageEventHandler, onGroup groupATMessageHandler, onReady event.ReadyHandler,
		gatewayURL string, ts oauth2.TokenSource, resumeID string, resumeSeq uint32) wsClient {
		// Subscribe to both C2C and group AT events: the two consts share
		// the GROUP_AND_C2C_EVENT intent bit, so OR-ing them delivers both
		// event kinds to the same gateway session.
		intent := dto.EventToIntent(dto.EventC2CMessageCreate) |
			dto.EventToIntent(dto.EventGroupAtMessageCreate)
		session := dto.Session{
			ID:          resumeID,
			URL:         gatewayURL,
			TokenSource: ts,
			Intent:      intent,
			LastSeq:     resumeSeq,
			Shards:      dto.ShardConfig{ShardID: 0, ShardCount: 1},
		}
		p.mu.Lock()
		host := p.host
		p.mu.Unlock()
		return newGovernedQQWebSocket(host, session, onC2C, onGroup, onReady)
	}
	// Mute the SDK's default console logger before anything dials: it
	// prints websocket frames and request bodies at INFO level, including
	// the identify payload's access token and user message content
	// (D-010; docs/architecture/LOGGING.md).
	botgo.SetLogger(quietLogger{})
	return p
}

// Start implements plugin.Channel. Fail-closed order: settings must decode
// and declare both env_key names, both credentials must resolve through
// the Host-pinned Secret, the access-token endpoint must accept the
// credential pair (first token fetch), and the official gateway must hand
// out a websocket URL before any ear is reported started. The first ws
// attempt must also go live (READY fires) before Start returns.
//
// The passed ctx stays the parent of the supervisor loop: cancelling it
// (or calling Stop) takes the ear down.
func (p *Plugin) Start(ctx context.Context, env plugin.ChannelEnv) error {
	p.mu.Lock()
	// A new Start is a new ear: a Stop that ran before this Start must
	// not deafen it. (Stop remains idempotent within an ear's lifetime.)
	p.stopped = false
	// The optional log face (CH-C6-N1): capture before the supervisor
	// goroutine starts, per the field's set-once contract.
	if lc, ok := env.(plugin.ChannelLogger); ok {
		p.logger = lc.Logger()
	}
	p.mu.Unlock()
	settings, err := DecodeSettings(env.Settings())
	if err != nil {
		return err
	}
	if settings.AppIDEnv == "" {
		return errors.New("qq: settings.app_id_env is required " +
			"(channels.qq.settings.app_id_env must name the open-platform app id variable)")
	}
	if settings.AppSecretEnv == "" {
		return errors.New("qq: settings.app_secret_env is required " +
			"(channels.qq.settings.app_secret_env must name the app secret variable)")
	}
	if settings.AppIDEnv == settings.AppSecretEnv {
		return errors.New("qq: settings.app_id_env and app_secret_env must name different variables")
	}
	appID, err := env.Secret(settings.AppIDEnv)
	if err != nil {
		return fmt.Errorf("qq: resolve app id through env %q: %w", settings.AppIDEnv, err)
	}
	appSecret, err := env.Secret(settings.AppSecretEnv)
	if err != nil {
		return fmt.Errorf("qq: resolve app secret through env %q: %w", settings.AppSecretEnv, err)
	}
	p.mu.Lock()
	p.host = env
	p.mu.Unlock()

	// The token source is built over the credential pair (values live only
	// in memory, D-010). The first Token() call hits the official token
	// endpoint; a rejected pair fails Start closed here, before any ws
	// attempt burns session-start quota.
	ts := p.newTokenSource(ctx, appID, appSecret)
	if _, err := ts.Token(); err != nil {
		return fmt.Errorf("qq: resolve access token: %w", err)
	}

	api := p.newAPI(appID, ts, settings.Sandbox)
	wsInfo, err := api.WS(ctx, nil, "")
	if err != nil {
		return fmt.Errorf("qq: fetch websocket gateway: %w", err)
	}
	gatewayURL := ""
	if wsInfo != nil {
		gatewayURL = strings.TrimSpace(wsInfo.URL)
	}
	if gatewayURL == "" {
		return errors.New("qq: websocket gateway returned an empty url")
	}

	p.mu.Lock()
	p.tokenSource = ts
	p.appID = appID
	p.api = api
	p.markdown = settings.Markdown
	p.gatewayURL = gatewayURL
	p.runCtx = ctx
	// Per-chat runtime state starts empty on every Start: a restarted
	// process has no passive-reply windows until chats message the bot
	// again (documented restart window).
	p.chats = make(map[string]*chatState)
	p.seen = make(map[string]time.Time)
	p.resumeID, p.resumeSeq = "", 0
	p.onC2C = p.c2cHandler(env)
	p.onGroup = p.groupHandler(env)
	p.onReady = p.readyHandler
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
	// gateway accepted the handshake (READY fired), the failure otherwise.
	// Waiting here keeps Start fail-closed — a deaf ear is never reported
	// as started, and a failed Start leaves neither a live websocket nor a
	// Send-able API client.
	select {
	case err := <-firstErr:
		if err != nil {
			cancel()
			p.mu.Lock()
			p.api = nil
			p.mu.Unlock()
			return fmt.Errorf("qq: connect: %w", err)
		}
	case <-ctx.Done():
		cancel()
		p.mu.Lock()
		p.api = nil
		p.mu.Unlock()
		return fmt.Errorf("qq: connect: %w", ctx.Err())
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
		return fmt.Errorf("qq: channel stopped while connecting")
	}
	return nil
}

// readyHandler is the SDK READY callback: it marks the matching attempt
// live and records the session id for gateway resume. The payload carries
// the session of the client that received the frame; matching it against
// the supervisor's current attempt keeps a late READY from a dying
// previous connection from marking a new attempt live.
func (p *Plugin) readyHandler(payload *dto.WSPayload, _ *dto.WSReadyData) {
	if payload == nil || payload.Session == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if payload.Session != p.currentSession {
		return
	}
	if id := strings.TrimSpace(payload.Session.ID); id != "" {
		p.resumeID = id
	}
	if p.readyCh != nil {
		select {
		case p.readyCh <- struct{}{}:
		default:
		}
	}
}

// c2cHandler builds the C2C_MESSAGE_CREATE handler for one Start lifetime:
// fence late callbacks, fence duplicate deliveries, classify the event,
// remember the passive-reply window, and publish the normalized envelope
// through the only world→kernel path. Everything the adapter does not
// serve this slice is dropped locally (the Host allow-list is the policy
// layer, not this shape filter).
func (p *Plugin) c2cHandler(env plugin.ChannelEnv) event.C2CMessageEventHandler {
	return func(payload *dto.WSPayload, data *dto.WSC2CMessageData) error {
		// Fence late events: the SDK dispatches every frame on its own
		// goroutine, so a frame read before Close can still reach this
		// handler after Stop has returned. A stopped ear must not remember
		// state or publish anything.
		p.mu.Lock()
		stopped, publishCtx := p.stopped, p.runCtx
		p.mu.Unlock()
		if stopped {
			return nil
		}
		// Track the gateway sequence for resume (the client's internal
		// session cannot be read after a connection died without racing
		// the SDK's drain goroutines).
		if payload != nil && payload.Seq > 0 {
			p.mu.Lock()
			p.resumeSeq = payload.Seq
			p.mu.Unlock()
		}
		msg, refs, publishable := normalizeC2C(data)
		if !publishable {
			return nil
		}
		// Image downloads are best-effort (§12): each success appends the
		// annotation plus the media part; a failure keeps the annotation
		// so the sender's image never vanishes without a trace.
		for _, ref := range refs {
			ann := plugin.Part{Kind: plugin.PartText, Text: "[image: " + ref.name + "]"}
			if media, ok := p.downloadAttachment(publishCtx, env, ref); ok {
				msg.Parts = append(msg.Parts, ann, plugin.Part{Kind: plugin.PartMedia, Media: media})
			} else {
				msg.Parts = append(msg.Parts, ann)
			}
		}
		// Duplicate fence: QQ redelivers the same msg_id for reachability.
		if !p.rememberSeen(msg.MessageID) {
			return nil
		}
		// The passive-reply window: latest inbound msg_id per chat wins,
		// and the reply sequence restarts for the new window.
		p.rememberChat(msg.ChatID, msg.MessageID, false)
		// PublishInbound is synchronous (journal + run start). A dispatch
		// failure must not kill the stream; the Host's structured logs own
		// the audit trail, so the adapter drops and continues. Returning
		// nil lets the SDK ack the frame as handled.
		_ = env.PublishInbound(publishCtx, msg)
		return nil
	}
}

// rememberSeen records a message id in the duplicate fence and reports
// whether it is new. Bounded: at the hard cap a sweep drops expired
// entries and, if the map is still full, the oldest entry is evicted.
func (p *Plugin) rememberSeen(messageID string) bool {
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	if seen, dup := p.seen[messageID]; dup {
		if now.Sub(seen) < dedupTTL {
			return false
		}
		// Expired entry: the id is new again.
		delete(p.seen, messageID)
	}
	if len(p.seen) >= dedupMaxEntries {
		p.sweepSeenLocked(now)
	}
	p.seen[messageID] = now
	return true
}

// sweepSeenLocked prunes the fence. Caller holds p.mu.
func (p *Plugin) sweepSeenLocked(now time.Time) {
	for id, seen := range p.seen {
		if now.Sub(seen) >= dedupTTL {
			delete(p.seen, id)
		}
	}
	for len(p.seen) >= dedupMaxEntries {
		var oldestID string
		var oldest time.Time
		for id, seen := range p.seen {
			if oldestID == "" || seen.Before(oldest) {
				oldestID, oldest = id, seen
			}
		}
		if oldestID == "" {
			return
		}
		delete(p.seen, oldestID)
	}
}

// rememberChat stores the passive-reply window for a chat: latest inbound
// msg_id wins, reply sequence restarts at 0 (Send hands out 1, 2, ...).
// The group flag routes later sends to the matching endpoint.
func (p *Plugin) rememberChat(chatID, msgID string, group bool) {
	if chatID == "" || msgID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.chats[chatID] = &chatState{msgID: msgID, group: group}
}

// groupHandler builds the GROUP_AT_MESSAGE_CREATE handler for one Start
// lifetime. It mirrors c2cHandler: fence late frames, fence duplicate
// deliveries, remember the group's passive-reply window, and publish the
// normalized envelope. Group AT messages are mention-only by construction
// — the event fires only when the bot is @-addressed, and the platform
// already strips the @bot prefix from the content.
func (p *Plugin) groupHandler(env plugin.ChannelEnv) groupATMessageHandler {
	return func(payload *dto.WSPayload, data *groupATMessage) error {
		// Fence late events exactly like c2cHandler: a stopped ear must
		// not remember state or publish anything.
		p.mu.Lock()
		stopped, publishCtx := p.stopped, p.runCtx
		p.mu.Unlock()
		if stopped {
			return nil
		}
		if payload != nil && payload.Seq > 0 {
			p.mu.Lock()
			p.resumeSeq = payload.Seq
			p.mu.Unlock()
		}
		msg, refs, publishable := normalizeGroup(data)
		if !publishable {
			return nil
		}
		// Image downloads are best-effort (§12): each success appends the
		// annotation plus the media part; a failure keeps the annotation
		// so the sender's image never vanishes without a trace.
		for _, ref := range refs {
			ann := plugin.Part{Kind: plugin.PartText, Text: "[image: " + ref.name + "]"}
			if media, ok := p.downloadAttachment(publishCtx, env, ref); ok {
				msg.Parts = append(msg.Parts, ann, plugin.Part{Kind: plugin.PartMedia, Media: media})
			} else {
				msg.Parts = append(msg.Parts, ann)
			}
		}
		// Duplicate fence: QQ redelivers the same msg_id for reachability.
		if !p.rememberSeen(msg.MessageID) {
			return nil
		}
		p.rememberChat(msg.ChatID, msg.MessageID, true)
		// PublishInbound is synchronous (journal + run start); a dispatch
		// failure drops and continues so the frame still acks.
		_ = env.PublishInbound(publishCtx, msg)
		return nil
	}
}

// normalizeGroup maps one GROUP_AT_MESSAGE_CREATE event to a kernel
// inbound envelope (tier-1 group ruling; the payload fields are verified
// against the official event table and recorded in the batch log).
// ChatID is the group_openid — the Send and typing addressing key;
// Sender keeps the C2C "qq:user_" format over the member's openid (a
// distinct openid namespace, same allow_from shape).
func normalizeGroup(data *groupATMessage) (plugin.InboundMessage, []*imageRef, bool) {
	if data == nil {
		return plugin.InboundMessage{}, nil, false
	}
	content := strings.TrimSpace(data.Content)
	if strings.TrimSpace(data.GroupOpenID) == "" {
		// Without the group address no reply could ever land.
		return plugin.InboundMessage{}, nil, false
	}
	member := strings.TrimSpace(data.Author.MemberOpenID)
	if member == "" {
		// No sender id: allow_from could never match this envelope.
		return plugin.InboundMessage{}, nil, false
	}
	messageID := strings.TrimSpace(data.ID)
	if messageID == "" {
		// The Host dispatch drops envelopes without a message id; do not
		// publish what cannot be journaled.
		return plugin.InboundMessage{}, nil, false
	}
	var refs []*imageRef
	var parts []plugin.Part
	if content != "" {
		parts = append(parts, plugin.Part{Kind: plugin.PartText, Text: content})
	}
	for _, att := range data.Attachments {
		if att == nil {
			continue
		}
		name := strings.TrimSpace(att.FileName)
		if imageAttachment(name, att.ContentType) {
			url := strings.TrimSpace(att.URL)
			if url == "" {
				continue
			}
			refs = append(refs, &imageRef{url: url, name: name, contentType: att.ContentType})
			continue
		}
		if name != "" {
			parts = append(parts, plugin.Part{Kind: plugin.PartText, Text: "[file: " + name + "]"})
		}
	}
	if len(parts) == 0 && len(refs) == 0 {
		return plugin.InboundMessage{}, nil, false
	}
	return plugin.InboundMessage{
		Channel:   ChannelName,
		ChatID:    strings.TrimSpace(data.GroupOpenID),
		Sender:    senderPrefix + senderIDPrefix + member,
		MessageID: messageID,
		ReplyTo:   "",
		TopicID:   "",
		Parts:     parts,
	}, refs, true
}

// supervise keeps the event gateway connected until the context is
// cancelled. botgo's own session manager is not used (its reconnect loop
// is not context-aware and has no stop); this loop is the sibling-shape
// replacement: one fresh client per attempt, the first attempt's outcome
// reported to Start through firstErr, later failures retried after the
// redial delay, and the gateway session resumed across attempts when the
// protocol allows it.
//
// Give-up rule: a close the gateway classifies as "cannot identify"
// (bot delisted or banned) ends the loop — re-identifying can never
// succeed and would only burn the platform's session-start quota. (botgo's
// own session manager recovers its internal panic there and silently
// retries forever; stopping the ear is the strictly safer supervised
// equivalent.) The ear stays "started but deaf" until the Host restarts
// the channel — the same terminal shape every sibling has.
func (p *Plugin) supervise(ctx context.Context, done chan struct{}, firstErr chan<- error) {
	defer close(done)

	// failures counts consecutive failed attempts since the last live
	// connection, purely for the CH-C6-N1 log lines below.
	failures := 0

	// reportOutcome hands the FIRST attempt's outcome to Start, exactly
	// once, and reports whether this call did the sending. Every exit path
	// calls it: Start has no other wakeup while the parent context is
	// still live, so a silent first-attempt exit (a Stop landing
	// mid-handshake, say) would hang Start forever. Later attempts never
	// report — the sent flag makes the call a no-op.
	reported := false
	report := func(err error) bool {
		if reported {
			return false
		}
		reported = true
		firstErr <- err
		return true
	}
	// stopOutcome is the outcome of an exit that lost a race with Stop (or
	// the run context) before the gateway answered anything.
	stopOutcome := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return errors.New("channel stopped while connecting")
	}
	// healthWhileRedialing and healthDead record the classified live state
	// (CH-R-1) alongside the log lines: redialing attempts are temporary —
	// the loop keeps retrying — while a give-up close is dead and needs an
	// operator (or a Host restart) to revive the ear.
	healthWhileRedialing := func(stage string, err error) {
		p.mu.Lock()
		p.health = &plugin.HealthError{Class: plugin.ClassTemporary, Err: fmt.Errorf("%s: %w", stage, err)}
		p.mu.Unlock()
	}
	healthDead := func(err error) {
		p.mu.Lock()
		p.health = &plugin.HealthError{Class: plugin.ClassDead, Err: fmt.Errorf("gateway gave up on the bot: %w", err)}
		p.mu.Unlock()
	}
	// handleDeath closes a dead (connected) attempt, classifies the
	// gateway close for the next attempt, and reports whether the
	// supervisor may redial. Shared by every death path — the same close
	// must give up on a banned bot whether it arrives during the READY
	// wait or mid-session.
	handleDeath := func(ws wsClient, err error) bool {
		ws.Close()
		p.retire(ws)
		if err == nil {
			return true
		}
		if manager.CanNotIdentify(err) {
			return false
		}
		p.mu.Lock()
		resumeID, resumeSeq := p.resumeID, p.resumeSeq
		if manager.CanNotResume(err) {
			resumeID, resumeSeq = "", 0
		}
		p.resumeID, p.resumeSeq = resumeID, resumeSeq
		p.mu.Unlock()
		return true
	}

	for {
		if !p.shouldContinue(ctx) {
			report(stopOutcome())
			return
		}

		ws, resuming := p.buildClient()

		// Dial. The gateway handshake is bounded by the SDK client's
		// dialer timeout (it is not context-aware — a Stop during a
		// hanging dial waits it out, then the loop exits at the next
		// checkpoint); a refused dial is a failed attempt, not a crash.
		// A failed dial must NOT be closed: the SDK client's Close
		// dereferences a conn that only a successful Connect installs.
		if err := ws.Connect(); err != nil {
			if report(fmt.Errorf("dial gateway: %w", err)) {
				return
			}
			failures++
			healthWhileRedialing("dial gateway", err)
			p.logRedialFailure("dial gateway", err, failures)
			if !p.pause(ctx) {
				return
			}
			continue
		}

		// The dial succeeded: the client may now be closed safely. From
		// here on the supervisor owns every Close for this client.
		sess := ws.Session()
		readyCh := make(chan struct{}, 1)
		p.mu.Lock()
		p.ws = ws
		p.currentSession = sess
		p.readyCh = readyCh
		p.mu.Unlock()

		// A carried session id means resume (events continue where the
		// dead connection left off); otherwise this is a fresh identify.
		var authErr error
		if resuming {
			authErr = ws.Resume()
		} else {
			authErr = ws.Identify()
		}
		if authErr != nil {
			p.closeAttempt(ws)
			if report(fmt.Errorf("authenticate: %w", authErr)) {
				return
			}
			failures++
			healthWhileRedialing("authenticate", authErr)
			p.logRedialFailure("authenticate", authErr, failures)
			if !p.pause(ctx) {
				return
			}
			continue
		}
		listenErr := make(chan error, 1)
		go func() { listenErr <- ws.Listening() }()

		// A fresh identify must be accepted before the attempt counts as
		// live: the gateway answers bad credentials (and unauthorized
		// intents) with a close frame, which Listening reports. A resumed
		// connection gets RESUMED, not READY, so it skips this wait — the
		// session was already accepted once.
		if !resuming {
			select {
			case err := <-listenErr:
				// The attempt died mid-handshake (or before the gateway
				// answered). Same handling as a mid-session death: report
				// the first attempt's outcome, classify the close, decide
				// whether to redial.
				if report(fmt.Errorf("gateway rejected the session: %w", err)) {
					ws.Close()
					p.retire(ws)
					return
				}
				if !handleDeath(ws, err) {
					healthDead(err)
					p.logGiveUp(err)
					return
				}
				failures++
				healthWhileRedialing("handshake", err)
				p.logRedialFailure("handshake", err, failures)
				if !p.pause(ctx) {
					return
				}
				continue
			case <-readyCh:
			case <-ctx.Done():
				p.closeAttempt(ws)
				report(ctx.Err())
				return
			}
		}

		// A connect that raced Stop must not leave an orphan socket behind.
		if !p.shouldContinue(ctx) {
			p.closeAttempt(ws)
			report(stopOutcome())
			return
		}

		report(nil)
		if failures > 0 {
			p.mu.Lock()
			p.health = nil
			p.mu.Unlock()
			if p.logger != nil {
				p.logger.Info("qq: gateway reconnected", "failed_attempts", failures)
			}
		}
		failures = 0

		// Hold the connection until it dies or the context is cancelled.
		select {
		case err := <-listenErr:
			// Classify the death for the next attempt. Resume state was
			// captured through this adapter's own handlers (READY session
			// id, last dispatched event sequence).
			if !handleDeath(ws, err) {
				healthDead(err)
				p.logGiveUp(err)
				return
			}
			failures++
			healthWhileRedialing("session", err)
			p.logRedialFailure("session", err, failures)
		case <-ctx.Done():
			p.closeAttempt(ws)
			return
		}
		if !p.pause(ctx) {
			return
		}
	}
}

// buildClient creates the next websocket client through the factory and
// reports whether it carries a session to resume. The factory re-registers
// the event handlers (a package global, see the production factory) and
// bakes the current resume state into the session.
func (p *Plugin) buildClient() (wsClient, bool) {
	p.mu.Lock()
	onC2C, onGroup, onReady := p.onC2C, p.onGroup, p.onReady
	gatewayURL, ts := p.gatewayURL, p.tokenSource
	resumeID, resumeSeq := p.resumeID, p.resumeSeq
	p.mu.Unlock()
	ws := p.newWS(onC2C, onGroup, onReady, gatewayURL, ts, resumeID, resumeSeq)
	return ws, resumeID != ""
}

// closeAttempt closes the client and drops it (and its session/ready
// slot) when it is still the current attempt, so Stop never closes an
// already-discarded client.
func (p *Plugin) closeAttempt(ws wsClient) {
	ws.Close()
	p.retire(ws)
}

// retire drops the client from the current slots (when it is still the
// one stored there).
func (p *Plugin) retire(ws wsClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ws == ws {
		p.ws = nil
		p.currentSession = nil
		p.readyCh = nil
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

// logRedialFailure warns one failed supervised attempt through the kernel
// log face (CH-C6-N1). A nil logger (env without the face) stays silent.
func (p *Plugin) logRedialFailure(stage string, err error, failures int) {
	if p.logger == nil {
		return
	}
	p.logger.Warn("qq: gateway attempt failed; will redial",
		"stage", stage, "failures", failures, "err", err)
}

// logGiveUp errors when the supervisor gives up on the gateway for good
// (a cannot-identify close): the ear stays started but deaf until the
// channel restarts, so this line is the operator's one visible signal.
func (p *Plugin) logGiveUp(err error) {
	if p.logger == nil {
		return
	}
	p.logger.Error("qq: gateway closed the bot permanently; the ear stays deaf until the channel restarts",
		"err", err)
}

// Stop implements plugin.Channel: latch stopped, cancel the supervisor,
// clear the live client slots, and wait (as long as the given context
// allows) for the supervisor goroutine to exit — the supervisor closes the
// live websocket itself, because the SDK client's Close is only safe on a
// client whose Connect succeeded and the supervisor tracks exactly that.
// Stop must return even when the context is already cancelled (worst case
// it outlives a hanging dial bounded by the SDK dialer timeout), and it is
// idempotent (a second Stop is a no-op).
func (p *Plugin) Stop(ctx context.Context) error {
	p.mu.Lock()
	p.stopped = true
	cancel, done := p.cancel, p.done
	p.cancel, p.done = nil, nil
	p.ws, p.api, p.host = nil, nil, nil
	p.currentSession, p.readyCh, p.runCtx = nil, nil, nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
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
// no network I/O. A live session is healthy; a redialing ear is temporary;
// the terminal give-up close (bot delisted/banned) is dead — only an
// operator or a Host restart revives it.
func (p *Plugin) Health(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.health
}

// Send implements plugin.Channel: deliver each text part as one official
// v2 C2C reply (POST /v2/users/{openid}/messages, msg_type 0 text) and
// return the platform message ids produced.
//
// Every reply is a PASSIVE reply: it must carry the msg_id of the inbound
// message it answers, and replies to the same msg_id need distinct
// msg_seq values (1, 2, ... per window). The window is runtime state
// remembered from the latest inbound event per chat; an unknown ChatID
// fails closed with a documented error — a chat must message the bot
// first, and after a restart again (the window does not survive a
// restart).
//
// OutboundMessage.ReplyTo threads the reply to that inbound msg_id when
// set (tier-1 reply threading); TopicID stays unused (no forum topics).
// Non-text parts are skipped (media is a later slice); an envelope with
// no text parts sends nothing and returns no ids. A platform success
// without a message id contributes no id.
func (p *Plugin) Send(ctx context.Context, msg plugin.OutboundMessage) ([]string, error) {
	p.mu.Lock()
	api := p.api
	state := p.chats[msg.ChatID]
	markdown := p.markdown
	p.mu.Unlock()
	if api == nil {
		return nil, errors.New("qq: channel not started")
	}
	if strings.TrimSpace(msg.ChatID) == "" {
		return nil, errors.New("qq: outbound chat id is empty")
	}
	if state == nil {
		return nil, fmt.Errorf("qq: no passive reply window for chat %q; the chat must message the "+
			"bot first (passive msg ids are runtime state and do not survive a restart)", msg.ChatID)
	}
	var ids []string
	for _, part := range msg.Parts {
		if part.Kind != plugin.PartText {
			continue // this slice: text out only
		}
		if part.Text == "" {
			continue
		}
		var messageID string
		var err error
		if markdown {
			// Native markdown first (tier-1 text loop); most robots lack
			// the markdown permission, so a rejection degrades this one
			// chunk to plain text instead of dropping the reply.
			messageID, err = p.sendMarkdown(ctx, api, state, msg.ChatID, msg.ReplyTo, part.Text)
			if err != nil {
				messageID, err = p.sendText(ctx, api, state, msg.ChatID, msg.ReplyTo, part.Text)
			}
		} else {
			messageID, err = p.sendText(ctx, api, state, msg.ChatID, msg.ReplyTo, part.Text)
		}
		if err != nil {
			return ids, fmt.Errorf("qq: send message to chat %q: %w", msg.ChatID, err)
		}
		if messageID != "" {
			ids = append(ids, messageID)
		}
	}
	return ids, nil
}

// maxOutboundImageBytes re-checks the shared attachment bound on the
// outbound side (§1): the Host validates before SendMedia, and this bound
// keeps the base64 upload body sane even if a future producer bypasses
// that path. Reject, never truncate.
const maxOutboundImageBytes = 5 << 20

// SendMedia implements plugin.MediaSender (§1 outbound media) as the
// official two-step contract: each image uploads through /files (base64
// file_data, file_type 1) and then leaves as one msg_type=7 rich-media
// passive reply riding the same msg_id/msg_seq window as text. The
// reply's text has already gone out through Send — QQ rich-media bodies
// carry no caption this batch. A failure fails the whole batch; the
// Host's ledger retries it with a fresh upload (at-least-once).
func (p *Plugin) SendMedia(ctx context.Context, chatID string, parts []plugin.Part) ([]string, error) {
	p.mu.Lock()
	api := p.api
	state := p.chats[chatID]
	p.mu.Unlock()
	if api == nil {
		return nil, errors.New("qq: channel not started")
	}
	if state == nil {
		return nil, fmt.Errorf("qq: no passive reply window for chat %q; the chat must message the "+
			"bot first (passive msg ids are runtime state and do not survive a restart)", chatID)
	}
	var ids []string
	for _, part := range parts {
		if part.Kind != plugin.PartMedia || len(part.Media.Data) == 0 {
			continue
		}
		if len(part.Media.Data) > maxOutboundImageBytes {
			return ids, fmt.Errorf("qq: media %q exceeds the %d KiB outbound bound",
				part.Media.Name, maxOutboundImageBytes>>10)
		}
		if !strings.HasPrefix(strings.ToLower(part.Media.MimeType), "image/") {
			return ids, fmt.Errorf("qq: media %q mime %q is not an image; only images are ruled in",
				part.Media.Name, part.Media.MimeType)
		}
		fileInfo, err := p.uploadMedia(ctx, api, state, chatID, part.Media)
		if err != nil {
			return ids, fmt.Errorf("qq: upload media for chat %q: %w", chatID, err)
		}
		p.mu.Lock()
		passiveID := state.msgID
		p.mu.Unlock()
		messageID, err := p.sendRichMedia(ctx, api, state, chatID, passiveID, fileInfo)
		if err != nil {
			return ids, fmt.Errorf("qq: send media to chat %q: %w", chatID, err)
		}
		if messageID != "" {
			ids = append(ids, messageID)
		}
	}
	return ids, nil
}

// uploadMedia runs step one: the base64 file upload, addressed by the
// chat kind the passive window tracks.
func (p *Plugin) uploadMedia(ctx context.Context, api qqAPI, state *chatState, chatID string, m plugin.Media) (string, error) {
	upload := qqMediaUpload{
		FileType: 1, // image (the only type this generation sends)
		FileData: base64.StdEncoding.EncodeToString(m.Data),
	}
	if state.group {
		return api.PostGroupMediaUpload(ctx, chatID, upload)
	}
	return api.PostC2CMediaUpload(ctx, chatID, upload)
}

// sendRichMedia posts step two: the msg_type=7 passive reply carrying the
// uploaded handle, through the same window bookkeeping as text.
func (p *Plugin) sendRichMedia(ctx context.Context, api qqAPI, state *chatState, chatID, passiveID, fileInfo string) (string, error) {
	p.mu.Lock()
	state.seq++
	seq := state.seq
	p.mu.Unlock()
	if passiveID == "" {
		return "", errors.New("passive reply window is empty")
	}
	body := qqRichMediaMessage{MsgType: int(dto.RichMediaMsg), MsgID: passiveID, MsgSeq: seq}
	body.Media.FileInfo = fileInfo
	var (
		sent *dto.Message
		err  error
	)
	if state.group {
		sent, err = api.PostGroupRichMedia(ctx, chatID, body)
	} else {
		sent, err = api.PostC2CRichMedia(ctx, chatID, body)
	}
	if err != nil {
		return "", err
	}
	if sent == nil {
		return "", nil
	}
	return sent.ID, nil
}

// sendText posts one plain-text passive reply through the SDK's C2C
// messaging API. The body is the official v2 contract (msg_type 0 text,
// content, msg_id passive window, msg_seq per-window reply counter); the
// access token is resolved by the SDK through the shared token source.
// A non-2xx platform answer is an error even when the body looks like
// QQ's {code,message} / {ret,...} shapes — botgo maps every non-success
// status to an error carrying the raw body, and this adapter preserves
// that cause chain.
func (p *Plugin) sendText(ctx context.Context, api qqAPI, state *chatState, chatID, replyTo, text string) (string, error) {
	// Reserve the reply sequence under the lock; the I/O itself never
	// holds it. A failed send burns one seq value — harmless, the platform
	// dedups on (msg_id, msg_seq) pairs and seq gaps are allowed. The
	// anchor is the triggering message when the host threads (tier-1) and
	// the window's latest inbound otherwise; an expired anchor is a
	// platform rejection that surfaces through the ordinary delivery
	// retry, matching the passive contract.
	p.mu.Lock()
	passiveID := state.msgID
	if replyTo != "" {
		passiveID = replyTo
	}
	state.seq++
	seq := state.seq
	p.mu.Unlock()
	if passiveID == "" {
		return "", errors.New("passive reply window is empty")
	}
	sent, err := postMessage(ctx, api, state, chatID, &dto.MessageToCreate{
		Content: text,
		MsgType: dto.TextMsg,
		MsgID:   passiveID,
		MsgSeq:  seq,
	})
	if err != nil {
		return "", err
	}
	if sent == nil {
		return "", nil
	}
	return strings.TrimSpace(sent.ID), nil
}

// sendMarkdown posts one native-markdown passive reply through the same
// C2C endpoint (msg_type 2, dto.Markdown.Content). The seq discipline is
// the passive window's — the platform dedups on (msg_id, msg_seq), so a
// rejected markdown send burns a seq exactly like a failed text send.
// The plain Content field stays empty: the markdown body travels in
// dto.Markdown only.
func (p *Plugin) sendMarkdown(ctx context.Context, api qqAPI, state *chatState, chatID, replyTo, text string) (string, error) {
	p.mu.Lock()
	passiveID := state.msgID
	if replyTo != "" {
		passiveID = replyTo
	}
	state.seq++
	seq := state.seq
	p.mu.Unlock()
	if passiveID == "" {
		return "", errors.New("passive reply window is empty")
	}
	sent, err := postMessage(ctx, api, state, chatID, &dto.MessageToCreate{
		MsgType:  dto.MarkdownMsg,
		Markdown: &dto.Markdown{Content: text},
		MsgID:    passiveID,
		MsgSeq:   seq,
	})
	if err != nil {
		return "", err
	}
	if sent == nil {
		return "", nil
	}
	return strings.TrimSpace(sent.ID), nil
}

// postMessage routes one passive reply to the endpoint its window came
// from: group windows post to /v2/groups/{group_openid}/messages, C2C
// windows to /v2/users/{openid}/messages. The MessageToCreate shape is
// identical on both.
func postMessage(ctx context.Context, api qqAPI, state *chatState, chatID string, msg *dto.MessageToCreate) (*dto.Message, error) {
	if state.group {
		return api.PostGroupMessage(ctx, chatID, msg)
	}
	return api.PostC2CMessage(ctx, chatID, msg)
}

// Typing implements plugin.Typing: one InputNotify (msg_type 6, "the other
// side is typing") over the chat's own passive endpoint. QQ anchors the
// input status to
// the same passive msg_id the next reply would carry; without an open
// window (no inbound yet, or state lost to a restart) there is nothing to
// anchor to, so the ping is skipped — typing is best-effort. InputNotify
// carries no msg_seq: the platform does not dedup input states.
func (p *Plugin) Typing(ctx context.Context, chatID string) error {
	p.mu.Lock()
	api := p.api
	var st *chatState
	if state := p.chats[chatID]; state != nil {
		st = state
	}
	p.mu.Unlock()
	if api == nil {
		return errors.New("qq: channel not started")
	}
	if st == nil || st.msgID == "" {
		return nil
	}
	_, err := postMessage(ctx, api, st, chatID, &dto.MessageToCreate{
		MsgType:     dto.InputNotifyMsg,
		MsgID:       st.msgID,
		InputNotify: &dto.InputNotify{InputType: 1, InputSecond: 10},
	})
	return err
}

// normalizeC2C maps one C2C_MESSAGE_CREATE event to a kernel inbound
// envelope plus its pre-screened image attachments. It accepts a single-
// chat message carrying text, image-class attachments, or both; image
// attachments return as download refs (the handler fetches them with the
// platform's auth headers), every other attachment survives as a
// `[file: name]` text annotation. Media-only messages with neither text
// nor annotations stay unpublishable, as do envelopes the Host dispatch
// would drop anyway (missing sender or message id).
//
// botgo v0.2.1 does not model the event's message_type field, so text is
// detected by content: the official payload puts the plain text in
// `content` (message_type 0), and cards/ark payloads carry no plain text.
// The sender identity is Author.ID — the official payload's per-app user
// openid (equal to author.user_openid on C2C events).
func normalizeC2C(data *dto.WSC2CMessageData) (plugin.InboundMessage, []*imageRef, bool) {
	if data == nil {
		return plugin.InboundMessage{}, nil, false
	}
	senderID := ""
	if data.Author != nil {
		senderID = strings.TrimSpace(data.Author.ID)
	}
	if senderID == "" {
		// No sender id: allow_from could never match this envelope.
		return plugin.InboundMessage{}, nil, false
	}
	messageID := strings.TrimSpace(data.ID)
	if messageID == "" {
		// The Host dispatch drops envelopes without a message id; do not
		// publish what cannot be journaled. The message id is also the
		// passive-reply key — without it the chat is unreachable.
		return plugin.InboundMessage{}, nil, false
	}
	content := strings.TrimSpace(data.Content)
	var refs []*imageRef
	var parts []plugin.Part
	if content != "" {
		parts = append(parts, plugin.Part{Kind: plugin.PartText, Text: content})
	}
	for _, att := range data.Attachments {
		if att == nil {
			continue
		}
		name := strings.TrimSpace(att.FileName)
		if imageAttachment(name, att.ContentType) {
			url := strings.TrimSpace(att.URL)
			if url == "" {
				continue
			}
			refs = append(refs, &imageRef{url: url, name: name, contentType: att.ContentType})
			continue
		}
		if name != "" {
			parts = append(parts, plugin.Part{Kind: plugin.PartText, Text: "[file: " + name + "]"})
		}
	}
	if len(parts) == 0 && len(refs) == 0 {
		// Pictures with no usable url, files, ark cards — nothing to
		// publish.
		return plugin.InboundMessage{}, nil, false
	}
	return plugin.InboundMessage{
		Channel: ChannelName,
		// The C2C chat IS the user: the openid doubles as the reply
		// address (/v2/users/{openid}/messages).
		ChatID:    senderID,
		Sender:    senderPrefix + senderIDPrefix + senderID,
		MessageID: messageID,
		// ReplyTo/TopicID stay empty this slice (single chat, no reply
		// threading, no forum topics).
		ReplyTo: "",
		TopicID: "",
		Parts:   parts,
	}, refs, true
}

// imageRef is one pre-screened image attachment: the CDN URL to download,
// the display name, and the platform's claimed content type (provisional
// — the Host sniffs the actual bytes).
type imageRef struct {
	url         string
	name        string
	contentType string
}

// maxInboundImageBytes mirrors the Host's shared attachment bound
// (internal/attachment, §1 media ruling): 5 MiB per image. The read is
// capped one byte over so an over-limit payload is detected, rejected,
// and never truncated into the turn.
const maxInboundImageBytes = 5 << 20

// imageAttachment reports whether an attachment is image-class by content
// type first, extension second (picoclaw's table, rewritten).
func imageAttachment(filename, contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if strings.HasPrefix(ct, "image/") {
		return true
	}
	dot := strings.LastIndex(filename, ".")
	if dot < 0 {
		return false
	}
	switch strings.ToLower(filename[dot:]) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

// imageMimeClaim turns a filename into the provisional MIME claim sent to
// the Host (which sniffs the real bytes against the whitelist anyway).
func imageMimeClaim(filename string) string {
	dot := strings.LastIndex(filename, ".")
	if dot < 0 {
		return "application/octet-stream"
	}
	switch strings.ToLower(filename[dot:]) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default: // .jpg/.jpeg and anything else image-class
		return "image/jpeg"
	}
}

// envWarn logs through the Host logger when one is available; the surface
// is advisory and must never panic on a nil logger (test doubles).
func envWarn(env plugin.ChannelEnv, msg string, args ...any) {
	if logger := env.Logger(); logger != nil {
		logger.Warn(msg, args...)
	}
}

// downloadAttachment fetches one image attachment through the governed
// transport with QQ's auth headers — X-Union-Appid (the app id) and the
// bearer token from the shared token source — reading at most
// maxInboundImageBytes+1 bytes. Every failure drops only the image part;
// logs carry the attachment name and byte counts, never the URL, the
// token, or a raw transport error.
func (p *Plugin) downloadAttachment(ctx context.Context, env plugin.ChannelEnv, ref *imageRef) (plugin.Media, bool) {
	if ref == nil || ref.url == "" {
		return plugin.Media{}, false
	}
	p.mu.Lock()
	ts, appID := p.tokenSource, p.appID
	p.mu.Unlock()
	if ts == nil {
		envWarn(env, "qq: attachment download has no token source; dropping the image part", "name", ref.name)
		return plugin.Media{}, false
	}
	tok, err := ts.Token()
	if err != nil || tok == nil || tok.AccessToken == "" {
		envWarn(env, "qq: attachment download could not resolve the access token; dropping the image part", "name", ref.name)
		return plugin.Media{}, false
	}
	client := &http.Client{Transport: env.HTTP().Transport} // nil transport = http.DefaultTransport
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ref.url, nil)
	if err != nil {
		envWarn(env, "qq: attachment request is invalid; dropping the image part", "name", ref.name)
		return plugin.Media{}, false
	}
	req.Header.Set("X-Union-Appid", appID)
	if tok.TokenType != "" {
		req.Header.Set("Authorization", tok.TokenType+" "+tok.AccessToken)
	} else {
		req.Header.Set("Authorization", tok.AccessToken)
	}
	resp, err := client.Do(req)
	if err != nil {
		envWarn(env, "qq: attachment download failed; dropping the image part", "name", ref.name)
		return plugin.Media{}, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		envWarn(env, "qq: attachment download returned a non-200 status; dropping the image part",
			"name", ref.name, "status", resp.StatusCode)
		return plugin.Media{}, false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxInboundImageBytes+1))
	if err != nil {
		envWarn(env, "qq: attachment download body failed; dropping the image part", "name", ref.name)
		return plugin.Media{}, false
	}
	if len(data) == 0 || len(data) > maxInboundImageBytes {
		envWarn(env, "qq: attachment is empty or over the inbound size bound; dropping the image part",
			"name", ref.name, "bytes", len(data))
		return plugin.Media{}, false
	}
	mime := strings.TrimSpace(ref.contentType)
	if mime == "" {
		mime = imageMimeClaim(ref.name)
	}
	// The claim is provisional: the Host sniffs the actual bytes against
	// the shared whitelist before the part reaches the turn.
	return plugin.Media{Name: ref.name, MimeType: mime, Data: data}, true
}
