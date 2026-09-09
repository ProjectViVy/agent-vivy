# 2026-08-30 Chat-box button cleanup (chatbox-buttons)

## What changed

The home-page chat box (`ui/src/components/chat/ChatInput.tsx`) is now the sole entry
point for session operations:

- **Move Create new session into the chat box**: add a Plus button to the top-bar
   right-side cluster (Create new session → History → Approval Center), calling
   `store.createSession()` (automatically selects the new session).
- **Remove the old entry points**:
  - `ConversationSidebar.tsx`: remove the sidebar Plus create-session button and the
    `onCreateSession/creating/createError` props plus their `_layout.tsx` arguments
    (remove `createAndOpen` as well).
  - `SessionDrawer.tsx`: remove the drawer's "Create new session" button and the
    `onCreateSession` prop.
- **Remove user-requested buttons**:
  - Voice (Mic): remove the button, `recording` state, and i18n copy (it was only a
    local-state fake demo, with no recognition or RPC).
  - Desktop companion (Cat): remove the button and i18n copy (it was a no-op
    "Desktop companion is not connected yet").
- **Wire the execution-mode dropdown** (Agent / Plan / Ask):
  - Extend `ChatInput`'s `onSend` signature to `(content, mode)`; `ChatView.submit`
    passes mode to `preflight/run` and `turn/start` (previously hard-coded to
    `'normal'`; the `api.startTurn/preflight` and `store.startRun` paths already
    supported mode).
  - Agent → `normal`, Plan → `plan` (the backend's `domain.RunMode` genuinely supports
    it; Plan blocks non-read-only tools).
  - Ask: retain the option, show "Ask mode is not connected yet" when selected, and
    leave the current selection unchanged (the backend has no corresponding RunMode).
  - `ChatView`'s `pending` preflight staging carries mode; "Continue" reruns with the
    same mode; `regenerate` uses the default normal mode.

## Already wired (verified only this time; unchanged)

Approval (ShieldCheck → review/list + review/respond) and permission control (dropdown
→ session/set_permission, locked during runs, Trust-mode confirmation dialog) were
already inside the chat box and connected to real RPC; the real-browser smoke recheck
passed this time.

## Kept per user instruction (not implemented; UI retained)

The following buttons have no backend capability; they are retained at the user's
request and called out during acceptance:

- Attachments (`attachmentUnavailable` prompt)
- AutoDream (`autodreamUnavailable` prompt)
- "+ More" (`moreUnavailable` prompt)
- Thinking mode dropdown (Auto / On / Off, local state only)
- Ask mode (now explicitly says it is not connected when selected, rather than being a
  silent fake choice)

Recorded in `docs/TODO.md` §0.1 **UI-COMPOSER**.

## What was explicitly not done

- Did not touch the existing approval/permission implementation or the Review Center
  panel.
- Did not touch MessageBubble placeholder buttons (edit / revert / fork).
- Did not touch the top bar (session Sheet, todos, mask/model switcher).
- Did not fix the two pre-existing e2e failures on main (see E2E-STALE); only the
  equally stale `Drawing` assertion blocking this verification was removed from this
  lane's spec.

## e2e

`ui/e2e/runtime.spec.ts`: remove the stale `Drawing` assertion; move `New session` (the
old sidebar entry) to the chat-box `Create new session` flow (including returning from
/masks to the home page before clicking); add negative `toHaveCount(0)` assertions for
voice and desktop companion.
