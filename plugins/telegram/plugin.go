// Package telegram is the first real seam-channel adapter (VIVY-CHANNEL-PACK.md
// §14.3): private-chat text and inbound photos in, plain text out, over
// outbound long polling, plus the typing live surface. It is the shape
// template for the feishu/qq/discord/dingtalk adapters — keep the file
// layout and the Start/Stop/Send structure when copying it.
//
// First-cut scope (what this adapter deliberately does NOT do): no webhook,
// no group triggers, no outbound media, no command menus, no MarkdownV2/HTML
// formatting suite, no voice. Private-chat TEXT and inbound PHOTOS only;
// photos become bounded image parts the Host validates against the shared
// attachment limits before they reach the turn.
//
// Policy boundaries:
//   - allow_from is enforced by the kernel ChannelHost at dispatch; the
//     adapter never filters senders itself.
//   - secrets travel only through ChannelEnv.Secret (env_key names, never
//     values in config).
//   - the adapter never opens a listen socket; long polling is outbound.
//   - photo downloads ride the Host-governed HTTP client (same
//     api.telegram.org destination as the Bot API calls); their error logs
//     carry message ids and sizes, never the download URL, because that URL
//     embeds the bot token (D-010).
package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"

	plugin "agent-vivy/sdk/port/channel"
)

// ChannelName is the platform name carried by every inbound envelope and
// the config envelope key this adapter consumes (channels.telegram).
const ChannelName = "telegram"

// senderPrefix is the allow_from entry format for a Telegram private-chat
// sender (contract §11): "telegram:<numeric user id>". The Host matches
// InboundMessage.Sender against allow_from exactly.
const senderPrefix = ChannelName + ":"

// pollTimeoutSeconds is the server-side long-poll hold time of one
// getUpdates request. Long enough to keep the request rate low, short
// enough for Stop to shed the open request quickly.
const pollTimeoutSeconds = 30

// maxInboundPhotoBytes mirrors the Host's shared attachment bound
// (internal/attachment, 5 MiB): the Host re-validates every media part and
// rejects oversize content, so the adapter refuses to even download or
// publish more. The duplicate bound is a transport safety net that keeps
// unbounded bytes out of the adapter, not a second policy.
const maxInboundPhotoBytes = 5 << 20

// Plugin is the transport implementation bound by the v1 ChannelProvider.
//
// After Start returns, bot/cancel/done are set once and never mutated, so
// Send and Stop may run on any goroutine without a lock (CH-C3-N2).
type Plugin struct {
	// bot is the telego handle; non-nil once Start succeeded, never
	// swapped afterwards.
	bot *telego.Bot
	// me is the authenticated bot's own identity (getMe); the mention-only
	// group gate matches against it. Set once by Start before the poll
	// goroutine runs, never swapped afterwards.
	me *telego.User
	// cancel stops the long-poll loop; nil until Start.
	cancel context.CancelFunc
	// done is closed when the poll goroutine exited; nil until Start.
	done chan struct{}
}

// Health implements plugin.HealthChecker (CH-R-1): read-only internal
// state, no network I/O. A started ear is healthy — telego retries
// transient getUpdates failures internally (default 8s retry, errors on
// the process stderr), so there is no adapter-visible redial state this
// generation; plumbing telego's poll-loop errors into a finer
// classification stays open. Not started is temporary.
func (p *Plugin) Health(context.Context) error {
	if p.bot == nil {
		return &plugin.HealthError{Class: plugin.ClassTemporary, Err: errors.New("long poll not started")}
	}
	return nil
}

func newAdapter() *Plugin { return &Plugin{} }

// Start implements plugin.Channel. Fail-closed order: settings must decode
// and declare token_env, the token must resolve through the Host-pinned
// Secret, and the bot must authenticate (telego validates the token with a
// getMe call through the injected HTTP client) before any ear goes live.
//
// Settings transport: the Host serializes the opaque config envelope
// settings to the plugin (ChannelEnv.Settings, the one ABI addition of
// CH-C4); this adapter decodes them strictly. settings.token_env is what
// the adapter resolves; the envelope's token_env is what the Host pins
// Secret to — the two must agree or Secret errors and Start fails.
func (p *Plugin) Start(ctx context.Context, env plugin.ChannelEnv) error {
	settings, err := DecodeSettings(env.Settings())
	if err != nil {
		return err
	}
	if settings.TokenEnv == "" {
		return errors.New("telegram: settings.token_env is required " +
			"(channels.telegram.settings.token_env must name the bot token variable)")
	}
	if settings.Proxy != "" {
		return errors.New("telegram: proxy is unsupported by the governed channel transport")
	}
	token, err := env.Secret(settings.TokenEnv)
	if err != nil {
		return fmt.Errorf("telegram: resolve bot token through env %q: %w", settings.TokenEnv, err)
	}

	bot, err := telego.NewBot(token, p.botOptions(env, settings)...)
	if err != nil {
		// The token value must never reach the error (D-010); telego's
		// error messages carry only validation outcomes.
		return fmt.Errorf("telegram: create bot: %w", err)
	}
	// telego validates the token format eagerly but authenticates lazily;
	// force the getMe round-trip now so a rejected token or an unreachable
	// API server fails Start instead of leaving a deaf ear that retries
	// forever. The bot's own identity anchors the mention-only group gate.
	me, err := bot.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("telegram: authenticate bot: %w", err)
	}

	pollCtx, cancel := context.WithCancel(ctx)
	// AllowedUpdates "message" filters server-side to new messages only:
	// no edited messages, no channel posts, no callback queries.
	updates, err := bot.UpdatesViaLongPolling(pollCtx, &telego.GetUpdatesParams{
		Timeout:        pollTimeoutSeconds,
		AllowedUpdates: []string{"message"},
	})
	if err != nil {
		cancel()
		return fmt.Errorf("telegram: start long polling: %w", err)
	}

	p.bot = bot
	p.me = me
	p.cancel = cancel
	p.done = make(chan struct{})
	go p.pollLoop(pollCtx, env, updates)
	return nil
}

// botOptions builds the telego options from the adapter settings. The HTTP
// client stays outbound-only and is derived from the Host's client; the
// Host client's overall Timeout is deliberately not inherited because one
// long-poll getUpdates request legitimately stays open for
// pollTimeoutSeconds plus latency — the poll timeout itself is the
// deadline. Proxy configuration fails closed in Start because replacing
// this transport would bypass the Host's sealed destination policy.
func (p *Plugin) botOptions(env plugin.ChannelEnv, s Settings) []telego.BotOption {
	transport := env.HTTP().Transport // nil = http.DefaultTransport
	client := &http.Client{Transport: transport}
	opts := []telego.BotOption{
		telego.WithHTTPClient(client),
		// Keep telego's default logger: it writes poll retries and send
		// failures to the process stderr and redacts the bot token, which
		// is the error visibility an adapter has without a kernel logger.
	}
	if s.BaseURL != "" {
		opts = append(opts, telego.WithAPIServer(s.BaseURL))
	}
	return opts
}

// pollLoop consumes telego updates until the context is cancelled or the
// long-poll channel closes. telego already retries transient getUpdates
// failures internally (default 8s retry timeout, errors visible on the
// process stderr), so a dead ear can only happen when the host cancels —
// the loop never gives up on its own.
func (p *Plugin) pollLoop(ctx context.Context, env plugin.ChannelEnv, updates <-chan telego.Update) {
	defer close(p.done)
	for {
		select {
		case <-ctx.Done():
			return
		case upd, ok := <-updates:
			if !ok {
				// Long polling stopped (context cancelled); exit cleanly.
				return
			}
			msg, photo, publishable := normalizeUpdate(upd, p.me)
			if !publishable {
				continue
			}
			if photo != nil {
				// The download is best-effort: a failure keeps the caption
				// (when present) and drops only the image part, so media
				// trouble never silences a text the sender wrote.
				if media, ok := p.downloadPhoto(ctx, env, photo); ok {
					msg.Parts = append(msg.Parts, plugin.Part{Kind: plugin.PartMedia, Media: media})
				} else if len(msg.Parts) == 0 {
					continue // nothing publishable survived: photo failed, no caption
				}
			}
			// PublishInbound is synchronous (journal + run start). A
			// dispatch failure must not kill the ear; the Host's structured
			// logs own the audit trail, so the adapter drops and continues.
			_ = env.PublishInbound(ctx, msg)
		}
	}
}

// Stop implements plugin.Channel: cancel the long-poll loop and wait (as
// long as the given context allows) for the poll goroutine to exit. It
// must return even when the context is already cancelled.
func (p *Plugin) Stop(ctx context.Context) error {
	if p.cancel == nil {
		return nil // never started
	}
	p.cancel()
	if p.done != nil {
		select {
		case <-p.done:
		case <-ctx.Done():
		}
	}
	return nil
}

// Send implements plugin.Channel: deliver each text part as one sendMessage
// and return the platform message ids produced.
//
// Text parts carry the model's markdown: each part is converted to
// Telegram's HTML subset and sent with parse_mode=HTML (tier-1 text loop).
// When the platform rejects the formatted body, the same chunk is resent
// as plain text, so a parse rejection degrades the formatting, never the
// reply. A non-empty ReplyTo threads the message to the triggering
// inbound one (tier-1 reply threading); AllowSendingWithoutReply keeps a
// deleted anchor from costing the reply. TopicID is ignored until the
// group feature lands. Non-text parts are skipped (media is a later
// slice); an envelope with no text parts sends nothing and returns no ids.
func (p *Plugin) Send(ctx context.Context, msg plugin.OutboundMessage) ([]string, error) {
	if p.bot == nil {
		return nil, errors.New("telegram: channel not started")
	}
	chatID, err := strconv.ParseInt(msg.ChatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("telegram: invalid chat id %q: %w", msg.ChatID, err)
	}
	var reply *telego.ReplyParameters
	if msg.ReplyTo != "" {
		replyID, perr := strconv.Atoi(msg.ReplyTo)
		if perr != nil {
			return nil, fmt.Errorf("telegram: invalid reply-to message id %q: %w", msg.ReplyTo, perr)
		}
		reply = &telego.ReplyParameters{
			MessageID:                replyID,
			AllowSendingWithoutReply: true,
		}
	}
	var threadID int
	if msg.TopicID != "" {
		tid, perr := strconv.Atoi(msg.TopicID)
		if perr != nil {
			return nil, fmt.Errorf("telegram: invalid topic id %q: %w", msg.TopicID, perr)
		}
		threadID = tid
	}
	var ids []string
	for _, part := range msg.Parts {
		if part.Kind != plugin.PartText {
			continue // this slice: text out only
		}
		if part.Text == "" {
			continue
		}
		sent, err := p.bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID:          telego.ChatID{ID: chatID},
			Text:            markdownToHTML(part.Text),
			ParseMode:       telego.ModeHTML,
			ReplyParameters: reply,
			MessageThreadID: threadID,
		})
		if err != nil {
			// The platform refused the formatted body — degrade this one
			// chunk to plain text instead of dropping the reply; only a
			// plain-text failure surfaces.
			sent, err = p.bot.SendMessage(ctx, &telego.SendMessageParams{
				ChatID:          telego.ChatID{ID: chatID},
				Text:            part.Text,
				ReplyParameters: reply,
				MessageThreadID: threadID,
			})
			if err != nil {
				return ids, fmt.Errorf("telegram: send message to chat %d: %w", chatID, err)
			}
		}
		ids = append(ids, strconv.Itoa(sent.MessageID))
	}
	return ids, nil
}

// Typing implements plugin.Typing: one "typing" chat action ping. The Host
// owns the resend cadence and stops at the run's terminal; the action
// expires platform-side within ~5 seconds either way, so a trailing ping
// is harmless.
func (p *Plugin) Typing(ctx context.Context, chatID string) error {
	if p.bot == nil {
		return errors.New("telegram: channel not started")
	}
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chat id %q: %w", chatID, err)
	}
	return p.bot.SendChatAction(ctx, &telego.SendChatActionParams{
		ChatID: telego.ChatID{ID: id},
		Action: telego.ChatActionTyping,
	})
}

// photoRef describes the one downloadable photo of an accepted update:
// the largest PhotoSize variant, addressed by its file id.
type photoRef struct {
	fileID    string
	messageID int64
}

// normalizeUpdate maps one Telegram update to a kernel inbound envelope.
// Private chats publish text and/or one photo: the caption is the text
// part, the largest photo variant becomes the media part after the
// bounded download. Group and supergroup chats follow the tier-1
// mention-only ruling: a group message publishes only when it explicitly
// addresses the bot — a @username mention entity, a text_mention entity
// for the bot's user, or a /command@botname command — and the matched
// mention text is stripped from the content; group turns stay text-only
// this slice (a group photo whose only text is a caption does not
// publish). Forum topics carry their thread id in TopicID so the session
// mapping keeps contexts apart. Everything else reports as not
// publishable: edited messages, channel posts, channels, stickers,
// documents, service messages, and the bot's own outgoing messages
// (Telegram echoes them back through getUpdates; without the IsBot filter
// every reply would loop back in as a new turn).
func normalizeUpdate(upd telego.Update, me *telego.User) (plugin.InboundMessage, *photoRef, bool) {
	msg := upd.Message // nil for edited_message, channel_post, callback_query, ...
	if msg == nil {
		return plugin.InboundMessage{}, nil, false
	}
	// From is nil for messages sent to channels; SenderChat set means the
	// message was sent on behalf of a chat (anonymous admins, linked
	// channels, business accounts) — none of those is a human sender this
	// adapter can allowlist.
	if msg.From == nil || msg.From.IsBot || msg.SenderChat != nil {
		return plugin.InboundMessage{}, nil, false
	}
	isGroup := msg.Chat.Type == "group" || msg.Chat.Type == "supergroup"
	if !isGroup && msg.Chat.Type != "private" {
		// Channels are broadcast-only; other chat kinds carry no turn.
		return plugin.InboundMessage{}, nil, false
	}
	if isGroup {
		// Mention gating reads msg.Text + msg.Entities (tier-1 ruling);
		// group turns stay text-only this slice — a group photo whose only
		// text is a caption does not publish.
		if me == nil {
			// Without the authenticated identity the mention gate cannot
			// match; drop rather than guess.
			return plugin.InboundMessage{}, nil, false
		}
		mentioned, content := groupMentioned(msg, me)
		if !mentioned || content == "" || len(msg.Photo) > 0 {
			// Not addressed to the bot, a bare mention with no text left,
			// or a group photo this slice does not carry.
			return plugin.InboundMessage{}, nil, false
		}
		topicID := ""
		if msg.MessageThreadID != 0 {
			topicID = strconv.Itoa(msg.MessageThreadID)
		}
		return plugin.InboundMessage{
			Channel:   ChannelName,
			ChatID:    strconv.FormatInt(msg.Chat.ID, 10),
			Sender:    senderPrefix + strconv.FormatInt(msg.From.ID, 10),
			MessageID: strconv.Itoa(msg.MessageID),
			ReplyTo:   "",
			TopicID:   topicID,
			Parts:     []plugin.Part{{Kind: plugin.PartText, Text: content}},
		}, nil, true
	}
	// Private chat: Message.Text is empty on photo messages; the caption
	// (if any) is the sender's text. A photo without a caption is still a
	// valid turn.
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}
	var photo *photoRef
	if len(msg.Photo) > 0 {
		photo = &photoRef{
			fileID:    largestPhoto(msg.Photo).FileID,
			messageID: int64(msg.MessageID),
		}
	} else if text == "" {
		// Stickers, documents, animations, service messages — nothing this
		// slice publishes.
		return plugin.InboundMessage{}, nil, false
	}
	var parts []plugin.Part
	if text != "" {
		parts = append(parts, plugin.Part{Kind: plugin.PartText, Text: text})
	}
	return plugin.InboundMessage{
		Channel: ChannelName,
		ChatID:  strconv.FormatInt(msg.Chat.ID, 10),
		Sender:  senderPrefix + strconv.FormatInt(msg.From.ID, 10),
		// MessageID is the unique identifier inside the chat; the kernel
		// uses it for provenance, not for addressing.
		MessageID: strconv.Itoa(msg.MessageID),
		// ReplyTo/TopicID stay empty on the private path (no reply
		// threading inbound, no forum topics).
		ReplyTo: "",
		TopicID: "",
		Parts:   parts,
	}, photo, true
}

// groupMentioned reports whether a group message explicitly addresses the
// bot and returns the content with the matched mention text stripped.
// Entities are authoritative: a mention entity spans exactly the
// @username text, a text_mention entity spans the display text of an
// inline user link, and a bot_command may carry the /cmd@botname suffix
// this bot is addressed with (another bot's command stays untouched).
func groupMentioned(msg *telego.Message, me *telego.User) (bool, string) {
	content := msg.Text
	mentioned := false
	// Strip back to front so earlier offsets stay valid.
	for i := len(msg.Entities) - 1; i >= 0; i-- {
		e := msg.Entities[i]
		if e.Offset < 0 || e.Offset+e.Length > len(content) {
			continue
		}
		span := content[e.Offset : e.Offset+e.Length]
		switch e.Type {
		case telego.EntityTypeMention:
			if strings.EqualFold(span, "@"+me.Username) {
				content = content[:e.Offset] + content[e.Offset+e.Length:]
				mentioned = true
			}
		case telego.EntityTypeTextMention:
			if e.User != nil && e.User.ID == me.ID {
				content = content[:e.Offset] + content[e.Offset+e.Length:]
				mentioned = true
			}
		case telego.EntityTypeBotCommand:
			if idx := strings.Index(span, "@"+me.Username); idx >= 0 {
				content = content[:e.Offset+idx] + content[e.Offset+e.Length:]
				mentioned = true
			}
		}
	}
	return mentioned, strings.TrimSpace(content)
}

// largestPhoto picks the biggest photo variant (Telegram sends an album of
// resolutions); dimensions are always present, unlike FileSize.
func largestPhoto(sizes []telego.PhotoSize) telego.PhotoSize {
	best := sizes[0]
	for _, size := range sizes[1:] {
		if size.Width*size.Height > best.Width*best.Height {
			best = size
		}
	}
	return best
}

// downloadPhoto fetches one photo through the governed transport:
// getFile resolves the file path, then a plain GET on the Bot API file URL
// streams at most maxInboundPhotoBytes+1 bytes. Every failure drops only
// the image part. Logs carry the message id and byte sizes — never the
// download URL or its embedded token (D-010), and never a raw transport
// error, whose text includes the full URL.
func (p *Plugin) downloadPhoto(ctx context.Context, env plugin.ChannelEnv, ref *photoRef) (plugin.Media, bool) {
	if p.bot == nil {
		return plugin.Media{}, false
	}
	file, err := p.bot.GetFile(ctx, &telego.GetFileParams{FileID: ref.fileID})
	if err != nil {
		envWarn(env, "telegram: photo getFile failed; dropping the image part",
			"message_id", ref.messageID)
		return plugin.Media{}, false
	}
	if file.FilePath == "" {
		envWarn(env, "telegram: photo has no file path; dropping the image part",
			"message_id", ref.messageID)
		return plugin.Media{}, false
	}
	if file.FileSize > maxInboundPhotoBytes {
		envWarn(env, "telegram: photo exceeds the inbound size bound; dropping the image part",
			"message_id", ref.messageID, "bytes", file.FileSize)
		return plugin.Media{}, false
	}
	client := &http.Client{Transport: env.HTTP().Transport} // nil transport = http.DefaultTransport
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.bot.FileDownloadURL(file.FilePath), nil)
	if err != nil {
		envWarn(env, "telegram: photo download request is invalid; dropping the image part",
			"message_id", ref.messageID)
		return plugin.Media{}, false
	}
	resp, err := client.Do(req)
	if err != nil {
		envWarn(env, "telegram: photo download failed; dropping the image part",
			"message_id", ref.messageID)
		return plugin.Media{}, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		envWarn(env, "telegram: photo download returned a non-200 status; dropping the image part",
			"message_id", ref.messageID, "status", resp.StatusCode)
		return plugin.Media{}, false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxInboundPhotoBytes+1))
	if err != nil {
		envWarn(env, "telegram: photo download body failed; dropping the image part",
			"message_id", ref.messageID)
		return plugin.Media{}, false
	}
	if len(data) == 0 || len(data) > maxInboundPhotoBytes {
		envWarn(env, "telegram: photo is empty or over the inbound size bound; dropping the image part",
			"message_id", ref.messageID, "bytes", len(data))
		return plugin.Media{}, false
	}
	// Telegram serves compressed photos as JPEG. The claim is provisional:
	// the Host sniffs the actual bytes against the shared whitelist before
	// the part reaches the turn.
	return plugin.Media{
		Name:     fmt.Sprintf("photo-%d.jpg", ref.messageID),
		MimeType: "image/jpeg",
		Data:     data,
	}, true
}

// envWarn logs through the Host logger when one is available; the surface
// is advisory and must never panic on a nil logger (test doubles).
func envWarn(env plugin.ChannelEnv, msg string, args ...any) {
	if logger := env.Logger(); logger != nil {
		logger.Warn(msg, args...)
	}
}
