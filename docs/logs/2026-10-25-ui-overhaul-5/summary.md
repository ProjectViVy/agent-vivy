# UI overhaul Block 5 — navigation, menus, dialogs

## What changed

Navigation chrome, menus, dialogs and the welcome wizard now follow the
semantic token contract and the shared geometry tokens, and the wizard is
split into single-owner parts.

- `ui/src/styles.css`: added `--container-sidebar: var(--sidebar-w)` to
  `@theme inline` so `w-sidebar` / `max-w-sidebar` utilities exist beside
  `max-w-content`.
- `ui/src/components/ui/dialog.tsx`: `DialogContent` gained a `size` prop
  (`sm | md | lg | sheet`, default `md`); `sheet` is the viewport-derived
  large-dialog size used by the history picker.
- `ui/src/routes/_layout.tsx`: desktop rail collapses with `w-sidebar` ↔
  `w-0`; mobile nav opens as a left `SheetContent` at `w-sidebar`; header,
  avatar, wordmark, status line and connection pill converted to
  `bg-surface`, `bg-selected`, `text-title`, `text-meta`, `text-fg-muted`,
  `bg-ok-soft/text-ok-fg`, `bg-warn-soft/text-warn-fg`.
- `ui/src/components/chat/ConversationSidebar.tsx`: nav links, drill rows,
  search, section headers, workspace rows and the view menu converted to
  `text-ui/text-meta/text-2xs` + `text-fg-muted`; long labels clamp to two
  lines with `title` tooltips instead of single-line truncation.
- `ui/src/components/chat/ModelSwitcher.tsx`: trigger shows a two-line
  provider/model stack (clamped, tooltip) instead of one truncated line;
  menu rows converted to tokens with `bg-sunken` icon wells and
  `text-brand` selection marks.
- `ui/src/components/chat/HistoryPicker.tsx`: mobile bottom sheet and
  desktop `DialogContent size="sheet"`; all text converted to the scale.
- `ui/src/components/settings/SettingsView.tsx`: content column
  `max-w-content`, `text-page` heading, tab list wraps (`flex-wrap`) instead
  of clipping at narrow widths, Run Inspector height viewport-derived,
  preview/vivy cards converted to `bg-brand-soft text-brand`.
- `ui/src/components/chat/WorkControlBar.tsx`: dock `max-w-content`,
  objective clamps to two lines with tooltip, tokens for chips/errors.
- `ui/src/components/chat/TodoProgressStrip.tsx`: `max-w-content` dock,
  label/progress clamp with tooltips, semantic tokens.
- `ui/src/components/chat/session-row.tsx`: shared session identity row
  (sidebar + history picker) converted to tokens; session titles clamp to
  two lines (`line-clamp-2 break-words`) keeping the `title` tooltip; delete
  affordance uses `text-danger hover:bg-danger-soft`.
- `ui/src/components/layout/WelcomeWizard.tsx` (549 → 363 lines) split: the
  shell keeps state, effects and the save flow; step views moved to
  `welcome/wizard-step-intro.tsx` (31), `welcome/wizard-step-model.tsx`
  (197), `welcome/wizard-step-done.tsx` (53). All ad-hoc sizes
  (`text-[22px]`, `text-[11px]`, `text-[10px]`) and palette literals
  (`bg-primary/10`, `bg-amber-500/10`, `bg-destructive/10`, `bg-muted/*`,
  `bg-border`) replaced by `text-page/text-2xs`, `bg-brand-soft`,
  `bg-warn-soft`, `bg-danger-soft`, `bg-sunken`, `bg-line`.
- `ui/src/theme/tokens.test.ts`: ratchet baselines lowered src palette
  148 → 137 and src textSize 52 → 35; ten cleaned file entries deleted.

## Explicitly not done

- Remaining kernel pages (`FilesPanel`, `McpView`, trajectory panels, cron,
  demo banner, `RunInspector`, `SessionTodoPanel`, settings cards) keep
  their legacy literals; that is Block 6.
- Plugin UI modules (Block 7), Studio shell skin (Block 8) and the dead-key
  sweep (Block 9) are untouched.
- No behavior change to wizard save semantics, provider registry writes or
  the PROV-P4 catalog contract; only view ownership and styling moved.
