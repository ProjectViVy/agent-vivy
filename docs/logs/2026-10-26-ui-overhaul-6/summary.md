# UI overhaul Block 6 — remaining kernel pages sweep

## What changed

The remaining kernel page and panel surfaces now follow the semantic token
contract, share one status-tone vocabulary and one page-heading primitive, and
no longer clip content at narrow widths. After this block the token ratchet
reports **zero** stock palette classes, zero arbitrary text sizes and zero hex
literals anywhere in `ui/src`.

### Token sweep (37 view/route files)

Legacy shadcn literals and stock text sizes were converted to the canonical
tokens across the approvals, chat, cron, dashboard, demo, files, lifecycle,
mcp, planning, settings, skills and trajectory surfaces plus the `dialog`,
`sheet`, `alert-dialog` and `DiffView` primitives and the `__root` splash:

- `bg-muted` → `bg-sunken`, `text-muted-foreground` → `text-fg-muted`
- `bg-card` → `bg-raised`, `text-card-foreground` → `text-fg`
- `bg-background` → `bg-surface`, `border-border` → `border-line`, `border-input` → `border-line-strong`
- `text-destructive` → `text-danger`, `bg-destructive` → `bg-danger`, `text-destructive-foreground` → `text-danger-fg`
- `text-primary`/`bg-primary`/`border-primary` → `text-brand`/`bg-brand`/`border-brand`, `text-primary-foreground` → `text-brand-fg`
- `hover:bg-accent` → `hover:bg-hover`, `bg-accent` → `bg-selected`, `text-foreground` → `text-fg`
- `text-sm` → `text-ui`, `text-xs`/`text-[11px]`/`text-[10px]` → `text-meta`/`text-2xs`, `text-[13px]` → `text-ui`

### New shared primitives

- `ui/src/lib/status.ts` — one `StatusTone` vocabulary (`ok | warn | danger |
  info | brand | neutral`) and the `TONE_CHIP` class map. Every status chip and
  badge now derives its colours from this single source instead of a per-view
  copy. (A speculative `TONE_DOT` export was removed — nothing consumed it.)
- `ui/src/components/ui/status-badge.tsx` — `StatusBadge`, a pill that renders
  `TONE_CHIP[tone]`. Replaces the duplicated health-badge class maps in
  `ChannelCard.tsx` and `ChannelsSettings.tsx`.
- `ui/src/components/ui/panel.tsx` — `PageHeader` (icon well, `text-page`
  title, muted description, actions row). Applied to the cron and mcp page
  headers (both the main and error states); the bespoke conditional headers in
  approvals, dashboard, lifecycle and skills were normalised in place to
  `text-page`/`text-title` + `text-ui text-fg-muted`.

### Display-completeness fixes

- `RunInspector.tsx`: the three id truncates (run id, workflow id, child id)
  gained `title` tooltips so the full identifier is reachable; the child
  history scroll area is viewport-derived (`max-h-[30vh]`).
- `PlanReview.tsx` plan `pre` and `ApprovalQuickCard.tsx` prompt/command `pre`
  caps made viewport-derived (`max-h-[40vh]`, `max-h-[30vh]`) so long content
  is not cut to a fixed 10 rem.
- `SkillsView.tsx`: the skill-name span now clamps with `truncate` + `title`
  instead of overflowing its row by ~9 px at 768.
- `CronTaskManagementView.tsx`: the MasterDetail detail column narrows to
  `16rem` at `md` (restoring `20rem` at `lg`) so the empty-state action button
  no longer clips in the ~78 px master column at the 768 breakpoint.

### Ratchet

- `ui/src/theme/tokens.test.ts`: `src` baselines lowered to `palette 0`,
  `textSize 0`, `hex 0` with the stale file entries deleted. `plugins`
  baselines are untouched at `29 / 45 / 0` (Block 7).

## Explicitly not done

- Plugin UI modules keep their palette/textSize literals; that is Block 7
  (ratchet `plugins` 29/45/0 unchanged).
- Studio shell skin (Block 8) and the dead-i18n-key sweep (Block 9) untouched.
- The `components/ui/*` shadcn primitives and a handful of files still ride the
  `styles.css` compat shim (`--color-muted`, `--color-card`, `--color-primary`,
  …) which maps the legacy token names onto the semantic variables. They render
  correctly and are not tracked by the ratchet; removing the shim entirely is
  deferred to the final sweep.
- No behavior change: cron/mcp/skills/approvals data flows, MasterDetail
  selection semantics, RunInspector attach and the channel health contract are
  unchanged — only styling, shared primitives and header ownership moved.
