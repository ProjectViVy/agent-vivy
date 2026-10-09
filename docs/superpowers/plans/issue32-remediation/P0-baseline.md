# P0: Baseline and traceability implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every finding, source baseline, task owner and evidence gate explicit before product implementation.

**Architecture:** The package index owns status; the written design owns behavior. Existing backlog and iteration records link to them without duplicating per-finding state. No product code or new test framework is introduced.

**Tech Stack:** Markdown, Git, repository scripts, GitHub read-only API.

**Spec:** [P0 contract](../../specs/2026-10-09-issue32-remediation-design.md#p0-baseline-and-traceability).

## Global Constraints

- Preserve one Service/Journal/policy path; no second runtime or persistence service.
- Keep Eino at `v0.9.13`; no dependency upgrades are authorized by the design.
- Plans and durable records are English. Conversation remains Chinese.
- Shared root has one write lane; overlapping product files are sequenced.
- Do not access tenant `data/vivy.db`, `data/demo/` or `data/workspaces/`.
- Product acceptance and publication are separate from design completion.

## Review Focus

- Remote main advances after planning: refresh full SHAs and recheck affected paths (P0.1).
- DIVA locks older VIVY/Laputa: record actual consumer pins separately (P0.1).
- Defect probe PASS is mistaken for a repair: classify evidence explicitly (P0.2).
- R2 is accidentally restored as core redaction: record supersession and owner decision (P0.2).
- Milestone W5/W6 is confused with finding W5/W6: maintain distinct scope labels (P0.2).

---

## Entry and exit

Entry: owner requested the work-package package. Exit: full baseline inventory,
28-row coverage, reachable phase links and declared test prerequisites; no
finding has been marked fixed. This is a documentary gate, so do not create
tests that merely mirror Markdown or run product CI for these notes.

### Task P0.1: Recheck source and execution prerequisites

**Files:**
- Read: `AGENTS.md`, `go.mod`, `laputa-source.lock.json`, applicable `.agents/skills/*/SKILL.md`.
- Read DIVA: `AGENTS.md`, `build/vivy-sources.lock.json`, `go.mod`, `go.sum`, `docs/plans/diva-next/wails/W5.md`, `W6.md`.
- Modify: `index.md` and the linked written design if reviewed identities advance.

**Interfaces:**
- Consumes: GitHub branch full commit SHAs, source-lock JSON, `git status --short`.
- Produces: exact reviewed source table and an execution environment inventory in the phase verification record.

- [ ] **Step 1:** Read both remote main heads and locks; compare them with the design's full identities. Record any changed paths relevant to the chosen task.
- [ ] **Step 2:** Verify repository cleanliness and human `user.name`/`user.email`. If another lane has work, create isolated execution checkout under the repository's worktree rules; do not stack edits.
- [ ] **Step 3:** Record actual Go/pnpm/Node/PowerShell/just versions, embedded UI prerequisite, DIVA's tracked pnpm lock SHA versus the canonical source lock, PostgreSQL test-service availability, Linux/Windows native target and microphone/voice availability. Do not read credentials into logs.
- [ ] **Step 4:** Run `git diff --check`; verify the selected plan's paths still exist and public interfaces match. Expected: no whitespace errors; source drift is either resolved or explicitly blocks affected implementation.
- [ ] **Step 5:** Commit any changed baseline/documentation with existing human attribution: `docs: refresh issue32 execution baseline`.

### Task P0.2: Reconcile finding disposition and evidence ownership

**Files:**
- Modify: `docs/TODO.md`, `docs/DEFER.MD`, `index.md`.
- Create/update: `docs/logs/YYYY-MM-DD-issue32-p0/{summary,verification,acceptance}.md`.
- Read: issue #32, issue #40, merged PR #42, existing W5/W6 records.

**Interfaces:**
- Consumes: original H1-H7/C1-C8/W1-W9/R1-R4 finding IDs and owner #40 disposition.
- Produces: 27 PLANNED repairs, R2 SUPERSEDED, one open backlog link and per-phase evidence locations. P1-P7 consume these IDs unchanged.

- [ ] **Step 1:** Check that every original ID occurs exactly once in the index coverage table and maps to a declared task. Expected: 28 rows, 5 P1, 21 P2, 2 P3; one supersession.
- [ ] **Step 2:** Link the package from one open backlog row; put R2's owner-approved supersession in the non-open board. Preserve original audit history and the faithful-data contract.
- [ ] **Step 3:** Label the disposable C1/R1/R3/R4 probes as reproductions. Record PostgreSQL/native/voice/full-CI checks that have not run without advancing acceptance.
- [ ] **Step 4:** Check local Markdown links and the phase graph. Confirm each repair has a test owner, every shared file has an integration lane, and P7 is blocked on engineering prerequisites.
- [ ] **Step 5:** Commit this concern as `docs: establish issue32 remediation traceability`. Record the documentation checks and user-visible read path.

## Handoff

P1 and P2 may begin in independent execution lanes after design/plan review.
P0 completion supplies a baseline, not approval to publish or delete transition
code. Update state only for work actually performed in the execution iteration.
