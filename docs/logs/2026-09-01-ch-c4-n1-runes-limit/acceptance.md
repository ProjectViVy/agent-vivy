# Acceptance

How a human can tell CH-C4-N1 is closed:

1. Talk to Vivy through a Telegram private chat and ask for something
   that produces a very long reply (e.g. "把 Vivy 的架构分层完整讲一遍").
   Before this slice the reply was silently lost (sendMessage 400).
   Now it arrives as several consecutive messages, each within the
   4096-character limit, in order, and together reading as the full
   reply.
2. A channel without a `max_message_runes` declaration (feishu/qq/
   discord/dingtalk today) behaves exactly as before — whole messages,
   no splitting.
3. `go test ./internal/channelhost -race` shows the splitting contract
   (`TestSplitRunes`, `TestDeliverySplitsAtAdapterRunesLimit`) green.
