# PG-5 final review fixes

The chat WorkControlBar now offers an explicit Edit Goal action. Its form starts with the current objective and round limit, retains the GoalRef captured when editing began, and keeps the draft open after a failed or conflicting commit. The existing store and `goal/edit` RPC path perform the mutation; the reducer continues to preserve admitted rounds.

The runtime now registers an atomically admitted Goal run in process-local authority before publishing `goal.round_admitted`. A subscriber refreshing WorkView from that event can see the RunID. The durable admission transaction remains authoritative.

The UI SDK Face store contract, English and Chinese labels, and Plan/Goal product contract reflect the edit operation. No Eino pin, orchestration, generated file, replay rule, database schema, or second run path changed. The replay-scan suggestion was excluded because PLAN-GOAL-PREDESIGN §7 requires strict folding for external `after_seq`.
