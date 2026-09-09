# CH-C4 — `plugins/telegram` Private-Chat Text (ABI Template)

## 1. Identity

| | |
|---|---|
| ID | CH-C4 |
| Stage | D ABI Template |
| Person-days | 2 |
| Milestone | M-CH2 |
| Dependency | CH-C3 |
| Successor | CH-C5; C6 may already be parallel |
| Branch | `feat/channel-c4` |
| Contract | §9, §14.3 telegram, C4 |

This is the first real ear. Later feishu/qq/discord adapters must **copy this package's shape**; do not invent a separate shape for each.

**Note:** Before formally writing the telegram adapter, first read the corresponding picoclaw implementation—the **most complete** Go sample among the five channels. Read-only rewrite; imports are prohibited. Reference directory: `.workspace/picoclaw/pkg/channels/telegram` or `C:\Users\Administrator\Desktop\morediva\.workspace\picoclaw\pkg\channels\telegram`. Borrow Start/Stop/Send, error classification, and token usage; do not borrow `init()` blank imports, empty allow_from approval, or plugin-created HTTP. See `00-standing-orders.md`.

## 2. Goal

`plugins/telegram` has an independent go.mod. `vivy-sdk verify` + `pack --with telegram` produces a candidate EXE that can send and receive private-chat text. The default `just ci` / species `go.mod` **does not contain** `github.com/mymmrac/telego`. Empty allow_from is still rejected.

## 3. Current State

- Host TCK recognizes only the fake channel.
- `plugins/` contains only `hello-fs` (species module).
- picoclaw reference (read-only): `.workspace/picoclaw/pkg/channels/telegram` or the user's machine at `C:\Users\Administrator\Desktop\morediva\.workspace\picoclaw`.
- The settings page still uses localStorage (wired in C5).

## 4. Target Structure

```text
plugins/telegram/
  go.mod                 module .../plugins/telegram
  vivy-plugin.json       seam: channel, grants: [channel.poll, secret.read]
  plugin.go              New() plugin.Plugin and implements Channel
  settings.go            decode opaque yaml (proxy/base_url may go here)
  plugin_test.go         no real network; fake updates
  README.md
```

Transport: outbound long-poll. Do not implement webhook.

## 5. File Inventory

**Create** `plugins/telegram/**`

**Modify** the pack recipe example (documentation or testdata recipe; do not change the default just run recipe); if the C3 Host finds a telegram-specific issue, it may fix only the Host's **generic envelope**, and must not add `TelegramSettings` to `internal/config`.

**Do Not Touch** direct telego requirements in a species' `go.mod`; `voice`; `internal/runtime/engine.go`.

## 6. Steps

1. `go mod init` an independent module; `replace` sdk/plugin with the repository-relative path.
2. **Rewrite** Start/Stop/Send, SenderInfo, and error classification from the picoclaw reference. Do not import its module.
3. Start: read `token_env` → `ChannelEnv.Secret`; run the poll loop; call inbound `PublishInbound`.
4. Host enforces allow_from; the plugin must not allow an empty list itself.
5. Unit-test settings decoding and inbound construction; no live Telegram.
6. `vivy-sdk verify plugins/telegram`.
7. `vivy-sdk pack --with telegram`; `inspect-artifact` contains telegram/telego; `go list` on the default tree does not contain telego.
8. Manual smoke test (optional, not a CI blocker): real Bot + non-empty allow_from.
9. `just ci` (default path).
10. Log to `docs/logs/YYYY-MM-DD-channel-c4/`.

## 7. Acceptance

- Default `go test ./...` does not compile the telego closure of `plugins/telegram`, or CI explicitly excludes vet/test for that module from the species `./...` (the independent module is not in `./...` anyway—confirm that `go test ./...` in `just ci` does not recurse into it with `-r`).
- The pack candidate links telego.
- verify rejects manifests containing tools.
- Empty allow_from cannot Start (Host behavior; regress C3 TCK + telegram configuration).

## 8. Prohibitions

- Webhook, group triggers, media, command menus, or the full MarkdownV2 feature set.
- Write telego into a species go.mod.
- `init()` blank imports.
- Plugin `net.Listen`.

## 9. Risks and Rollback

- The telego API may differ from picoclaw's vintage: prioritize reliably receiving private-chat text; do not chase the newest API.
- The replace path must resolve in the pack-generation environment.
- Rollback: omit --with telegram from the recipe; the default body remains unchanged.

## 10. Handoff

[CH-C5.md](CH-C5.md) requires the compiled-in name `telegram`. C6/C7 copy this package's directory shape.

> **DONE (2026-08-30)**: Delivered; see `docs/logs/2026-08-30-channel-c4/`.
> Handoff notes: compiled-in name `telegram`; settings are passed to the plugin through `ChannelEnv.Settings()`
> (`json.RawMessage`, `{}` means absent)—the only ABI addition in this batch;
> `token_env` is declared in two places (envelope = audit declaration, settings = plugin lookup name),
> and the Host pins `Secret` to the envelope name; pack uses `-modfile` for the independent module to merge
> the require/go.sum closure (`-mod=mod` completes the merge in a temporary directory), with no writes to the real
> go.mod/go.sum. Remaining: CH-C4-N1 (no one enforces the 4096-rune outbound limit),
> CH-C4-N2 (EnsureSession concurrency tests).
