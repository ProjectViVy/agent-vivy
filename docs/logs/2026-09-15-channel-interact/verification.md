# Verification — channel interact

All commands run in the `agent-vivy-channel-interact` worktree (base
`029f6cc` + five commits). The final rebase + full-gate re-run happens at
landing and is recorded in the landing commit.

## Per-commit checks

- `go build ./...` / `go vet ./...` per touched module — clean. Note: a
  fresh worktree needs `ui/dist` for the `internal/app` embed; a local
  gitignored placeholder (or the `pnpm build` below) provides it.
- `go test ./plugins/telegram` — ok (7.0s): edit HTML + plain-text
  fallback, `message is not modified` = success with no retry, empty
  payload rejected, delete carries chat+message ids, placeholder fixed
  copy + id, not-started faces fail closed.
- `go test ./plugins/discord` — ok (0.3s): edits/deletes/placeholder land
  on the never-opened send client and never on an ear session, empty
  payload rejected, fixed placeholder copy + id, fail-closed faces.
- `go test ./plugins/feishu` — ok (3.4s): card shape (schema 2.0, one
  markdown element), 11310 → plain-text fallback (2 calls, ids kept),
  other codes surface with no fallback, Patch edit content, delete,
  card placeholder, ack pool discipline (default/empty/trimming),
  react→withdraw by reaction id, disabled pool touches nothing,
  fail-closed faces.
- `go test ./internal/channelhost ./internal/app ./internal/rpc -count=1`
  — all ok (86s/88s/113s) including the live-surface matrix:
  - placeholder + ack at an accepted turn; exactly one armed ledger row
    (the live surface rides no row);
  - completed terminal → placeholder deleted + ack withdrawn + fresh
    delivery + zero open rows;
  - failed and cancelled terminals settle the same way with no sends;
  - approval-required leaves the surface untouched (waits), the following
    completed terminal settles it;
  - StopAll sweeps placeholder and ack without any terminal;
  - the 10-minute TTL (shrunk) ends an orphaned run's surface;
  - a channel without the faces gets no live entry and delivery is
    unchanged.
- P9 rotation: `go run ./sdk/internal/cmd/source-hash` fixed points
  verified for internal + telegram + discord + feishu after writing the
  new pins; `TestChannelProvidersAdvertiseExactlyTheirAdapterSurface` ok
  (flipped pins).
- `go test ./sdk/internal/conformance -run TestCheckedInProviderConformance
  -count=1` — ok (139s). First run failed only on the UI fixture suites
  (fresh worktree, no `ui/node_modules`), fixed by
  `pnpm install --frozen-lockfile && pnpm build` exactly as the gate-0 log
  prescribes; second run green.

## Full gate

- `just ci` — green (exit 0, no recipe failures) after two rounds:
  round 1 failed at `fmt-check` (gofmt drift in
  `plugins/telegram/plugin_test.go` and `sdk/port/channel/channel.go`
  from this batch, plus a pre-existing blank-line drift in
  `internal/channelhost/host_test.go` carried by the base commit) and at
  the generation failure matrix (the gofmt rewrite had shifted the
  telegram + internal hashes past the rotation). Both fixed: formatting
  applied, digests re-rotated (telegram `…→32b8f1c2…`, internal
  `c45ae681…→182bc905…`, fixed points re-verified), P9 gate re-run green
  (104s), round 2 fully green.

## Not run, and why

- **Real-platform smoke (telegram/discord/feishu)**: no bot tokens in this
  environment. Skipped honestly, following the channel-hardening log
  precedent; every behavior above is asserted on loopback stubs, and the
  adapters' platform calls are the pinned SDKs' own surfaces.
- **Browser UI smoke at :3015**: no UI change in this batch; the settings
  write path was verified by code reading (pointer-semantics patch, no
  settings-block write).
