# CH-C1 — Ledger: `channel.inbound` + Message provenance (summary)

Date: 2026-08-30. Branch: `feat/channel-c1` (cut from
`feat/channel-super-contract` 82ecf14, in an independent worktree).
PLAN: `docs/plans/channel-epic/CH-C1.md`. Contract:
`docs/architecture/VIVY-CHANNEL-PACK.md` §7.3/§8/§12.

## What changed

The Journal now knows "what came in through which ear from the world." The body still
has no ears: no adapter, ChannelHost, or sdk/plugin ABI change, and the Eino loop is
untouched.

1. **Four Message provenance fields** (`internal/domain/session.go`): `Source`
   (`"ui" | "channel"`, with empty read as ui), `Channel`, `ChatID`, and
   `ChannelMessageID`. Added `EffectiveSource()` (empty → `"ui"`) and a zero-value
   compatibility test (`TestMessageEffectiveSource`). All `domain.Message{` literals
   across the repository use keyed fields, so adding fields causes no compile break
   (confirmed by the explore scan).
2. **New event type** (`internal/domain/event.go`): `EventChannelInbound =
   "channel.inbound"`, registered in `EventTypes` (35→36); it is non-terminal, and
   `Terminal()`/`RunStatus()` are unchanged. `TestEventVocabulary` is updated to
   36.
3. **Event schema**: created `schemas/events/payloads/channel.inbound.json`
   (`channel/chat_id/sender/message_id/session_id` required, `run_id` optional,
   `additionalProperties: false`; no token, raw webhook body, or content fields).
   Added `"channel.inbound"` to the `run-event.schema.json` enum as a one-for-one
   mirror of `EventTypes`.
4. **Storage migration (both engines)**:
   - sqlite: `migration016` adds four `ALTER TABLE messages ADD COLUMN ... TEXT NOT NULL
     DEFAULT ''` statements (following the migration011 pattern; tests run the full
     migration chain through `Open`).
   - postgres: `schemaVersion` 14→15; `migrate()` adds an in-place upgrade branch—
     databases recorded at version 14 run `schemaV15Upgrade` (the same four ALTERs),
     while new databases use the full `schemaV15` DDL. Existing postgres databases
     are therefore not left behind.
   - The two `messages.go` implementations (INSERT/SELECT/Scan, 9→13 columns) remain
     byte-for-byte isomorphic.
5. **Conformance CN-17**, "message provenance round-trip": exact round-trip of the
   four channel fields plus an empty-Source row reading back as
   `EffectiveSource()=="ui"`; guard 16→17. Both backends mount it automatically.
6. **UI path**: `RunWithOptions` in `internal/runtime/service.go` explicitly sets
   `Source: "ui"` on user rows; assistant/tool projections remain empty (empty = ui),
   so semantics are unchanged.
7. **Upgrade-path test**: `internal/storage/postgres/upgrade_test.go` manually
   creates a database from the v14 DDL frozen at 82ecf14 → upgrades it in place through
   production `OpenSchema` → asserts version history `[14 15]`, four
   `NOT NULL DEFAULT ''` columns, survival of old rows with
   `EffectiveSource()=="ui"`, and round-tripping of a provenance-bearing write.
8. **Documentation consistency**: the conformance count in
   `docs/AGENT-VIVY-ARCHITECTURE-V0.md` changed from CN-01..CN-16 to CN-01..CN-17.

## Difference from the contract (implemented per PLAN; architect confirmation pending)

The Journal sketch in contract §12 specifies
`{channel, peer, message_id, content_digest, bytes}`; CH-C1 PLAN §4 defines
`{channel, chat_id, sender, message_id, session_id, run_id?}` (peer is split into
chat_id+sender, with no content_digest/bytes). Per the authority order (the PLAN is
this slice's start order), that definition was implemented and recorded in
`docs/TODO.md` §0.1 for the architect to confirm whether the contract should be
updated.

## Explicitly not done

- No ChannelHost (creation of `internal/channelhost` is prohibited), five adapters,
  or platform SDK; `go.mod` has no new dependencies.
- No changes to `sdk/plugin`, `internal/pluginhost`, `plugins/`, or
  `internal/generated/plugins/zz_register.go` (it still `return nil`).
- RPC `messageResult` does not project provenance (not visible in UI/JSON-RPC); left
  for the CH-C5 inspect/Settings slice and recorded in §0.1.
- `Source` has no typed vocabulary validation (any non-empty value is passed through);
  the vocabulary will be settled with the contract when the C2 SDK seam lands, and is
  recorded in §0.1.
- No Go payload struct or emitter for `channel.inbound` (without a Host there is no
  emitter; C3 grows it with the mapper).
- The envelope tension between required `run_id` in the `run-event` envelope and
  `channel.inbound` (which occurs before a run exists) was left for a decision before
  C3 design and recorded in §0.1.
- Not pushed.
