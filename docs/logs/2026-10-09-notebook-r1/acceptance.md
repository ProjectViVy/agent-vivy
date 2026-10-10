# R1 acceptance

Spec §5–§10, §16 evidence:

- Users submit only `{period, window, operation_key, target?}`; the sealed
  4-node program (`report/v1`, content-hashed at admission) is owned by the
  trusted control session — no authored graph, prompt, or tools on the wire.
- Manual defaults create/read one disabled `cron_jobs` row per (scope,
  period) through Storage (`ReportSettingsJobID` deterministic id, CAS via
  `revision`); a disabled row never blocks manual generation.
- Narrative failure ⇒ deterministic `fallback` with categorized reason;
  `empty` skips the model; partial coverage is truthful (`truncated`,
  `missing_dates`, omitted count); full source failure ⇒ error; cancel ⇒
  no fallback and no generation receipt.
- Lost persist ack ⇒ the effect re-reads the committed receipt keyed by
  `report-persist-<operation_key>` inside `notebook_mutations` order; the
  generation row is `UNIQUE(scope,run_id)` and carries the full provenance.
- Publication races resolve to promoted/candidate/destination_deleted; the
  head CAS (`version` + `head_revision_id` + `deleted_at=0`) never recreates
  a deleted destination.
- Report runs are excluded from memory/cognitive capture by
  `runs.purpose='report'` (R0) and cursors keep advancing.
