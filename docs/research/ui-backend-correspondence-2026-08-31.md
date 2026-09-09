# Comprehensive UI ↔ backend correspondence review (channels excluded)

Date: 2026-08-31  
Review branch: `feat/ui-backend-correspondence-audit`  
Scope: bidirectional comparison of Vivy's daily Web UI (`ui/src`) with the current JSON-RPC control plane, runtime, and product contracts.  
Explicit exclusions: channels; the Studio shell; demo functionality for which the backend does not yet provide an RPC/data model; production Journal and `data/`.

## Conclusion summary

The main path is not wholly detached from the backend: real RPC counterparts have been found for sessions/messages/runs and SSE replay, permission presets, the Review Center's basic queue and decisions, Provider/network/MCP settings, Token statistics, context readings, background runs, and child-runs. The problems fall into these categories:

1. The UI exposes a backend capability but does not pass its parameter (chat Plan mode).
2. The UI hides existing backend read-only capabilities behind a local demo layer (Skills, Dashboard overview).
3. The UI treats existing species-side compatibility RPCs as daily product authority (Lifecycle), violating the Studio boundary.
4. The UI does not surface existing backend Review/event details in the run inspector and detail views, and its busy-state scope is too broad.

Ten items are recorded: 6 P1 and 4 P2; no P0 security/data-destruction item was found within this scope. Fix recommendations are ordered P1 → P2; see “Recommended execution order.”

## Evidence baseline

- UI RPC types and calls: `ui/src/lib/api.ts`, `ui/src/lib/store.ts`.
- Control-plane dispatch and wire DTOs: `internal/rpc/control.go`.
- Run/compaction semantics: `internal/runtime/compaction_service.go`, `internal/runtime/compaction_middleware.go`.
- Product boundaries: `docs/architecture/VIVY-STUDIO.md`, `docs/architecture/VIVY-GATEWAY-AND-STUDIO.md`.
- Review contract: `docs/architecture/hitl-review-center.md`.
- V0 UI/log requirements: `docs/research/prd-agent-vivy-v0.md`.

## Findings

### P1 — UI-AUDIT-CHAT-MODE: Plan-mode selection does not reach the backend

**Evidence**

- `ui/src/lib/api.ts:20-21,200-201` explicitly defines the backend `RunMode` as only `normal | plan`; both `preflight` and `startTurn` accept mode.
- `ui/src/components/chat/ChatInput.tsx:20,29,37,60,121,159-170` defines and switches `agent/plan/ask`, but `onSend` passes only `content` and does not pass `execMode` when sending.
- `ui/src/components/chat/ChatView.tsx:44,55,89` hardcodes `'normal'` for `preflight`, then calls `startRun` with default parameters; it does not map the menu selection to the backend mode.

**Impact**

After the user selects Plan, both preflight and the actual run remain normal; the UI state is inconsistent with `preflight.mode` and the run record. Options such as `ask` and thinking that have no corresponding backend mode are not included in this item and are recorded only as out of scope.

**Required change**

Make the `ChatInput` callback carry a backend-expressible mode, and have `ChatView` pass it to both `preflight` and `startRun`; mark only options with a backend contract as executable in the menu. Update the existing `UI-CHAT-TOOLBAR` note in `docs/TODO.md`.

### P1 — UI-AUDIT-SKILLS-LIVE: Skills list/details still use the localStorage demo layer

**Evidence**

- `ui/src/routes/_layout.skills.tsx:3-4` wraps the entire skills page in `DemoBanner`.
- `ui/src/hooks/useSkills.ts:6-7,11,22,35` reads the list and documents from `@/lib/demo-api`, using demo fields such as `slug/enabled/source/markdown` from `SkillDto/SkillDocument`.
- `ui/src/lib/api.ts:373-391` already has real `SkillSummary` / `SkillView` and `skills/list`, `skills/get`.
- `internal/rpc/control.go:41-42,368,480-482,792-825` already registers and implements the read-only skills catalog.

**Impact**

The real `skills_root` name, description, hash, warnings, relative path, and supporting files are not displayed; seed data in browser localStorage may be completely different from the backend's installed content. The backend capability exists for list/details, but is invisible.

**Required change**

Change the list/details hook to `api.listSkills/getSkill`, use `name` as the identity, and display the fields already provided by the backend; remove DemoBanner from this real page. Keep demo tabs for skill change requests/enable/delete and similar actions that have no current RPC explicitly marked Demo, or remove them; they must not pretend to be backend-connected.

### P1 — UI-AUDIT-DASHBOARD-LIVE: Dashboard overview displays fixed demo numbers

**Evidence**

- `ui/src/routes/_layout.dashboard.tsx:3` directly renders `DashboardDemoView`, without `DemoBanner`.
- `ui/src/components/demo/DashboardDemoView.tsx:5,18,52-60` calls `getDemoDashboard()` and renders `sessionCount/activeRuns/pendingReviews` as overview state.
- The defaults in `ui/src/lib/demo-api.ts:1622-1624` are fixed at `12/2/1` and are written to localStorage.
- The backend already provides `session/list`, `background/list`, and `review/list`, with the control-plane capability at `internal/rpc/control.go:363-368`; the Token subpage in `DashboardDemoView.tsx:96` has already switched to real `stats/tokens`, which is the correct approach.

**Impact**

The user sees “run status” semantics, but the values may always be local seed numbers; multiple windows, restarts, and real sessions are not reflected. The page name and placement also do not indicate that these numbers are demo data.

**Required change**

Compute the three counts from real session, background, and review data in the store/API; “Recent activity” has no existing backend event-query aggregation endpoint, so delete it or show only sources that can already be proven. Retain the real Token subpage; because the backend has no corresponding endpoint for the trajectory tab, treat it as out of scope and continue to mark it explicitly as Demo.

### P1 — UI-AUDIT-LIFECYCLE-HOME: species UI exposes Generation/Eval/Promote write authority

**Evidence**

- `ui/src/routes/_layout.lifecycle.tsx:2-3` exposes `/lifecycle`.
- `ui/src/components/lifecycle/LifecycleView.tsx:17-26` calls `generations/create/reject`, `evals/start/record`, and `promotions/promote` through the store, and provides write forms.
- `ui/src/components/settings/SettingsView.tsx:204` provides an “Open lifecycle” entry on the daily settings page.
- The canonical source `docs/architecture/VIVY-STUDIO.md:19-21,52,127-145,258,354-361` explicitly gives the independent Studio ownership of the development/evaluation/release lifecycle, while the species retains only read-only inspect; these species-side tables are the “wrong home.” `docs/architecture/VIVY-GATEWAY-AND-STUDIO.md:224-225,412-417,507-512` likewise freezes species-side authority.

**Impact**

Although the backend RPCs still exist as compatibility implementations, the daily Vivy UI presents them as product authority, confusing the resident product with Studio and allowing the user to start evaluation/promotion at the wrong boundary.

**Required change**

Remove the lifecycle write page and Settings card from daily Vivy; if diagnostics are needed, retain only the read-only `species/inspect` view. Move Generation/Eval/Promotion writes and ledger entry points to Studio, without expanding species-side product semantics.

### P1 — UI-AUDIT-REVIEW-INSPECTOR: run inspector lacks a Review inline surface

**Evidence**

- `docs/architecture/hitl-review-center.md:31-38` requires decisions/expiry/stale states to be replayable from the run inspector, and requires the inspector to use the same `renderReviewCard` as the Review Center for inline decisions.
- HITL-04 in `docs/TODO.md` is already recorded as complete: “Review Center + run inspector Review tab.”
- The current `ui/src/components/chat/RunInspector.tsx:27-31` has only three tabs—current/background/children; the current tab shows only the event list, does not import `ReviewItem`, does not show a Review tab, and has no `review/respond` controls.
- The backend already provides `review/list/get/respond` in `ui/src/lib/api.ts:73,218-220` and `internal/rpc/control.go:1054-1162`.

**Impact**

The user cannot see suspended approval/question items in the run context or complete a decision from the run inspector; they must leave the current run to open the Review Center, and the implementation is inconsistent with the recorded HITL-04 status.

**Required change**

Add a Review tab/inline card to the inspector, reuse the Review Center's `ReviewItem` renderer and separate approval/question controls, filter by run/session, and refresh when events arrive while preserving the backend's first-writer-wins behavior.

### P1 — UI-AUDIT-RUN-DETAIL: structured event payload is only in the title and does not satisfy readable-log requirements

**Evidence**

- The V0 canonical source `docs/research/prd-agent-vivy-v0.md:90-96,301-302` makes persisted events, structured errors, and run-detail/event-log readability without development tools product requirements.
- `RunLogEvent` in `ui/src/lib/api.ts:50` explicitly contains `seq/type/created_at/payload_version/payload`.
- `ui/src/components/chat/RunInspector.tsx:28` uses only `event.type` as visible text and puts `JSON.stringify(event.payload)` in the HTML `title`; it has no expandable details, timestamp, payload version, error category, or safe field summary.

**Impact**

The rough JSON is visible only on desktop mouse hover and is unreadable on narrow screens, keyboards, or touch; backend-recorded information such as tool calls, policies, retries, compaction, and failure reasons cannot serve as a user-readable audit trail.

**Required change**

Provide a keyboard-accessible event-detail/collapsible panel showing seq, time, type, payload version, and a safe summary rendered by event type; use the backend's redaction for sensitive parameters and do not place raw secrets in the UI.

### P2 — UI-AUDIT-REVIEW-FIELDS: Review details omit existing backend audit fields

**Evidence**

- `reviewResult` in `internal/rpc/control.go:314-342` contains source, actor, created/expires/decided times, precondition hash, decision/stale/error reason, and more; `ui/src/lib/api.ts:73` mirrors these fields.
- `ui/src/components/approvals/ApprovalsView.tsx:83-119` renders only run, action, target, effect, reversibility, scope, trust, prompt, preview, risk, and redacted arguments; it does not render source/actor/timestamps/expiry/precondition/terminal reason.
- `docs/architecture/hitl-review-center.md:7-12,23-32` lists these fields and the expiry/stale lifecycle as part of the ReviewItem contract.

**Impact**

States such as expired, stale, denied, and cancelled have no timeline or reason; the user cannot determine who submitted the proposal, whether it expired, or which precondition failed, weakening auditability.

**Required change**

Add localized displays for source/actor, creation/expiry/decision times, precondition hash, and decision/stale/error reason; use different actions and explanations for pending and terminal states.

### P2 — UI-AUDIT-COMPACTION-BUSY: the immediate-compaction button does not reflect backend busy state

**Evidence**

- The “Compact now” control in `ui/src/components/settings/CompactionSettingsCard.tsx:66-85,150-153` is disabled only when there is no active session or when it is itself `compacting`; it does not check the current/background run.
- `internal/runtime/compaction_service.go:110-125` returns `ErrCompactionBusy` when any active/pending run exists; `internal/rpc/control.go:768-789` returns it as a conflict.

**Impact**

During a run, the user can click an operation that appears available and then receive a backend 409; even when the busy run is a background run in another session, the UI does not explain the failure in advance.

**Required change**

Expose active/background busy semantics from the store, disable the control during a run, and explain “compaction occurs automatically within a run”; refresh context after termination before re-enabling it. The backend remains the final authority.

### P2 — UI-AUDIT-REVIEW-BUSY-SCOPE: responding to one Review locks the entire queue

**Evidence**

- The backend `review/respond` is a conditional response for a single `review_id` (`internal/rpc/control.go:1100-1162`).
- The store records only one `reviewBusyId` (`ui/src/lib/store.ts:387-398`), but `ui/src/components/approvals/ApprovalsView.tsx:57,70,122,126-132,150` uses `busyId !== null` to disable every list item, detail action, and refresh.

**Impact**

While one approval is being handled, questions/reviews from other sessions cannot be viewed or handled; the busy-state scope is broader than the backend request scope.

**Required change**

Lock only the row and actions for the current `reviewBusyId`; allow browsing/selecting other reviews and, when necessary, block only duplicate submission of the same item. Refresh can continue after the request completes.

### P2 — UI-AUDIT-REVIEW-NAV: the full Review route is not in the main navigation

**Evidence**

- `ui/src/routes/_layout.approvals.tsx:2-3` contains the full `/approvals` main-area page.
- `NAV_ITEMS/VIVY_ITEMS/TOOL_ITEMS` in `ui/src/components/chat/ConversationSidebar.tsx:4-9` do not include `/approvals`; `ui/src/components/chat/ChatInput.tsx:241-242` has only a shield button in the chat box that opens the sheet.
- The Review contract in `docs/architecture/hitl-review-center.md:36-38` requires the Review Center to be a full main-area surface spanning sessions.

**Impact**

The user must enter chat first to discover the cross-session queue; although the direct main-area route exists, it has no stable entry point, and returning to the Review Center from Settings, Dashboard, or a narrow layout is difficult.

**Required change**

Add the Review Center to the main navigation or provide a prominent global entry point; retain the existing sheet for in-place handling and do not remove the full route.

## Verified correspondences, therefore not listed as defects

- Session creation/list/rename/delete, message list, run start/cancel/reconnect replay: `ui/src/lib/store.ts` uses `session/*`, `turn/*`, and `run/*` RPCs.
- Permission presets are passed to `session/set_permission` and locked during a run; the Provider registry, network tools, Sandbox, Compaction settings, and MCP all use the corresponding settings RPCs and respect read-only/frozen.
- The Review Center main queue already uses `review/list/get/respond`, with separate approval and question controls; this report identifies only detail-field, inline-inspector, busy-scope, and navigation gaps.
- The Token statistics page already uses `stats/tokens` (`DashboardDemoView.tsx:96`); it is not recategorized as a demo mismatch.
- Todo progress uses real read-only `session/todos`; the backend has no human mutation, so “cannot edit” is not counted in this scope.
- Background-run attach and basic child-run list/open/wait/cancel already have corresponding RPCs; tree visualization is an existing deferred `UI-TREE` item and is not registered again.

## Explicit exclusions (not in the backend and not included in this findings list)

- Channel configuration and readiness report: `UI-CHANNELS-BE`; the user explicitly requested that channels be excluded.
- Evolution/AutoDream, Persona/Memory, Notebook, Cron, trajectory, tool-rule demos, generation parameters, and Diva preview: there are currently no corresponding Vivy control-plane endpoints; follow existing TODOs such as `UI-EVO` and `UI-TRAJ`, and do not treat the “demo layer” itself as a backend mismatch.
- Chat `ask`/thinking, attachments, AutoDream, desktop companion, voice, and message edit/rewind/fork: the backend has no corresponding current RPC; `UI-CHAT-TOOLBAR` and `UI-CHAT-ACT` are already registered.
- Skill change-request, enable/delete/edit: this review covers only the backend's existing list/get; requests/write operations have no backend endpoint, so keep the Demo marker or create a separate capability proposal.
- Standalone `http_request` settings, human todo mutation, and goal: existing TODOs explicitly record these as backend-capability/product-proposal gaps; they are excluded as requested by the user.

## Recommended execution order

1. P1: fix the Plan-mode parameter chain first; simultaneously remove/downgrade the Lifecycle write entry points from daily Vivy.
2. P1: switch Skills list/details and Dashboard overview to real RPCs; delete Dashboard activity items if they have no endpoint.
3. P1: add Review inline and readable event details to Run Inspector, restoring the HITL-04/log-canonical-source requirements.
4. P2: add Review audit fields, per-item busy scope, and a main-navigation entry, then fix Compaction busy preflight.

## Verification status

- `pnpm install --frozen-lockfile`: passed (isolated worktree).
- `pnpm build`: passed.
- `just ci`: passed; includes Go vet, Go tests, UI typecheck, 175 UI tests, and UI build.
- Real split processes: `just run` + `pnpm dev --host 127.0.0.1` started successfully; both `GET http://127.0.0.1:8787/healthz` and `GET http://127.0.0.1:3015/` returned HTTP 200.
- Browser visual smoke: no available browser-runtime instance in the current environment (`agent.browsers.list()` returned empty), so screenshot/interaction verification is not claimed; this is recorded as an environment limitation, not product-pass evidence.

This delivery produced only the review document/TODO entries; it did not modify the UI or backend implementation.
