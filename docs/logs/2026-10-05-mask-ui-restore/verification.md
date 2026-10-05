# Verification

All required gates passed against frozen executable source.

## Environment

Go 1.26.4, just 1.40.0, PowerShell 7.5.4, pnpm 11.19.0, and the locked UI
dependencies were used. The repository's npm mirror is unavailable in this
environment; a temporary PATH wrapper selects `registry.npmjs.org` for
installation without changing repository configuration. Go archives obtained
from upstream repositories were normalized to module zips and checked against
the exact `go.sum` hashes. Temporary tools and state are outside committed
source. Browser verification uses system Chromium through a temporary
Playwright config.

## Regression evidence

Before the fixes, React integration probes failed for a late session-A write,
independent page/header state, and writable builtin details. A subsequent
unavailable-selection test failed because the persisted ID was shown as
unmasked. The final Module suite has 14 tests covering these behaviors,
conflict retry, pagination, focus refresh, draft preservation, and the
same-session refresh/write race. It is included in the standard UI gate.

An independent read-only reviewer identified the pending-write race and
unavailable-selection presentation issue; both were corrected and rechecked.
The review also caught checkout-only CSS scanning; Vite now derives the
scan path from the active Assembly entry.

## Commands

- `just ci` — passed, exit 0. UI: 71 test files and 530 tests, including
  the 14 Module regressions. Includes formatting, UI typecheck/tests/
  bundle, bilingual completeness and cross-face checks, Go vet/tests,
  headless compilation, and independent plugin checks.
- `go run ./sdk verify plugins/vivy-masks-ui` — passed.
- `go run ./sdk pack --recipe recipes/masks-selected.vivy.yml --output .workspace/mask-verify/isolated-final`
  — passed; checkout Module staging was temporarily moved out
  of the source directory to prove the packed CSS uses isolated Assembly
  staging.
- `go run ./sdk inspect-artifact .workspace/mask-verify/isolated-final`
  — passed. Generation: `f6b214cc8727f233c77f9a869b60db6e8e8f49701579ca9ef1466df978741c60`.
  The selected first-party UI provider is T1 and pins source
  `57acc9881647d147ce1b517307bef503699ddceb21e2a81a9a47f8ec2e61bff5`.
  Packed CSS contains the library/editor grid and compact selector sizing.
- `pnpm --dir ui exec playwright test --config .playwright-mask-restore.config.ts`
  — passed, 7/7 stories (no skips). Uses a packed backend, disposable state,
  a hanging
  local provider, and the real split Vite server on port 3015. Seven stories
  cover library rendering, persistence, custom CRUD/conflicts/delete refusal,
  independent code mode, active-run choice, synchronized duplication, and
  mobile composer bounds. The temporary config selects `/usr/bin/chromium`;
  the committed config and spec work with the standard Playwright browser.
- `git diff --check` — passed.

Early CI runs encountered unavailable package mirrors and source-digest
changes while fixes were still being edited. The final gate uses a frozen
source tree. Some early active-run browser attempts timed out during full
SDK compilation; the isolated seven-story browser run subsequently passed.
No live provider or production data is used.
