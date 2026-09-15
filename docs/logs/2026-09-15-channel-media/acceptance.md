# Acceptance — channel media (batch 2)

How a human can tell the batch works. Configure at least one ear in
Settings (telegram is the easiest to exercise end to end) with a real bot
token, send a message from an allow-listed account, and watch the chat.

## Inbound media (chat → Vivy)

1. **A photo becomes part of the conversation.** Send one picture to the
   bot (with or without a caption). The reply should respond to the
   image's content, not just the caption. In the web UI's session
   history the user turn shows the image inline, exactly like a picture
   uploaded through the chat box.
2. **An album is one conversation.** Send 2–5 pictures as a Telegram
   album. Vivy answers once, about the whole set — not once per picture.
3. **A voice note does not become silence.** Send a voice message or a
   document (e.g. a PDF). The bot cannot see the bytes, but its answer
   acknowledges the attachment ("you sent a file: …" level of awareness
   via the `[file: name]` annotation). Nothing crashes.
4. **Big is rejected, not mangled.** Send an image over 5 MiB (easy on
   telegram with a high-res photo as a *file*). The turn still works from
   its text; the log (gateway log, `channelhost` warnings) shows the
   oversized part being dropped — never a corrupted half-image.
5. **A broken download does not eat the message.** (Needs log access.)
   If the platform's CDN hiccups, the text still becomes a turn and the
   `[image: …]` annotation stays in the transcript.

## Outbound media (Vivy → chat)

Outbound media needs a producer, which does not exist yet (a future
slice adds one, e.g. an image-generation tool). Until then the visible
acceptance is:

6. **Inspect advertises the media face.** In Settings → channel inspect,
   the ears with media support now report `Media: true` (telegram,
   discord, qq, feishu; dingtalk stays false). Before this batch every
   ear reported false.
7. **The ledger treats media like any reply.** (Needs log access or a
   deliberately broken token.) When an ear's media send fails, the
   delivery intent retries and eventually parks as a failed delivery —
   the same row the redeliver button in Settings already drives; pressing
   it re-uploads the media instead of replaying a cached result.

## Contract reading

8. `docs/architecture/VIVY-CHANNEL-PACK.md` §1 now contains the two
   media rulings (inbound four-ear images-only; outbound batch
   MediaSender with re-upload semantics), and §12 describes both paths.
   DingTalk's row says media is skipped by ruling with the reason.

## Regression guards

9. Existing text behavior is unchanged: markdown rendering, reply
   threading, typing indicators, group mention gates, HITL approval
   commands — the batch's commits touch those files additively and the
   per-ear test suites cover them.
