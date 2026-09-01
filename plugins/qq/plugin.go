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
// alive (and redialing) after Stop. This adapter drives botgo's exported
// websocket protocol client directly, one fresh client per supervised
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
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi/options"
	"github.com/tencent-connect/botgo/sessions/manager"
	"github.com/tencent-connect/botgo/token"
	"github.com/tencent-connect/botgo/websocket"
	"golang.org/x/oauth2"

	"agent-vivy/sdk/plugin"
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

// wsClient is the slice of botgo's websocket protocol client this adapter
// drives (websocket.ClientImpl, registered by the SDK's own init). It
// exists so tests can stand in for the real gateway client; the production
// factory returns the SDK's client built over one dto.Session.
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

// qqAPI is the slice of botgo's OpenAPI client this adapter drives. It
// exists so Start (gateway discovery) and Send (C2C reply) can be tested
// against a loopback stub: botgo's base domain is a package constant, not
// a per-client option. The production factory returns the SDK's client
// (botgo.NewOpenAPI / botgo.NewSandboxOpenAPI), which manages the access
// token through the shared oauth2.TokenSource — this adapter never
// touches tokens itself.
type qqAPI interface {
	// WS fetches the websocket gateway URL and session limits.
	WS(ctx context.Context, params map[string]string, body string) (*dto.WebsocketAP, error)
	// PostC2CMessage posts one message to a C2C user
	// (POST /v2/users/{user_id}/messages).
	PostC2CMessage(ctx context.Context, userID string, msg dto.APIMessage, opt ...options.Option) (*dto.Message, error)
}

// chatState is the plugin-side per-chat runtime state: the passive-reply
// window. QQ's v2 reply API has no "send to chat" call for this bot class
// — every reply must carry the msg_id of the inbound message it answers
// (60-minute passive window, 4 replies per msg_id), and replies to the
// same msg_id need distinct msg_seq values. This is runtime state only
// (never persisted, never logged, empty after restart).
type chatState struct {
	msgID string // latest inbound msg_id (the passive window)
	seq   uint32 // next msg_seq to hand out for that msg_id (starts at 1)
}

// Plugin is the qq channel adapter. It implements both plugin.Plugin (so
// Register() can carry it) and plugin.Channel (so the kernel ChannelHost
// can start it); the compile-time assertions below pin that.
type Plugin struct {
	// mu guards the mutable fields below. Events arrive on SDK goroutines
	// while Send may run on Host goroutines and Stop on any other.
	mu sync.Mutex
	// chats maps ChatID (the C2C user openid) to the passive-reply window.
	chats map[string]*chatState
	// seen is the duplicate-event fence: msg_id → first-seen time.
	seen map[string]time.Time
	// api is the OpenAPI client used by Start (gateway discovery) and
	// Send; non-nil once Start succeeded, nil after Stop or a failed Start.
	api qqAPI
	// tokenSource is the access-token source both SDK clients share.
	tokenSource oauth2.TokenSource
	// gatewayURL is the websocket gateway URL fetched once per Start;
	// redials reuse it exactly like botgo's own session manager does.
	gatewayURL string
	// runCtx is the lifetime context of the started ear; event handlers
	// hand it to PublishInbound so a dispatch cannot outlive Stop.
	runCtx context.Context
	// onC2C and onReady are the event handlers handed to the ws factory;
	// set once by Start.
	onC2C   event.C2CMessageEventHandler
	onReady event.ReadyHandler
	// newWS is the websocket client factory; New pins the production
	// constructor and tests swap it. Never mutated after New.
	newWS func(onC2C event.C2CMessageEventHandler, onReady event.ReadyHandler,
		gatewayURL string, ts oauth2.TokenSource, resumeID string, resumeSeq uint32) wsClient
	// newAPI is the OpenAPI client factory; New pins the production
	// constructor and tests swap it. Never mutated after New.
	newAPI func(appID string, ts oauth2.TokenSource, sandbox bool) qqAPI
	// newTokenSource is the access-token source factory; New pins the
	// production constructor and tests swap it. Never mutated after New.
	newTokenSource func(appID, appSecret string) oauth2.TokenSource
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
}

// Compile-time assertions: a seam-channel plugin IS a Channel and a
// Plugin.
var (
	_ plugin.Plugin  = (*Plugin)(nil)
	_ plugin.Channel = (*Plugin)(nil)
)

// New is the pack-generated entry point (Register calls qq.New()).
func New() plugin.Plugin {
	p := &Plugin{}
	p.newTokenSource = func(appID, appSecret string) oauth2.TokenSource {
		// The SDK fetches and caches the access token lazily through this
		// source (single-flight, expiry-checked); Start makes the first
		// call eagerly so a rejected credential fails closed.
		return token.NewQQBotTokenSource(&token.QQBotCredentials{AppID: appID, AppSecret: appSecret})
	}
	p.newAPI = func(appID string, ts oauth2.TokenSource, sandbox bool) qqAPI {
		if sandbox {
			return botgo.NewSandboxOpenAPI(appID, ts)
		}
		return botgo.NewOpenAPI(appID, ts)
	}
	p.newWS = func(onC2C event.C2CMessageEventHandler, onReady event.ReadyHandler,
		gatewayURL string, ts oauth2.TokenSource, resumeID string, resumeSeq uint32) wsClient {
		// Register the event handlers on the SDK's dispatcher. The
		// registry is a package global (botgo's design): one QQ adapter
		// instance per process, and the latest Start's closures win — a
		// restarted channel re-registers its own handlers. Registering the
		// C2C handler requests the group/C2C intent bit; group frames may
		// still arrive on it and are dropped by the SDK dispatch (no
		// group handler is registered — see the package comment).
		intent := event.RegisterHandlers(onReady, onC2C)
		session := dto.Session{
			ID:          resumeID,
			URL:         gatewayURL,
			TokenSource: ts,
			Intent:      intent,
			LastSeq:     resumeSeq,
			Shards:      dto.ShardConfig{ShardID: 0, ShardCount: 1},
		}
		return websocket.ClientImpl.New(session)
	}
	// Mute the SDK's default console logger before anything dials: it
	// prints websocket frames and request bodies at INFO level, including
	// the identify payload's access token and user message content
	// (D-010; docs/architecture/LOGGING.md).
	botgo.SetLogger(quietLogger{})
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

	// The token source is built over the credential pair (values live only
	// in memory, D-010). The first Token() call hits the official token
	// endpoint; a rejected pair fails Start closed here, before any ws
	// attempt burns session-start quota.
	ts := p.newTokenSource(appID, appSecret)
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
	p.api = api
	p.gatewayURL = gatewayURL
	p.runCtx = ctx
	// Per-chat runtime state starts empty on every Start: a restarted
	// process has no passive-reply windows until chats message the bot
	// again (documented restart window).
	p.chats = make(map[string]*chatState)
	p.seen = make(map[string]time.Time)
	p.resumeID, p.resumeSeq = "", 0
	p.onC2C = p.c2cHandler(env)
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
		msg, publishable := normalizeC2C(data)
		if !publishable {
			return nil
		}
		// Duplicate fence: QQ redelivers the same msg_id for reachability.
		if !p.rememberSeen(msg.MessageID) {
			return nil
		}
		// The passive-reply window: latest inbound msg_id per chat wins,
		// and the reply sequence restarts for the new window.
		p.rememberChat(msg.ChatID, msg.MessageID)
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
func (p *Plugin) rememberChat(chatID, msgID string) {
	if chatID == "" || msgID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.chats[chatID] = &chatState{msgID: msgID}
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
					p.logGiveUp(err)
					return
				}
				failures++
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
		if failures > 0 && p.logger != nil {
			p.logger.Info("qq: gateway reconnected", "failed_attempts", failures)
		}
		failures = 0

		// Hold the connection until it dies or the context is cancelled.
		select {
		case err := <-listenErr:
			// Classify the death for the next attempt. Resume state was
			// captured through this adapter's own handlers (READY session
			// id, last dispatched event sequence).
			if !handleDeath(ws, err) {
				p.logGiveUp(err)
				return
			}
			failures++
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
	onC2C, onReady := p.onC2C, p.onReady
	gatewayURL, ts := p.gatewayURL, p.tokenSource
	resumeID, resumeSeq := p.resumeID, p.resumeSeq
	p.mu.Unlock()
	ws := p.newWS(onC2C, onReady, gatewayURL, ts, resumeID, resumeSeq)
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
	p.ws, p.api = nil, nil
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
// OutboundMessage.ReplyTo and TopicID are ignored this slice. Non-text
// parts are skipped (media is a later slice); an envelope with no text
// parts sends nothing and returns no ids. A platform success without a
// message id contributes no id.
func (p *Plugin) Send(ctx context.Context, msg plugin.OutboundMessage) ([]string, error) {
	p.mu.Lock()
	api := p.api
	state := p.chats[msg.ChatID]
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
		messageID, err := p.sendText(ctx, api, state, msg.ChatID, part.Text)
		if err != nil {
			return ids, fmt.Errorf("qq: send message to chat %q: %w", msg.ChatID, err)
		}
		if messageID != "" {
			ids = append(ids, messageID)
		}
	}
	return ids, nil
}

// sendText posts one plain-text passive reply through the SDK's C2C
// messaging API. The body is the official v2 contract (msg_type 0 text,
// content, msg_id passive window, msg_seq per-window reply counter); the
// access token is resolved by the SDK through the shared token source.
// A non-2xx platform answer is an error even when the body looks like
// QQ's {code,message} / {ret,...} shapes — botgo maps every non-success
// status to an error carrying the raw body, and this adapter preserves
// that cause chain.
func (p *Plugin) sendText(ctx context.Context, api qqAPI, state *chatState, chatID, text string) (string, error) {
	// Reserve the reply sequence under the lock; the I/O itself never
	// holds it. A failed send burns one seq value — harmless, the platform
	// dedups on (msg_id, msg_seq) pairs and seq gaps are allowed.
	p.mu.Lock()
	passiveID := state.msgID
	state.seq++
	seq := state.seq
	p.mu.Unlock()
	if passiveID == "" {
		return "", errors.New("passive reply window is empty")
	}
	sent, err := api.PostC2CMessage(ctx, chatID, &dto.MessageToCreate{
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

// normalizeC2C maps one C2C_MESSAGE_CREATE event to a kernel inbound
// envelope. It accepts exactly one shape this slice — a single-chat TEXT
// message with a sender, a message id and non-empty plain text content —
// and reports everything else as not publishable: media-only messages
// (attachments without text; media in is a later slice), empty content,
// and envelopes the Host dispatch would drop anyway (missing sender or
// message id).
//
// botgo v0.2.1 does not model the event's message_type field, so text is
// detected by content: the official payload puts the plain text in
// `content` (message_type 0), and cards/ark payloads carry no plain text.
// The sender identity is Author.ID — the official payload's per-app user
// openid (equal to author.user_openid on C2C events).
func normalizeC2C(data *dto.WSC2CMessageData) (plugin.InboundMessage, bool) {
	if data == nil {
		return plugin.InboundMessage{}, false
	}
	senderID := ""
	if data.Author != nil {
		senderID = strings.TrimSpace(data.Author.ID)
	}
	if senderID == "" {
		// No sender id: allow_from could never match this envelope.
		return plugin.InboundMessage{}, false
	}
	messageID := strings.TrimSpace(data.ID)
	if messageID == "" {
		// The Host dispatch drops envelopes without a message id; do not
		// publish what cannot be journaled. The message id is also the
		// passive-reply key — without it the chat is unreachable.
		return plugin.InboundMessage{}, false
	}
	content := strings.TrimSpace(data.Content)
	if content == "" {
		// Pictures, files, ark cards — no text part to publish.
		return plugin.InboundMessage{}, false
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
		Parts:   []plugin.Part{{Kind: plugin.PartText, Text: content}},
	}, true
}
