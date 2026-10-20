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

## Sequential integration follow-up

D03 c065a82b is integrated as398fe41b; the only conformance conflict was its
retired sourceSha256 values, so all executed outcomes were retained unchanged.
The integrated wiring gate passes, and kernel vet plus1711 executed cases
pass (one explicitly skipped ripgrep case); minimal Pack/Inspect/boot also passes.
Affected d02-ci and d02-web-ci pass, including102 Web tests and both actual
selected-UI/lite artifact checks. Prior failed attempts remain failures.

All nine executable Recipe cases pass (default, lite, minimal, headless,
vivy-code, scx, masks-selected, masks-omitted, masks-backend-only), as does
DIVA shared. Go-host tests hit the verification copy's old Git index naming
a deleted sourcehash file; the dedicated Linux checkout has now been fetched
and reset to the exact committed integration candidate via a native bundle.
The final full SDK run will use that exact checkout.

Historical App/settings/embedded/CODE retries passed. The external public API
fixture first failed on default proxy checksum-network timeout; direct fetching
was stopped after a hang. It then passed3.872s with process-scoped
GOPROXY=https://goproxy.cn,direct and unchanged GOSUMDB=sum.golang.org.

Actual native entry points `go generate ./internal/generated/assembly` and
`pnpm --dir ui run stage:ui` each ran twice with explicit authoring-default:
all77 emitted files were byte-identical on pass2. A first pnpm attempt lacked
Go on subprocess PATH; adding the installed toolchain fixed invocation only.
Native shell privacy integration passes after reconciling its stale pre-D03
expected label with 'bash (script redacted)'; the raw-script negative check stays.

Real split Vite3015 browser: Lifecycle displays authoring-default with no hash
rows, refresh preserves it, and backend-confirmed English-to-Chinese language
change renders the compiled catalog after reload. Screenshots accompany this log.
Home/config are disposable /tmp/vivy-d06-browser, never tenant data; both owned
split processes stopped after the walkthrough.
