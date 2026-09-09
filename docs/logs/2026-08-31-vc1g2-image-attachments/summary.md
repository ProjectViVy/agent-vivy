# VC-1g-2 image-attachment path

Date: 2026-08-31　Branch: `feat/vc1a-bash-tool` (worktree `agent-vivy-vc0`)　Task: VC-1g second delivery

## Scope

User messages can carry image attachments; the complete path from the UI to kernel multimodal input is connected:

- **UI**: ChatInput's paperclip selects images / the text box directly pastes screenshots (aligned with Crush's image-paste interaction);
  pending thumbnails can be removed individually; client-side gates match the server (png/jpeg/gif/webp, 5MB per image,
  at most 4 per message), with inline notices for violations; sending during a run uses the VC-1g-1 queue and carries the attachments.
- **RPC**: `turn/start` adds `attachments: [{name?, mime_type, data(base64)}]`;
  the server is the authoritative gate (MIME allowlist / base64 decoding / 5 MiB / 4 images; out-of-range input returns
  InvalidParams with a 1-based index). `session/messages` returns
  `attachments: [{name?, mime_type, data_url}]` for user messages (the server builds the data URL).
- **Storage**: new `message_attachments` table (message_id / position / name / mime_type /
  data BLOB), SQLite migration 18, Postgres schemaV17 (a fresh start bootstraps V15+
  and upgrades to 17); AppendMessage transactionally writes the message + attachments; ListMessages backfills via JOIN;
  DeleteSession explicitly removes attachment rows.
- **Kernel**: `runtime.RunOptions.Attachments` passes through to the Journal; `buildRunContext`
  projects user messages with attachments into eino-standard multimodal input
  (`schema.UserInputMultiContent`: a text part + an `image_url` part with
  Base64Data/MIMEType); the no-attachment path keeps `schema.UserMessage` unchanged.
- **Compaction safety**: the compaction summary transcript remains plain text; attachments render as
  `[image attachment: name]` placeholder lines, and the summarizer never touches binary data.
- **Byte budget**: attachment bytes are deliberately excluded from `ContextPolicy.MaxBytes` (images are billed in visual tokens
  rather than text bytes; a single 5MB image becomes ~6.7MB after base64 and would falsely trigger
  `ErrContextBudgetExceeded` if counted). The reason is documented in the context.go comment.
- **UI rendering**: thumbnails appear inside the user bubble (server data_url and local optimistic row use the same rendering).

## Explicitly not done (Crush boundary alignment; no unapproved additions)

- **SupportsImages model gating**: Crush uses model metadata to reject images on models that do not support them.
  Vivy currently has no model-metadata pipeline; gating belongs to VC-2 (model metadata) and will be done there.
  Current behavior: the request reaches the provider, which upstream/provider reports as an error.
- Non-image attachments (PDFs, text files) are outside this slice (Crush has none either).
- Tool-result image workaround (Crush's headless screenshot path in the CLI) is not included.
- Channel (feishu, etc.) media entry points are not included; this slice covers the web UI only.

## Legal and alignment

Crush is FSL-1.1-MIT: this slice provides behavior/protocol alignment only (attachment limits, type allowlist, image-paste interaction,
and queued attachments), with zero code copying; the implementation is based entirely on eino's native multimodal API and our own storage design.

## Verification

See `verification.md`.

## Known limitations

- When the model does not support vision, the error comes from the provider (VC-2 will reject it earlier once SupportsImages gating lands).
- The queue pill's chip shows text only; attachment count is not shown separately (text is required, so the chip is non-empty).
