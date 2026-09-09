# CH-C1 — acceptance (how a person can tell it worked)

Date: 2026-08-30.

## Product view: users see nothing in this slice—and that is correct

CH-C1 is the Stage A "genetic material": the Journal first learns about the world's
entry point, while the body still has no ears. Therefore:

- The local UI conversation (`http://127.0.0.1:3015` or embedded at `:8787`) has
  **no behavioral change**—sending, receiving replies, and approvals work as before.
  User rows in the ledger now have an explicit `Source="ui"`; the semantics are
  exactly the same as before (empty = ui).
- Telegram/DingTalk/Feishu/QQ/Discord are not configurable or connectable—nonexistent
  features do not appear in Settings.
- The default `vivy.exe` body is unchanged: `Register()` is still nil, and `go.mod`
  has no platform SDK.

## Human-verifiable points (without reading code)

1. **The gate is green**: run `just ci` from the repository root and get exit code 0
   (all Go packages pass, UI has 21 files / 175 tests, and the build succeeds).
2. **The ledger has a new page**: open `schemas/events/payloads/channel.inbound.json`
   and see the defined world-entry fields
   (channel/chat_id/sender/message_id/session_id, with optional run_id), with no
   token/key/raw-payload fields.
3. **The old ledger remains sound**: start the new version with a pre-upgrade SQLite
   Journal (the engine containing `data/vivy.db`); historical sessions still open and
   messages still display—migration016 only adds four columns with empty-string
   defaults to the `messages` table.
4. **Provenance semantics** (for developers/acceptors): any Message with an empty
   `Source` or `"ui"` is interpreted as a local UI message; rows carrying
   `Channel/ChatID/ChannelMessageID` are messages entering from the world (from C3
   onward). Conformance `CN-17` pins both rules in tests.

## Explicitly not part of this slice's acceptance

- Sending and receiving real platform messages → CH-C4/C6/C7.
- Seeing channels or body-ear toggles in inspect/Settings → CH-C5.
- Seeing a real `channel.inbound` event in the Journal → CH-C3 (the Host is the
  emitter).
- Seeing message provenance in UI/JSON-RPC → CH-C5 (this slice's RPC does not project
  provenance).

## Rollback

Revert this branch: there is no runtime ear, and the risk surface is limited to four
new storage columns (`DEFAULT ''`, backward compatible) and one new event enum value
(with no emitter).
