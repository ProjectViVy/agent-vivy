# Verification

Baseline: e2c1518e; native worktree C:/Users/Administrator/.codex/worktrees/issue91-d06/agent-vivy.
Independent Linux clone: /home/mastwet/work/vivy-issue91-d06/agent-vivy, with own
Laputa ff3936f44ff8cf08c12af2cf698c194cfe474fd3.
WSL tools: Go 1.26.4, Node 24.21.0, pnpm 10.33.0.

Observed: explicit identity and source-without-hash regressions RED before change,
then GREEN under `go test -tags vivy_headless ./sdk/internal/assembly -run TestD06`.
UI-without-hash regression RED on missing_hash before producer retirement.
Locked UI dependency install passed. Frontend regression initially could not
collect because actual Module staging was not yet built; no behavioral result claimed.

Generation-wide suites, Recipe matrix, scoped gates, browser/native acceptance
and independent final review are pending. Source changes invalidate prior
acceptance where affected. Historical D01/D02 failures remain historical evidence.

## Integration checkpoint

Strict executed Provider/Host reproduction passed in the independent Linux clone
(69.169s, no update flag). Full SDK execution exposed missing selected CODE/LSP
module requirements; the packer now derives those requirements from selected
source owners. The complete matrix is being rerun. Wiring passed 16 script tests
and 105 executed assembly tests, zero skips. The first kernel run failed
TestINOFYWorkflowCancelPropagates (engine status cancelled instead of recovery_required);
this failure is retained and kernel acceptance is not yet claimed.

Native Studio conversion/ledger/service suites passed after a native RED exposed
Windows file URI authority parsing and path-form assertions. Cross-platform
evidence-path rejection is covered by native conformance regressions. Independent
behavior/residue reviews found those path assertions, SDK Inspect DTO residue and
obsolete comments; fixes are included. The native UI build remains blocked by
esbuild temporary-file Access denied; actual Linux typecheck, 654 Web tests and
production UI build passed. No full-product release CI is claimed.

D03 became available as committed c065a82b during this cut; it will be integrated
sequentially before the shared Generation schema retirement. No root edits were copied.
