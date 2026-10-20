-- Offline target transaction fragment, never embedded in runtime migrations.
-- Bind $1 to the conversion timestamp. D14 invokes this with D08 interaction,
-- checkpoint and single-terminal Run settlement in the same transaction.
UPDATE approvals
SET decision = 'expired', decided_at = $1, actor = 'upgrade_conversion',
    decision_reason = convert_to('upgrade_recovery_required', 'UTF8')
WHERE decision IN ('pending', 'approved')
  AND run_id IN (
      SELECT id FROM runs
      WHERE status IN ('accepted', 'queued', 'active')
        AND NOT EXISTS (
            SELECT 1 FROM run_events
            WHERE run_events.run_id = runs.id
              AND type IN ('run.completed', 'run.failed', 'run.cancelled',
                           'child.completed', 'child.failed', 'child.cancelled')
        )
  );
