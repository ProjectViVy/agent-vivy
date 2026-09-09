# Verification — 2026-08-31 default-full-channel-body

All commands were run in worktree `../agent-vivy-full-channels` (branch
`feat/default-full-channels`).

## Build

| Command | Result |
|---|---|
| `go build ./...` | OK (requires `ui: pnpm build` first to produce the embedded dist) |
| `go build -o vivy-sdk.exe ./sdk` | OK |
| `vivy-sdk verify plugins/telegram` | ok (the other 4 channels are covered by verify inside the SDK test suite) |
| `vivy-sdk pack --with telegram --out dist/pack-smoke` | Before the fix: `conflicting replacements for example.com/vivy/plugins/telegram` (reproduced the idempotence gap); after the fix: passed as part of the `sdk/internal` test suite |

## Gates

| Command | Result |
|---|---|
| `just ci` (fmt-check + vet + test + headless-compile + ui-ci) | **ultimately all green (exit 0)** |
| `go test -count=1 ./...` | One `TestCronAtJobDeletesAfterSuccessfulRun` (internal/runtime) timed out after 6.15s; two isolated reruns were green (0.66s / 12.8s), judged to be an existing timing-sensitive flake unrelated to this change → `docs/TODO.md` §0.1 `TFLAKE-CRON` |
| `just test` (two reruns) | all green |
| `go test ./sdk/...` | all green (including the new idempotence/parseReplaceTargets tests and real pack build) |

## Real-path smoke test (browser, split Vite)

- Worktree backend: `VIVY_CONFIG=data/smoke/config-smoke.yaml ./dist/smoke-vivy.exe`
  → listening on 127.0.0.1:**8788** (8787 was occupied by the user's existing root backend, so the port was changed),
  with 5 startup log entries `channelhost: channel compiled-in but not configured; not started`
  (one each for telegram/dingtalk/discord/feishu/qq).
- Worktree Vite: `VIVY_BACKEND_ADDR=http://127.0.0.1:8788 pnpm exec vite --port 3016`.
- The browser (IAB) opened `http://127.0.0.1:3016/settings?tab=channels`:
  - The Channels page rendered 5 cards: DingTalk / Discord / Feishu / QQ / Telegram, all showing the literal `Disabled · no config envelope`, with `Enable` / `Edit` / `Close ears` controls;
  - the **`This generation has no ears` empty state did not appear**;
  - screenshot:
    `C:\Users\Administrator\.zcode\cli\artifacts\sess_1c6805f3-d9ba-441a-a6da-29fc6540ad45\call_a48084bf968d4b8d964ed41b-tool-result-a7404cd1-14b9-4114-887f-ce69ca692872.png`
- After the smoke test, the 8788 backend and 3016 Vite were stopped and the ports were released.

## Air gap and isolation

- The smoke configuration `data/smoke/config-smoke.yaml` pinned `data_dir`/`sqlite.path`
  to worktree `data/smoke/home` and did not touch the repository-root `data/`.
- The first smoke attempt omitted `VIVY_CONFIG`; before the process exited after failing to bind 8787, it opened the default
  data root `~/.vivy` (the directory already existed on 2026-08-30 as an existing development data root; this run only
  performed idempotent migrations/read queries, and the missing-cron-table warning matched the existing state since 8/30). See notes.

## Commit

- Branch `feat/default-full-channels` has one commit; `ui/src/routeTree.gen.ts`
  contained only dev-server line-ending noise and was restored, so it was not included in the commit.
- Fast-forwarded back to `main`; not pushed.
