# Channel media (batch 2): 2026-09-15

## What shipped

Channel media for four ears — inbound images into the turn, outbound
images through the durable delivery path — on `feat/channel-tier1`, on
top of the tier-2 foundation absorbed into the lane at the start of the
batch (three cherry-picked commits: the by-value `PartMedia` port ruling,
the `internal/attachment` single source for the image limits, and the
telegram single-photo inbound path).

1. **Contract first** (`029f6cc`). §1 Decision Record gained two rulings:
   inbound media widens from telegram-only to telegram / discord / qq /
   feishu with an images-only scope (png/jpeg/gif/webp, 5 MiB, ≤4 per
   message, host re-validation as the authority, non-image attachments
   kept as text annotations, telegram album aggregation, no transcoding,
   dingtalk skipped for lack of a reference); outbound media ruled in the
   batch `SendMedia(ctx, chatID, []Part)` signature, the terminal
   assistant row as the byte source, per-attempt re-read and re-upload
   under the at-least-once ledger with orphan-file tolerance. §12, §8's
   note, and §14/§14.3 aligned.

2. **Port + host path** (`0d5e04e`). `plugin.MediaSender` grew to a batch
   signature at zero breakage (no implementer existed). `deliver` re-reads
   the assistant row's attachments with the text on every attempt and
   hands them to the ear as one batch: invalid parts dropped with a log
   (reject, never truncate), a media failure burns the attempt like any
   send failure, an ear without the face logs a warning and delivers the
   text. `domain.Message.Attachments` widened to name assistant rows as
   the outbound source.

3. **Inbound images, four ears** (one commit per ear). Discord
   (`d4e395f`), QQ (`74be4dd`), Feishu (`2676ca6`), Telegram albums
   (`1423c9c`). Each adapter pre-screens image-class attachments and
   downloads through the governed transport with a bounded read; QQ adds
   its `X-Union-Appid` + bearer headers, Feishu falls back from
   `MessageResource.Get` to `Image.Get`, Telegram aggregates albums under
   a sliding 500 ms window that `Stop` drains. Non-image attachments
   survive as `[file: name]` annotations; a failed download keeps its
   `[image: name]` annotation, so media trouble never silences a turn.

4. **Outbound media, four ears** (one commit per ear). Discord
   (`01a2c38`) one complex send; Telegram (`65733b9`) photo / album
   (≤10 per group) with the `PHOTO_INVALID_DIMENSIONS`→document fallback;
   QQ (`c9e7298`) the two-step `/files` upload then `msg_type=7` with
   plugin-owned bodies (botgo would base64 `file_info` twice); Feishu
   (`6b908fe`) `Image.Create` then a `msg_type=image` message.

5. **P9 evidence** (`aaefaad`). Digests for the four ears and the
   internal tree refreshed through the two-pass source-hash flow.

## Explicitly not done

- DingTalk media in both directions (no picoclaw reference; skipped by
  ruling, recorded in §1).
- Voice / audio / video / document bytes anywhere in the pipeline — text
  annotations only. Real transcription, TTS, and audio parts need their
  own capability proposal.
- Media producers: the model cannot generate attachments yet. The
  outbound path is verified with injected assistant rows in tests; a
  producer (e.g. an image tool) is a later slice.
- Caption handling on outbound media (text is delivered as text chunks
  first; per-picoclaw caption splitting applies only when a producer
  exists to attach captions).
- Group media on Telegram (group turns stay text-only this slice under
  the mention-gate ruling; the other ears carry group images behind their
  existing mention gates).
- Forum topics as a media addressing target (`MediaSender` carries no
  topic id this slice).

## Absorbed foundation

The tier-2 lane (`feat/channel-tier2`, six commits) was the second
agent's work. Its contract ruling and media foundation were cherry-picked
into this lane (`70aac22`, `5b3d144`, `5e27431`) with conflicts resolved
against the tier-1 text loop; its typing implementation was superseded by
the tier-1 one already on this branch. The tier-2 branch remains as a
record and can be deleted.
