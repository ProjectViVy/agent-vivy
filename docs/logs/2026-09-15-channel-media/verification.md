# Verification — channel media (batch 2)

All commands run from the repository root on `feat/channel-tier1`
(Windows, Git Bash). Tests are hermetic: every network-shaped call lands
on a loopback stub; no live platform credentials exist in this
environment.

## Per-commit checks

| Commit | Checks run | Result |
|---|---|---|
| tier-2 absorption (`70aac22`/`5b3d144`/`5e27431`) | `go build ./...`; `go test ./internal/attachment/ ./internal/rpc/ ./internal/channelhost/ ./internal/app/ ./internal/runtime/ -count=1`; `go test ./plugins/telegram/...` | pass |
| Contract (`029f6cc`) | `git diff --check` (editorial) | pass |
| Host media path (`0d5e04e`) | `go test ./internal/channelhost/ -count=1` (incl. new `deliver_media_test.go`: delivery, retry-reupload, degradation, invalid-part drops, Discover Media bit) | pass |
| Discord inbound (`d4e395f`) | `go test ./plugins/discord/... -count=1` | pass |
| QQ inbound (`74be4dd`) | `go test ./plugins/qq/... -count=1`; `-race` rerun | pass |
| Feishu inbound (`2676ca6`) | `go test ./plugins/feishu/... -count=1`; `-race` rerun | pass |
| Telegram albums (`1423c9c`) | `go test ./plugins/telegram/... -count=1`; `-race` rerun | pass |
| Discord outbound (`01a2c38`) | `go test ./plugins/discord/... -count=1` | pass |
| Telegram outbound (`65733b9`) | `go test ./plugins/telegram/... -count=1`; `-race` rerun | pass |
| QQ outbound (`c9e7298`) | `go test ./plugins/qq/... -count=1`; `-race` rerun (loopback body asserts raw `file_info`, no double base64) | pass |
| Feishu outbound (`6b908fe`) | `go test ./plugins/feishu/... -count=1`; `-race` rerun | pass |
| P9 refresh (`aaefaad`) | `go test ./sdk/internal/conformance/ -run TestCheckedInProviderConformance -count=1` | pass (421 s) |
| Capability matrix (`a0fcef2`) | `go test ./sdk/internal/assembly/ -count=1`; then `go test ./sdk/internal/... -count=1` | pass |

## P9 digest refresh

Two-pass source-hash flow per moved source (four ears + `internal/`):
compute with the old declared value, pin the new digest at the fixed
points (`vivy-module.yaml`, `module_v1.go`,
`reproduction_test.go` `releaseSuiteCases` / `internalSHA`,
`conformance_results.json`), then re-hash with the new declared value to
confirm the fixed point. One environment repair was needed first: this
checkout's `ui/node_modules` was missing the checked-in Vite tool the
SDK pack test builds with — `pnpm install` in `ui/` repaired it (the
`pnpm-lock.yaml` diff this produced was left unstaged; it is tooling
state, not part of this batch).

## Full gate

- `just ci` — **green** on the branch head (`a0fcef2`, exit 0). The first
  run caught one extra pin: the sdk assembly capability-matrix test still
  expected `Media: false` on the four ears; updated with the commit above
  and the whole gate rerun clean.

## Live-platform smoke

Skipped honestly, matching the C4–C7 / hardening precedent
(`docs/logs/2026-09-14-channel-hardening/verification.md`): no real bot
credentials in this environment. Every media path above is covered by a
loopback stub that serves real bytes, so the download/upload, bound, and
fallback behaviors were exercised for real on the wire level — only the
platform endpoints themselves are stand-ins.
