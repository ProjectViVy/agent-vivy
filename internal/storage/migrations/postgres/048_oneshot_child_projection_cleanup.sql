-- One-shot child transcripts are internal execution detail: their Journal is the
-- source of truth and they must never appear in the session conversation.
DELETE FROM messages
WHERE run_id IN (
  SELECT id FROM runs
  WHERE kind = 'child' AND child_mode <> 'continuable'
);
