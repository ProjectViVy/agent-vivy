# CH-C4-N1 — outbound max_message_runes enforcement

## What changed

The envelope field declared in every channel manifest
(`channel.max_message_runes`, telegram: 4096) was validated at pack time
but enforced nowhere: an assistant reply longer than the platform limit
failed Telegram's `sendMessage` and the whole delivery was lost (the
motivating observation of the TODO row). Enforcement point decision:
**Host-side generic splitting**, one implementation serving every
adapter, per the row's stated options.

- `sdk/plugin/channel.go`: new optional capability `RunesLimiter`
  (`MaxMessageRunes() int`), same type-assertion pattern as the other
  optional channel capabilities. Adapters declare their platform truth;
  adapters without the capability keep receiving whole messages.
- `internal/channelhost`:
  - `outboundTarget` carries `maxRunes`, resolved once at dispatch time
    via the capability assertion.
  - `deliverCompleted` sends the reply as sequential chunk sends; a
    failure mid-reply logs what was delivered before stopping (partial
    delivery beats total loss, and the error stays loud).
  - `splitRunes` chunks at the rune bound, preferring the last newline
    inside the window (paragraph shapes survive) with a hard break as
    fallback; no empty chunk; chunks always reassemble to the original.
- `plugins/telegram`: implements `MaxMessageRunes() int { return 4096 }`,
  doc-commented to stay in sync with `vivy-plugin.json`.
- `internal/channelhost/fake` stays capability-free (package contract);
  tests use a local `runesLimited` wrapper instead, following the
  `typingStub` pattern.

## Semantics and limits

- Runes approximate platform character counts. Telegram counts UTF-16
  code units, so an astral-heavy (emoji-dense) reply can still edge past
  the true ceiling — but never by the order of magnitude that caused the
  original total loss. Recorded as accepted v1 semantics.
- The manifest field and the adapter method can drift (both say 4096
  today, no cross-check). A verify-time equality check is possible
  future hardening, noted in the TODO row's closure rather than added
  here (the manifest is pack-time-only today; no runtime surface).

## Explicitly not done

- No dedup/replay protection for inbound (separate concern, CH-C3-N1).
- No media-part splitting (v1 is text-only).
- No UI/config knob — the bound is the adapter's platform property, not
  a user preference.
