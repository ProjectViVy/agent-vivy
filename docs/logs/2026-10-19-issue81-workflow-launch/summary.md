# Workflow launch: honest eligibility for the host parent run

Issue #81 / Epic E3 (L2 Workflow), story W2. Same lane
`feat/issue81-workflow`, on top of the W1 commit.

## Defect

`WorkflowPage` treated any `currentRun` as permission to launch
(`faceState.currentRun != null`), and `FaceBridge` injected that run's id
without checking its status or session — and left a caller-supplied
`parent_run_id` in place. The backend is strict (`currentChildAuthorizer`:
same session, `active`, live tool ceiling), so:

- an idle session or a finished run showed an enabled **Run** button that
  failed with `412 revision_conflict`/`404 not_found` from the backend;
- a Run from another session could be named as parent by a caller;
- the failure surfaced as a raw transport error with no explanation of the
  product rule.

## Change

- New `plugins/vivy-workflow/ui/vivy-workflow/src/launch-context.ts`:
  `workflowLaunchParent(state)` returns the parent Run id only when the host
  has a non-empty active session, a current run, `status === 'active'`, and
  `currentRun.session_id === activeSessionId`; otherwise `null`. The predicate
  is advisory and derived from host state only.
- `WorkflowPage.tsx`: `canRun` comes from that predicate (used by both panes).
- `face-bridge.ts`: `inofy.startRun` now deletes any caller-supplied
  `parent_run_id`, refuses ineligible launches with
  `TransportError(0, { code: 'unsupported_feature' })` **before** the RPC, and
  otherwise names the host parent; `operation_id` stays caller-supplied when
  bounded, generated otherwise. Backend refusals still propagate unchanged.
- `editor/EditorPane.tsx` and `panes/WorkflowsPane.tsx`: an inline disabled
  reason reuses the existing localized `plugin.vivy/workflow-ui.editor.runDisabled`
  copy (en/zh) — no catalog change, no `assembly.ts` regeneration.
- `internal/runtime/workflow_product_launch_test.go`: pins the backend half —
  an active parent admits (and owns the child), a terminal parent is
  `storage.ErrChildAdmissionConflict`, an unknown parent is
  `storage.ErrNotFound`, and no rejected launch admits a child; the admitted
  revision record keeps the parent, session, and operation key.
- Browser cases in `ui/e2e/workflow-correctness.spec.ts`: idle session and
  finished run show the reason and a disabled Run; with a held-open parent Run
  the author can publish and launch, the Runs tab reaches `completed`, and the
  model request bodies contain the authored literal (consumption, not just
  submission). `ui/e2e/continuity-setup.ts` gained the `hang` reply used to
  hold the parent run open; the workflow suite got its own ports, Chrome
  channel, and disposable workdir.

## Explicitly not done

- No backend, RPC, storage, admission, or schema change; the strict admission
  path is reused as-is, not relaxed.
- Independent workflow launch from an idle session (no parent Run at all) is
  **not** enabled. That is a new product capability outside E3: it needs a
  decision on run rootage, authority, and tool ceiling. Restart condition: an
  accepted contract for a session-less workflow root run, plus admission tests
  and a parentless launch path. Deliberately not scheduled here; recorded in
  this log because the product-gaps package reserves `docs/TODO.md` /
  `docs/DEFER.MD` for one centralized writer.
- No locale catalog growth: the existing `runDisabled` copy is reused verbatim.
