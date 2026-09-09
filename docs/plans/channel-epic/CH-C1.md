# CH-C1 — Ledger: `channel.inbound` + Message Provenance

## 1. Identity

| | |
|---|---|
| ID | CH-C1 |
| Stage | A Genetic Material |
| Person-days | 2 |
| Milestone | M-CH1 Foundation |
| Dependency | CH-0 (DONE) |
| Successor | CH-C2 |
| Branch | **New** `feat/channel-c1` (do not write into `feat/channel-super-contract`) |
| Contract | `VIVY-CHANNEL-PACK.md` §7.3, §8, C1 |
| Evolution | `VIVY-CHANNEL-EVOLUTION.md` Stage A |

Before starting: `00-standing-orders.md`.

## 2. Goal

Journal can record "what the world said and through which ear it entered." Local UI conversations remain unchanged. The body still has no ears.

Success: `just ci` is green; the new event schema exists; old Messages read back with `Source=ui`; no adapter, Host, or sdk/plugin behavior changes.

## 3. Current State

- `internal/domain/session.go` `Message`: ID/SessionID/RunID/Role/CreatedAt/Content/Tool*. No provenance.
- `internal/domain/event.go` has no `channel.*`.
- `schemas/events/payloads/` has no `channel.inbound.json`.
- `internal/storage/sqlite/messages.go` inserts nine columns; `postgres/schema.go` `CREATE TABLE messages` does the same.
- `internal/runtime/service.go` `RunWithOptions` directly `AppendMessage`s the user row, with no Source.
- No ChannelHost.

## 4. Target Structure

This slice grows only the L0 genetic material, not the Host.

```text
domain.Message
  + Source            "ui" | "channel" (empty means ui)
  + Channel           platform name; empty on ui rows
  + ChatID            part of the session key; empty on ui rows
  + ChannelMessageID  platform message_id; empty on ui rows
  Forbidden: token, raw JSON blob, or platform-private metadata as primary columns

domain.EventChannelInbound = "channel.inbound"
  payload: channel, chat_id, sender, message_id, session_id, run_id?
  No secrets and no raw webhook body

messages table: new column DEFAULT ''; existing rows = ui
```

## 5. File Inventory

**Modify**

- `internal/domain/session.go` — Message fields + validation
- `internal/domain/event.go` — EventType
- `schemas/events/payloads/channel.inbound.json`
- `schemas/events/run-event.schema.json` — add the type to the enum (if present)
- `internal/storage/sqlite/sqlite.go` — messages DDL / migration
- `internal/storage/sqlite/messages.go` — INSERT/SELECT
- `internal/storage/postgres/schema.go` + `postgres/messages.go`
- `internal/storage/conformance` — provenance round trip
- `internal/runtime/service.go` — explicitly set `Source: "ui"` on the UI path (or empty=ui)
- Test fixtures that touch Message literals

**Do Not Touch**

- `sdk/plugin`, `internal/pluginhost`, `plugins/`, or platform SDKs
- The Eino loop in `internal/runtime/engine.go`
- UI settings pages

## 6. Steps

1. Add fields to `Message`; treat an empty Source as `ui`. Unit-test zero-value compatibility.
2. Add `EventChannelInbound` + JSON schema. The payload must not contain a token.
3. sqlite/pg: use `ALTER` or rebuild the test-database DDL, with DEFAULT `''`. Conformance: write a channel row and read it back.
4. `Service.Run` UI path: keep Source as ui. Existing runtime tests must remain green.
5. Do not implement the Host. A pure-function test for "construct inbound payload" may be added.
6. `just ci`.
7. `docs/logs/YYYY-MM-DD-channel-c1/`. TODO CH-C1 → DONE.

## 7. Acceptance

- `just ci` is green.
- No `telego` or similar appears in a species' `go.mod`.
- A new-database Message read without the provenance column fails = unacceptable; old rows must still be Listable.
- The event schema contains `channel.inbound`; fixtures contain no secrets.
- UI conversations (runtime tests) still run without requiring the new fields.

## 8. Prohibitions

- Create `internal/channelhost`.
- Change the `sdk/plugin` ABI.
- Treat `Metadata map[string]string` as the primary contract.
- Put secrets into Journal / events.
- Mix this slice with C2 in one commit.

## 9. Risks and Rollback

- The sqlite test database uses full DDL rather than migrations: both schemas must be changed.
- Adding fields to the Message struct can break unnamed composite literals: search the repository for `domain.Message{`.
- Rollback: revert this branch; with no runtime ears, the risk is limited to storage columns.

## 10. Handoff

After completion: the next AGENT reads [CH-C2.md](CH-C2.md). C2 depends on this slice's Message provenance field names; do not rename them in C2.

> **DONE 2026-08-30** — Branch `feat/channel-c1`; finalized fields `Source` / `Channel` / `ChatID` / `ChannelMessageID` (empty Source=ui, `Message.EffectiveSource()`). C2 directly depends on these names. Filing: `docs/logs/2026-08-30-channel-c1/`.
