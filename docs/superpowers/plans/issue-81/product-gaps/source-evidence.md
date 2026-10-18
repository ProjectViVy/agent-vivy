# Source-of-truth reconciliation — PG-1

**Inspection date:** 2026-10-18 UTC. **Main commit:** `7b42b2920f72de6f37de8b9e4ff4bf6a110a762f`. Remote ref/tree reads and later local fetch agree. Recursive remote tree was complete (`truncated=false`). All links below are pinned source evidence, not fresh executed product tests.

## Authority and prior plan reuse

1. Owner's delegated scope and [#81 current boundary](https://github.com/ProjectViVy/agent-vivy/issues/81) select E2/E3/E4. Archived proposal text does not reschedule rejected/deferred features.
2. Repository [AGENTS.md](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/AGENTS.md) was read remotely first and again locally when the environment became ready. No docs-scoped AGENTS exists at this baseline; ui/AGENTS applies to planned future UI edits.
3. Reuse [Notebook/Reports index](../../notebook-reports/index.md), [R2](../../notebook-reports/R2.md) and [existing report design](../../../specs/2026-10-09-notebook-reports-design.md), not a new Notebook design.
4. Reuse [INOFY cutover index](../../2026-09-29-inofy-cutover/README.md), [UI plan](../../2026-09-29-inofy-cutover/07-ui.md) and [existing design](../../../specs/2026-09-29-inofy-cutover-design.md). Foundations delivered by #24/#25 are not reopened.
5. Reuse [Toolkit TK-13](../../toolkits/TK-13.md), [canonical requirements](../../toolkits/acceptance.md), [shared contracts](../../toolkits/contracts.md) and [current partial evidence](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/docs/logs/2026-10-12-toolkits-closure/acceptance.md). This package adds execution deltas and does not supersede their requirement meanings.
6. Remote plan/ docs branch inventory and main plan tree were checked before creating this branch; no existing issue-81/product-gaps package was found. Existing provider planning branches were not changed.

## E2 report findings

- [view.tsx](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/plugins/vivy-notebook/ui/vivy-notebook/src/view.tsx): report controls occur only within selected-entry detail; empty/unselected state only offers selection messaging.
- [reports.tsx](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/plugins/vivy-notebook/ui/vivy-notebook/src/reports.tsx): existing nullable entry, capability read, generate/settings actions and run-ID tracking can serve first use.
- [browser test lines 71–80](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/ui/e2e/notebook-reports.spec.ts#L71): first report is currently seeded by RPC, then UI panel is exercised. Header explicitly describes deterministic fallback without provider credentials. This proves neither natural first entry nor real narrative quality.
- #5's reopened historical "broader reports deferred" wording predates merged report implementation. Its feature owner remains #5; #39 is already closed. R1 is a minimal entry repair.

## E3 workflow findings

- [NodePanel.tsx](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/plugins/vivy-workflow/ui/vivy-workflow/src/editor/NodePanel.tsx) initializes/commits inputs through config. [schema.ts](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/plugins/vivy-workflow/ui/vivy-workflow/src/studio/schema.ts#L41) has separate node.inputs and Binding {literal?,source?,pointer?}.
- [inofy_admission.go](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/internal/runtime/inofy_admission.go#L79) accepts only call/vivy.child-task@1, rejects retry/fallback, and decodes task/tool_names with DisallowUnknownFields. [inofy_executor.go](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/internal/runtime/inofy_executor.go#L176) includes resolved input in child task text.
- [WorkflowPage.tsx](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/plugins/vivy-workflow/ui/vivy-workflow/src/WorkflowPage.tsx) tests only currentRun != null for canRun. [FaceBridge](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/plugins/vivy-workflow/ui/vivy-workflow/src/face-bridge.ts#L96) binds currentRun without active/session eligibility.
- [RPC startRun](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/internal/rpc/inofy_product.go#L276) requires parent/operation identity; [workflow_product.go](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/internal/runtime/workflow_product.go#L279) enforces same-session parent; [currentChildAuthorizer](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/internal/runtime/child_sessions.go#L672) requires active status plus authority and tool ceiling. W2 therefore uses explicit unavailability, not a guessed historical parent.
- [Vitest config](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/ui/vitest.config.ts) includes notebook/Toolkit UI tests but not workflow source tests. Test discovery must be repaired before a green workflow unit claim. No workflow Playwright config/spec exists in this main tree.
- Existing runtime product tests cover draft lifecycle, root-pointer output, revision binding, draft execution, capabilities, cancellation and omitted-product core path. W1/W2 extend those proofs, not duplicate definitions/publish implementation.

## E4 Toolkit findings

- [Toolkit config](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/ui/playwright.toolkits.config.ts) hardcodes ports and requires prebuilt vivy; accessibility inherits it. [Vite config](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/ui/vite.config.ts#L50) already supports VIVY_UI_PORT. [justfile](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/justfile) has no Toolkit browser recipe.
- [2026-10-17 verification](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/docs/logs/2026-10-17-toolkit-ui-redesign/verification.md) records temporary 18887/3115 config and native binary workaround, both existing suites passed there, required CI/pin/timeout limits retained. [TODO TOOLKITS-E2E-RECIPE](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/docs/TODO.md#L151) identifies precisely the small launcher tail.
- [TK-13 reconciliation](https://github.com/ProjectViVy/agent-vivy/blob/7b42b2920f72de6f37de8b9e4ff4bf6a110a762f/docs/logs/2026-10-12-toolkits-closure/acceptance.md) explicitly retains publication/crash, admission/concurrency, revocation, Hook/MCP, UI and both-driver compatibility gaps. Historical live PostgreSQL/official MCP and packaged browser evidence exists and is preserved.
- Existing manager tests cover publication order, captured restore, GC references and recovery; runtime tests cover root/child/drain/fence and exact consent; real App tests cover lifecycle, MCP refresh and authority. Driver conformance entry is `TestBundleStoreConformance` in both `internal/storage/{sqlite,postgres}/conformance_test.go`. Add only missing rows.
- #20's suite proposal reflects older wording; E4 uses the current native Toolkit implementation/acceptance boundary. No #67 content publication is part of this plan.

## Skills actually read

- `superpowers:using-superpowers`, `superpowers:brainstorming`, `superpowers:writing-plans`, `superpowers:using-git-worktrees` and `superpowers:verification-before-completion` via the skill reader.
- `supermanagement` main SKILL.md via `skill://user/6aae8b92a6448191b99bb43c180ba1d1/supermanagement/SKILL.md`.
- Repository `.agents/skills/vivy-plugin/SKILL.md` and Eino compose capability reference, plus relevant normative architecture/plan source.

Supermanagement main reads succeeded, but `references/architecture.md`, `references/coordination.md` and `references/story-plan.md` returned "failed to read skill resource" at the exact advertised resource URIs. They were not recovered in the local environment skill-directory search. This limitation is disclosed; no claim is made that those ancillary files were read. Core skills are available, so the explicitly requested planning continues using their actual primary instructions.

This is an existing-contract closeout resumed at detailed planning. User explicitly requested a complete executable plan and separate branch, so no new product-design approval or implementation action is inferred. All new source edits remain future proposals; model/browser/DB/API product acceptance is not performed here.
