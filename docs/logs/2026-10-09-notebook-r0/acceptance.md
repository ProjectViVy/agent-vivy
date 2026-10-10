# R0 Acceptance

Story R0 acceptance against `docs/superpowers/plans/notebook-reports/R0.md`
and spec §9.1/§16.

| Requirement | Evidence | Status |
|---|---|---|
| `internal/reportcontract/{types,ports}.go` with `ReportRequest`, `AdmissionContext`, `ReportAdmission`, `Factory`, `FactoryInput`, `Bundle`, `ScopedActions` | `internal/reportcontract/types.go`, `internal/reportcontract/ports.go` | Done |
| `func (s *Service) StartReport(ctx, AdmissionContext, ReportRequest) (ReportAdmission, error)` | `internal/runtime/report_admission.go` | Done |
| One hidden control Session per notebook scope (trusted purpose, no synthetic user message, no Agent parent) | `reportControlSessionID` + `ensureReportControlSession`; `Session.Purpose=report-control`, `Hidden()`; `ListSessions` excludes it | Done |
| Report Run `kind=workflow, depth=0, parent_id empty, root_id=self, purpose=report` | `ValidateWorkflowAdmission` RootPurpose branch; `StartReport` admission commit | Done |
| Hidden Sessions excluded from chat/sidebar/report sources but included in recovery/administration | `ListSessions` purpose predicate; `ListSessionsForRecovery` used by `terminalRunIDs` and recovery enumeration | Done |
| `WorkflowRevision` + `AdmissionNamespace`/`RequestDigest`/`TargetKey` + root-purpose | domain + 038 migrations + `CommitWorkflowAdmission` | Done |
| Child namespace from parent Run, report namespace from control Session/scope | namespace fallback `AdmissionNamespace else parent_run_id` | Done |
| Same key/different request → idempotency conflict; distinct key/same active target → existing Run/busy | `TestReportRootAdmission` conflict + busy cases, both dialects | Done |
| SQLite admission tx serialization + PG Session row lock `FOR UPDATE` | unchanged locked admission tx; `CommitWorkflowAdmission` runs inside it | Done |
| Sealed node types collect/narrate/validate-render/persist only | `reportNodeExecutor` type check; escalation test fails closed | Done |
| No successful generation advertised through placeholder effects; unbound → capability unavailable | `TestReportExecutorRejectsEscalation` asserts `ErrAuthorityDenied` | Done |
| Epoch fences + INOFY terminal ownership preserved | executor re-verifies Run/Revision identity and terminal state; INOFY launch path shared | Done |
| Required tests present and passing | see `verification.md` | Done |
| `go test ./internal/storage/sqlite ./internal/storage/postgres -run 'TestReport.*Admission|Test.*WorkflowAdmission'` | PASS both dialects | Done |
| `go test ./internal/runtime -run 'TestReport|TestINOFY|Test.*Workflow'` | PASS | Done |
| Plugin pressure matrix | all five commands PASS | Done |
| SDK pack/Inspect on selected/omitted/backend-only recipes | 3 packs verified (default absent, no-notebook absent, reports-backend present + factory binding) | Done |
| `go generate ./internal/generated/assembly` | regenerated; default accessors return false/nil | Done |
| `just ci` | end-to-end green (run recorded in this log update) | Done |

## Scope not delivered (deferred per plan)

- Manual `vivy.reports.*` actions, `ReportSettings`, CronJob scheduling, and
  the bound generation effect are R1/R3 scope; R0 admits the lane and fails
  closed before any effect.

## Commit

- `feat(reports): add trusted root workflow admission`
