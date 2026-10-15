# Settings crash: `model/thinking/levels` answered `null` (2026-10-15)

## What was wrong

Opening `http://127.0.0.1:3015/settings` replaced the whole app frame with the
root `PresentationErrorBoundary` fallback:

```text
Vivy UI Module failed
UI root component failed: Cannot read properties of null (reading 'length')
```

`ThinkingSettingsCard` renders `levels.length` (and `levels.filter`) from the
`model/thinking/levels` reply. For an active model that declares no thinking
levels — the reported setup is provider `openai` / model `abab6.5-chat`, which
declares none — the handler returned `info.ThinkingLevels` unchanged. That is a
nil slice, so it marshals to JSON `null`, not to the `[]` the verb's own
contract documents ("a non-thinking model reports an empty list") and not to
the `levels: string[]` declared in `ui/src/lib/api.ts`. The first render after
the reply threw and the boundary unmounted navigation plus the route slot.

Reproduced in a real browser against the running split pair; the captured
React error names the component and the frame:

```text
TypeError: Cannot read properties of null (reading 'length')
    at ThinkingSettingsCard (…/src/components/settings/ThinkingSettingsCard.tsx)
… The above error occurred in the <ThinkingSettingsCard> component.
React will try to recreate this component tree from scratch using the error
boundary you provided, PresentationErrorBoundary.
```

## What changed

- `internal/rpc/control.go` — `thinkingLevels` normalizes an absent level
  surface to an empty, non-nil list, so the wire value is `[]`. A
  thinking-capable model with no declared levels still reports the seven-level
  surface; `thinkingReport`'s `supported` field is untouched because
  `ThinkingReport.supported` is declared `string[] | null`.
- `internal/rpc/control_thinking_test.go` — the non-thinking case now pins the
  encoded value (`"levels":[]`, never `null`), not just the Go slice length.
- `ui/src/components/settings/ThinkingSettingsCard.tsx` — the card coalesces a
  malformed reply (`nextLevels.levels ?? []`) so a backend contract leak can no
  longer unmount the app frame. Backend input stays untrusted; the field is
  still rendered from what the backend declares.
- `ui/src/components/settings/ThinkingSettingsCard.test.tsx` — new regression
  test: `levels: null` renders the unsupported state, and the declared-levels
  case still lists the levels plus the three aliases.

## Scope

Two source files, two tests, this record, and two `docs/TODO.md` §0.1 rows for
gaps found but not fixed here:

- `DEV-LEASE-RESTART` — `just dev` restarting inside the 30 s organism lease
  dies with `organism lease held` (only the Studio console waits the TTL out).
- `UI-ROUTE-ERROR-BOUNDARY` — a page-level render error still takes down the
  whole frame because routes and Module pages have no boundary of their own.

## Explicitly not done

- No per-page/per-Module error boundary and no change to the root fallback's
  wording — that is the `UI-ROUTE-ERROR-BOUNDARY` decision above.
- No `dev.ps1` lease wait.
- The `model/thinking` `supported` field keeps its nullable contract.
- No multi-instance / shared-Journal work (the owner's "multiple selves with
  synchronized sessions" direction is a contract decision, not part of this
  fix).
