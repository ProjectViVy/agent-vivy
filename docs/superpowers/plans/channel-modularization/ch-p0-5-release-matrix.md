# CH-P0-5 Release Matrix and Epic Closeout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:executing-plans` or `superpowers:subagent-driven-development`,
> `superpowers:test-driven-development`,
> `superpowers:requesting-code-review`, and
> `superpowers:verification-before-completion`. This Story closes evidence; it
> does not redesign P0-3/P0-4 product behavior.

**Story:** CH-P0-5

**Parent:** [#42 — Channel subsystem modularization](https://github.com/ProjectViVy/agent-vivy/issues/42)

**Index:** [Channel Modularization Epic Execution Index](index.md)

**Status:** BLOCKED pending accepted CH-P0-3 and CH-P0-4 outputs

**Planning baseline:** `feat/channel-modularization@1be71c4d0892967fd8f4192abb1c2414dad9f498`; execution rebases onto the accepted CH-P0-4 aggregate head

**Immediate predecessors:** accepted CH-P0-3 backend artifacts/evidence and
accepted CH-P0-4 UI artifacts/evidence

**Requirements:** CH-R1 through CH-R7, with primary ownership of CH-R6

**Goal:** Package, inspect, exercise, compare, and roll back the complete
four-Generation Channel matrix, then publish reproducible evidence that closes
issue #42 without adding new Channel behavior.

**Architecture:** Treat each Recipe as an immutable Generation candidate. One
test-owned release harness builds twice, inspects the embedded Manifest and UI
tree, launches the executable, exercises JSON-RPC and browser surfaces, records
normalized evidence, and checks whole-Generation rollback. Existing Pack,
Inspect, Eval, Studio ledger/install/rollback, PresentationHost, and Playwright
remain the authorities; P0-5 adds no product loader or feature switch.

**Tech Stack:** Go 1.26, `vivy-sdk pack`/`inspect-artifact`, Studio
release/install/rollback, JSON-RPC/WebSocket, pnpm/Vitest/Vite, Playwright
Chromium, GitHub Actions Windows runners, Markdown/JSON evidence.

**Spec:** `docs/architecture/CHANNEL-MODULARIZATION.md` and the accepted
CH-P0-3/P0-4 logs linked by the Epic index.

## Global Constraints

- Start only when P0-3 and P0-4 are `COMPLETE` in the index and their
  acceptance logs contain no deferred blocker for this Story.
- Use `.agents/skills/vivy-plugin/SKILL.md` and `vivy-kernel-ci` for the
  release pressure matrix. Use `oil-frontend` when available for browser
  verification.
- Do not change Module/Port contracts, Channel backend/UI ownership, Recipe
  selection, config semantics, generated imports, or application behavior to
  make evidence easier.
- Product-source changes are allowed only for a verified release-blocking bug
  found by the matrix. Stop, run systematic debugging, update the owning
  P0-3/P0-4 acceptance record, and request scope approval before such a fix.
- Do not commit executables, temporary homes, browser traces containing tokens,
  or credentials. Commit normalized manifests, hashes, request-method lists,
  screenshots with no secrets, and human-readable logs.
- All platform Providers remain unconfigured in automated tests. No Telegram,
  Discord, Feishu, DingTalk, or QQ network is contacted.
- A green unit test alone is not release acceptance. Windows packaged binaries,
  browser/network evidence, deterministic rebuilds, and real Studio rollback
  must all pass.
- Rollback restores a coherent prior backend+UI Generation. It never rewrites
  settings or Journal data.

## Review Focus

- Every evidence row names the exact Recipe, commit, Generation ID, executable
  SHA-256, UI artifact SHA-256, and CI run.
- Inspect data comes from the packaged executable/artifact directory, not an
  in-memory plan.
- No-channel dependency closure and UI omission remain physical; default parity
  remains functional; backend-without-UI proves backend/UI independence;
  Telegram-only proves subset selection.
- Browser capture distinguishes HTTP asset traffic from JSON-RPC frames and
  demonstrates zero `channel/*` frames when UI is disabled/omitted.
- Rollback verifies prior embedded Manifest bytes and artifact hash, plus
  unchanged settings and durable historical Journal rows.
- The final Epic index contains no ambiguous `Ready`/`Done` claim unsupported
  by a linked acceptance artifact.

## Fixed release matrix

| Matrix ID | Recipe | Backend expectation | UI expectation |
|---|---|---|---|
| `CH-M1` | `recipes/default.vivy.yml` | Host + all five Providers | Channel UI compiled and enabled by default |
| `CH-M2` | `recipes/web-no-channels.vivy.yml` | no Host/Provider/Channel methods/dependencies | no Channel UI/catalog/assets |
| `CH-M3` | `recipes/web-channels-no-ui.vivy.yml` | Host + all five Providers and methods | no Channel UI/catalog/assets or browser Channel frames |
| `CH-M4` | `recipes/web-telegram-only.vivy.yml` | Host + Telegram only | Channel UI compiled and Telegram-only inventory |

A fifth runtime overlay case, `CH-M1-DISABLED`, launches the CH-M1 artifact
with `ui.extensions.vivy/channel-ui.enabled: false`. It is not a distinct
Generation and must retain CH-M1's Generation/artifact hashes.

## Evidence schema

Create `docs/logs/YYYY-MM-DD-channel-p0-5/release-matrix.json` with this stable
shape:

```json
{
  "commit": "full git sha",
  "ci_run": "GitHub Actions URL",
  "cases": [{
    "id": "CH-M1",
    "recipe": "recipes/default.vivy.yml",
    "generation_id": "sha256 identity",
    "rebuild_generation_id": "same identity",
    "artifact_sha256": "digest",
    "ui_artifact_sha256": "digest",
    "modules": [],
    "channels": [],
    "channel_capability_state": "UNCONFIGURED",
    "ui_extensions": [],
    "go_dependency_evidence_sha256": "digest",
    "rpc_capabilities": [],
    "rpc_methods_observed": [],
    "browser_channel_frames": 0,
    "platform_network_requests": 0,
    "result": "PASS"
  }]
}
```

The harness writes the initial normalized file; a reviewer validates it against
raw CI artifacts before it is committed. Arrays are sorted. Tokens, absolute
runner paths, timestamps, PIDs, ports, and secrets are excluded.

---

### Task 1: Add an executable four-Recipe release conformance harness

**Files:**
- Create: `sdk/internal/channel_release_matrix_test.go`
- Create: `sdk/internal/channel_release_evidence_test.go`
- Reuse: `sdk/internal/removal_conformance_test.go`
- Reuse: `sdk/internal/channel_omission_smoke_test.go`
- Verify: `sdk/internal/frontend_v1.go`

**Interfaces:**
- Consumes: production Pack/Inspect and the P0-3 dependency-build-input helper
- Produces: normalized in-memory `channelReleaseCase` rows and optional JSON
  output when `VIVY_CHANNEL_EVIDENCE_OUT` is set

- [ ] **Step 1: Write the matrix test table**

Define exact expected inventories:

| Case | Host | Providers | UI extension |
|---|---:|---|---:|
| CH-M1 | yes | dingtalk, discord, feishu, qq, telegram | yes |
| CH-M2 | no | none | no |
| CH-M3 | yes | dingtalk, discord, feishu, qq, telegram | no |
| CH-M4 | yes | telegram | yes |

For each Recipe call Pack twice into separate fresh directories. Assert equal
Generation IDs, canonical Manifest bytes, runtime Assembly source, UI Assembly
source, executable SHA-256, and final UI artifact digest. A difference is a
hard failure with the first differing field.

- [ ] **Step 2: Add exact Inspect assertions**

Inspect each packaged directory and validate:

- Module, Port edge, Grant, Channel Provider, capability state, UI extension,
  catalog, source/lock hash, and asset hash inventories;
- CH-M2 dependency closure contains no Channel implementation/platform SDK;
- CH-M3 has backend imports/dependencies but no Channel UI source/catalog;
- CH-M4 contains only Telegram plugin/platform dependencies;
- all four retain core chat/tool/session modules and supported Port evidence;
- no omitted Provider conformance row/catalog leaks into Manifest.

- [ ] **Step 3: Add executable RPC assertions**

Launch one build of every case in a fresh isolated home. Use the shared
bootstrap/WebSocket client to record initialize capabilities and results:

- all cases: health, initialize, `session/create`, ordinary deterministic
  chat/tool smoke;
- CH-M1/M3/M4: `channel/inspect` succeeds with exact compiled inventory;
- CH-M2: all three Channel methods are `MethodNotFound` and no `channel.*`
  capability exists;
- CH-M1-DISABLED uses the same CH-M1 binary/hash and projects
  `vivy/channel-ui enabled:false`;
- no process contacts a platform host.

The deterministic chat/tool smoke must use the existing fake model/tool
harness or an air-gapped fixture; never live credentials or a remote model.

- [ ] **Step 4: Add evidence serialization tests**

Test JSON canonicalization, sorted arrays, required non-empty hashes, no
absolute paths, no token-like keys/values, and exact five runtime case IDs.
Writing occurs only when the environment variable is explicitly set; ordinary
tests leave the worktree untouched.

- [ ] **Step 5: Run and commit**

```bash
go test ./sdk/internal \
  -run 'TestChannelRelease(Matrix|Evidence)' -count=1
git add sdk/internal/channel_release_matrix_test.go \
  sdk/internal/channel_release_evidence_test.go
git commit -m "test(channel): add packaged release matrix"
```

### Task 2: Add release browser and JSON-RPC network evidence

**Files:**
- Modify: `ui/e2e/channel-modularization.spec.ts`
- Create: `ui/e2e/channel-release-matrix.spec.ts`
- Modify: `ui/playwright.config.ts` only if parameterized server URLs require it
- Create: `scripts/run-channel-release-matrix.ps1`
- Create: `scripts/run-channel-release-matrix.test.mjs`

**Interfaces:**
- Consumes: four prepacked artifact directories and CH-M1 disabled config
- Produces: Playwright results plus redacted `network-evidence.json`

- [ ] **Step 1: Write the PowerShell orchestrator test first**

The Node test parses/executes the script's pure argument builder and requires
five unique loopback addresses/homes, four artifact paths, bounded health
timeouts, PID cleanup in `finally`, and no shell interpolation of Recipe or
output paths. The orchestrator must never use a production home/config.

- [ ] **Step 2: Implement packaging/server orchestration**

The script:

1. accepts a repository root and evidence directory;
2. packs CH-M1..M4 once (the Go test owns rebuild determinism);
3. copies CH-M1's config to a separate disabled home with only the UI override;
4. starts five packaged executables on assigned loopback ports;
5. waits for every `/healthz` or dumps logs and fails;
6. exports URLs to Playwright;
7. always stops every PID and redacts bootstrap tokens from retained logs.

- [ ] **Step 3: Add browser matrix assertions**

For each URL record page requests and WebSocket send/receive payload method
names, not bodies containing tokens. Assert:

| Case | Channel tab | Open tab frame | Cron Channel target | stale deep link |
|---|---:|---|---:|---|
| CH-M1 | present | inspect/get allowed | present | opens Channel |
| CH-M1-DISABLED | absent | zero `channel/*` | absent | falls to General |
| CH-M2 | absent | zero `channel/*` | absent | falls to General |
| CH-M3 | absent | zero `channel/*` | absent | falls to General |
| CH-M4 | present | inspect/get; inventory Telegram only | present, Telegram only | opens Channel |

All cases must render ordinary Settings, Cron list, and a historical
Channel-provenance fixture without console errors. Unknown historical Provider
uses generic text/icon fallback.

- [ ] **Step 4: Record backend platform-network absence**

The orchestrator supplies intentionally empty credential environment and
captures backend logs. Fail if logs or a test-owned dial observer show attempts
to the known platform hosts from default grant constraints. This is separate
from browser frame counting: CH-M1/M3/M4 may expose management methods while
starting zero platform connections when unconfigured.

- [ ] **Step 5: Run and commit**

```bash
node --test scripts/run-channel-release-matrix.test.mjs
pnpm --dir ui exec playwright test e2e/channel-release-matrix.spec.ts
git add scripts/run-channel-release-matrix.ps1 \
  scripts/run-channel-release-matrix.test.mjs \
  ui/e2e/channel-modularization.spec.ts \
  ui/e2e/channel-release-matrix.spec.ts ui/playwright.config.ts
git commit -m "test(channel): add release browser matrix"
```

### Task 3: Prove real whole-Generation rollback and data preservation

**Files:**
- Create: `sdk/internal/conformance/channel_rollback_test.go`
- Modify: `sdk/internal/conformance/rollback_test.go` only to share a private
  fixture helper
- Reuse: `internal/studiocore` release/install/rollback APIs

**Interfaces:**
- Consumes: real CH-M1 and CH-M2 packaged artifacts
- Produces: rollback proof from no-channel candidate to the prior default
  Generation without settings/Journal mutation

- [ ] **Step 1: Write the real-artifact rollback test**

Pack CH-M1 as prior and CH-M2 as candidate. Before install:

- create an isolated settings document containing disabled Telegram and Discord
  overlays with opaque unknown fields;
- create a SQLite Journal/session/message with Channel provenance and a second
  ordinary message;
- retain exact settings bytes and query both durable rows.

Release both through Studio using successful isolated Eval records, install
prior, install candidate, then call `Studio.Rollback`.

- [ ] **Step 2: Assert coherent restoration**

After rollback require:

- returned release ID equals prior release;
- installed executable SHA-256 and embedded Manifest bytes equal CH-M1 exactly;
- Inspect reports all five Providers and `vivy/channel-ui` with CH-M1's UI
  artifact digest;
- settings bytes are unchanged, including dormant opaque Provider fields;
- both historical Journal messages are unchanged and readable;
- launching restored executable produces CH-M1 initialize/UI projection and
  Channel inspect behavior; and
- no file from CH-M2 remains as active Generation state.

- [ ] **Step 3: Add failure-path cleanup**

Force candidate launch/promotion failure and assert prior install remains
active, temporary install paths are removed, and settings/Journal are
untouched. Repeated rollback is deterministic/idempotent according to existing
Studio semantics.

- [ ] **Step 4: Run existing and new rollback suites**

```bash
go test ./sdk/internal/conformance \
  -run 'TestGenerationRollbackRestoresCatalogAndLocaleIdentity|TestChannelGenerationRollback' -count=1
```

- [ ] **Step 5: Commit**

```bash
git add sdk/internal/conformance/channel_rollback_test.go \
  sdk/internal/conformance/rollback_test.go
git commit -m "test(channel): prove whole-generation rollback"
```

### Task 4: Add a required Windows release lane

**Files:**
- Modify: `.github/workflows/ci.yml`
- Verify: `justfile`
- Reuse: `scripts/run-channel-release-matrix.ps1`

- [ ] **Step 1: Add `channel-release` job**

Use `windows-latest` with a 60-minute timeout. Pin the same checkout, Go, Node,
pnpm, Chromium, and `just` actions/versions as existing lanes. Steps:

1. install frozen UI dependencies and build baseline assets;
2. run focused Go release/rollback tests;
3. run `scripts/run-channel-release-matrix.ps1`;
4. upload normalized matrix, network evidence, Playwright report, and redacted
   server logs on both success and failure;
5. upload no bootstrap token, user home, config secret, or executable unless
   repository release policy explicitly permits it.

- [ ] **Step 2: Make aggregate CI require the lane**

Add `channel-release` to `aggregate.needs`, print its result, and require
`success`. Keep existing backend, UI, and full-UI browser lanes unchanged.

- [ ] **Step 3: Validate workflow syntax and local components**

```bash
node --test scripts/run-channel-release-matrix.test.mjs
git diff --check
```

If a local workflow linter is configured, run it. Otherwise push the Story
branch and use `workflow_dispatch`; GitHub Actions execution is mandatory
evidence.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci(channel): require release matrix"
```

### Task 5: Run final pressure matrix and publish Epic acceptance

**Files:**
- Create: `docs/logs/YYYY-MM-DD-channel-p0-5/release-matrix.json`
- Create: `docs/logs/YYYY-MM-DD-channel-p0-5/network-evidence.json`
- Create: `docs/logs/YYYY-MM-DD-channel-p0-5/summary.md`
- Create: `docs/logs/YYYY-MM-DD-channel-p0-5/verification.md`
- Create: `docs/logs/YYYY-MM-DD-channel-p0-5/acceptance.md`
- Modify: `docs/architecture/CHANNEL-MODULARIZATION.md` status only
- Modify: `docs/superpowers/plans/channel-modularization/index.md`
- Modify: issue #42 only after the repository change is merged and CI is green

- [ ] **Step 1: Run the required plugin/Generation pressure matrix**

```bash
go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1
go test ./sdk/internal/conformance -run 'TestGeneration(FailureMatrixEvidence|RollbackRestoresCatalogAndLocaleIdentity|ChannelGenerationRollback)' -count=1
go test ./sdk/internal -run 'Test(GenerationFailureMatrixExecutesEveryCase|MinimalArtifactPhysicallyOmitsOptionalModules|ChannelReleaseMatrix)' -count=1
go test ./sdk/internal/assembly -run 'Test(CompilePluginV1GraphFixtures|StartFailureRollsBackEveryConstructedOwner)' -count=1
go test ./internal/toolhost -run 'TestMiddleware(TimeoutFailsClosed|PanicAndInvalidDecisionFailClosed)' -count=1
```

- [ ] **Step 2: Run focused Channel and full repository gates**

```bash
go test ./internal/channelcontract ./internal/rpccontract ./internal/modules/channel \
  ./internal/channelhost ./internal/rpc ./internal/app/settings ./internal/app \
  ./internal/moduleport ./internal/modules/... ./sdk/internal/assembly \
  ./sdk/internal ./sdk/internal/conformance -count=1
pnpm --dir ui typecheck
pnpm --dir ui test
pnpm --dir ui build
just ci
```

If `just`/PowerShell is unavailable locally, run its complete documented
equivalents and record that limitation. This does not waive the required
Windows `channel-release` and aggregate Actions lanes.

- [ ] **Step 3: Generate and validate normalized evidence**

Set `VIVY_CHANNEL_EVIDENCE_OUT` to a temporary file while running the matrix,
copy only the validated normalized result into the dated log directory, and
validate it with `channel_release_evidence_test.go`. Copy Playwright's redacted
network-method summary. Manually cross-check all hashes/IDs against uploaded CI
artifacts.

- [ ] **Step 4: Request final independent review**

Review must cover requirement mapping CH-R1..R7, scope fences, all four Recipe
inventories, disabled overlay, dependency/UI asset omission, no-request
captures, default parity, data preservation, rollback, and CI. Resolve every
Critical/Important finding and rerun affected gates.

- [ ] **Step 5: Close repository status**

Only after the required GitHub run is green:

- mark CH-P0-5 and the Epic `COMPLETE` in the index;
- change architecture status to
  `Normative architecture; CH-P0-1 through CH-P0-5 complete` without rewriting
  decisions;
- link the exact CI run, commits, matrix JSON, network JSON, and rollback test
  in `acceptance.md`;
- state any non-blocking future improvements under a separate backlog heading,
  not as hidden incomplete P0 work.

- [ ] **Step 6: Commit closeout**

```bash
git add docs/logs/YYYY-MM-DD-channel-p0-5 \
  docs/architecture/CHANNEL-MODULARIZATION.md \
  docs/superpowers/plans/channel-modularization/index.md
git commit -m "docs(channel): close modularization epic"
```

After merge to the aggregate branch and a green required run, update/close
issue #42 with links. This external issue mutation requires the release
maintainer's normal repository authority; the implementation agent must not
close it early.

## Story and Epic acceptance checklist

- [ ] CH-M1..CH-M4 each build twice with stable identities/hashes.
- [ ] Packaged Inspect inventory matches exact backend/UI expectations.
- [ ] Live RPC and deterministic chat/tool paths pass for every case.
- [ ] Disabled/omitted UI produces zero Channel browser frames and no surface.
- [ ] Unconfigured builds make zero platform-network attempts.
- [ ] No-channel dependency closure and no-UI asset omission remain proven.
- [ ] Real Studio rollback restores CH-M1 bytes/Manifest/UI identity and leaves
  settings/Journal unchanged.
- [ ] Plugin pressure matrix, `just ci`, Windows `channel-release`, browser, and
  aggregate lanes are green.
- [ ] Independent review accepts CH-R1..R7 with linked evidence.
- [ ] No product behavior or successor scope was smuggled into evidence work.
- [ ] Architecture, Epic index, acceptance logs, and issue status agree.
