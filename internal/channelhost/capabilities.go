package channelhost

import plugin "agent-vivy/sdk/port/channel"

// Capabilities is the discovered optional surface of one channel plugin
// (VIVY-CHANNEL-PACK.md §8 matrix). The host reports it via inspect:
// compiled-in is what the generated Assembly holds, advertised is Discover's result,
// enabled is what the channels envelope configures.
type Capabilities struct {
	// Typing maps plugin.Typing ("交互": typing indicator).
	Typing bool
	// Edit maps plugin.MessageEditor ("delivery": edit).
	Edit bool
	// Delete maps plugin.MessageDeleter ("delivery": delete).
	Delete bool
	// Reaction maps plugin.ReactionSender ("交互": emoji reaction).
	Reaction bool
	// Placeholder maps plugin.Placeholder ("交互": replaceable working message).
	Placeholder bool
	// Media maps plugin.MediaSender ("介质": media part delivery).
	Media bool
	// MediaStore maps the ChannelEnv.Media handle ("介质": media ingestion).
	MediaStore bool
	// Webhook maps plugin.WebhookHandler ("入站面": webhook on a
	// host-owned socket).
	Webhook bool
	// Listen maps plugin.ListenHandler ("入站面": dedicated listen
	// transport on a host-owned socket).
	Listen bool
	// Stream maps plugin.StreamingCapable ("交互": outbound deltas).
	Stream bool
	// Health maps plugin.HealthChecker ("可靠": platform health probe).
	Health bool
	// TaskLifecycle and PipeServer (A2A / NeuroLink, stage H) stay
	// zero-method reserved slots in this generation: they are part of the
	// ABI catalog but assert to nothing here and are not reported.
}

// Discover reports the optional capability interfaces a channel plugin
// implements, via type assertions against the focused v1 Channel Port. A
// plugin implementing none of them (the v1 text-only cut) yields the zero
// value. Assembly wrappers are transparent: the probe follows
// CapabilitySource links to the innermost target and asserts there, so a
// bound channel reports exactly what its adapter implements — never what a
// wrapper happens to declare. A missing or nil link falls back to
// asserting the wrapper itself, which honestly reports nothing.
func Discover(ch plugin.Channel) Capabilities {
	var c Capabilities
	target := any(ch)
	for {
		source, ok := target.(plugin.CapabilitySource)
		if !ok {
			break
		}
		next := source.CapabilityTarget()
		if next == nil {
			break
		}
		target = next
	}
	if _, ok := target.(plugin.Typing); ok {
		c.Typing = true
	}
	if _, ok := target.(plugin.MessageEditor); ok {
		c.Edit = true
	}
	if _, ok := target.(plugin.MessageDeleter); ok {
		c.Delete = true
	}
	if _, ok := target.(plugin.ReactionSender); ok {
		c.Reaction = true
	}
	if _, ok := target.(plugin.Placeholder); ok {
		c.Placeholder = true
	}
	if _, ok := target.(plugin.MediaSender); ok {
		c.Media = true
	}
	if _, ok := target.(plugin.MediaStore); ok {
		c.MediaStore = true
	}
	if _, ok := target.(plugin.WebhookHandler); ok {
		c.Webhook = true
	}
	if _, ok := target.(plugin.ListenHandler); ok {
		c.Listen = true
	}
	if _, ok := target.(plugin.StreamingCapable); ok {
		c.Stream = true
	}
	if _, ok := target.(plugin.HealthChecker); ok {
		c.Health = true
	}
	return c
}
