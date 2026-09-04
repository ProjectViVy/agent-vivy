# Vivy Code TUI image attachments

## Delivered

- Added the shared `/image` (`/attach`) command with `remove <index>` and
  `clear`, metadata-only pending chips, compact history chips, and local
  command routing for both fullscreen TUI drivers.
- Added `image_support_known` and `image_supported` to the session-context
  contract. The image command fails closed until the model capability is
  known and supported; the existing server-side `SupportsImages` gate remains
  authoritative for direct `turn/start` callers.
- Added the control-plane `attachments/resolve` read-only seam. Only the
  explicitly injected canonical code project root is used; packed faces and
  TUI code receive no filesystem grant. Paths reject absolute/drive-relative,
  UNC, NUL, traversal, directories, symlink/junction escapes, non-images and
  size/count violations. Bytes are bounded to 5 MiB + 1 and MIME is sniffed
  from content against the PNG/JPEG/GIF/WEBP allowlist; display names are
  control-character-stripped and bounded before entering terminal/history UI.
- `turn/start` accepts `attachment_paths`, combines them with inline images
  under the existing four-image limit, and projects resolved bytes through the
  existing domain, persistence and Eino multimodal path. The bounded RPC frame
  now genuinely carries the documented maximum inline envelope.
- Queue entries snapshot attachments and carry their originating SessionID;
  failed starts restore drafts, successful starts clear them, and a queued
  turn is never retargeted after a session switch. History DTOs are unified
  between `session/get` and `session/messages`; terminal rendering contains
  metadata only (`[image: name]`), never base64.
- Terminal history requests explicitly select metadata-only attachment DTOs;
  web callers retain the default data URL representation without forcing TUI
  clients to receive up to 20 MiB of image bytes.
- `app.New` no longer treats `Runtime.WorkspaceRoot` as an attachment root by
  default. `codeface.Prepare` canonicalizes the project root and
  `codeface.Run` injects it through the explicit `WithCodeProjectRoot` seam.
  A generated packed TUI gets the same canonical current-project seam while
  non-TUI faces remain isolated. Final file opening uses Go's rooted directory
  handle API so rename/symlink races cannot escape the project.

## Explicitly not done

- No new image model or provider pipeline was introduced.
- Split diff, model selection, richer token/cost presentation, governed shell
  execution and `@file` context remain separate follow-up tracks.
- No production tenant Journal or Studio state was read or written.
