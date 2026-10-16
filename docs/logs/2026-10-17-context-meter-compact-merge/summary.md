# Chat composer: the context meter is the compact control

## What changed

`ui/src/components/chat/ChatInput.tsx` — the composer's bottom row had two
neighbours: the context-usage display (ring + `%`, passive `role="progressbar"`)
and a separate `Shrink` button that opened a `Popover` with the compact
instructions and `立即压缩`. They are now one control.

- When the session reports `compaction_enabled`, the usage group itself is the
  `Popover` trigger: ring + `%` + a compact affordance icon inside one
  `<button>`. Clicking it opens the same "压缩上下文" window anchored at the
  meter.
- When compaction is unavailable (disabled, or no active session) the meter
  stays exactly the passive `role="progressbar"` display it was; no dead or
  unexplainable grey button is introduced.

Kept as-is: the `context/compact` RPC through `store.compactSession`, the
done/skipped/failed notices, the usage tooltip (`contextTitleText()` → percent,
tokens, bytes, compaction status), the ring/token/color math, and the popover
body (title, hint, instructions field, `立即压缩` / `压缩中…`).

Busy behaviour: the trigger is no longer disabled by the composer's
`disabled`/`running` flags. A run in flight opens the window, which now shows
`chatInput.compactBusy` in place of the old trigger tooltip, and the server's
busy rejection still surfaces in the composer notice — the pre-existing
"clickable while a run is active, error surfaced" contract.

Accessibility: the trigger's accessible name is the sr-only
`chatInput.contextLabel` plus the visible percentage (e.g. "上下文占用 6%"),
and the ancestor `title` keeps supplying the usage tooltip. A nested
`progressbar` would be pruned inside a `button` (ARIA makes button children
presentational), so the `progressbar` semantics remain only on the passive
branch, where they are still accurate.

## Scope

- Web composer only. No SDK Face type, store action, RPC, or `SessionContext`
  change; `ChatInput`'s props are unchanged.
- No i18n catalog change and no `scripts/i18n-cross-face-contract.json` row:
  every rendered string reuses an existing key (`chatInput.contextLabel`,
  `chatInput.contextValueText`, `chatInput.contextTitle`,
  `chatInput.compact`, `chatInput.compactHint`,
  `chatInput.compactInstructionsPlaceholder`, `chatInput.compactNow`,
  `chatInput.compacting`, `chatInput.compactBusy`).

## Tests

- `ui/src/components/chat/ChatInput.compact.test.tsx`: the trigger is now found
  by its usage label; a new assertion proves the usage display lives inside the
  trigger; the compaction-disabled case asserts the passive progressbar stays;
  the instructions / blank-instructions / run-active-busy-error cases are
  unchanged in intent.
- `ui/e2e/runtime.spec.ts`: the old `progressbar` assertion is replaced by the
  merged control, plus a real browser round trip (click → compress window →
  Escape) after the welcome wizard is dismissed.

## Explicitly not done

- Settings → 通用 → 上下文压缩 card and `CompactionSettingsCard`'s busy
  precheck are untouched, as is the TUI face.
- No new copy or state for "compaction disabled": the meter stays a passive
  display.
- No change to the composer's `runBusy` / `disabled` semantics.
- No change to any packing, Recipe, or Module source; the staged
  `ui/src/generated/assembly.ts` line-ending artifact is left uncommitted.
