# Acceptance

1. Start the split pair (`just dev`) and send a turn that spawns sub-agents
   through the `agent` tool, then let the run finish.
2. Open `session/context` for that session (chat input ring tooltip, or the
   compaction card in settings). `feed_tokens` / `model_limit_tokens` must
   describe the parent transcript only; the percentage must not jump because
   sub-agent transcripts exist. `total_messages` still counts every stored row,
   including child transcripts.
3. Send the next turn. The request's `model.request` v5 row must list only the
   parent history plus the current user message; no child assistant/tool rows.
   The sub-agent's answer remains visible in the transcript as its `agent` tool
   result.
4. A session whose history contains a child run and a legacy row (no run id)
   still feeds the legacy row — the filter never hides pre-lineage history.
