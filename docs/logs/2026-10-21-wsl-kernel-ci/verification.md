# Verification

Candidate: stable at merge `392bc5bd93fe2286aa86509ec92a8013164d9cb4` plus the
focused pending delivery. Verification copy:
`/home/mastwet/work/vivy-issue91-stable/agent-vivy`, with its own clean sibling
Laputa `ff3936f44ff8cf08c12af2cf698c194cfe474fd3`. Toolchain: Debian WSL2,
Go 1.26.4, Node 24.21.0, pnpm 10.33.0, just 1.40.0, GCC 14.2. One heavy lane
runs at a time; dependencies and temporary homes belong to this checkout.

## Observed results

- Bootstrap fixtures: RED without the portable owner, then 9/9 pass. Cold setup,
  repeatability and refusal of dirty/wrong source, pin, module and Go version
  are exercised. No global Go proxy setting is changed.
- Execution/readiness guards: RED on false Windows and mounted-path readiness,
  then 7/7 pass. Combined Node gate: 16/16, no skips.
- `just wiring-ci`: 105 executed Go cases pass, zero skips, plus Node guards.
- `just kernel-ci`: vet passes; 1654 executed cases pass, one optional ripgrep
  case skips because ripgrep is absent in this Linux environment. The actual
  minimal Pack/Inspect/health test passes separately. It first failed on missing
  Vite in the fresh no-node_modules checkout, then passed without installing UI.
- `just d02-ci`: runtime/RPC 81, shared Faces/CODE 175, causal App 1, shutdown/
  headless 13, standalone headless 9 executed cases pass; no skips.
- Affected Web: 102 tests and typecheck pass. The first selected-UI Pack failed
  on absent repository `ui/dist`; the correction passes the actual selected-UI
  pack. The lite regression first failed on missing `index.html`. Minimal,
  lite and selected-UI Pack/Inspect now all pass (30.460s combined).
  The strengthened lite check also verifies that the executable embeds its
  published index bytes; its final focused run passes (14.338s).
- Final `just d02-web-ci`: typecheck, 102 Web cases and both selected-UI/lite
  artifact cases pass with zero skips. A temporary process-wide headless tag
  invalidated the embedded-marker check in one attempt; correcting that test
  invocation restores the actual artifact check without changing its assertion.
- Affected SDK/CODE/App/RPC vet passes.
- Ordinary strict producer comparison passes all 23 Provider suites plus update
  safety guards (69.078s). It reproduces the actual regenerated evidence. This
  separate full-Provider obligation needs real UI build inputs; its first cold
  WSL attempt failed and was stopped, then passed after `pnpm --dir ui build`
  prepared those inputs in this independent checkout.
- Generation failure matrix (all 25 handlers), recipe removal and prior sealed
  Generation rollback pass (77.205s combined).
- `just test-postgres`: 115 executed cases pass, zero skips, on real disposable
  WSL PostgreSQL. An unset DSN remains a failure.
- `just race-ci`: vet and 1654 executed cases pass under the Linux race
  detector; the same single optional ripgrep case skips. No race is reported.

## Review and limits

Independent read-only review found the lite base-shell regression and false
scoped readiness. Both have observed failing regressions and passing fixes.
The narrow follow-up found no remaining substantiated issues, including cold
overlay handling. No duplicate broad test run was requested from the reviewer.

The single ripgrep skip is not a pass. It covers an optional protected search
implementation, not minimal composition or the D02 framing/Face contract.
The same unchanged case passed in native focused verification. Full-product
release/default/platform gates remain separate; no Windows full-CI rerun or
successful Linux `just ci` is claimed. Historical failed attempts remain in
their original records.

Disposable PostgreSQL is a task-owned postgres:16 container in WSL Docker,
not a tenant database. Native Windows acceptance evidence for the unchanged
D02 protocol and split-browser causal smoke remains in the D02 logs.
