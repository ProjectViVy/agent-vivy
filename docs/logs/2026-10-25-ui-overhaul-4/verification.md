# Block 4 — verification

All commands run in the `feat/ui-overhaul` worktree
(`/Users/mastwet-macmini/pjvv/agent-vivy/.worktrees/ui-overhaul/agent-vivy`).
Go-invoking UI scripts need the pinned toolchain on this host:
`PATH=$HOME/.local/bin:/usr/local/go/bin:$PATH GOTOOLCHAIN=go1.26.4`.

## Code gates

```text
cd ui && pnpm typecheck
# exit 0 (stage-ui-assembly first; ui/src/generated unchanged — git diff empty)

cd ui && pnpm vitest run src/components/chat/ChatInput.queue.test.tsx \
  src/components/chat/ChatInput.approval.test.tsx \
  src/components/chat/ChatInput.commands.test.tsx \
  src/components/chat/ChatInput.compact.test.tsx \
  src/components/chat/ChatView.test.tsx
# 5 files, 45 tests passed — assertions unchanged from before the split

cd ui && pnpm test
# 88 files, 710 tests: 709 passed, 1 failed
#   src/lib/store.test.ts > keeps the run.failed payload as the conversation error
#   pre-existing red, tracked as STORE-RUN-FAILED-COPY in docs/TODO.md §0.1

cd ui && pnpm build
# exit 0, built in 1.79s (pre-existing chunk-size warning only)

cd ui && pnpm vitest run src/theme/tokens.test.ts
# 7 passed; baselines now src palette 148 / text-size 52 / hex 0,
# plugins 29 / 45 / 0 — both lists only shrank this block
```

## i18n gates

```text
node scripts/check-i18n-cross-face.js        # exit 0 (was exit 1)
node --test scripts/check-i18n-cross-face.test.js   # 8 passed
node scripts/check-i18n-completeness.js      # exit 1, pre-existing only:
#   Uncatalogued Han: ui/src/lib/failure.ts:56-58 (I18N-CHECK-RED, narrowed)
```

## Browser evidence

Isolated split pair, never the owner's `:8787`/`:3015` and never `data/`:

```text
VIVY_ADDR=127.0.0.1:8796 VIVY_USER_HOME=/tmp/vivy-ui-overhaul \
VIVY_PROVIDER=deepseek VIVY_API_BASE=http://127.0.0.1:9913/v1 \
VIVY_MODEL=mock-fast DEEPSEEK_API_KEY=mock-key go run ./cmd/vivy   # :8796
VIVY_BACKEND_ADDR=http://127.0.0.1:8796 pnpm exec vite --port 3016 --host 127.0.0.1 --strictPort
```

`/tmp/vivy-mock-llm.py` (loopback only) streams 40 slow lines for `VIVY-SLOW`
and a `write_file` call for `VIVY-APPROVAL`. The capture
(`ui/.audit/capture-block4.mjs`, gitignored) seeds
`vivy.ui.welcome.completed=1` because a fresh profile auto-opens the welcome
wizard whose backdrop swallows clicks.

`composer-report.json` (this directory) reports, per state and width:

| state | 1280 | 390 |
| --- | --- | --- |
| idle | doc 1280/1280, cols 832/830, toolbar 830/830, textarea 40/40 | doc 390/390, cols 390/364, toolbar 364/364, textarea 64/64 |
| 6-line paste | textarea 136/136 (grew, under the 360px cap) | textarea 160/160 (under the 337.6px cap) |
| 24-line paste | textarea 360/496, max-height 360px → capped, scrolls inside | textarea 338/1168, max-height 337.6px → capped, scrolls inside |
| queue (2 pills) | toolbar 830/830, doc 1280/1280 | toolbar 364/364, doc 390/390 |
| after remove / after clear | queue row gone, no offenders beyond sr-only | same |
| approval card open | doc 1280/1280 | doc 390/390; card content wraps, Approve/Deny/Approval center visible |

Screenshots: `composer-{idle,paste,longpaste,queue,approval}-{1280,390}.png`.
Remaining offenders in the report are `sr-only` spans (1px boxes by design)
and the sidebar session titles / header model name at 1280 — both Block 5
scope, unchanged by this block. Console: no errors.

## Unrun lanes

- `just ci` Go lanes (`bootstrap-test`, `vet`, `test`, `headless-compile`,
  `plugin-ci`): unchanged by a UI-only diff and not runnable on this macOS
  host per `docs/TODO.md` kernel-gate notes; recorded as unrun.
- `pnpm e2e`: red on stable for EMBEDDED-E2E-LOCALE, unrelated to the composer.
