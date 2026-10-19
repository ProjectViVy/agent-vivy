# Verification

## Passed

- `git diff --check`.
- `node scripts/check-i18n-completeness.js`: en/zh each have 1565 keys and
  162 placeholders; runtime copy audit clean. Baseline was 1569 keys and
  163 placeholders before deleting the four exclusive keys.
- `node --test scripts/check-i18n-cross-face.test.js`: 8 tests passed.
- `node scripts/check-i18n-cross-face.js`: 14 shared semantic units conform.
- Supplemental direct test command from `ui/`:
  `pnpm exec vitest run src/components/chat/CodeModeControl.test.tsx src/components/chat/ChatView.test.tsx src/i18n/index.test.ts`:
  3 files / 24 tests passed. This command bypasses the blocked assembly
  pre-hook and is not a substitute for the product gate.
- A source search found no remaining `ConversationModePlaceholder` import,
  mount, or deleted translation-key reference in authoritative UI/plugin
  sources or scripts.

## Incomplete required gates

- `just ci`: exit 127, `just: command not found`. This host also lacks Go
  and PowerShell, which the repository gate requires.
- `pnpm --dir ui test -- src/components/chat/CodeModeControl.test.tsx src/components/chat/ChatView.test.tsx`:
  the mandatory staging pre-hook failed with `spawnSync go ENOENT`.
- `pnpm dev` from `ui/`: failed in that same staging pre-hook. The required
  real-path split UI smoke at `http://127.0.0.1:3015` could not run.
- `pnpm --dir ui exec playwright test --config playwright.masks.config.ts --grep 'chat header keeps the selected mask'`:
  stopped before test execution, exit 127, because the packed masks binary
  `.workspace/mask-verify/selected/vivy` is absent. The existing acceptance
  test was updated before removal, but no red/green browser result is claimed.

## Supplemental full-suite result

One direct `pnpm exec vitest run` completed: 78 files / 558 tests passed;
3 suites failed to load because the unavailable Go staging step had not
created `src/generated/ui/vivy-persona/src/index.tsx`:

- `src/plugins/conformance.test.tsx`
- `src/plugins/presentation-host.test.tsx`
- `src/components/chat/ConversationSidebar.test.tsx`

`src/components/demo/TokenStatsPanel.test.tsx` passed with existing React
`act(...)` environment warnings. No generated assembly or staged module
source was hand-edited to bypass these failures. The suite was not repeated.

Dependencies were temporarily reused through a local `ui/node_modules`
symlink; that symlink and disposable Playwright output were removed at
handoff. The worker did not commit or publish; the integrator owns delivery.

## Integration verification

After applying the exact worktree patch to the delivery branch, the integrator
reviewed all six code/test/catalog diffs and reran the i18n completeness audit,
eight cross-face tests and 14-unit cross-face check: all passed. The supplemental
direct command `node node_modules/vitest/vitest.mjs run
src/components/chat/CodeModeControl.test.tsx src/components/chat/ChatView.test.tsx
src/i18n/index.test.ts` from `ui/` passed all 24 tests. Full product and browser
gates remain incomplete as stated above; this does not establish installed behavior.
