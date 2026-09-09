# CH-C7b — `plugins/qq` official Bot text (summary)

Date: 2026-08-30. Branch `feat/channel-c7b` (cut from 12a2a70 (C7a, on the `feat/channel-c6` line); the sequential slice reused the same worktree).
PLAN: `docs/plans/channel-epic/CH-C7b.md`. Contract: `VIVY-CHANNEL-PACK.md` §14.1 (qq row: official platform bot, not a personal account).

## What changed

The third real ear: an official QQ Open Platform bot (botgo v0.2.1, pinned to the same version as picoclaw), completing the WS Gateway one-to-one (C2C) text loop. The code and documentation both state: **not a personal account, not OneBot, and not NapCat**.

1. **Self-driven protocol client**: botgo's `local.ChanManager` reconnects forever and cannot be stopped (its internal panic is recovered by itself and then **silently retries forever**, even for a banned bot—the review corrected our initial claim that it "panics the process"); token `StartRefreshAccessToken` panics raw after 11 consecutive failures. Neither is used: the plugin directly drives `websocket.ClientImpl` (supervised redial + Gateway **resume**: session ID from READY, seq from event frames), while the token uses a lazy cache source plus one proactive fetch at Start for fail-closed credential validation.
2. **First-connection fence (review fix)**: every first-attempt exit path in `supervise` delivers to `firstErr` exactly once—Stop/cancellation during the first connection always makes Start return within a bound (new `TestStopDuringFirstHandshakeReturns`); a ban (`cannot-identify`) landing while waiting for READY also gives up deterministically (shared `handleDeath`, closing the race window reproduced by the race test).
3. **Inbound**: C2C text only. Group events are out of scope with source evidence—botgo v0.2.1's `dto.Message` has only a `group_id` field, so `group_openid` in real group payloads cannot be decoded (picoclaw has the same issue). QQ Official retries the same `msg_id`: the plugin adds a deduplication fence (TTL 5 min / capacity 10,000 / evict oldest, with no background goroutine).
4. **Outbound**: passive reply via `/v2/users/{openid}/messages` (`msg_type` 0 + incrementing `msg_seq` + inbound `msg_id`, under the passive-window contract); `msg_id` is stored per session in plugin memory and missing values fail closed (restart window documented); a real botgo OpenAPI client hits a loopback httptest for an end-to-end assertion (path, `QQBot <token>` header, `X-Union-Appid`, body, and surfaced errcode).
5. **Quiet logger (D-010)**: botgo's default logger prints identify payloads (including the bare access token) at INFO and full message frames—`botgo.SetLogger(quietLogger{})` globally silences it, retaining only Error; a test pins this down (`TestNewInstallsQuietLogger`).
6. settings: `app_id_env` + `app_secret_env` (the C6 `*_env` pattern) + optional `sandbox`.

## Explicitly not done (non-goals)

- Group chat (@) / the full channel/Guild suite—the official v2 events cannot decode group addresses in botgo v0.2.1; wait for the SDK to fill the gap or raise a separate proposal; do not invent one.
- Personal accounts / OneBot / NapCat / voice / large files—the contract prohibits them.
- Real network WS frame-level testing (botgo's own client is used only for compile/link validation; protocol logic is covered with fakes)—same depth as the two sibling plugins.
- Kernel-level deduplication guarantees (only the plugin-side TTL/capacity window); group-member `allow_from` semantics (meaningless while groups are not implemented).
- Real QQ Open Platform smoke testing was not done (no credentials; rollback = omit it from the recipe).

## Additional finding (not introduced by this slice)

- `internal/runtime` `TestServiceApprovalApproveFlow` flaked once under full load (isolated rerun + package rerun + a full `just ci` rerun were all green; runtime has had zero changes since C3) — registered in §0.1 (TEST-2).
