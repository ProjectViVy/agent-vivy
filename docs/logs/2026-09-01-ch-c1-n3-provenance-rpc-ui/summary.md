# CH-C1-N3 — Message provenance over RPC/UI

## What changed

Provenance has been stored with user-message rows since CH-C1 (the
`Source`/`Channel`/`ChatID`/`ChannelMessageID` fields of `domain.Message`), but
`messageResult` projected only ID/RunID/Role/Content/CreatedAt, making it
invisible to JSON-RPC. This slice completes the projection and display:

- `internal/rpc/control.go`: `messageResult` adds
  `Provenance *messageProvenanceResult` (`json:"provenance,omitempty"`).
  `messageProvenance(message)` projects according to the domain rule: it emits
  an object (`{source:"channel", channel, chat_id, channel_message_id}`) only
  when `EffectiveSource()=="channel"`; UI turns (including historical rows with
  an empty Source and in-process additions) omit the field entirely, consistent
  with the domain-layer rule that nil Provenance or an empty Source is read as
  built-in UI. Both projection points, `session/get` and `session/messages`, are
  wired up.
- `ui/src/lib/api.ts`: adds the `MessageProvenance` type and
  `Message.provenance?`.
- `ui/src/components/chat/MessageBubble.tsx`: when a user message has
  provenance, it renders a 10px muted provenance marker on the upper-right side
  above the bubble:
  `<channel|source> · <chat_id>` (plain data text; the channel name and session
  ID are opaque data, so no i18n key is introduced). UI turns have zero visual
  change. Assistant rows are not stamped with provenance and have no marker.

## What was explicitly not done

- Provenance vocabulary validation (`Source` passes through any non-empty value)
  is CH-C1-N4. The `ui|channel` vocabulary remains pending the CH-C2 SDK seam
  and contract decision; it is unchanged in this slice.
- Contract §12 payload writeback (digest/bytes vs identifiers-only) is CH-C1-N2
  and remains pending an architect decision. This slice projects the fields
  actually present on the Message row (there is no sender field in the domain).
- No UI edit/filter entry point: provenance is display-only, with no per-channel
  filtering.
