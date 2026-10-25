# UI overhaul — Block 2: primitives consolidation and dead-code deletion

- Date: 2026-10-25
- Branch: `feat/ui-overhaul` (worktree `.worktrees/ui-overhaul/agent-vivy`)
- Plan: `~/.qoder-cn/plans/tall-fjord-lark.md` §Block 2
- Previous block: `docs/logs/2026-10-25-ui-overhaul-1/`

## Outcome

`ui/src/components` now contains one primitive per interaction role and no
unimported file. Net effect: **27 files / 3,222 lines deleted**, one shared
session row added (90 lines), and 23 npm dependencies dropped.

### Deleted (each re-grepped to zero importers across `ui`, `plugins`, `sdk`,
`studio/packages` before removal)

- 25 primitives: `accordion`, `alert`, `aspect-ratio`, `avatar`, `breadcrumb`,
  `calendar`, `carousel`, `chart`, `collapsible`, `command`, `context-menu`,
  `drawer`, `form`, `hover-card`, `input-otp`, `menubar`, `navigation-menu`,
  `pagination`, `radio-group`, `resizable`, `sidebar`, `sonner`, `toggle`,
  `toggle-group`, `tooltip`. `components/ui` went from 48 entries to 23.
- `ui/src/components/chat/SessionDrawer.tsx` (0 imports) and
  `ui/src/components/chat/ConversationModePlaceholder.tsx`, a local-state-only
  toggle rendered at `routes/_layout.tsx:134`.
- The store/face field behind them: `sessionDrawerOpen` and
  `setSessionDrawerOpen` in `ui/src/lib/store.ts` **and** in the module contract
  `sdk/ui/src/module.ts` (`FaceStoreState`). Leaving the SDK copy would have
  made `useVivyStore` stop satisfying `FaceClientStore`; the type-level checks
  in `ui/src/lib/ui-build-provenance.test.ts` and
  `ui/src/lib/ui-sdk-face-compat.test.ts` caught it.
- Dependencies used only by those primitives: `@hookform/resolvers`,
  `react-hook-form`, `sonner`, `recharts`, `vaul`, `cmdk`, `react-day-picker`,
  `embla-carousel-react`, `input-otp`, `react-resizable-panels`, `date-fns`,
  and Radix `accordion`, `aspect-ratio`, `avatar`, `collapsible`,
  `context-menu`, `hover-card`, `menubar`, `navigation-menu`, `radio-group`,
  `toggle`, `toggle-group`, `tooltip`. `ui/pnpm-lock.yaml` lost 899 lines.

### Consolidated

- **One session row.** New `ui/src/components/chat/session-row.tsx` renders the
  identity, selected/busy state, untitled placeholder and optional rename /
  delete actions. `ConversationSidebar.tsx` and `HistoryPicker.tsx` both use it;
  the history picker no longer owns a second row implementation.
- **One menu grammar.** The hand-rolled `role="menu"` (with `viewMenuRef`, a
  document listener and `role="menuitemradio"` buttons) in
  `ConversationSidebar.tsx` is replaced by `components/ui/dropdown-menu.tsx`
  (`DropdownMenu` + `RadioGroup`/`RadioItem`), the primitive `ModelSwitcher`
  and `ChatInput` already use. Radix now owns focus, dismissal and keyboard
  handling.

### Ratchet

`ui/src/theme/tokens.test.ts` baselines lowered to what remains:

| Group | Category | Before | After |
| --- | --- | --- | --- |
| src | palette | 159 / 31 files | 158 / 30 files |
| src | textSize | 85 / 26 files | 81 / 24 files |
| src | hex | 5 / 1 file | 0 / 0 files |
| plugins | palette / textSize / hex | 29 / 45 / 0 | unchanged |

`ui/src/i18n/surfaces.test.tsx` lost the pagination, breadcrumb and calendar
cases along with those primitives; the remaining locale-picker and chat/review
cases are unchanged.

## Deviations from the plan (recorded, not silently dropped)

1. **`lib/status.ts` + `ui/status-badge.tsx`, `ui/panel.tsx` + page-header and
   `dialog.tsx` size variants are deferred** to the block that introduces their
   first consumer (Block 6 for status/panel, Block 5 for dialog sizes). Adding
   them now would land unused abstractions in the same commit whose goal is
   "nothing unused in `components/ui`".
2. **The composer listbox stays.** `ComposerCommands.tsx` renders
   `role="listbox"/"option"` — that is the popup of an ARIA combobox driven
   from the textarea, not a menu, and `ChatInput.commands.test.tsx` asserts it.
   It is the single implementation of that role. The Block 2 acceptance line
   `rg 'role="(menu|listbox)"'` = 0 was therefore met for menus only.
3. **`tooltip.tsx` was deleted rather than kept for the session row.** Nothing
   imports Radix tooltip after `sidebar.tsx` left, and the project convention
   for long names is the native `title` attribute (the row now sets it). A
   shared tooltip primitive returns when a block has a real consumer.
4. **Dead i18n keys are left in the dictionaries** (`codeMode.*`,
   `sessionDrawer.*`, `pagination.*`, `calendar.*`, `breadcrumb.*`). Block 9 owns
   the dictionary sweep together with the `scripts/i18n-cross-face-contract.json`
   update; removing them here would split one contract change across two blocks.

## Follow-ups recorded on the board

- `docs/TODO.md` §0.1 `STORE-RUN-FAILED-COPY`: `ui/src/lib/store.test.ts`
  "keeps the run.failed payload as the conversation error" fails at `49544156`
  as well (proved by restoring `HEAD`'s `store.ts`), independent of this block.
