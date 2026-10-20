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

- Independent reviewer found two P2 gaps (partial target validation and interrupted
  rollback); both reproduced RED and corrected with GREEN full BML suite/vet.
- Actual converter-to-runtime bridge passes both store and artifact cases.
- Eino capability check: pinned v0.9.21 retains the existing typed ADK runner;
  this cut changes BML persistence/import identity and an ActionHost management
  payload, not prompting, tool orchestration or runtime context composition.
  No Eino substitute or imports outside runtime/provider are introduced.
- Review integration concern about quoted/renamed DDL was checked against the
  actual separately delivered converter: target runtime reopen passes.

- Final code17e3cb24: wiring/kernel pass; optional unchanged Linux ripgrep case
  skips. PG115, Linux BML113 and UI10 pass with zero skips.
- Actual alternate-port split smoke passes with a disposable home; 3015-specific
  required smoke remains unrun because a stable checkout owns the standard pair.
- Converter independent review found reconstructable RecordsDigest not checked
  and overly strict closed/acyclic graph assumptions; separate artifact owner
  is correcting them before the fragment is considered verified.
