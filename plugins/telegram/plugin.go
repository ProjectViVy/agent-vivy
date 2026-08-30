// Package telegram is the first real seam-channel adapter (VIVY-CHANNEL-PACK.md
// §14.3): private-chat text in / text out over outbound long polling. It is
// the shape template for the feishu/qq/discord/dingtalk adapters — keep the
// file layout and the Start/Stop/Send structure when copying it.
//
// First-cut scope (what this adapter deliberately does NOT do): no webhook,
// no group triggers, no media, no command menus, no MarkdownV2/HTML
// formatting suite, no voice. Private-chat TEXT only.
//
// Policy boundaries:
//   - allow_from is enforced by the kernel ChannelHost at dispatch; the
//     adapter never filters senders itself.
//   - secrets travel only through ChannelEnv.Secret (env_key names, never
//     values in config).
//   - the adapter never opens a listen socket; long polling is outbound.
package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/mymmrac/telego"

	"agent-vivy/sdk/plugin"
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

// Plugin is the telegram channel adapter. It implements both plugin.Plugin
// (so Register() can carry it) and plugin.Channel (so the kernel
// ChannelHost can start it); the compile-time assertion below pins that.
//
// After Start returns, bot/cancel/done are set once and never mutated, so
// Send and Stop may run on any goroutine without a lock (CH-C3-N2).
type Plugin struct {
	// bot is the telego handle; non-nil once Start succeeded, never
	// swapped afterwards.
	bot *telego.Bot
	// cancel stops the long-poll loop; nil until Start.
	cancel context.CancelFunc
	// done is closed when the poll goroutine exited; nil until Start.
	done chan struct{}
}

// Compile-time assertion: a seam-channel plugin IS a Channel.
var _ plugin.Channel = (*Plugin)(nil)

// New is the pack-generated entry point (Register calls telegram.New()).
func New() plugin.Plugin { return &Plugin{} }

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return ChannelName }

// Seam implements plugin.Plugin: the channel seam, never the tool table.
func (p *Plugin) Seam() plugin.Seam { return plugin.SeamChannel }

// Grants implements plugin.Plugin. channel.poll is the only exception to
// the "no long-running background service" rule: outbound long polling.
func (p *Plugin) Grants() []plugin.Grant {
	return []plugin.Grant{plugin.GrantChannelPoll, plugin.GrantSecretRead}
}

// Tools implements plugin.Plugin: channel plugins carry no tools.
func (p *Plugin) Tools() []plugin.Tool { return nil }

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
	var proxyURL *url.URL
	if settings.Proxy != "" {
		u, err := url.Parse(settings.Proxy)
		if err != nil {
			return fmt.Errorf("telegram: invalid proxy URL %q: %w", settings.Proxy, err)
		}
		proxyURL = u
	}
	token, err := env.Secret(settings.TokenEnv)
	if err != nil {
		return fmt.Errorf("telegram: resolve bot token through env %q: %w", settings.TokenEnv, err)
	}

	bot, err := telego.NewBot(token, p.botOptions(env, settings, proxyURL)...)
	if err != nil {
		// The token value must never reach the error (D-010); telego's
		// error messages carry only validation outcomes.
		return fmt.Errorf("telegram: create bot: %w", err)
	}
	// telego validates the token format eagerly but authenticates lazily;
	// force the getMe round-trip now so a rejected token or an unreachable
	// API server fails Start instead of leaving a deaf ear that retries
	// forever.
	if _, err := bot.GetMe(ctx); err != nil {
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
// deadline. proxyURL may be nil (direct dialing).
func (p *Plugin) botOptions(env plugin.ChannelEnv, s Settings, proxyURL *url.URL) []telego.BotOption {
	var transport http.RoundTripper = env.HTTP().Transport // nil = http.DefaultTransport
	if proxyURL != nil {
		proxy := http.ProxyURL(proxyURL)
		if base, ok := env.HTTP().Transport.(*http.Transport); ok && base != nil {
			clone := base.Clone()
			clone.Proxy = proxy
			transport = clone
		} else {
			transport = &http.Transport{Proxy: proxy}
		}
	}
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
			msg, publishable := normalizeUpdate(upd)
			if !publishable {
				continue
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

// Send implements plugin.Channel: deliver each text part as one plain-text
// sendMessage and return the platform message ids produced.
//
// OutboundMessage.ReplyTo and TopicID are ignored this slice — private-chat
// text has no reply threading or forum topics in the first cut. Non-text
// parts are skipped (media is a later slice); an envelope with no text
// parts sends nothing and returns no ids.
func (p *Plugin) Send(ctx context.Context, msg plugin.OutboundMessage) ([]string, error) {
	if p.bot == nil {
		return nil, errors.New("telegram: channel not started")
	}
	chatID, err := strconv.ParseInt(msg.ChatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("telegram: invalid chat id %q: %w", msg.ChatID, err)
	}
	var ids []string
	for _, part := range msg.Parts {
		if part.Kind != plugin.PartText {
			continue // this slice: text out only
		}
		if part.Text == "" {
			continue
		}
		// ParseMode is intentionally left unset: plain text only, no
		// MarkdownV2/HTML suite this slice.
		sent, err := p.bot.SendMessage(ctx, &telego.SendMessageParams{
			ChatID: telego.ChatID{ID: chatID},
			Text:   part.Text,
		})
		if err != nil {
			return ids, fmt.Errorf("telegram: send message to chat %d: %w", chatID, err)
		}
		ids = append(ids, strconv.Itoa(sent.MessageID))
	}
	return ids, nil
}

// normalizeUpdate maps one Telegram update to a kernel inbound envelope.
// It accepts exactly one shape this slice — a new text message from a
// human in a private chat — and reports everything else as not
// publishable: edited messages, channel posts, groups/supergroups/channels,
// media, stickers, service messages, and the bot's own outgoing messages
// (Telegram echoes them back through getUpdates; without the IsBot filter
// every reply would loop back in as a new turn).
func normalizeUpdate(upd telego.Update) (plugin.InboundMessage, bool) {
	msg := upd.Message // nil for edited_message, channel_post, callback_query, ...
	if msg == nil {
		return plugin.InboundMessage{}, false
	}
	// From is nil for messages sent to channels; SenderChat set means the
	// message was sent on behalf of a chat (anonymous admins, linked
	// channels, business accounts) — none of those is a private-chat
	// human sender this slice can allowlist.
	if msg.From == nil || msg.From.IsBot || msg.SenderChat != nil {
		return plugin.InboundMessage{}, false
	}
	if msg.Chat.Type != "private" {
		return plugin.InboundMessage{}, false
	}
	if msg.Text == "" {
		// Stickers, photos, captions, service messages — no text part to
		// publish (media in is a later slice).
		return plugin.InboundMessage{}, false
	}
	return plugin.InboundMessage{
		Channel: ChannelName,
		ChatID:  strconv.FormatInt(msg.Chat.ID, 10),
		Sender:  senderPrefix + strconv.FormatInt(msg.From.ID, 10),
		// MessageID is the unique identifier inside the chat; the kernel
		// uses it for provenance, not for addressing.
		MessageID: strconv.Itoa(msg.MessageID),
		// ReplyTo/TopicID stay empty this slice (private chat, no reply
		// threading, no forum topics).
		ReplyTo: "",
		TopicID: "",
		Parts:   []plugin.Part{{Kind: plugin.PartText, Text: msg.Text}},
	}, true
}
