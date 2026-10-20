# Execution ledger

Plan: approved Data and Feature Lane D12 -> D09 -> D10 -> D11.
Worktree: codex/issue91-data-features, isolated from stable root.

- D12 independent BML and direct consumer implementation: in progress.
- Shared application runner/preflight integration: waiting for D03 commit.
- D09: waiting for accepted D03.
- D10: waiting for accepted D08/D12; exact Laputa pin has required digest APIs.
- D11: waiting for accepted D03/D08 and hash-free INOFY upstream delivery.
- D14 complete finite converter and final release acceptance: separately owned.

The manifest is the one durable owner of admitted import inputs and allocations;
records.json is a checked generated output, not an independent editable authority.
No new registry, runtime parser negotiation, or migration runner is introduced.
