# R0 — Trusted Root Report Workflow Admission

## Scope

Deliver the trusted root report Run admission seam from
`docs/superpowers/plans/notebook-reports/R0.md` and spec §9.1/§16: a hidden
report-control Session per scope, a depth-0 self-rooted `purpose=report`
workflow Run, sealed `report/v1` strategy admission, namespace-typed
idempotency + busy-target arbitration, observer exclusion, and the optional
`vivy/reports` module bound through the closed `core/report-service@v1` port.
R0 advertises no generation output: sealed nodes fail closed until R1 binds
the report effect capability.

## Delivered

- `internal/reportcontract` — `ReportRequest` (period, explicit window
  selector, operation key, optional target), `AdmissionContext`,
  `ReportAdmission{RunID, Created, Rejoined, Busy}`, `Factory`,
  `FactoryInput`, `Bundle{Service, AttachAdmission, Close}` + scoped actions,
  error codes (`invalid_request`, `not_found`, `idempotency_conflict`,
  `capability_unavailable`, `busy`, `unavailable`).
- Domain: `Run.Purpose` (`report`), `Session.Purpose` (`report-control`) +
  `Hidden()`, `WorkflowRevision.{RootPurpose, AdmissionNamespace,
  RequestDigest, TargetKey}`.
- Migration 038 both dialects: `runs.purpose`, `sessions.purpose`;
  `workflow_revisions` gains `parent_run_id NULL`, `admission_namespace`,
  `request_digest`, `target_key`, `root_purpose` with
  `UNIQUE(admission_namespace, operation_key)`; Postgres drops the
  `workflow_revisions_parent_run_id_fkey` constraint so report roots admit
  `parent_run_id = ''`. Backfill maps child namespace = parent_run_id and
  request_digest = descriptor_digest.
- Storage: `ValidateWorkflowAdmission` branches on `RootPurpose` (depth-0
  self-root, empty parent) vs existing parent-child invariants; namespaced
  `GetWorkflowRevisionByNamespace`; same-lock busy-target check (JOIN runs
  non-terminal on `target_key`) returning `WorkflowAdmissionResult.Busy`;
  `ListSessions` excludes non-empty purpose; `ListSessionsForRecovery` added
  to `SessionStore` + both dialects for recovery/admin enumeration; all run
  readers/projections carry `purpose`.
- Runtime: `TrustedStrategyReport = "report/v1"`, sealed linear program
  collect → narrate → validate-render → persist (`vivy-report/0`),
  `reportStrategyAdmission` compiled against `reportStrategyCatalog`;
  `reportNodeExecutor` verifies call binding, sealed node type/impl, Run
  kind/purpose/terminal, revision SchemaVersion/ProgramDigest/HostBindingID/
  RootPurpose, then fails closed with `ErrAuthorityDenied` "capability is
  not configured" — no placeholder success is advertised.
- `Service.StartReport(ctx, AdmissionContext, ReportRequest)`: validates the
  request, ensures one hidden control Session per scope, computes
  RequestDigest (scope + period + window + target) and TargetKey
  (`period/window/section/entry`), then inside `workflowStartMu` +
  `projectionMu` performs dedup replay (same namespace + key + digest →
  `Rejoined`), idempotency conflict (same key, different digest →
  `CodeIdempotencyConflict`), busy target (`Busy: true`), commits
  `CommitWorkflowAdmission`, activates the ledger/session maps, and launches
  the INOFY workflow. Recovery re-admits report roots without Cognitive via
  `recoverWorkflowRun`'s `RootPurpose` branch.
- Observer exclusion: `ingestExclusionPredicate(backend)` excludes Runs with
  `purpose=report` (or tool-ops exclusion) from memory and cognitive capture
  while cursors still advance; report Sessions never appear in
  chat/sidebar listings.
- Module seam: closed `core/report-service@v1` (owner `vivy/reports`,
  AtMostOne, conditional); `internal/modules/reports` (owner/Instance,
  `AttachAdmission` injection, fails closed `capability_unavailable` without
  an admission port); `defaults/catalog.go` record + `Binding.ReportFactory`
  + `Requires` on `core/action-host@v1` + `core/notebook-service@v1`;
  `GoBinding.ReportFactory` through `source.go`, `frontend_v1.go`,
  `generate-default`, `runtime_generate.go` (emits
  `ReportFactory reportcontract.Factory` + `HasReportFactory()` /
  `ReportFactoryValue()` accessors); `zz_default.go` regenerated (absent by
  default); `internal/app/assembly_reports.go` wires the factory into the
  App and attaches the admission port.
- `recipes/reports-backend.vivy.yml` — notebook-core + reports only, no
  notebook-tools: a backend-only report surface for pack/Inspect evidence.

## Decisions

- Admission namespace resolution: `AdmissionNamespace` when set, else
  `parent_run_id` — so legacy children and report roots share one UNIQUE
  index and one dedup/scan path; no second admission implementation.
- RequestDigest excludes the operation key on purpose: same key + different
  request is detected as `RequestDigest` mismatch → idempotency conflict.
- Busy target is a committed-transaction result flag (`Busy: true`), not an
  error, so the runtime maps it to `ReportAdmission.Busy` without a second
  admission round.
- The control Session ID derives deterministically from the scope
  (`sha256("vivy.report.control" + NUL + scope)`), giving exactly one
  hidden session per scope without a lookup table.
- Report runs keep `kind=workflow`, `depth=0`, `parent_id=''`,
  `root_id=self`, `purpose=report`; the sealed program is registered through
  `authority.TrustedStrategy` so recovery and node dispatch share the
  existing workflow fences.
