# Verification — UI overhaul Block 0

Host: macOS arm64. Tree: `.worktrees/ui-overhaul/agent-vivy` on
`feat/ui-overhaul`, based on `stable@033a57f6`. The root checkout has unrelated
Go work in it, so this lane runs in its own worktree.

## Environment prerequisites this lane needed

| Problem | Fix used |
| --- | --- |
| `go run ./cmd/vivy` cannot resolve `../laputa/garden` from the worktree | symlink `.worktrees/ui-overhaul/laputa -> /Users/mastwet-macmini/pjvv/laputa` (module `replace` targets a sibling of the repo root) |
| `ui/embed.go` needs `dist/` and a fresh worktree has none | `pnpm build` before the first `go build`/`go run` |
| ambient Go 1.27.1 breaks pinned `grpc v1.64.1` (`undefined: http2.TrailerPrefix`) | every UI script that shells out to Go is run as `GOTOOLCHAIN=go1.26.4 pnpm …`; `pretypecheck`/`pretest`/`prebuild`/`predev` all call `stage-ui-assembly.mjs`, which builds Go |
| ports 8787, 3015 and 8798 already held by the owner's instances and another checkout | isolated pair on 8796 (backend) + 3016 (Vite); the owner's processes were never touched |

Isolated pair used for every browser observation:

```text
GOTOOLCHAIN=go1.26.4 VIVY_ADDR=127.0.0.1:8796 VIVY_USER_HOME=/tmp/vivy-ui-overhaul \
  go run ./cmd/vivy                                   # /tmp/vivy-backend-8796.log

VIVY_BACKEND_ADDR=http://127.0.0.1:8796 \
  pnpm exec vite --port 3016 --host 127.0.0.1 --strictPort   # /tmp/vite-3016.log
```

Isolation confirmed before capture: `/tmp/vivy-ui-overhaul/vivy.db` contains 0
sessions, the Vite process environment carries `VIVY_BACKEND_ADDR=http://127.0.0.1:8796`,
and the audit manifest records `baseUrl: http://127.0.0.1:3016`. No repository
`data/` path was read or written.

## Gates

```text
cd ui && GOTOOLCHAIN=go1.26.4 pnpm typecheck    # ok: "tsc --noEmit" with no output after the stage step
cd ui && npx vitest run src/theme/tokens.test.ts # 7 passed
```

Ratchet output (the counts the baseline now freezes):

```text
[token-ratchet] src/palette: 159 (baseline 159); 31 files (baseline 31)
[token-ratchet] src/textSize: 85 (baseline 85); 26 files (baseline 26)
[token-ratchet] src/hex: 21 (baseline 21); 2 files (baseline 2)
[token-ratchet] plugins/palette: 29 (baseline 29); 7 files (baseline 7)
[token-ratchet] plugins/textSize: 45 (baseline 45); 8 files (baseline 8)
[token-ratchet] plugins/hex: 0 (baseline 0); 0 files (baseline 0)
```

The ratchet was also checked against a deliberately widened input: after adding
`white`/`black` to the palette pattern the count rose from 150 to 159 and the
file set from 26 to 31, so the baseline was raised once to the measured value
before this commit. From here it only goes down.

### Full suite, recorded as not green and not caused by this change

```text
cd ui && GOTOOLCHAIN=go1.26.4 pnpm test
# 656 passed, 45 failed across 9 files
```

Every failure is the same shape: `TypeError: localStorage.clear is not a
function` (and siblings) because Node v24.18.0 on this host exposes a global
`localStorage` that shadows the per-file happy-dom window. This matches open
row `UI-VITEST-NODE25` in `docs/TODO.md`, which reproduced 45 failures in 9
files on an untouched root checkout on 2026-10-24. No Block 0 file is imported
by those suites; `tokens.test.ts` passes in isolation and in the full run.
This is reported as a pre-existing red, not as a pass.

## Screenshot baseline

```text
cd ui && pnpm run audit -- --base-url http://127.0.0.1:3016 \
  --out .audit/baseline-2026-10-25
# audit: 120 screenshots, 0 failures -> ui/.audit/baseline-2026-10-25
```

120 = 10 surfaces × 4 persisted themes × 3 widths (390 / 768 / 1280). Themes
are the four storage ids in use today: `default` (light), `pink` (light),
`dark` (dark), `miku` (dark). Surfaces: chat empty, settings, dashboard,
approvals, MCP, skills, cron tasks, lifecycle, welcome wizard, and the nav
sheet opened from the header button.

`.audit/` is gitignored, so six representative frames are committed in this log
directory: chat-empty at 390 (Miku), 768 and 1280 (Vivy light), 1280 (Vivy
dark), and settings at 390 (Vivy light) and 1280 (Diva). The refresh command is
the one above; nothing in the baseline depends on hidden state beyond the
isolated `VIVY_USER_HOME`.

Browser fallback note: the Playwright browser cache is absent on this host, so
`launchBrowser()` falls back to the installed Google Chrome
(`chromium.launch({ channel: 'chrome' })`). `pnpm run audit` therefore needs a
working Chrome, not a Playwright download.

## Not run, and why

- `just ci` and `just wiring-ci` / `just kernel-ci`: they need an independent
  Linux-filesystem checkout; not available on this macOS host.
- Real-provider streaming, approval and queue screenshots: no provider key in
  this lane. They arrive from Block 3 with the frozen-env provider described in
  `testing-vivy-ui`.
- Studio (`just studio`) is untouched by this block and its submodule is not
  part of this worktree.
