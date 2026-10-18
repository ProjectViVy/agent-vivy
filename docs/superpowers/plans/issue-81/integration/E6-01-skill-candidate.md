# E6-01 Existing Skill Bridge Candidate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task after implementation is authorized. Steps use checkbox (`- [ ]`) syntax. Do not create agents in this planning session.

**Goal:** Recover #69 and its exact #9 dependency onto the selected VIVY candidate while preserving one reviewed Skill authority.

**Architecture:** Reuse `submitCognitiveSkillProposal`, existing `SkillRevision` persistence and Review Center decisions. The effect producer proposes; VIVY alone validates writable ownership, source evidence, base hash, approval and application.

**Tech Stack:** Go 1.26.4; existing SQLite/PostgreSQL stores; Laputa typed evolution effect; existing Eino Skill backend.

**Spec:** [#64](https://github.com/ProjectViVy/agent-vivy/issues/64), [architecture decisions](architecture.md), branch `f2be7f614035927547571127bea73c25b516d614`. Status/dependencies: [package](package.md).

## Global Constraints

Follow architecture.md and repository AGENTS.md. No second Skill store, UI approval center, scheduler, generic strategy framework, EvoMap, autonomous approval or DIVA app work. No mutation of peer #79/#10/#80. Use the selected candidate's Eino pin without introducing Eino imports outside runtime/provider. No schema migration is planned.

## Review Focus

Replay after approval must not produce a second revision; same-target competing pending proposals must be deterministic; hash changes must preserve newer bytes; denied/expired/cancelled revisions cannot later apply; a file write followed by failed status persistence must recover truthfully. Each has a task assertion below.

## Files and interfaces

Recover branch-only `internal/runtime/cognitive_skill_proposal.go` and its `_test.go`. Reconcile branch modifications to `internal/runtime/{cognitive_binding,bundle_skills,skills_backend,service}.go`, `internal/domain/{skill,tool}.go`, `internal/tools/skills.go`, `internal/app/app.go`, `internal/storage/{sqlite,postgres}/approvals.go` and associated tests. Dependency pin files are coordinator-owned; stage only the agreed final closure.

Consumes `evolution.Effect` with `SkillProposalPayload`, existing `tools.SkillOperations` and `tools.SkillRevisionLifecycle`. Preserve:

```go
func (s *Service) submitCognitiveSkillProposal(ctx context.Context, runID domain.RunID, skills tools.SkillOperations, effect evolution.Effect) (evolution.EffectReceipt, error)
func (b *EinoSkillBackend) ApplySkillRevision(ctx context.Context, runID domain.RunID, revisionID string) (tools.SkillManageResult, error)
```

Produces the same methods and existing `recoverCognitiveSkillRevisions(ctx context.Context) error`, not a new public API. `SkillManageRequest.Evolution` carries origin, operation, approval and admitted source references in the durable revision payload. Receipt submission is not application success.

## Tasks

### 1. Reconcile exact sources and dependency

- [ ] Compare main/selected candidate against #69's 20-file delta; retain current unrelated changes. Verify #9's `KindSkillProposal`/payload and discovery/read contract exist in the selected published Laputa closure. Record VIVY/Laputa full SHAs and all three module versions/source lock in the Story log.
- [ ] Recover existing bridge code/tests selectively. Do not copy #69's older complete `app.go`, `service.go`, lock or go.mod over E5/E7/E9 changes. Stop before build if required typed contracts are absent; parent must select the compatible closure.
- [ ] Run `go test ./internal/runtime -run '^TestCognitiveSkillProposalReplayRejectExpiryAndRestart$' -count=1 -v`. Record actual subtest outcomes; the existing “restarted Service” test reuses the open backend and does not prove real disk reopen.

### 2. Pin authority, duplicate and failure behavior

- [ ] Add missing table cases in `cognitive_skill_proposal_test.go` under proposed `TestCognitiveSkillProposalAuthorityMatrix`: foreign/installed/disabled target; wrong source scope; base-hash mismatch; identical content; duplicate source replay; another pending proposal for the same normalized target; changed content after review; approve/deny/cancel/expire; injected revision-status failure after the file write. Reuse covered cases instead of duplicating them.
- [ ] Assertions: bytes stay unchanged before human approval; invalid source/ownership cannot create an effective revision; replay returns original target ref; same-target duplicate cannot create two pending effective updates; identical bytes create no effective version; stale newer bytes survive; terminal revision status agrees with actual effect; lost status acknowledgment is not reported as applied until reconciled. Validate pending behavior against the existing suppression contract, not automatic merging.
- [ ] Run `go test ./internal/runtime -run '^TestCognitiveSkillProposal' -count=1 -v` before any correction. A missing regression must fail for the observed gap; if a case passes, retain the existing behavior and record coverage, not an invented repair.
- [ ] Apply only the smallest missing lifecycle correction through existing revision/review methods. Keep `ApprovalPolicyAsk` and explicit human-only decision regardless of timeout settings. Preserve original source evidence and cause chains; no automatic decline-to-success conversion.
- [ ] Run `go test ./internal/runtime -run 'Test(CognitiveSkillProposal|EinoSkill)' -count=1`, then `go test -race ./internal/runtime -run '^TestCognitiveSkillProposal' -count=1`. Expected: every selected case executes and passes; no race or hidden skip.

### 3. Validate persistence and handoff

- [ ] Run `go test ./internal/storage/sqlite ./internal/storage/postgres ./internal/app -count=1` with disposable PostgreSQL when available; report skipped DSN-backed tests as pending. Run repository `just ci` on the resulting candidate; no isolated suite substitutes for its required gate.
- [ ] Record baseline failures separately from regressions. Recheck the PR's historical `TestCodeFaceRPCModeGoldenTranscript` timeout on this candidate; no skip/timeout inflation to declare success.
- [ ] Obtain independent review of exact effect intake, approval state transitions, CAS and crash window; commit explicit relevant paths with human attribution, then hand E6-02 the candidate SHAs and test inventory.

**Acceptance:** Existing bridge composes with main and selected dependency, denied/stale/replayed proposals behave correctly, and the shared review/Skill authority is preserved. This Story does not claim a complete real UI/later-use loop.

**Stop conditions:** Incompatible Laputa closure; unauthorized writable target; a required new public Port/schema/approval mode; evidence outside admitted scope; or unresolved conflict with peer recovery. Name only this Story's blocked task; continue unrelated packages.
