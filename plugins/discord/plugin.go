// Package discord is a seam-channel adapter (VIVY-CHANNEL-PACK.md §9):
// Discord gateway TEXT in / REST text out. It is the discord instance of
// the shape pinned by plugins/telegram and carried on by plugins/dingtalk,
// plugins/feishu and plugins/qq — same file layout, same Start/Stop/Send
// skeleton, same supervised-redial lifecycle.
//
// First-cut scope (what this adapter deliberately does NOT do): no voice
// (no voice.go, no pion/webrtc, no TTS — the entire voice/WebRTC surface
// is banned in plugins and the SDK verifier rejects its imports), no
// slash-command suite (interactions never reach the adapter; command-type
// messages are dropped by the shape filter), no webhook serving (the
// Gateway websocket counts as transport poll, contract §14.3), no embeds
// or media, no reactions, no typing indicators, no message edits, no
// forum topics, no thread management. Plain TEXT only: DMs and guild text
// channels, Message Content Intent required (see below).
//
// Transport: an OUTBOUND websocket to the Discord Gateway (manifest
// transport "poll", grant channel.poll) plus the plain REST send call
// (POST /channels/{id}/messages). The adapter never opens a listen
// socket.
//
// SDK: upstream github.com/bwmarrin/discordgo v0.29.0, unmodified. The
// picoclaw reference pins the same version behind a fork replace
// (yeongaori/discordgo-fork); this adapter does NOT carry that replace —
// the species talks to the upstream module only. The reference's Gateway
// text path (Message Content Intent, DM + guild text scope, 2000-char
// content budget) informed the shape; its voice.go, TTS and typing
// surfaces were explicitly not ported.
//
// Lifecycle, read from the discordgo v0.29.0 source (wsapi.go):
//
//   - Session.Open is synchronous through the FULL gateway handshake —
//     REST gateway fetch, websocket dial, HELLO, IDENTIFY, and it reads
//     the READY/RESUMED frame itself. Open returning nil means the
//     gateway accepted the session, so a rejected token or a disallowed
//     intent fails Start closed on the first attempt, like the siblings.
//   - Session.reconnect loops FOREVER (backoff capped at 600s) when
//     ShouldReconnectOnError is set, and Close does not stop it (no
//     internal flag flip in v0.29): a stopped ear would resurrect
//     itself. The adapter therefore builds one fresh session per
//     supervised attempt with ShouldReconnectOnError=false — discordgo's
//     own reconnect loop is never entered — and redials itself, exactly
//     the trade plugins/dingtalk and plugins/feishu make with their SDKs.
//   - Every connection-death path (read loop, heartbeat failure, gateway
//     op7) closes the socket and emits discordgo's synthetic DISCONNECT
//     event; with the SDK reconnect off, that event IS the death signal
//     the supervisor waits on.
//   - There is no resume across attempts: the gateway session id and
//     sequence are unexported in v0.29, so a fresh session can only
//     IDENTIFY. Events fired while the ear is between attempts are lost
//     to this process (bounded by the redial delay). Stop-safety beats
//     resume; the deviation is deliberate and documented in the README.
//
// Policy boundaries:
//
//   - allow_from is enforced by the kernel ChannelHost at dispatch; the
//     adapter never filters senders itself.
//   - secrets travel only through ChannelEnv.Secret (env_key names, never
//     values in config); the single bot token is declared by token_env
//     (C3/C4 pinning: the settings copy must match the envelope).
//   - no duplicate-event fence: Discord's gateway does not redeliver
//     dispatched events. Resume would replay events after the last
//     RECEIVED sequence, and discordgo stores that sequence on receipt
//     (before dispatch), so a frame this process already received is
//     never replayed; a process restart cannot resume at all (fresh
//     identify). Unlike QQ (plugins/qq), the platform gives no at-least-
//     once redelivery to fence.
//
// Message Content Intent (operational prerequisite): the gateway only
// delivers non-empty content for guild messages when the bot owner has
// enabled the privileged "Message Content Intent" in the developer
// portal. The adapter requests it on every identify; a portal that has
// not enabled it makes the gateway reject the session (close 4014) and
// the first Open fails closed. DM content is exempt from the intent, but
// the adapter requests it unconditionally so guild text works.
package discord

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"

	plugin "agent-vivy/sdk/port/channel"
)

// ChannelName is the platform name carried by every inbound envelope and
// the config envelope key this adapter consumes (channels.discord).
const ChannelName = "discord"

// senderPrefix is the allow_from entry format for a Discord sender
// (contract §11): "discord:<user id>". The Host matches
// InboundMessage.Sender against allow_from exactly.
const senderPrefix = ChannelName + ":"

// wsRedialDelay is how often the supervisor offers the gateway a fresh
// session after a connection died. discordgo's own reconnect loop is not
// used (see the package comment); this loop is the context-aware
// replacement. A var so lifecycle tests can shrink it; production value
// here. Tests never run in parallel.
var wsRedialDelay = 3 * time.Second

// messageHandlerFunc is the typed MESSAGE_CREATE callback handed to the
// session factory. It is an ALIAS, not a defined type, on purpose:
// discordgo's AddHandler matches handlers through a type switch on the
// exact function type, and a defined named type would never match (the
// handler would silently never fire).
type messageHandlerFunc = func(*discordgo.Session, *discordgo.MessageCreate)

// disconnectHandlerFunc is the typed synthetic DISCONNECT callback handed
// to the session factory (discordgo fires it on every connection-death
// path; see the package comment). Alias for the same reason as
// messageHandlerFunc.
type disconnectHandlerFunc = func(*discordgo.Session, *discordgo.Disconnect)

// session is the slice of discordgo this adapter drives. It exists so
// tests can stand in for the real gateway session (CH-C7c; the dingtalk
// streamClient pattern). discordgo has no per-session endpoint override
// (the gateway URL field is unexported; the package endpoint vars are
// process-global), so a loopback test dials through this seam instead of
// a URL knob — the same reason every sibling carries a client seam.
//
// Attempt semantics (discordgo v0.29.0): Open runs the full gateway
// handshake synchronously — REST gateway fetch, websocket dial, HELLO,
// IDENTIFY, and it reads the READY frame itself — and returns an error
// when any of that fails. ChannelMessageSend is a plain REST call over
// the session's HTTP client and rate limiter; it never touches the
// websocket, so it works on a session that was never Opened.
type session interface {
	Open() error
	Close() error
	ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

// Plugin is the transport implementation bound by the v1 ChannelProvider.
type Plugin struct {
	// mu guards the mutable fields below. Events arrive on discordgo
	// goroutines while Send may run on Host goroutines and Stop on any
	// other.
	mu   sync.Mutex
	host plugin.ChannelEnv
	// token is the resolved bot token (memory only, D-010); set once by
	// Start, read by the session factory on every attempt.
	token string
	// onMessage is the env-bound MESSAGE_CREATE handler handed to the
	// session factory; set once by Start.
	onMessage messageHandlerFunc
	// newSession is the session factory; New pins the production
	// constructor and tests swap it. Never mutated after New. Nil
	// handlers mean "send client only": no gateway handlers are
	// registered and the session is never Opened.
	newSession func(token string, onMessage messageHandlerFunc, onDisconnect disconnectHandlerFunc) (session, error)
	// sender is the never-opened session driving the REST send path;
	// ChannelMessageSend needs only the token, HTTP client and rate
	// limiter, so replies do not depend on the ear's websocket staying
	// up — the api/ear split every sibling uses. Set once by Start,
	// dropped by Stop (a never-opened session holds no sockets of its
	// own).
	sender session
	// current is the live ear of the running supervisor attempt; the
	// supervisor — never Stop — closes it, because a Close racing a
	// hanging Open would block on the session mutex. Nil before Start,
	// between redials, and after Stop.
	current session
	// runCtx is the lifetime context of the started ear; the event
	// handler hands it to PublishInbound so a dispatch cannot outlive
	// Stop.
	runCtx context.Context
	// cancel stops the supervisor loop; nil until Start.
	cancel context.CancelFunc
	// done is closed when the supervisor goroutine exited; nil until
	// Start. Stop and supervise communicate only through this channel
	// and the mutex, never by re-reading the fields after setup.
	done chan struct{}
	// stopped latches once Stop ran; the supervisor checks it before
	// every redial and the event handler before every publish, so
	// neither a redial nor a late callback can resurrect a stopped ear.
	stopped bool
}

// Compile-time assertions: a seam-channel plugin IS a Channel and a
// Plugin.
func newAdapter() *Plugin {
	p := &Plugin{}
	p.newSession = func(token string, onMessage messageHandlerFunc, onDisconnect disconnectHandlerFunc) (session, error) {
		// The "Bot " scheme prefix is what discordgo sends as the
		// Authorization header; the token value itself stays in memory
		// only (D-010).
		s, err := discordgo.New("Bot " + token)
		if err != nil {
			return nil, err
		}
		p.mu.Lock()
		host := p.host
		p.mu.Unlock()
		if host == nil {
			return nil, errors.New("discord: network host is not bound")
		}
		s.Client = host.HTTP()
		dialer := *websocket.DefaultDialer
		dialer.Proxy = nil
		dialer.NetDialContext = func(context.Context, string, string) (net.Conn, error) { return nil, plugin.ErrDenied }
		dialer.NetDialTLSContext = host.DialTLS
		s.Dialer = &dialer
		// discordgo's own reconnect loop is disabled on purpose: v0.29.0's
		// reconnect() redials forever with backoff and does not consult
		// Close, so a stopped ear would resurrect itself. p.supervise is
		// the context-aware replacement, one fresh session per attempt.
		s.ShouldReconnectOnError = false
		// Voice is banned in this species (no voice.go, no pion); the
		// adapter never joins a voice channel and never touches
		// Session.VoiceConnections. The flag is pinned false so even a
		// future refactor cannot ask the SDK to resurrect voice state.
		s.ShouldReconnectVoiceOnSessionError = false
		// Pin the log level: LogError is the zero value today, but the
		// no-token-dump invariant must not depend on a library default
		// (discordgo dumps the Identify packet incl. token at LogDebug).
		s.LogLevel = discordgo.LogError
		// TEXT only: guild text messages, DM messages, and the privileged
		// Message Content intent that makes non-empty content arrive for
		// guild messages at all. The portal must have the intent enabled
		// or the gateway rejects the identify (close 4014) and Open fails
		// closed — see the package comment and README.
		s.Identify.Intents = discordgo.IntentsGuildMessages |
			discordgo.IntentsDirectMessages |
			discordgo.IntentsMessageContent
		if onMessage != nil {
			s.AddHandler(onMessage)
		}
		if onDisconnect != nil {
			s.AddHandler(onDisconnect)
		}
		return s, nil
	}
	return p
}

// Start implements plugin.Channel. Fail-closed order: settings must
// decode and declare token_env, the token must resolve through the
// Host-pinned Secret, and the first gateway handshake must be accepted
// (Open reads the READY frame itself) before any ear is reported
// started.
//
// The passed ctx stays the parent of the supervisor loop: cancelling it
// (or calling Stop) takes the ear down.
func (p *Plugin) Start(ctx context.Context, env plugin.ChannelEnv) error {
	p.mu.Lock()
	// A new Start is a new ear: a Stop that ran before this Start must
	// not deafen it. (Stop remains idempotent within an ear's lifetime.)
	p.stopped = false
	p.mu.Unlock()
	settings, err := DecodeSettings(env.Settings())
	if err != nil {
		return err
	}
	if settings.TokenEnv == "" {
		return errors.New("discord: settings.token_env is required " +
			"(channels.discord.settings.token_env must name the bot token variable)")
	}
	token, err := env.Secret(settings.TokenEnv)
	if err != nil {
		return fmt.Errorf("discord: resolve bot token through env %q: %w", settings.TokenEnv, err)
	}
	p.mu.Lock()
	p.host = env
	p.mu.Unlock()

	// The send client is built up front and is never Opened: discordgo's
	// ChannelMessageSend is plain REST (token + HTTP client + rate
	// limiter, no websocket, no state dependency), so replies survive ear
	// redials and only Stop takes them away — the qq api/ear split.
	sender, err := p.newSession(token, nil, nil)
	if err != nil {
		return fmt.Errorf("discord: create send client: %w", err)
	}

	p.mu.Lock()
	p.token = token
	p.onMessage = p.messageHandler(env)
	p.sender = sender
	p.runCtx = ctx
	p.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	firstErr := make(chan error, 1)
	p.mu.Lock()
	p.cancel = cancel
	p.done = done
	p.mu.Unlock()
	go p.supervise(runCtx, done, firstErr)

	// The supervisor reports the first attempt: nil once the gateway
	// accepted the handshake, the failure otherwise. Waiting here keeps
	// Start fail-closed — a deaf ear is never reported as started, and a
	// failed Start leaves neither a live websocket nor a Send-able client.
	select {
	case err := <-firstErr:
		if err != nil {
			cancel()
			p.mu.Lock()
			p.sender = nil
			p.mu.Unlock()
			return fmt.Errorf("discord: connect: %w", err)
		}
	case <-ctx.Done():
		cancel()
		p.mu.Lock()
		p.sender = nil
		p.mu.Unlock()
		return fmt.Errorf("discord: connect: %w", ctx.Err())
	}
	// Stop may have raced the first connect; a stopped ear is not a
	// started ear.
	p.mu.Lock()
	stopped := p.stopped
	p.mu.Unlock()
	if stopped {
		cancel()
		p.mu.Lock()
		p.sender = nil
		p.mu.Unlock()
		return fmt.Errorf("discord: channel stopped while connecting")
	}
	return nil
}

// messageHandler builds the MESSAGE_CREATE handler for one Start
// lifetime: fence late callbacks, classify the event, and publish the
// normalized envelope through the only world→kernel path. Everything the
// adapter does not serve this slice is dropped locally (the Host
// allow-list is the policy layer, not this shape filter).
func (p *Plugin) messageHandler(env plugin.ChannelEnv) messageHandlerFunc {
	return func(_ *discordgo.Session, m *discordgo.MessageCreate) {
		// Fence late events: discordgo dispatches every event on its own
		// goroutine, so a frame read before the session closed can still
		// reach this handler after Stop has returned. A stopped ear must
		// not publish anything.
		p.mu.Lock()
		stopped, publishCtx := p.stopped, p.runCtx
		p.mu.Unlock()
		if stopped {
			return
		}
		msg, publishable := normalizeMessage(m)
		if !publishable {
			return
		}
		// PublishInbound is synchronous (journal + run start). A dispatch
		// failure must not kill the stream; the Host's structured logs own
		// the audit trail, so the adapter drops and continues.
		_ = env.PublishInbound(publishCtx, msg)
	}
}

// supervise keeps the gateway connected until the context is cancelled.
// discordgo's own session manager is not used (its reconnect loop is
// endless and unstoppable — see the package comment); this loop is the
// sibling-shape replacement: one fresh session per attempt, the first
// attempt's outcome reported to Start through firstErr, later failures
// retried after the redial delay.
//
// The ear waits for discordgo's synthetic DISCONNECT event: with the SDK
// reconnect off, every connection-death path (read error, heartbeat
// failure, gateway op7) closes the socket and emits DISCONNECT, so the
// event is the death signal and discordgo's goroutines own the socket
// teardown on natural death. Give-up rule: none this slice — a failed
// handshake (bad token, disallowed intent) is reported to Start on the
// first attempt and retried on later ones, exactly like the siblings;
// the ear stays "started but deaf" if the platform degrades after a
// successful start.
func (p *Plugin) supervise(ctx context.Context, done chan struct{}, firstErr chan<- error) {
	defer close(done)

	// reportOutcome hands the FIRST attempt's outcome to Start, exactly
	// once, and reports whether this call did the sending. Every exit
	// path calls it: Start has no other wakeup while the parent context
	// is still live, so a silent first-attempt exit (a Stop landing
	// mid-handshake, say) would hang Start forever. Later attempts never
	// report — the reported flag makes the call a no-op.
	reported := false
	report := func(err error) bool {
		if reported {
			return false
		}
		reported = true
		firstErr <- err
		return true
	}
	// stopOutcome is the outcome of an exit that lost a race with Stop
	// (or the run context) before the gateway answered anything.
	stopOutcome := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return errors.New("channel stopped while connecting")
	}

	for {
		if !p.shouldContinue(ctx) {
			report(stopOutcome())
			return
		}

		// The death signal of this attempt. discordgo can emit DISCONNECT
		// more than once for a dying socket (its read loop and heartbeat
		// both route through Close), so the channel is closed exactly
		// once under a Once.
		dead := make(chan struct{})
		signalDeath := sync.OnceFunc(func() { close(dead) })
		sess, err := p.buildSession(func(*discordgo.Session, *discordgo.Disconnect) {
			signalDeath()
		})
		if err != nil {
			// A factory failure is a construction error, not a gateway
			// condition; it fails the first attempt and is retried on
			// later ones.
			if report(fmt.Errorf("build session: %w", err)) {
				return
			}
			if !p.pause(ctx) {
				return
			}
			continue
		}
		p.mu.Lock()
		p.current = sess
		p.mu.Unlock()

		// Open runs the full handshake synchronously (see the package
		// comment): REST gateway fetch, websocket dial, HELLO, IDENTIFY,
		// READY. A rejected token or a disallowed Message Content intent
		// fails right here. The supervisor owns every Close for a
		// successfully dialed attempt; a failed Open cleaned its own
		// socket (no CloseWithCode ran, so no DISCONNECT fires for it —
		// the abandoned dead channel is simply dropped).
		if err := sess.Open(); err != nil {
			p.retire(sess)
			if report(fmt.Errorf("gateway handshake: %w", err)) {
				return
			}
			if !p.pause(ctx) {
				return
			}
			continue
		}

		// A connect that raced Stop must not leave an orphan socket behind.
		if !p.shouldContinue(ctx) {
			sess.Close()
			p.retire(sess)
			report(stopOutcome())
			return
		}

		report(nil)

		// Hold the attempt until the connection dies or the context (or a
		// Stop) ends the run.
		select {
		case <-dead:
			// discordgo's own read/heartbeat goroutines closed the socket
			// on their way out; the supervisor only retires the slot and
			// schedules a fresh session.
			p.retire(sess)
		case <-ctx.Done():
			// The supervisor — not Stop — closes the session: Stop must
			// return even while a hanging Open holds the session mutex.
			sess.Close()
			p.retire(sess)
			return
		}
		if !p.pause(ctx) {
			return
		}
	}
}

// buildSession creates the next gateway session through the factory. The
// factory registers the env-bound MESSAGE_CREATE handler and the
// attempt's death signal on the real session.
func (p *Plugin) buildSession(onDisconnect disconnectHandlerFunc) (session, error) {
	p.mu.Lock()
	token, onMessage := p.token, p.onMessage
	p.mu.Unlock()
	return p.newSession(token, onMessage, onDisconnect)
}

// retire drops the session from the current slot (when it is still the
// one stored there) so Stop never observes a discarded session.
func (p *Plugin) retire(sess session) {
	p.mu.Lock()
	if p.current == sess {
		p.current = nil
	}
	p.mu.Unlock()
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

// Stop implements plugin.Channel: latch stopped, cancel the supervisor,
// drop the client slots, and wait (as long as the given context allows)
// for the supervisor goroutine to exit — the supervisor closes the live
// session itself, because a Close from here could race a hanging Open
// and block on the session mutex. Stop must return even when the context
// is already cancelled, and it is idempotent (a second Stop is a no-op).
// VoiceConnections is never touched: the adapter never joins voice.
func (p *Plugin) Stop(ctx context.Context) error {
	p.mu.Lock()
	p.stopped = true
	cancel, done := p.cancel, p.done
	p.cancel, p.done = nil, nil
	p.current, p.sender, p.host = nil, nil, nil
	p.runCtx = nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	// The send client was never Opened (no sockets of its own, REST is
	// per-request), so dropping the reference is enough.
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
		}
	}
	return nil
}

// Send implements plugin.Channel: deliver each text part as one plain
// REST message (POST /channels/{id}/messages) and return the platform
// message ids it produced.
//
// The content is sent as-is: Discord renders its own markdown natively,
// and this adapter neither strips nor adds formatting (no
// markdown-forcing, no embeds). OutboundMessage.ReplyTo and TopicID are
// ignored this slice — no threading behavior. Non-text parts are skipped
// (media is a later slice); an envelope with no text parts sends nothing
// and returns no ids. A platform success without a message id
// contributes no id, and a platform error (rate limit, permissions,
// unknown channel) surfaces with its cause chain intact.
//
// Unlike the passive-reply platforms (dingtalk, qq), Discord lets a bot
// send to any channel it can see, so no runtime reply window exists:
// ChatID is the channel id (a DM channel id works as the target).
func (p *Plugin) Send(ctx context.Context, msg plugin.OutboundMessage) ([]string, error) {
	p.mu.Lock()
	sender := p.sender
	p.mu.Unlock()
	if sender == nil {
		return nil, errors.New("discord: channel not started")
	}
	if strings.TrimSpace(msg.ChatID) == "" {
		return nil, errors.New("discord: outbound chat id is empty")
	}
	var ids []string
	for _, part := range msg.Parts {
		if part.Kind != plugin.PartText {
			continue // this slice: text out only
		}
		if part.Text == "" {
			continue
		}
		sent, err := sender.ChannelMessageSend(msg.ChatID, part.Text)
		if err != nil {
			return ids, fmt.Errorf("discord: send message to chat %q: %w", msg.ChatID, err)
		}
		if sent != nil && strings.TrimSpace(sent.ID) != "" {
			ids = append(ids, strings.TrimSpace(sent.ID))
		}
	}
	return ids, nil
}

// normalizeMessage maps one MESSAGE_CREATE event to a kernel inbound
// envelope. It accepts exactly one shape this slice — a human-authored
// text message (DM or guild text channel) with a sender, a message id
// and non-empty plain text content — and reports everything else as not
// publishable: bot authors (the bot's own echo and any other bot's
// messages — the echo guard), command-type payloads (slash and
// context-menu commands arrive as message types 20/23; interaction
// events proper never reach the adapter because only MESSAGE_CREATE is
// registered), system messages (member joins, pins, boosts), media-only
// messages (attachments/embeds/stickers; media in is a later slice),
// empty content, and envelopes the Host dispatch would drop anyway
// (missing sender, message id, or chat id).
//
// Addresses: ChatID is the channel id — for a DM the DM channel id,
// which doubles as the ChannelMessageSend target; for a guild the text
// channel id. Sender is "discord:<author id>". ReplyTo captures
// referenced_message.id when Discord attaches it (type-19 replies); the
// slot is filled but carries no threading behavior this slice. TopicID
// stays empty (no forum topics).
func normalizeMessage(m *discordgo.MessageCreate) (plugin.InboundMessage, bool) {
	if m == nil || m.Message == nil {
		return plugin.InboundMessage{}, false
	}
	author := m.Author
	if author == nil {
		return plugin.InboundMessage{}, false
	}
	if author.Bot {
		// Echo guard: the gateway delivers the bot's own outgoing
		// messages (and every other bot's) as MESSAGE_CREATE; a bot
		// sender must never loop back in as a new turn.
		return plugin.InboundMessage{}, false
	}
	senderID := strings.TrimSpace(author.ID)
	if senderID == "" {
		// No sender id: allow_from could never match this envelope.
		return plugin.InboundMessage{}, false
	}
	if m.Type != discordgo.MessageTypeDefault && m.Type != discordgo.MessageTypeReply {
		// Slash commands (CHAT_INPUT_COMMAND, 20), context-menu commands
		// (23), thread starters, system pings — none carry a
		// conversational turn this slice. Interactions proper never reach
		// this handler at all (only MESSAGE_CREATE is registered).
		return plugin.InboundMessage{}, false
	}
	content := strings.TrimSpace(m.Content)
	if content == "" {
		// Attachments, embeds, stickers — no text part to publish.
		return plugin.InboundMessage{}, false
	}
	messageID := strings.TrimSpace(m.ID)
	if messageID == "" {
		// The Host dispatch drops envelopes without a message id; do not
		// publish what cannot be journaled.
		return plugin.InboundMessage{}, false
	}
	chatID := strings.TrimSpace(m.ChannelID)
	if chatID == "" {
		// The channel id is the Send addressing key; without it Vivy
		// could not reply.
		return plugin.InboundMessage{}, false
	}
	msg := plugin.InboundMessage{
		Channel:   ChannelName,
		ChatID:    chatID,
		Sender:    senderPrefix + senderID,
		MessageID: messageID,
		// TopicID stays empty this slice (no forum topics).
		TopicID: "",
		Parts:   []plugin.Part{{Kind: plugin.PartText, Text: content}},
	}
	if m.ReferencedMessage != nil && strings.TrimSpace(m.ReferencedMessage.ID) != "" {
		// Fill the reply slot; no threading behavior this slice.
		msg.ReplyTo = strings.TrimSpace(m.ReferencedMessage.ID)
	}
	return msg, true
}
