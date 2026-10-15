# Verification: settings thinking-level crash (2026-10-15)

## Reproduction (before the fix)

The split pair was running (backend `127.0.0.1:8787`, Vite `127.0.0.1:3015`,
scratch home `.workspace/dev-home`). A real browser session
(`chrome-headless-shell` + CDP) walked `/`, `/dashboard`, `/settings` with
`vivy.ui.welcome.completed=1` (returning user, so the first-run wizard does not
hide the shell). `/settings` produced the reported panel; the console captured
the component and the frame:

```text
TypeError: Cannot read properties of null (reading 'length')
    at ThinkingSettingsCard (…/src/components/settings/ThinkingSettingsCard.tsx)
The above error occurred in the <ThinkingSettingsCard> component.
React will try to recreate this component tree from scratch using the error
boundary you provided, PresentationErrorBoundary.
```

`/` and `/dashboard` were clean, so the failure is route-scoped, not a general
shell defect.

## Commands run

| Command | Result |
|---|---|
| `go test ./internal/rpc/ -run Thinking -count=1` | PASS (8.485 s) |
| `pnpm vitest run src/components/settings/ThinkingSettingsCard.test.tsx` (in `ui/`) | PASS — 2 tests |
| `just ci` (repo root) | FAIL — host privilege only; see below |

`just ci` lanes that passed on this tree: `ensure-laputa` + `bootstrap-test`
(11 tests), `fmt-check`, `ui-ci` (`tsc --noEmit`; 692/692 Vitest tests in 92
files including the new one; `vite build`; i18n completeness + cross-face),
`vet`, and the ordinary Go test lane for every package — including
`internal/rpc` (98.5 s), `internal/app`, `internal/runtime`, `internal/storage`.
The recipe then failed in the SDK pack packages: `sdk/internal`,
`sdk/internal/assembly`, `sdk/internal/conformance`. Every failure in those
packages is one of two host conditions, never an assertion:

```text
symlink … : A required privilege is not held by the client
open C:\Users\com01\AppData\Local\go-build\…-d: Access is denied
```

Both are environmental: those tests build isolated UI/Go boundaries through
Windows symbolic links, and the nested `go build` writes the shared build cache.
An independent probe in the same shell confirms the privilege is absent:

```text
PS> New-Item -ItemType SymbolicLink …
symlink denied: 此操作需要管理员权限。
PS> whoami /priv | Select-String 'SymbolicLink|SeCreate'
(no output — the token does not hold SeCreateSymbolicLinkPrivilege)
```

The same tree passed a complete `just ci` (exit 0) earlier on 2026-10-15 in a
session that held that privilege (`docs/logs/2026-10-15-laputa-persona-authority/verification.md`),
so this is a limitation of the current confined shell, not a regression from
this change. No source file touched by this fix is in those packages.

## Real-path check (after the fix)

Split pair restarted with the rebuilt backend, then the same browser walk with
the fixed UI guard applied to the *unmodified* backend first (it still answered
`levels: null`) — `/settings` already stopped crashing, which isolates the
render-side guard. After the backend rebuild, with both halves in place:

```text
crash panel: absent
thinking card unsupported state: true
thinking card title present: true
errors in console: 0
```

Screenshot: `.workspace/tmp/settings-after-fix.png` (scratch, gitignored).

## Not verified here

- `just ci` end to end on a shell that holds `SeCreateSymbolicLinkPrivilege`
  (needs an unconfined terminal; the owner can run it).
- PostgreSQL side of the thinking verb: the change is in the RPC projection, not
  in storage, and no storage or schema path is touched.
