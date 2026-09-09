# Chat-input toolbar port (Agent-DIVA → Vivy, UI only)

Date: 2026-08-27
Status: complete

## Outcome

Using `chat-input-toolbar` in `agent-diva/agent-diva-gui/src/components/ChatView.vue`
(the real toolbar above the chat input) as the reference, its content and interactions
were ported to Vivy's `ui/src/components/chat/ChatInput.tsx`. Within the scope confirmed
by the user: **fully match the DIVA layout (replacing the existing toolbar) + retain
「Not connected yet」 notice bars for backend-dependent buttons** (UI only, with no backend
changes).

## Ported toolbar (left to right)

1. **Execution-mode selector** (dropdown, opens upward with `side="top"`): Agent (Zap) /
   Plan (Settings2) / Ask (Brain), with menu items consisting of an icon + title +
   description + a checkmark on the current item; the trigger icon and text update
   immediately after selection (matching DIVA `modeOptions` + `mode-menu`).
2. **Attachments** (Paperclip): clicking displays a 「Attachments not connected yet」 notice
   bar (stub).
3. **Thinking-mode selector** (dropdown, `side="bottom"`): Auto (Lightbulb outline) /
   On (solid Lightbulb, `fill="currentColor"`) / Off (LightbulbOff) (matching DIVA
   `ThinkingToggle`).
4. **AutoDream trigger** (GitBranch): clicking displays an 「AutoDream not connected yet」
   notice bar.
5. **Desktop companion** (Cat): clicking displays a 「Desktop companion not connected yet」
   notice bar.
6. **Permission-mode selector** (dropdown, opens upward): Cautious (Shield) / Smart
   (Sparkles) / Trusted (CheckCircle), with the same menu-item structure as the mode
   dropdown (matching DIVA `permissionOptions`).
7. **Divider** (`h-4 w-px bg-border`).
8. **Right-side group** (`ml-auto`, matching DIVA `.chat-corner-actions`):
   - **History** (Clock): clicking opens the right-side 「Sessions」 drawer (reusing the
     existing `SessionDrawer`; in DIVA this button toggles the session sidebar).
   - **Approvals** (ShieldCheck): clicking opens the approvals Sheet; the pending count
     is shown as a DIVA-style **numeric badge** (red background, white text, rounded pill,
     replacing the original red dot), and `aria-expanded` reflects the open state.

## Delivered

- `ui/src/components/chat/ChatInput.tsx` — rewrote the entire toolbar row item by item to
  match DIVA; the three dropdowns use Radix `DropdownMenu`
  (`ui/components/dropdown-menu`), with triggers using `asChild`
  (`aria-expanded`/`aria-haspopup` supplied by Radix); icon buttons retain Vivy's
  `rounded-lg p-1.5 text-muted-foreground hover:bg-accent` and `title`/`aria-label`,
  and the visuals use Vivy Tailwind tokens without introducing DIVA CSS variables.
  Removed the original 「Draw (Palette)」 and 「Smart (Sparkles)」 buttons and the boolean
  agentMode toggle (upgraded to a three-state dropdown). The textarea and footer
  (context ring / notice bar / more / voice / send · stop) remain unchanged.
- `ui/src/lib/store.ts` — added `sessionDrawerOpen` state and
  `setSessionDrawerOpen` (following the existing `reviewCenterOpen` pattern).
- `ui/src/routes/_layout.tsx` — changed the session Sheet from local `sessionOpen` to the
  store's `sessionDrawerOpen`, allowing ChatInput's History button to open the same drawer
  (the close logic after creating/selecting a session was migrated as well).
- `ui/src/i18n/zh.ts` / `en.ts` — synchronized the `chatInput` entries to the zh-authoritative
  structure: added `planMode`/`askMode`/`agentModeDesc`/`planModeDesc`/`askModeDesc`/
  `thinkingMode`/`thinkingModeAuto`/`thinkingModeOn`/`thinkingModeOff`/
  `autodreamTrigger`/`autodreamUnavailable`/`openMate`/`mateUnavailable`/
  `permissionCautious`/`permissionSmart`/`permissionTrusted` and three `*Desc` entries;
  removed obsolete keys `switchedToNormal`/`switchedToAgent`/`draw`/
  `drawUnavailable`/`smart`/`smartStrategy`/`branch`/`branchUnavailable`/
  `historyHint`.

## Semantic differences (from Agent-DIVA)

- **Mode / thinking / permission selections are UI-only state**: they do not change the
  send semantics (`ChatView.submit` still goes through
  `preflight(sessionId, text, 'normal')` → `startRun`) and are not hidden backend
  switches; in DIVA these values are reported with the send event, and will be connected
  once the Vivy core supports the corresponding execution modes.
- **Attachments / AutoDream / companion / voice are stubs**: in DIVA they perform real
  uploads, trigger AutoDream, open the desktop companion, and record audio respectively;
  these Vivy capabilities are not connected, so per the user's confirmation they retain
  `showNotice` notice bars without pretending to perform the actions.
- **History button**: DIVA toggles the session sidebar (including list selection), while
  Vivy reuses `SessionDrawer` (the same source as the top-right session button), closing
  the interaction loop while remaining UI-only.

## Explicitly not done

- No Go backend, RPC, or `api.ts` transport changes were added; Journal semantics were
  untouched.
- Real attachment selection/upload preview, AutoDream triggering, desktop companion, and
  voice recording were not implemented (the 「Not connected yet」 notice bars remain).
- Mode/thinking/permission selections were not persisted locally (DIVA stores
  permissionMode in localStorage `agent-diva.permissionMode`; this round keeps them as
  session state, and any future persistence should use a `vivy.ui.*` key).
- The footer row (context ring, notice bar, Plus, Mic, send/stop) is outside this round's
  scope and remains unchanged (its structure already corresponds to the DIVA footer).
- No component-level unit tests were added (ui has no @testing-library foundation; in line
  with the principle of not introducing automation infrastructure for a small change,
  coverage uses typecheck + existing vitest + a real-path smoke test).
