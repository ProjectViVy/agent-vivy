# SSH-08 optional Generation and product acceptance implementation plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkboxes; follow the index's readiness gates before coding.

**Goal:** Ship an explicitly selected removable SSH workspace Generation with installation/recovery and real acceptance evidence.
**Architecture:** Sealed code-ssh Recipe includes the plugin and pinned worker contract; default/local Generation omits it physically.
**Tech Stack:** Existing vivy-sdk verify/pack/Inspect, Go independent modules, OpenSSH, CI and split GUI.
**Spec:** [SSH-D1](../../specs/2026-10-11-code-ssh-design.md) §8; [SSH-C1](contracts.md); baseline `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`.
**Epic / requirements:** E3 / R1-R9. State and predecessors: [index](index.md).

## Global constraints and review focus

BLOCKED until all SSH-07 predecessor evidence and H1-H6 closure exist. No default
Recipe activation, hand-edited generated imports, auto-install, service daemon,
credential commit or feature code in internal. All established first-party
features retain default composition. Inspect identity alone is not conformance.
Review nested module source closure and independent worker CI coverage.

## Prerequisites and contracts

Consume SSH-07 complete contribution set/control/GUI and transitively A01-A07.
Exact supported public Ports/versions and source hashes must be present, not
SPECIFIED placeholders. Produce `recipes/code-ssh.vivy.yml`, pinned worker
build/install/recovery docs and A08-A10. Worker is explicitly operator-installed;
no plugin pack silently installs it on a server.

## Task 1: Explicit Recipe, source closure and omission

**Files:** NEW proposed `recipes/code-ssh.vivy.yml`, `docs/dev/code-ssh.md`,
plugin `packaging_test.go` and `scripts/test-code-ssh-pack.ps1`. UPDATE source
pins/descriptors through supported tooling only. Catalog/SDK/internal changes
are not authorized by this Story; unresolved pin resolution returns to SSH-00.

- [ ] Write `TestSelectedSSHArtifactHasExactSourceClosure`,
  `TestOmittedSSHArtifactHasNoCodeUIGrantsAssets`,
  `TestLocalGenerationStartsWithoutSSH`. Test sealed selected/omitted outputs
  using existing SDK behavior, plugin-owned harness and a PATH without ssh.
  Assert imports/module manifest/assets/Grants/catalog identity; text grep alone
  is insufficient proof of physical omission.
- [ ] Run focused packaging tests for red. Define explicit Recipe sources and
  Grants using exact descriptor/source pins, allow-listed target/auth profiles
  and proc/egress constraints from the accepted Host path. Do not invent grant
  vocabulary or copy unverified SSH options into the Recipe. Include all selected
  UI/action/context/status consumers and the data-only wire source closure.
- [ ] From root run `go run ./sdk verify plugins/coding/ssh-workspace`, then
  `go run ./sdk pack --recipe recipes/code-ssh.vivy.yml --source plugins/coding/ssh-workspace --output <fresh-selected-output>`
  and `go run ./sdk inspect-artifact <selected-output>`. Output directory must
  not exist. Pack the unchanged default Recipe to a different fresh output;
  inspect both. Inspect the actual omitted artifact and start it without SSH.
- [ ] If shared wire module replacement cannot be sealed, stop and record exact
  resolver failure, supported no-new-loader alternatives and affected paths.
  No runtime discovery or whole repository import merely to hide missing closure.
- [ ] Record A08 source/Port/Grant/asset evidence and seven artifacts against
  actual current hashes. Commit as `feat: package optional code SSH Generation`.

## Task 2: Worker install, platform and CI closure

**Files:** UPDATE `docs/dev/code-ssh.md`, existing `justfile` and
`.github/workflows/ci.yml`; NEW proposed `scripts/test-code-ssh-worker.ps1`.
No changes to protected tools or ordinary runtime behavior.

- [ ] Existing plugin-ci scans plugins/faces, not extensions. Add an explicit
  worker vet/test lane and Linux process/fault execution job; preserve existing
  full CI gates. Compile Linux amd64/arm64 artifacts to fresh output paths with
  version/build digest and record exact hashes; execute both on actual compatible
  runners/targets. Cross-compilation alone leaves execution unverified.
- [ ] Document prerequisites, fixed operator-approved profile/root/state, strict
  host key setup, explicit pinned binary placement, hello digest verification,
  capability/minimum-kernel restrictions and current sandbox availability.
  Do not promise supported isolation or retained jobs without evidence.
- [ ] Document receipt/output retention and operator cleanup: reconcile all
  uncertain operations before any explicit maintenance; retain identity tombstones
  or refuse replay. No automatic receipt deletion. Explain missing worker/auth,
  unknown outcomes and how to change target only after owned operations settle.
- [ ] Execute `just ci`, SDK pressure matrix from the plugin skill, focused worker
  tests/vet, selected UI checks with VIVY_UI_RECIPE and normal default UI checks.
  Record actual Windows/macOS/Linux client and Linux arch results; unavailable
  runners stay pending. Commit as `ci: verify independent SSH worker and installation`.

## Task 3: Real remote loop and failure acceptance

**Files:** NEW proposed `scripts/test-code-ssh-live.ps1`,
`ui/e2e/code-ssh.spec.ts`, `docs/logs/YYYY-MM-DD-code-ssh-08/{summary,verification,acceptance}.md`;
UPDATE this acceptance matrix/index with actual evidence only.

- [ ] Provision an explicitly authorized disposable SSH project with distinct
  local/remote sentinels and a small real test/build task. Test target configuration
  stays outside git; never print auth secrets. Start selected backend in a fresh
  Vivy user home and Vite :3015 using code-ssh Recipe override. Use embedded sealed
  product only for the later release smoke, not the development loop.
- [ ] Through real GUI/Run: select/connect → inspect files → load remote project
  instructions → make digest-checked edit → run tests → build an artifact →
  inspect jobs/output → present → download. Compare exact remote/download digest;
  verify local sentinel unchanged and origin always visible. Exercise disconnect,
  stale status, cancellation and denied target switching from the same GUI.
- [ ] Run the entire [failure matrix](acceptance.md) with controlled remote
  disconnect/worker termination and receipt/storage injection. Count side effects
  after reply loss and reconnect; original operation executes once. A new
  intentional identity executes separately. Preserve unknown outcomes where
  cleanup/settlement cannot be confirmed; verify no blind replay/local fallback.
- [ ] Collect measurements/costs listed in acceptance without invented SLOs.
  Review A01-A10, default omission, seven artifacts and full CI. If a check is
  unavailable, mark release pending and record exact environment needed. Do not
  close #15 or publish/merge from this Story without separate conversation scope.
- [ ] Commit executable acceptance/docs as
  `test: verify code SSH remote loop and interruption recovery`. Update Story state
  only to the level actually evidenced; product release remains blocked by any
  missing platform/GUI/authority test.

## Acceptance and rollback

Return exact selected/omitted artifact identity, A08-A10 logs, full CI and real
GUI results, worker hashes/platform matrix and unresolved limitations. Revert
the optional Recipe/plugin selection to remove this capability; local defaults
require no SSH uninstall. Stop/close owned jobs first; unconfirmed jobs retain
unknown evidence. Never remove operator project files, host keys or receipt
history as an automatic rollback step.
