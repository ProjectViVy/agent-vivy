# Verification — composer single primary + child transcript leak

## Environment and deviations

- macOS 27 arm64, Node 24.18, pnpm. The host `go` is 1.27, so every Go entry
  point ran with `GOTOOLCHAIN=go1.26.4` (1.27 cannot compile the pinned
  `grpc@v1.64.1`, `undefined: http2.TrailerPrefix`). This also applies to the
  UI's `stage-ui-assembly` pre-step, which shells out to `go`.
- `just ci` cannot finish on this host: `fmt-check` and `plugin-ci` invoke
  `powershell`, which is absent (`sh: powershell: command not found`, exit 127).
  Each gate was therefore run individually with the same intent (commands
  below). The full ordered `just ci` run stays a Linux/Windows gate.
- The user's split pair already held `127.0.0.1:8787` / `127.0.0.1:3015`, so the
  smoke used the same binaries with different ports: backend `8890`
  (`VIVY_USER_HOME=/tmp/vivy-smoke-house`), Vite `3016`
  (`VIVY_BACKEND_ADDR=http://127.0.0.1:8890`), and a second non-frozen backend
  `8891` / Vite `3017` for the settings-persistence check. Nothing in the user's
  pair was touched.
- No provider key: the backend ran with the frozen env provider
  (`VIVY_PROVIDER`, `VIVY_API_BASE`, `VIVY_MODEL`, `DEEPSEEK_API_KEY`) against a
  local OpenAI-compatible mock on `127.0.0.1:9911` (outside the repository).
  Frozen settings are read-only by design (`ReadOnly = SettingsPath == "" ||
  Frozen`), so `busy_enter` persistence was checked on the second, non-frozen
  backend. Persona was initialized once (IDENTITY.MD r1) before the first run.

## Backend

| Command | Result |
| --- | --- |
| `GOTOOLCHAIN=go1.26.4 go test ./internal/app/settings ./internal/runtime ./internal/rpc -count=1` | pass — settings 1.1s, runtime 69.5s, rpc 20.2s (re-run after the `settings/update` echo fix) |
| `GOTOOLCHAIN=go1.26.4 go test -timeout 35m ./...` | every package passes except `agent-vivy/sdk/internal`, see note |
| `GOTOOLCHAIN=go1.26.4 gofmt -l internal cmd sdk/tui` | empty |
| `GOTOOLCHAIN=go1.26.4 go vet ./internal/rpc ./internal/runtime ./internal/app/settings` | clean |
| `GOTOOLCHAIN=go1.26.4 go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui` (headless-compile) | pass |
| `node --test scripts/ensure-laputa.test.mjs` (bootstrap-test) | 10 pass, 0 fail |
| `go vet ./...` + `go test ./...` in each of the 18 `plugins/**` and `faces/**` modules (plugin-ci equivalent) | pass |

`sdk/internal` note: the first run failed on module downloads timing out against
`proxy.golang.org`; re-running that package with
`GOPROXY=https://goproxy.cn,direct` left only D13
`TestMinimalCausalExchange` failing with `gateway did not confirm the requested
workspace binding`. The comparison is an absolute path equality in
`examples/minimal/bind-workspace/main.go`, and macOS resolves `t.TempDir()` to
`/var/...` while the gateway reports `/private/var/...`. It is a host
(path-symlink) limitation, does not touch this change, and is unchanged from
before it.

## UI and SDK

| Command | Result |
| --- | --- |
| `cd ui && GOTOOLCHAIN=go1.26.4 pnpm typecheck` | clean |
| `cd ui && GOTOOLCHAIN=go1.26.4 NODE_OPTIONS="--localstorage-file=/tmp/vivy-vitest-localstorage" pnpm test` | 87 files, 706 tests pass (re-run after "Send queued now" and its switch-away handling) |
| `cd ui && GOTOOLCHAIN=go1.26.4 pnpm build` | pass (embedded asset build) |
| `cd sdk/ui && pnpm install --offline && pnpm typecheck && pnpm test` | clean; 29 tests pass |
| `node --test scripts/check-i18n-cross-face.test.js` | 8 pass |
| `node scripts/check-i18n-cross-face.js` | PASS: 13 shared semantic units |
| `node scripts/check-i18n-completeness.js` | PASS: en=1590 keys / 163 placeholders, zh the same, runtime copy audit clean |

The `NODE_OPTIONS` shim is pre-existing (`UI-VITEST-NODE25`): Node 24.18 ships
no global `localStorage` for happy-dom. `sdk/ui` is consumed through a pnpm
`file:` copy under `ui/node_modules`, so `pnpm install --offline` is required
after editing `sdk/ui/src` before the UI typecheck reflects it.

## Real-path smoke (split pair, ports 8890 / 3016)

Composer:

- Idle: exactly one primary button (Send). Enter sends; the mock answers and
  the run completes.
- Running with an empty draft: the primary button is Stop (`Cancel run`).
  A second Escape (no queue) cancels the run.
- Running with a draft: the primary button reads **Queue** and Enter queues.
  The queue pill shows the item under the `queued` lane with per-item remove and
  Clear queue; the transcript stays clean and the running turn is not
  interrupted.
- `Cmd+Enter` on the same draft steers: the pill shows it under the `steer`
  lane; `Cmd+Enter` is the inverse of the plain gesture.
- Clear queue returns the queued text to the composer.
- With queued items, `Escape` clears the queue and leaves the run alone; the
  next `Escape` cancels.
- No lightning (send-now) or clock (follow-up) send button exists.

Settings:

- Settings → General → "Send while running" renders Queue/Steer with the stored
  value and "In force now: …".
- On the frozen backend the radios and Save are disabled with the read-only
  note, as designed for a frozen settings file.
- On the non-frozen backend (8891/3017) switching to Steer and saving flips the
  composer label to Steer immediately; after a backend restart and a page
  reload the stored mode is still Steer; switching back to Queue behaves
  symmetrically. `settings.yaml` contains `busy_enter: steer` / `queue`
  accordingly.

Child transcript:

- Fresh session, one message: `VIVY-AGENT delegate once and finish`.
- Parent timeline shows the user turn, one `Tool call agent · …` card, and the
  parent's final answer. Expanding the card shows the task input and the child's
  returned summary. No child user/assistant bubbles appear.
- Storage confirms the split: the session stores positions 1–4 only, all owned
  by the primary run (`user`, assistant tool call, tool result, final answer).
  The child run `child_…` keeps its own journal
  (`child.requested`, `child.started`, `model.requested`, `model.delta`,
  `model.completed`, `child.completed` with the summary), which is what Run
  Inspector reads, so the child stays auditable.

## Defect found during the smoke (fixed)

After Save the card showed "Send mode saved" but both radios appeared
unchecked and "In force now: Queue": `settings/update` built its response
without `BusyEnter`, so the client received `busy_enter: ""`, and `?? 'queue'`
does not treat `""` as absent. Fixed by echoing the effective value from
`updateSettings`, normalizing an empty stored value in the settings card,
`settingsUpdateFrom`, and the `ChatView` selector, and asserting the echo plus
the empty-value default in `internal/rpc/control_test.go` and
`ComposerSettingsCard.test.tsx`. The browser checks above were re-run after the
fix.

One run in the smoke ended `failed` before reaching the provider (the mock log
showed no request) with `message 4: tool result answers a call that does not
precede it`. It ran in a session my earlier looping mock had filled with 2,067
messages, all reusing the same tool-call id; the wire-contract guard now rejects
that malformed history instead of sending it. A fresh session with unique ids
completed as described above, so this is a smoke-harness artifact, not a
product regression.

## Real-path smoke: "Send queued now" (split pair, ports 8890 / 3016)

Added after the first cut; the same disposable home, mock and ports as above,
with the user's pair untouched.

- Idle: one Send button (disabled on an empty draft), as before.
- A hanging run (`VIVY-HANG`) is active; plain Enter on "queued one" and
  "queued two" puts two items on the pill under the `queued` lane, with
  **Send queued now** immediately left of **Clear queue**. The composer is empty.
- Clicking Send queued now: the running turn ends `cancelled`, the pill
  disappears, the composer stays empty (the cleared texts are not restored), and
  the next run starts immediately with "queued one"; "queued two" is admitted as
  its follow-up and both complete.
- Journal (read-only sqlite): runs in order `cancelled`, `completed`,
  `completed`; message rows are `user VIVY-HANG primary run`, `user queued one`,
  `assistant ack: queued one`, `user queued two`, `assistant ack: queued two` —
  original order, no duplicates, and the cancelled turn keeps only its user row.
- Repeat with the inverse gesture: `Cmd+Enter` puts "steer me" on the `steer`
  lane; Send queued now cancels that run and re-sends "steer me" as a new run
  (journal: `cancelled`, `completed`, with `user steer me` / `assistant
  ack: steer me`), so lane order is preserved through the flush.
- Browser console clean (only Vite/React devtools lines); backend log has no
  `error`/`warn` beyond the expected `config.yaml not found` and the
  `VIVY_ADDR` override.

Coverage for the paths a browser could not reach cheaply: `store.test.ts`
asserts the idle-queue flush (no `run/cancel` call), the local-FIFO item with an
attachment staying on the local FIFO, and the `cleared`-reason suppression of
the composer restore. `ChatInput.queue.test.tsx` asserts both pill buttons exist
exactly once and that the button calls the action.

Switch-away race (found while reviewing the flush, fixed with tests): if the
user selects another session between the clear and the re-send, the texts were
already gone from the server and the early return left `runBusy` true for the
new session. `selectSession` now resets `runBusy`, and the flush re-dispatches
the collected messages to the original session (`turn/start` then
`turn/follow_up`, degrading to follow-up when a run is still active); attachment
or reference items, which only the face-local FIFO can carry, are buffered and
merged back when the user returns to that session, draining immediately if it is
idle. Covered by `redispatches the flush to the original session when the user
switches away` and `keeps attachment items of an interrupted flush until the
session returns`.

## Not run

- Ordered `just ci` (no PowerShell on this host) and `just kernel-ci` /
  `just wiring-ci` (need an independent Linux-filesystem checkout with its own
  pinned Laputa; not available here). The individual gates above stand in for
  `ci`; the kernel lanes remain open.
- `pnpm e2e` (Playwright) — not exercised.
- A live provider run (no key; local mock only) and a continuable-child UI
  walkthrough (covered by `TestSessionViewMessagesDropsOnlyOneShotChildRows` and
  the existing Run Inspector reconciliation path).
