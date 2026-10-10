# R3 acceptance

- `vivy.reports.settings.write` replaces settings under revision CAS only; the commit is receipt-backed (`report.settings` on `notebook_mutations`), so a retried write converges on one durable outcome and a divergent payload on the same key is `idempotency_conflict`.
- Report cron rows carry `Payload.Kind = report`; the scheduler's typed dispatch calls the same `StartReport` admission as manual generation — same operation key (`cron.report.<job>.<nextRunAtMs>`) yields one Run across manual trigger, loop fire, and crash re-fire. `updateCron`/`deleteCron` refuse report rows; no generic cron mutation can author or rewrite a report schedule.
- Bounded catch-up: at most the latest missed completed period generates per restart; skipped windows ride the committed run input and surface via `ReportResult.SkippedWindows`/`GenerationProvenance.Skipped`. Current/incomplete windows never publish.
- Crash recovery: committed receipts reconcile on restart; fenced resume keeps the committed revision lineage; an ambiguous model outcome is `recovery_required`, never replayed; a generation omitting `vivy/reports` fences active report runs as `recovery_required` — no generic executor touches them.
- Documented distinctions: schedule disable (`enabled=false` — settings durable, no fire), capability disable (module present, unbound — committed runs recoverable), UI omission (module absent — in-flight runs fenced). No silent re-billing after ambiguous inference: recovery-required propagates instead of minting a second revision.
- Settings UI reuses the R2 surface: expected-revision save, conflict re-read, EN/ZH; capability-absent generations hide the controls.
- PG parity is non-skipped evidence: `CommitReportSettings` parity test passed on the real vivy-pg Postgres.
