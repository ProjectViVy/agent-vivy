# Upstream thinking-level null fix and compact-control merge

Date: 2026-10-20
Base: `2bb3aaba` (`main`: clean vivy-code parity line plus the provider catalog
prune, provider-agentic migration, and the welcome-wizard model step).

## What happened

Opening Settings on the clean line replaced the whole frame with
`Vivy UI Module failed` / `UI root component failed: Cannot read properties of
null (reading 'length')`. The failing component was `ThinkingSettingsCard`:
`model/thinking/levels` returned JSON `null` for a model that declares no
thinking levels, while the wire contract and `ui/src/lib/api.ts` declare an
array, and the card reads `levels.length` on every render.

The upstream old line already had the fix and a related composer improvement:

- `8fceacc4` `fix(settings): answer an empty thinking-level list instead of null`
  normalizes the RPC reply to `[]`, coalesces a malformed UI reply, and pins
  both paths with RPC and component tests.
- `536b9f59` `feat(ui): make the context meter the compact control` merges the
  separate Shrink button into the context-usage meter: when the session reports
  `compaction_enabled`, the meter is the popover trigger for the compact dialog;
  otherwise it stays a passive progressbar.

Those commits lived on the pre-split line (`feat/context-compact-merge`).
Upstream now preserves the old history only under the `remake/*` branch family,
and the local `main` deliberately excludes that bloated line. Merging the whole
line would reintroduce the discarded history, so the two commits were
cherry-picked onto the clean base:

```text
37dbbdd3 fix(settings): answer an empty thinking-level list instead of null
71490441 feat(ui): make the context meter the compact control
```

## Files

- `internal/rpc/control.go`, `internal/rpc/control_thinking_test.go`
- `ui/src/components/settings/ThinkingSettingsCard.tsx`
- `ui/src/components/settings/ThinkingSettingsCard.test.tsx` (new)
- `ui/src/components/chat/ChatInput.tsx`
- `ui/src/components/chat/ChatInput.compact.test.tsx`
- `ui/e2e/runtime.spec.ts`
- upstream records: `docs/logs/2026-10-15-settings-thinking-levels-null/`,
  `docs/logs/2026-10-17-context-meter-compact-merge/`
- `docs/TODO.md`: the upstream `DEV-LEASE-RESTART` and
  `UI-ROUTE-ERROR-BOUNDARY` rows are retained.

## Scope and decisions

- Cherry-pick, not a branch merge: the clean line keeps its linear history and
  does not absorb the old `remake/*` donor history.
- No Go architecture, provider, schema, or migration change; the RPC change only
  normalizes an existing response shape.
- No push.

## Explicitly not done

- Full `just ci` was stopped by the owner before this merge and was not restarted.
- The upstream `remake/*` donor branches were not merged wholesale.
- No route-level error boundary was added; it remains `UI-ROUTE-ERROR-BOUNDARY`
  in `docs/TODO.md`.
