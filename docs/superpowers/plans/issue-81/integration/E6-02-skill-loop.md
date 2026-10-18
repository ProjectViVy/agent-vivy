# E6-02 VIVY Skill Review and Later Use Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans after implementation authorization. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Prove experience-driven updates reach human review, change the same Skill and become usable after a true restart.

**Architecture:** Exercise the E6-01 authority through real VIVY operations and the current Review Center/Skill surfaces, using scripted local inference and isolated disk state. Reuse existing UI and fixture infrastructure.

**Tech Stack:** Go fixtures, SQLite, Playwright/Vite, current Control RPC and Skill tools.

**Spec:** #64/#69, architecture.md D-SKILL/D-SKILL-RECOVERY. Immediate predecessor and status: package.md E6-01.

## Global Constraints

No DIVA UI/installation, EvoMap, production data, paid inference, new management platform or activation of #57/#66 presets. The deterministic fixture establishes plumbing and exact content use, not real-model effectiveness. Browser runs use the supported split Vite origin and independent home/ports.

## Review Focus

The old test's new Service over an open store is insufficient; restart must close actual DB/files. UI approved status cannot imply applied bytes. Next use must show the new content in actual tool/model input, while pending/rejected bytes remain absent. Enable/disable must preserve revision provenance. Two successive relevant experiences must target one Skill identity.

## Files and interfaces

Extend existing `internal/runtime/cognitive_skill_proposal_test.go` (branch-only before E6-01), `internal/runtime/skills_backend_test.go`, `internal/rpc/control_test.go` only for missing operation coverage. Proposed fixture `internal/app/skill_proposal_loop_test.go`, proposed browser `ui/e2e/skill-proposal-loop.spec.ts` and proposed split config `ui/playwright.skill-proposal.config.ts`. Reuse `ui/playwright.composer.config.ts` launch/model patterns with a distinct disposable workdir and no server reuse; main generic `playwright.config.ts` exercises embedded UI and is unsuitable for this split-UI proof. Existing `ui/src/components/skills/SkillsView.tsx` and proposed `SkillsView.test.tsx` own selected-content refresh behavior; existing ReviewCard already renders target/preview and choices.

Consumes E6-01 typed proposal fixture, `review/list`, `review/get`, `review/respond`, `skills/list`, `skills/get`, `skills/revisions/list`, `skills/set-enabled`, and existing `skills_list`/`skill_view` tools. Inspect live RPC schemas before coding; do not invent `skills/apply` or a new review endpoint. The current `skills/revisions/list` exposes pending revisions only; durable historical revision assertions belong to existing storage operations in the Go fixture, not an invented history endpoint/UI. Produces evidence containing target slug, revision/approval IDs, base/new hashes, source refs, actual model request and candidate identity. Secrets and unrelated task content are excluded from diagnostic artifacts.

## Tasks

### 1. True reopen and same identity

- [ ] Add `TestCognitiveSkillProposalDiskReopenLaterUse` with a disk-backed disposable store and Skill directory. Start from enabled user Skill `release-notes`; submit a typed admitted experience update. Save exact original bytes/hash and review/revision IDs.
- [ ] Close Service/backend and reopen the same disk paths before deciding the pending review. Assert one unchanged pending item, unchanged Skill bytes and preserved provenance. Approve using the normal authority; assert applied revision and target bytes. Close/reopen again; run normal Skill discovery/view and a subsequent scripted model turn invoking the Skill. Assert its actual tool result/model request contains the applied text and new hash, not old/pending text.
- [ ] Submit a second relevant experience to `release-notes`; approve and assert one Skill slug with successive revisions. Identical/replayed input adds no effective version. Toggle enabled state using normal CAS, reopen, and assert origin/source metadata remains in revision history.
- [ ] Add a branch of the same fixture for reject, stale base, expiration and failure after file-write before status update. Assert no rejected/stale content is used and restart resolves known bytes truthfully without overwriting foreign changes. Run `go test ./internal/runtime -run '^TestCognitiveSkillProposalDiskReopenLaterUse$' -count=1 -v`; if existing code fails, reproduce first and correct only E6-01's owning lifecycle seam.

### 2. Actual VIVY operation/UI loop

- [ ] Build the minimal app fixture using the existing admitted effect/Service path and scripted localhost model. It must stage via the real bridge, not insert a fake approval row. Test `TestSkillProposalLoopOperations` checks review target/diff/source, explicit decision, revisions, enable/disable and later `skills/get` against actual bytes.
- [ ] Add browser test `updates the same Skill after review and uses the accepted revision`. Open the actual Review Center pending row, inspect target and before/after diff, reject one proposal and approve a later eligible update, then open the existing Skill detail, refresh it, and launch a normal later use. Validate durable historical revisions in the Go operation fixture; no new history UI is required. Verify authoritative operation state; no RPC call that fabricates an “applied” result.
- [ ] Add source test `refresh reloads the selected Skill after an accepted revision` in proposed `ui/src/components/skills/SkillsView.test.tsx`: initial selection shows old text, backend now returns accepted text/hash, existing Refresh must update selected content and catalog. Run red first; if needed, make the minimal `loadSkills`/`openSkill` refetch correction in SkillsView, preserving component signatures and toggle errors. Existing ReviewCard supplies review affordances; no new DIVA/evolution/history UI.
- [ ] Run fixture `go test ./internal/app -run '^TestSkillProposalLoopOperations$' -count=1 -v`, UI `pnpm -C ui typecheck`, `pnpm -C ui test`, and `pnpm -C ui exec playwright test --config=playwright.skill-proposal.config.ts`. The dedicated fixture backend stages through an admitted scripted cognitive workflow, never a fake review row; split Vite uses port 3015 with isolated backend 18789, no existing-server reuse, and resolved GO_EXE/PNPM_EXE applications for Windows. Expected: test body executes on split Vite with real backend and passes. Record exact command/config, ports and test counts; a setup failure is Blocked.

### 3. Review and acceptance record

- [ ] Run `just ci` on the exact integrated candidate. Record meaningful UI screenshots/transcript plus operation IDs/hashes and actual reopen sequence in `docs/logs/<delivery-date>-issue81-skill-loop/{summary,verification,acceptance}.md` (future paths).
- [ ] Independent reviewer traces source admission -> review -> CAS write -> persisted state -> next Skill use and checks negative cases. Human acceptance reviews the actual diff/decision clarity. Record effectiveness as unverified/inconclusive unless separately demonstrated by an authorized representative real task; do not block deterministic plumbing on paid API access.
- [ ] Commit explicit paths under the human contributor. Supply parent E6 exit evidence; do not close #64/#69 or merge.

**Acceptance:** Same Skill identity accumulates reviewed revisions; approval is necessary and not confused with apply; true restart preserves status/provenance and next supported use consumes accepted content. No DIVA experience is claimed.

**Stop conditions:** No E6-01 candidate; missing real operation authority; browser environment unavailable; new proposal-generation policy or live-model effectiveness requirement. Mark the corresponding evidence row blocked and retain completed operation/reopen results.
