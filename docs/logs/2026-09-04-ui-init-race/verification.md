# UI-INIT-RACE Verification Record

## Automated verification process

### 1. Frontend unit tests and regression
- **Command**: `pnpm test` (in the `ui/` directory)
- **Result**:
  - Test files: 24 passed (24)
  - Test cases: 201 passed (201)
  - New tests covered:
    - `does not overwrite session created while initialize is in-flight`
    - `does not create redundant default session if user created one during initialize`
    - `throws and records runError when startRun is called with mismatched session`
    - `throws and records runError when editSession is called with mismatched session`

### 2. TypeScript type checking
- **Command**: `pnpm typecheck` (`tsc --noEmit`)
- **Result**: exit code 0, with no type errors.

### 3. Vite production build
- **Command**: `pnpm build`
- **Result**: production bundle built successfully, with assets correctly included in `dist/`.

### 4. Gate verification (`just ci`)
- **Command**: `just ci`
- **Steps included**:
  - `fmt-check`: Go source formatting check passed.
  - `ui-ci`: dependency lockfile check, type check, Vitest run (201/201 green), and Vite build passed.
  - `vet`: backend static analysis passed.
  - `test`: the full backend unit-test suite passed (including internal/app, channelhost, runtime, and others).
  - `headless-compile`: Headless build-tag check passed.
  - `plugin-ci`: checks passed for all plugins and face modules.
- **Result**: exit code 0, CI-EXIT:0.

### 5. Playwright end-to-end specification test (`just ui-e2e`)
- **Command**: `just ui-e2e`
- **Result**:
  - 21 passed, 1 skipped (cron-tasks skipped as expected offline)
  - Specifications involving session creation and settlement, including `chat-act.spec.ts` and `files-panel.spec.ts`, all passed stably.
