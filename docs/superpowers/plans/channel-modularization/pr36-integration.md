# PR #36 Channel Arc Integration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans`, plus `superpowers:test-driven-development`,
> `superpowers:requesting-code-review`, and
> `superpowers:verification-before-completion`. Use
> `superpowers:using-git-worktrees` before editing. Implement this integration
> gate only; do not begin CH-P0-3, CH-P0-4, or CH-P0-5.

**Integration ID:** CH-INT-36

**Parent delivery:** [#42 — Channel subsystem modularization](https://github.com/ProjectViVy/agent-vivy/issues/42)

**Source:** [PR #36 — channel arc](https://github.com/ProjectViVy/agent-vivy/pull/36)

**Index:** [Channel Modularization Epic Execution Index](index.md)

**Status:** READY

**Source lock:** `feat/channel-tier1@eb8fee3c0996746e93def297d700c70fb65b8540`

**Source merge base:** `fe60b1869a166490da72a07eef8f1d754caa2e3e`

**Target code baseline:** `feat/channel-modularization@0c47eb96c733a29be25cace9cd048be0c367ca10`
(tree `27d50ea95dd58fecdc0a5cdd5f1e2ede6ffd0d9a`; later planning-only
commits on the aggregate branch are allowed)

**Immediate predecessor:** CH-P0-2, accepted at
`docs/logs/2026-09-19-channel-p0-2/acceptance.md`

**Relationship to #42:** This is an integration prerequisite, not a sixth
product Story and not a new issue #42 requirement. It changes the behavioral
baseline that CH-P0-3 must preserve.

**Goal:** Merge the complete, source-locked PR #36 Channel arc into the
aggregate branch while preserving CH-P0-1/2's canonical build-owned owner,
generic RPC contribution boundary, current `main` behavior, and later Story
scope fences.

**Architecture:** Preserve the PR's behavior and history with one real merge,
but adapt its old direct app/RPC ownership to the already-landed
`channelcontract.Factory` / `channelcontract.Owned` seam. The canonical
`internal/modules/channel` owner receives the durable-delivery, media,
approval, health, capability, and interaction dependencies; it remains the
only package that owns `internal/channelhost.Host` and all `channel/*`
management bindings. App supplies focused authorities and the one
`Service.RunWithOptions` callback; RPC core remains Channel-agnostic.

**Tech Stack:** Go 1.26, SQLite/PostgreSQL storage conformance, Vivy Module and
Port v1, generated Assembly evidence, TypeScript/React/Vitest/Playwright,
Git/GitHub Actions.

**Specs:**

- `docs/architecture/CHANNEL-MODULARIZATION.md` — ownership and omission
  authority; CH-03 through CH-07, CH-11, and CH-12 are non-regression gates.
- `docs/architecture/VIVY-CHANNEL-PACK.md` — PR #36 behavior authority.
- `.agents/skills/vivy-plugin/SKILL.md` and
  `.agents/skills/vivy-kernel-ci/SKILL.md` — packing, evidence, and final gate.

## Global Constraints

- Fetch and merge the exact source SHA above. If `origin/feat/channel-tier1`
  points elsewhere, stop and ask the supervisor to revise the source lock;
  never silently absorb later commits.
- Start from the then-current `origin/feat/channel-modularization` containing
  this plan. A planning-only descendant of the target baseline is valid; a
  product-code descendant must be reconciled and reviewed before the merge.
- Use a dedicated `feat/channel-pr36-integration` worktree. Do not merge PR
  #36 to `main`, close it, retarget it, or change its metadata in this Story.
- Preserve the source branch's 69-commit ancestry with a merge commit. Do not
  squash, cherry-pick the batches, or use `-Xours`, `-Xtheirs`, or a blanket
  checkout of either side.
- Preserve the current provider registry, workflow plans, P0-1/2 contracts,
  startup cleanup, settings authority, generated factory, and generic RPC
  contribution implementation. Source-branch CI proves only the source head;
  it is not acceptance evidence for the integrated tree.
- `internal/app` may import `internal/channelcontract` but not
  `internal/channelhost` or `internal/modules/channel`. `internal/rpc` may
  import `internal/rpccontract` but must contain no Channel Host import,
  Channel-specific DTO, `channel/*` switch case, or hard-coded Channel
  capability.
- `internal/modules/channel` remains the sole owner of Provider binding,
  Channel Host construction/lifecycle, Channel-specific settings projection,
  inspection, failed-delivery management, and all five Channel RPC bindings.
- Inbound work must use the one `Service.RunWithOptions` path. Approval
  decisions must use the one `Service.DecideApprovalAsActor` path. Do not
  create a second run, delivery, settings, or RPC dispatcher path.
- Keep `std/channel@v1` and all existing platform Module descriptors source
  compatible. This gate may extend only internal composition contracts needed
  to carry the already-implemented PR #36 behavior.
- Do not add reduced Recipes, conditional backend imports, Channel UI Module
  packaging, generated UI selection, release matrix lanes, or rollback work.
  Those remain CH-P0-3 through CH-P0-5.
- Conformance hashes are outputs. Never choose either conflict side's
  `sourceSha256`; execute the reproduction suite against the final tree and
  rotate only values it proves stale.
- Do not use real bot tokens. Loopback platform stubs and the split browser
  path are the acceptance environment; record real-platform smoke as skipped
  for missing credentials rather than claiming it.

## Review Focus

- A fast terminal run must not publish a terminal event before its durable
  delivery intent and live target are armed; a failed prepare callback must
  leave no user message, run row, or run event.
- Restart recovery may redeliver only through a compiled and actually started
  Provider. Missing or inactive Providers keep the intent recoverable without
  panic or retry-budget consumption.
- Optional capability discovery must traverse the Module-owned
  `providerChannel` wrapper to the live adapter after Start, while its
  Definition-owned rune limit remains visible before Start.
- Channel media, approval commands, typing, placeholder, and reaction surfaces
  must still settle on every terminal, Stop, and TTL path without entering the
  Journal or delivery ledger as a second source of truth.
- The final conformance bundle must bind the exact integrated `internal/` and
  five Provider source trees; source PR hashes and aggregate hashes must not be
  mixed by hand.

## Verified Planning Evidence

| Fact | Verified value | Planning consequence |
|---|---|---|
| PR state | Open, not draft, not merged | This plan integrates its head into the aggregate branch only. |
| Source head | `eb8fee3c0996746e93def297d700c70fb65b8540` | Exact immutable merge input. |
| Source size | 69 commits, 114 files, +13,436 / -757 | One ancestry-preserving merge is cheaper and safer than replaying batches. |
| Source CI | [GitHub Actions CI run 35022843667](https://github.com/ProjectViVy/agent-vivy/actions/runs/35022843667) / run #172: backend, UI, full browser smoke, and aggregate `just ci` all passed | Valid source evidence, to be rerun on the integrated tree. |
| Source reviews | No submitted review or unresolved review thread returned by GitHub | No external review decision is inherited; integration still requires its own review. |
| Current merge result | Git reports a non-clean merge | Integration cannot be represented as a direct PR merge. |
| Double-touched paths | 13 | Review all 13 even when Git auto-merges them. |
| Text/rename conflicts | 8 | Resolve from the ownership rules below, not line preference. |

The planning merge-tree command was:

```bash
git merge-tree --write-tree --name-only \
  feat/channel-modularization eb8fee3c0996746e93def297d700c70fb65b8540
```

It reported these conflict paths:

| Conflict path | Resolution authority |
|---|---|
| `docs/COMPLETE.MD` | Union current completed work with PR #36 completion rows; current records never disappear. |
| `internal/app/app.go` | Keep `channelcontract.Owned`; adapt PR behavior through focused dependencies and callbacks. |
| `internal/app/channels_test.go` | Keep deleted; move still-valid capability tests to `internal/modules/channel/binding_test.go`. |
| `internal/modules/channel/binding.go` | Keep Module ownership and add PR capability-target behavior here. |
| `internal/rpc/control.go` | Keep RPC core generic; move delivery/health surface into Module contribution. |
| `internal/rpc/control_test.go` | Keep generic/P0 tests; move Channel behavior tests into Module/Host tests. |
| `sdk/internal/assembly/conformance_results.json` | Regenerate from the final integrated sources. |
| `sdk/internal/conformance/reproduction_test.go` | Keep dynamic final-tree internal digest; preserve PR Provider cases without a hard-coded old internal hash. |

The remaining double-touched paths require semantic review even when Git
auto-merges them:

```text
docs/TODO.md
scripts/i18n-cross-face-contract.json
sdk/ui/src/module.ts
ui/src/i18n/en.ts
ui/src/i18n/zh.ts
ui/src/lib/api.ts
```

## Minimal Internal Contract Delta

The integration extends the existing private composition contract; it does
not expose a new public Port.

```go
// internal/channelcontract/contract.go
type PrepareRunCallback func(domain.RunID) error

type RunCallback func(
    context.Context,
    domain.SessionID,
    string,
    []domain.Attachment,
    *domain.Provenance,
    PrepareRunCallback,
) (domain.RunID, error)

type DecideApprovalCallback func(
    context.Context,
    string, // approval ID
    string, // approved | denied
    string, // server-attributed actor
) error
```

`channelcontract.Dependencies` additionally carries the existing focused
authorities that PR #36 requires:

```go
Deliveries    storage.ChannelDeliveryStore
Maintenance   storage.ChannelMaintenanceStore // optional retention extension
Approvals     storage.ApprovalStore
Runs          storage.RunStore
DecideApproval DecideApprovalCallback
```

`channelcontract.Owned` does not grow a public management method. Its existing
`RPCBindings()` contribution closes over the private owner and produces:

```text
channel/inspect                -> channel.inspect
channel/get                    -> channel.get
channel/update                 -> channel.update
channel/deliveries/list        -> channel.deliveries.list
channel/deliveries/redeliver   -> channel.deliveries.redeliver
```

The generated Assembly still supplies one `channelcontract.Factory`. The
Module adapts the private callbacks to `channelhost.RunPreparedFunc` and
`channelhost.DecideApprovalFunc`; app and RPC never receive the concrete Host.

---

### Task 1: Freeze inputs and open the ancestry-preserving merge

**Files:**
- Create during execution: `docs/logs/2026-09-20-pr36-channel-integration/summary.md`
- Create during execution: `docs/logs/2026-09-20-pr36-channel-integration/verification.md`
- Create during execution: `docs/logs/2026-09-20-pr36-channel-integration/acceptance.md`
- Verify only: repository state and source refs

**Interfaces:**
- Consumes: current aggregate head and exact PR #36 source head
- Produces: one unresolved merge worktree with a recorded target SHA, source
  SHA, merge base, source file count, and exact conflict list

- [ ] **Step 1: Create the isolated execution worktree**

From a clean repository checkout:

```bash
git fetch origin main feat/channel-tier1 feat/channel-modularization
git worktree add ../agent-vivy-pr36-integration \
  -b feat/channel-pr36-integration origin/feat/channel-modularization
cd ../agent-vivy-pr36-integration
```

Expected: the new worktree is clean and contains this plan. Do not reuse the
planning worktree or an implementation worktree for another Story.

- [ ] **Step 2: Pin and verify the merge inputs**

```bash
PR36_SOURCE=eb8fee3c0996746e93def297d700c70fb65b8540
INTEGRATION_TARGET=$(git rev-parse HEAD)
PR36_BASE=$(git merge-base "$INTEGRATION_TARGET" "$PR36_SOURCE")
test "$(git rev-parse origin/feat/channel-tier1)" = "$PR36_SOURCE"
test "$PR36_BASE" = fe60b1869a166490da72a07eef8f1d754caa2e3e
test "$(git diff --name-only "$PR36_BASE".."$PR36_SOURCE" | wc -l | tr -d ' ')" = 114
```

Expected: all commands exit 0. Record all three SHAs in `verification.md`.
If the aggregate contains product commits after the plan baseline, stop and
have the supervisor classify them before merging.

- [ ] **Step 3: Reproduce the known conflict set without changing the index**

```bash
git merge-tree --write-tree --name-only "$INTEGRATION_TARGET" "$PR36_SOURCE"
```

Expected: exit 1 and exactly the eight conflict paths in the planning table.
Additional conflicts are a plan change, not an improvisation point.

- [ ] **Step 4: Start the real merge without strategy shortcuts**

```bash
git merge --no-ff --no-commit "$PR36_SOURCE"
git diff --name-only --diff-filter=U
```

Expected: merge stops for manual resolution and lists the same eight paths.
Record the output before editing. Do not stage a resolution yet.

### Task 2: Adapt durable ingress and lifecycle to the canonical owner

**Files:**
- Modify: `internal/channelcontract/contract.go`
- Modify: `internal/channelcontract/contract_test.go`
- Modify: `internal/modules/channel/module.go`
- Modify: `internal/modules/channel/module_test.go`
- Modify: `internal/modules/channel/binding.go`
- Modify: `internal/modules/channel/binding_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/channel_owner_test.go`
- Verify deleted: `internal/app/channels.go`
- Resolve as deleted after moving valid tests: `internal/app/channels_test.go`
- Accept and verify source behavior: `internal/channelhost/*.go`
- Accept and verify source behavior: `internal/attachment/*.go`
- Accept and verify source behavior: `internal/runtime/service.go`
- Accept and verify source behavior: `internal/runtime/approval_test.go`
- Accept and verify source behavior: `internal/runtime/provenance_test.go`

**Interfaces:**
- Consumes: the private contract delta above, PR #36's
  `channelhost.RunPreparedFunc`, delivery/approval stores, and one runtime
  Service
- Produces: one Module-owned `channelhost.Host`, durable pre-start arm, media
  attachment forwarding, attributed approvals, retention, and idempotent
  lifecycle through the existing `channelcontract.Owned`

- [ ] **Step 1: Resolve and extend the contract tests first**

In `internal/channelcontract/contract_test.go`, make the fake callback accept
attachments and `PrepareRunCallback`, call the prepare function with a known
run ID, and compile-check `Deliveries`, `Maintenance`, `Approvals`, `Runs`, and
`DecideApproval`. Keep the `Owned` interface unchanged.

In `internal/modules/channel/binding_test.go`, move the source branch's
capability-wrapper case from `internal/app/channels_test.go` and express it
through `BindProviders`. Require:

```go
caps := channelhost.Discover(bound[0])
if !caps.Typing || !caps.Health { t.Fatal(caps) }
limited := bound[0].(channelport.RunesLimiter)
if limited.MaxMessageRunes() != 1234 { t.Fatal(limited.MaxMessageRunes()) }
```

Add an AST/import assertion in `internal/app/channel_owner_test.go` that
`internal/app` imports neither `internal/channelhost` nor
`internal/modules/channel`.

- [ ] **Step 2: Create a compilable target-authoritative RED skeleton**

Resolve `internal/modules/channel/binding.go` to the current Module-owned
version without adding the source branch's `capabilityTarget` field or method.
Resolve `internal/app/channels_test.go` as deleted after moving its valid test.
This removes conflict syntax without implementing the behavior under test.
Run:

```bash
go test ./internal/channelcontract ./internal/modules/channel \
  -run 'Test.*(Dependencies|Capability|Prepared)' -count=1
```

Expected: FAIL because the contract lacks the new dependencies/callback shape
or the Module wrapper hides the adapter capability target. Record the named
failures.

- [ ] **Step 3: Extend only the private composition contract**

Implement the exact `PrepareRunCallback`, `RunCallback`, and
`DecideApprovalCallback` signatures from this plan. Add the five focused
dependency fields. Do not import `internal/channelhost` in the contract and do
not expose the concrete Host through `Owned`.

- [ ] **Step 4: Move PR capability propagation into the Module binding**

Keep `internal/modules/channel/binding.go` and its `channelport` names. Add
`capabilityTarget any` to `providerChannel`, capture a Provider's optional
`channelport.CapabilitySource` at bind time, and implement:

```go
func (c *providerChannel) CapabilityTarget() any {
    if source, ok := c.instance.(channelport.CapabilitySource); ok {
        if target := source.CapabilityTarget(); target != nil {
            return target
        }
    }
    return c.capabilityTarget
}
```

Do not restore `internal/app/channels.go`. Keep Grant enforcement and opaque
settings conversion in the Module-owned files.

- [ ] **Step 5: Construct the PR #36 Host behind the factory**

In `internal/modules/channel/module.go`, pass the new focused authorities to
`channelhost.New`. Adapt contract callback types explicitly; do not make app
depend on Host callback types. The resulting shape is:

```go
host := channelhost.New(channelhost.Deps{
    Journal: deps.Journal, Messages: deps.Messages, Sessions: deps.Sessions,
    Deliveries: deps.Deliveries,
    RunPrepared: func(ctx context.Context, sessionID domain.SessionID, text string,
        attachments []domain.Attachment, provenance *domain.Provenance,
        prepare channelhost.PrepareRunFunc) (domain.RunID, error) {
        return deps.Run(ctx, sessionID, text, attachments, provenance,
            channelcontract.PrepareRunCallback(prepare))
    },
    Approvals: deps.Approvals, Runs: deps.Runs,
    DecideApproval: channelhost.DecideApprovalFunc(deps.DecideApproval),
    Channels: providers, Config: configured,
    Credentials: deps.Credentials, Logger: deps.Logger,
})
```

Require `Deliveries` whenever process availability can start ears. Keep
approval dependencies optional as the PR Host specifies. In `owned.Start`,
run the PR's non-fatal inbound-retention prune through optional
`deps.Maintenance` before `host.StartAll`; use the existing retention constant,
logger, and post-recovery lifecycle position. `WithoutEars()` must construct
the owner and expose compiled inventory without pruning, networking, or
starting live surfaces.

- [ ] **Step 6: Wire app only through `channelcontract.Dependencies`**

Resolve `internal/app/app.go` by retaining `channelOwner` and deleting the
source-side `channelHost` variable, direct Host construction, direct prune,
direct start, direct shutdown, and concrete Host RPC dependency. Supply:

```go
Run: func(ctx context.Context, sessionID domain.SessionID, text string,
    attachments []domain.Attachment, provenance *domain.Provenance,
    prepare channelcontract.PrepareRunCallback) (domain.RunID, error) {
    if svc == nil { return "", errors.New("app: runtime service is not wired") }
    if len(attachments) > 0 {
        info := svc.GetModelInfo(ctx)
        if info.ContextWindow > 0 && !info.SupportsImages {
            channelName := ""
            if provenance != nil { channelName = provenance.Channel }
            logger.Warn("app: dropping channel image attachments; the active model does not support images",
                "session", string(sessionID), "channel", channelName,
                "attachments", len(attachments))
            attachments = nil
        }
    }
    return svc.RunWithOptions(ctx, sessionID, text, runtime.RunOptions{
        Provenance: provenance,
        Attachments: attachments,
        BeforeStart: prepare,
    })
},
Deliveries: backend, Maintenance: backend,
Approvals: backend, Runs: backend,
DecideApproval: func(ctx context.Context, approvalID, decision, actor string) error {
    if svc == nil { return errors.New("app: runtime service is not wired") }
    return svc.DecideApprovalAsActor(ctx, approvalID, decision, "", actor)
},
```

Keep the current startup cleanup stack, owner Run hook, owner deliverer,
post-`svc.Recover` Start/Ready order, and Stop/Close order. Do not copy the
source branch's older manual cleanup pattern.

- [ ] **Step 7: Prove the contract, Module, and private Host GREEN**

```bash
go test ./internal/channelcontract ./internal/modules/channel \
  ./internal/channelhost -count=1
```

Expected: PASS, including fast-terminal preparation, restart delivery,
approval attribution, media, live-surface settlement, capability discovery,
and owner lifecycle. The app package remains temporarily untestable while the
RPC merge conflict is unresolved; Task 3 runs app owner/import/rollback tests
after restoring a compilable generic dispatcher.

### Task 3: Keep all Channel RPC behavior in the typed contribution

**Files:**
- Modify: `internal/modules/channel/management.go`
- Modify: `internal/modules/channel/management_test.go`
- Resolve: `internal/rpc/control.go`
- Resolve: `internal/rpc/control_test.go`
- Verify: `internal/rpc/contribution.go`
- Verify: `internal/rpc/contribution_dispatch.go`
- Verify: `internal/rpc/contribution_dispatch_test.go`
- Verify: `internal/rpc/protocol.go`
- Verify: `internal/rpc/protocol_test.go`

**Interfaces:**
- Consumes: `channelhost.Host.Inspect`, `FailedDeliveries`, and
  `RedeliverDelivery`; existing focused settings authority; generic
  `rpccontract.MethodBinding`
- Produces: five Module-owned method/capability pairs and standard
  `MethodNotFound` when the Module contribution is absent

- [ ] **Step 1: Restore a compilable generic RPC skeleton and move tests**

Resolve `internal/rpc/control.go` to the current generic contribution shape
first. Preserve unrelated PR attachment changes, but omit the source branch's
Channel imports, `ControlDeps` fields, switch cases, DTOs, and methods. Resolve
`internal/rpc/control_test.go` by preserving current P0 generic
absence/projection tests; omit the source's direct Channel behavior block.
This is a compilable target-authoritative skeleton, not the final behavior.

Move the health projection and failed-delivery list/redeliver assertions from
the source branch's conflicted test block into
`internal/modules/channel/management_test.go`. Use the existing Module owner,
SQLite loopback store, and fake Channel Provider. Test exactly:

- no contribution -> `MethodNotFound` and no `channel.*` capabilities;
- contribution -> five exact method/capability pairs;
- inspect health is `null` when not started/unsupported and classified when a
  started adapter reports `rate-limit`, `temporary`, or `dead`;
- list exposes failed rows only and never includes reply content;
- empty/unknown run IDs return `InvalidParams` / `CodeNotFound`;
- inactive or draining Provider returns `CodeConflict`; and
- successful redelivery re-arms the intent, sends once, and removes it from
  the failed listing.

- [ ] **Step 2: Confirm RED against the three-binding owner**

```bash
go test ./internal/modules/channel ./internal/rpc \
  -run 'Test.*(Management|Deliver|Health|Contribution|Capability)' -count=1
```

Expected: FAIL because the owner contributes only inspect/get/update and its
wire projection lacks PR #36 health/delivery behavior.

- [ ] **Step 3: Extend the Module-owned wire projection**

Add `channelHealthResult`, the nullable `Health` field, and failed-delivery DTO
to `management.go`. Keep identifiers only. Add handlers equivalent to the PR
behavior and map errors without exposing internal details:

```text
missing run_id                         -> InvalidParams
no failed intent                       -> CodeNotFound
channel not running / host draining    -> CodeConflict
store failure                          -> InternalError
```

Append the two delivery bindings to `owned.RPCBindings()`. The contribution
validator remains the only attachment point and derives advertised
capabilities from those bindings.

- [ ] **Step 4: Resolve RPC core in favor of generic dispatch**

In `internal/rpc/control.go`, retain the current `rpccontract` import and
generic contribution dispatch. Recheck that the merge source's `attachment`
and `channelhost` imports, `ControlDeps` Channel fields, Channel switch cases,
Channel DTOs/helpers, and hard-coded capability names are absent. Preserve
unrelated PR attachment RPC changes.

In `internal/rpc/control_test.go`, retain P0 generic absence/projection and
contribution tests. Do not retain duplicate direct Channel behavior tests.

- [ ] **Step 5: Prove the generic boundary GREEN**

```bash
go test ./internal/modules/channel ./internal/rpc ./internal/rpccontract \
  ./internal/app \
  -run 'Test.*(Management|Deliver|Health|Contribution|Capability|MethodNotFound|ChannelOwner)' \
  -count=1
! rg -n '"agent-vivy/internal/(channelhost|modules/channel)"' internal/app internal/rpc
! rg -n '"channel/(inspect|get|update|deliveries/list|deliveries/redeliver)"' \
  internal/rpc --glob '!**/*_test.go'
```

Expected: tests PASS and both searches return no matches.

### Task 4: Reconcile storage and runtime invariants without renumbering main

**Files:**
- Modify/verify from source: `internal/domain/session.go`
- Modify/verify from source: `internal/runtime/service.go`
- Modify/verify from source: `internal/runtime/approval_test.go`
- Modify/verify from source: `internal/runtime/provenance_test.go`
- Modify/verify from source: `internal/storage/contracts.go`
- Modify/verify from source: `internal/storage/conformance/suite.go`
- Modify/verify from source: `internal/storage/sqlite/sqlite.go`
- Modify/verify from source: `internal/storage/sqlite/channels.go`
- Modify/verify from source: `internal/storage/sqlite/sessions.go`
- Modify/verify from source: `internal/storage/postgres/postgres.go`
- Modify/verify from source: `internal/storage/postgres/schema.go`
- Modify/verify from source: `internal/storage/postgres/channels.go`
- Modify/verify from source: `internal/storage/postgres/sessions.go`
- Verify: `internal/channelhost/deliver_recovery_test.go`
- Verify: `internal/channelhost/deliver_redeliver_test.go`

**Interfaces:**
- Consumes: current SQLite migration 23, PostgreSQL schema 21, and storage
  conformance CN-27 workspace ownership
- Produces: SQLite migration 24, PostgreSQL schema 22, CN-28 durable delivery,
  CN-29 inbound retention, session-delete cleanup, closed provenance
  vocabulary, and runtime `BeforeStart`

- [ ] **Step 1: Lock monotonic identifiers with tests**

Retain current main's workspace identifiers and require the source Channel
work to follow them:

| Surface | Existing main | PR #36 integrated |
|---|---:|---:|
| SQLite | migration 23 workspace | migration 24 channel deliveries |
| PostgreSQL | schema 21 workspace | schema 22 channel deliveries |
| Storage conformance | CN-27 workspace | CN-28 delivery, CN-29 retention |

Add or retain upgrade tests that open the immediately preceding schema,
upgrade once, and verify both the workspace and Channel artifacts. Fix stale
comments that call the Channel table migration 23 while registering it as 24.

- [ ] **Step 2: Preserve the runtime preparation ordering**

Require `runtime.RunOptions.BeforeStart` to run after run ID creation and
before any user message, run row, or event. Retain tests for:

```text
prepare error -> no persisted message, run, or event
fast terminal -> delivery intent already armed
invalid source -> rejected before persistence
channel actor -> same approval lifecycle and journal path
```

Do not create a Channel-specific runtime method.

- [ ] **Step 3: Prove storage behavior on both backends**

```bash
go test ./internal/storage/conformance ./internal/storage/sqlite \
  ./internal/storage/postgres -count=1
go test ./internal/runtime ./internal/channelhost \
  -run 'Test.*(BeforeStart|Prepared|Provenance|Approval|Deliver|Recovery|Retention|Session)' \
  -count=1
```

Expected: PASS. Session deletion removes its delivery rows transactionally;
retention deletes only old `chanin_*` events; recovery leaves unavailable
Provider intents open without consuming attempts.

### Task 5: Preserve both P0 projection and PR #36 UI behavior

**Files:**
- Resolve/modify: `sdk/ui/src/module.ts`
- Verify: `sdk/ui/src/module.test.ts`
- Resolve/modify: `ui/src/lib/api.ts`
- Modify/verify from source: `ui/src/components/settings/ChannelCard.tsx`
- Modify/verify from source: `ui/src/components/settings/ChannelsSettings.tsx`
- Modify/verify from source: `ui/src/components/settings/channel-store.ts`
- Modify/verify from source: `ui/src/components/settings/channel-store.test.ts`
- Resolve/modify: `ui/src/i18n/en.ts`
- Resolve/modify: `ui/src/i18n/zh.ts`
- Resolve/modify: `scripts/i18n-cross-face-contract.json`
- Verify: `ui/playwright.config.ts`

**Interfaces:**
- Consumes: generic `ui_extensions` projection from CH-P0-1/2 and the five
  Module-contributed Channel capabilities
- Produces: unchanged static Channel settings UI for this gate, including
  health state, failed-delivery list/redeliver, and PR interaction/media wire
  types; preserves the future P0-4 extension projection

- [ ] **Step 1: Add a combined API regression before resolving auto-merges**

Extend the relevant TypeScript tests to assert in one fixture that:

```ts
capabilities.ui_extensions?.[0]?.id === 'vivy/channel-ui'
capabilities.capabilities.includes('channel.deliveries.list')
capabilities.capabilities.includes('channel.deliveries.redeliver')
```

Also require a `ChannelStatus` health projection and the exact
`channel/deliveries/list` / `channel/deliveries/redeliver` request names. This
guards against an apparently clean textual merge dropping one side's shape.

- [ ] **Step 2: Confirm RED or record clean semantic compatibility**

```bash
pnpm --dir ui exec vitest run \
  src/components/settings/channel-store.test.ts
pnpm --dir ui typecheck
```

Expected before resolution: either a named type/test failure, or PASS with a
record that Git's auto-merge already retained both shapes. A clean semantic
result is acceptable here; do not manufacture a failure by weakening types.

- [ ] **Step 3: Resolve the wire and localization union**

Keep `UIExtensionProjection` and optional `ui_extensions`. Keep PR #36's
Channel capability, health, delivery DTO, API calls, store actions, UI copy,
and source-closed provenance type. Reconcile English/Chinese keys and the
cross-face contract as one semantic union. Do not move components into a UI
Module, edit `ui/src/generated/assembly.ts`, or add build-owned visibility;
that remains P0-4.

- [ ] **Step 4: Run UI and i18n gates**

```bash
pnpm --dir ui test
pnpm --dir ui typecheck
pnpm --dir ui build
node scripts/check-i18n-completeness.js
node --test scripts/check-i18n-cross-face.test.js
node scripts/check-i18n-cross-face.js
```

Expected: PASS; the only permitted build warning is an already-known bundle
size warning recorded in the integration verification log.

- [ ] **Step 5: Exercise the split browser path**

Start `just dev`, open `http://127.0.0.1:3015`, and verify without bot tokens:

1. Settings -> Channels loads from live `channel/inspect` and `channel/get`.
2. Health and configured/running states render without secret values.
3. Failed-delivery controls make no request when the capabilities are absent.
4. When the capabilities are advertised by the default Generation, the list
   request is made and an empty list renders safely.
5. Ordinary chat still starts and displays a run.

Record a browser/network trace summary in `verification.md`; screenshots alone
are not acceptance evidence.

### Task 6: Resolve durable records and regenerate exact-source evidence

**Files:**
- Resolve: `docs/COMPLETE.MD`
- Resolve/verify: `docs/TODO.md`
- Modify/verify from source: `docs/architecture/VIVY-CHANNEL-PACK.md`
- Preserve unchanged: `docs/architecture/CHANNEL-MODULARIZATION.md`
- Accept source records: `docs/logs/2026-09-14-channel-hardening/*`
- Accept source records: `docs/logs/2026-09-15-channel-*/*`
- Accept source records: `docs/logs/2026-09-15-pr36-review-fixes/*`
- Accept source plan: `docs/superpowers/plans/2026-09-15-pr36-review-fixes.md`
- Resolve: `sdk/internal/assembly/conformance_results.json`
- Resolve: `sdk/internal/conformance/reproduction_test.go`
- Update during execution: `docs/logs/2026-09-20-pr36-channel-integration/*`

**Interfaces:**
- Consumes: final merged source tree and executable reproduction suites
- Produces: unioned completion/backlog history, unchanged #42 architecture
  authority, exact source digests, and an auditable integration record

- [ ] **Step 1: Resolve documentation as a union, not a side selection**

Keep the current provider-registry and P0-1/2 completion rows, then add the PR
#36 Channel completion rows. Retain all open current `docs/TODO.md` entries and
all PR Channel entries; deduplicate only byte-equivalent rows. Preserve
`CHANNEL-MODULARIZATION.md` and every plan in this directory.

The imported PR iteration logs remain historical source evidence. The new
integration log is the only authority for the combined tree.

- [ ] **Step 2: Resolve reproduction code in favor of final-tree identity**

Keep the current signature:

```go
func releaseSuiteCases(internalDigest string) []releaseSuiteCase
```

Do not restore the source branch's hard-coded `internalSHA`. Retain the PR's
five Channel Provider cases and their executable commands. Resolve the JSON
conflict to a syntactically valid provisional bundle by keeping the target's
internal-root hashes; the source Provider hashes that Git merged cleanly stay
in place. This is only an input to the producer check, never accepted
evidence. Run:

```bash
go test ./sdk/internal/conformance \
  -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1
```

Expected first result: FAIL only with explicit stale source digests or a real
suite failure. Fix real failures first.

- [ ] **Step 3: Rotate only evidence proven stale**

Compute the internal source identity with the repository tool:

```bash
go run ./sdk/internal/cmd/source-hash internal ""
```

Update only result entries whose reported source root changed. Provider hashes
must equal the final contents of their own `plugins/<provider>` roots. Then
rerun:

```bash
go test ./sdk/internal/conformance \
  -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1
go test ./sdk/internal/conformance \
  -run 'TestGeneration(FailureMatrixEvidence|RollbackRestoresCatalogAndLocaleIdentity)' \
  -count=1
go test ./sdk/internal/assembly \
  -run 'Test(CompilePluginV1GraphFixtures|StartFailureRollsBackEveryConstructedOwner)' \
  -count=1
```

Expected: PASS on one final source fixed point.

### Task 7: Prove scope, package the default Generation, and land to aggregate

**Files:**
- Update: `docs/logs/2026-09-20-pr36-channel-integration/summary.md`
- Update: `docs/logs/2026-09-20-pr36-channel-integration/verification.md`
- Update: `docs/logs/2026-09-20-pr36-channel-integration/acceptance.md`
- Update after accepted integration: `docs/superpowers/plans/channel-modularization/index.md`
- Update after accepted integration: `docs/superpowers/plans/channel-modularization/ch-p0-3-backend-omission.md`

**Interfaces:**
- Consumes: resolved merge, focused suites, exact conformance bundle, default
  Recipe, split browser evidence, independent review, and final CI
- Produces: accepted `CH-INT-36` on `feat/channel-modularization`, source
  ancestry proof, and a READY CH-P0-3 baseline

- [ ] **Step 1: Remove merge artifacts and enforce ownership/scope guards**

```bash
test -z "$(git diff --name-only --diff-filter=U)"
! rg -n '^(<<<<<<<|=======|>>>>>>>)' --glob '!vendor/**' --glob '!node_modules/**'
git diff --check
! rg -n '"agent-vivy/internal/(channelhost|modules/channel)"' internal/app internal/rpc
! rg -n '"channel/(inspect|get|update|deliveries/list|deliveries/redeliver)"' \
  internal/rpc --glob '!**/*_test.go'
git diff "$INTEGRATION_TARGET" -- internal/provider docs/plans/provider-registry recipes
git diff "$INTEGRATION_TARGET" -- ui/src/generated/assembly.ts
```

Expected: no unresolved markers or whitespace errors; both forbidden-boundary
searches and both scope diffs are empty. The merge may change provider-facing
Channel plugins, but not the provider registry or Recipe/UI Assembly selection.

- [ ] **Step 2: Run focused and whole-product gates**

```bash
go test ./internal/channelcontract ./internal/modules/channel \
  ./internal/channelhost ./internal/attachment ./internal/runtime \
  ./internal/storage/conformance ./internal/storage/sqlite \
  ./internal/storage/postgres ./internal/rpc ./internal/app -count=1
go test ./sdk/internal ./sdk/internal/assembly ./sdk/internal/conformance -count=1
for module_dir in plugins/* faces/*; do
  test -f "$module_dir/go.mod" || continue
  (cd "$module_dir" && go test ./...)
done
just ci
```

Expected: every command PASS. If the executor environment cannot run literal
`just ci`, stop before acceptance and obtain a GitHub Actions run that executes
the repository gate; do not substitute an undocumented partial gate.

- [ ] **Step 3: Pack and inspect the unchanged default Recipe**

```bash
PACK_PARENT=$(mktemp -d)
PACK_OUTPUT="$PACK_PARENT/default"
go run ./sdk verify plugins/telegram
go run ./sdk verify plugins/discord
go run ./sdk verify plugins/feishu
go run ./sdk verify plugins/qq
go run ./sdk verify plugins/dingtalk
go run ./sdk pack --recipe recipes/default.vivy.yml --output "$PACK_OUTPUT"
go run ./sdk inspect-artifact "$PACK_OUTPUT"
```

Expected: Inspect reports the canonical Channel Host, all five Providers,
their updated capabilities, and source-bound conformance. This proves default
parity only; it is not CH-P0-3 omission evidence.

- [ ] **Step 4: Request independent review before concluding the merge**

The reviewer receives this plan, both architecture specs, source SHA, target
SHA, the eight-conflict matrix, final diff, focused/full verification, and
pack/Inspect output. Reject any review resolution that restores concrete Host
knowledge to app/RPC or starts P0-3/P0-4 work.

Resolve findings and rerun every affected gate. Record findings and resolution
in the integration log.

- [ ] **Step 5: Finish the source merge commit**

Stage explicit paths after all conflicts and review findings are resolved:

```bash
git add docs internal plugins schemas scripts sdk ui
git status --short
git commit -m "merge(channel): integrate PR 36 into modular owner"
git merge-base --is-ancestor \
  eb8fee3c0996746e93def297d700c70fb65b8540 HEAD
```

Expected: one merge commit with two parents, and the ancestry check exits 0.
Do not stage unrelated files or temporary Pack output.

- [ ] **Step 6: Obtain final branch CI and close the integration gate**

Push `feat/channel-pr36-integration`, obtain a green CI run with backend, UI,
browser smoke, and aggregate `just ci`, then add its exact run URL and commit
SHA to the integration `verification.md` and `acceptance.md`. Commit that
evidence as:

```bash
git add docs/logs/2026-09-20-pr36-channel-integration
git commit -m "docs(channel): accept PR 36 aggregate integration"
```

Merge the reviewed integration branch into `feat/channel-modularization`, not
`main`. Update the index only after verifying the aggregate tree contains the
accepted commit. Change `CH-INT-36` to `COMPLETE`, link the acceptance log,
change CH-P0-3 to `READY`, and update CH-P0-3's immediate-predecessor text.

## Acceptance Map

| Acceptance criterion | Required evidence |
|---|---|
| Exact PR integration | `eb8fee3...` is an ancestor of the accepted aggregate commit; source/target/base recorded. |
| Canonical ownership preserved | app/RPC import and hard-coded method scans empty; Module owns five bindings and concrete Host. |
| PR hardening retained | durable prepared-run, restart recovery, Stop drain, retention, redelivery, and session-delete tests PASS. |
| Text/media/interaction retained | five Provider suites and Host media/live-surface/health/approval tests PASS. |
| Current main retained | provider-registry/workflow/P0 records remain; protected scope diffs are empty. |
| P0 scope fence retained | no reduced Recipe, conditional omission, UI Module, generated UI selection, or release matrix change. |
| Exact conformance | reproduction and Generation matrices PASS at the final source fixed point. |
| Default product parity | default Pack/Inspect, split browser trace, `just ci`, and final GitHub Actions lanes PASS. |
| Independent review | findings and resolutions recorded; no open Critical or Important finding. |

## Stop Conditions

Stop and return the work to the supervisor when any of these occurs:

- the PR source head changes from the locked SHA;
- the aggregate branch has product changes not represented in this plan;
- the merge conflict set expands into another product subsystem;
- preserving PR behavior requires a public `std/channel@v1` break;
- a proposed fix imports the concrete Channel Host into app/RPC or creates a
  second runtime/settings/dispatcher path;
- a storage migration number is already occupied by different accepted work;
- a real PR behavior conflicts with the normative #42 architecture; or
- acceptance would require beginning CH-P0-3, CH-P0-4, or CH-P0-5.

## Execution Handoff

Give the execution agent this plan, the Epic index, both architecture specs,
the P0-2 acceptance log, the exact source SHA, and the current aggregate head.
One agent owns the merge worktree because every task shares merge state and
the same owner contracts. Use separate agents for task-level review and final
whole-branch review; do not parallel-edit the unresolved merge.
