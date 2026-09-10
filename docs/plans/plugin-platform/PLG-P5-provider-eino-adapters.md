# PLG-P5 Provider Profile and Eino Adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Provider Profiles publicly declarative while keeping all model
execution in one internal ModelHost backed only by verified pinned Eino/EinoExt
adapters.

**Architecture:** Public Modules describe profiles and Secret references.
ModelHost resolves a profile to a T1 adapter in `internal/provider`; no public
code implements a ChatModel and no Profile can introduce execution machinery.

**Tech Stack:** Go, Eino v0.9.13 model interfaces, EinoExt OpenAI v0.1.13,
EinoExt Claude v0.1.25, existing config/RPC/UI model settings, `just ci`.

**Spec:** `docs/architecture/VIVY-PORT-CATALOG.md` section 6 and root
`AGENTS.md` Provider model-ID rules.

## Global Constraints

- State: `IN PROGRESS · manually scheduled 2026-09-10`; depends on P2. P7 Task 2 consumes its ModelHost, so P5
  is a required P7 and SCX Gate B input even when SCX uses the default model
  Provider.
- There is no `std/model-provider@v1`.
- Provider raw model IDs remain unchanged for native compatible endpoints.
- Missing pinned Eino/EinoExt execution or OAuth capability is
  `DEFERRED-INDEFINITE`; custom replacements are forbidden.

---

### Task 1: Produce the pinned Provider capability matrix

**Files:**

- Create: `docs/research/plugin-v1-provider-eino-matrix.md`
- Modify: phase iteration `verification.md`

**Interfaces:**

- Consumes: exact `go.mod` pins and module-cache source.
- Produces: profile family -> package/API -> decision evidence.

- [ ] Verify `openai.NewChatModel` in EinoExt OpenAI v0.1.13 and
  `claude.NewChatModel`/`claude.WithThinking` in EinoExt Claude v0.1.25.
- [ ] Inventory each requested provider family against the pinned source; do
  not use current online/latest docs as evidence.
- [ ] Record `ADAPT` only when the API satisfies Vivy invariants.
- [ ] Record every missing family/OAuth path as `DEFERRED-INDEFINITE` with no
  implementation placeholder.
- [ ] Commit `docs(provider): pin plugin v1 adapter decisions`.

### Task 2: Define declarative Provider Profiles

**Files:**

- Create: `sdk/port/providerprofile/providerprofile.go`
- Create: `sdk/port/providerprofile/providerprofile_test.go`
- Create: `internal/modelhost/profile.go`
- Create: `internal/modelhost/profile_test.go`

**Interfaces:**

- Consumes: public profile Definitions and configuration instance values.
- Produces: validated `Profile` records with adapter family, raw model IDs,
  endpoint class, option schema, and Secret references.

```go
type Profile struct {
    ID            string
    AdapterFamily string
    ModelIDs      []string
    EndpointClass string
    SecretRefs    []string
    OptionsSchema json.RawMessage
}
```

- [ ] Write `TestProfileContainsNoExecutableProvider`; expected RED is any
  callback, factory, Eino model, or arbitrary code field.
- [ ] Write tests for duplicate IDs, unsupported family, prefixed raw model ID,
  inline Secret, invalid option schema, and unavailable adapter.
- [ ] Implement pure data and ModelHost validation without Eino imports.
- [ ] Run `go test ./sdk/port/providerprofile ./internal/modelhost`.
- [ ] Commit `feat(provider): define declarative provider profiles`.

### Task 3: Put existing adapters behind one ModelHost

**Files:**

- Create: `internal/modelhost/host.go`
- Create: `internal/modelhost/host_test.go`
- Modify: `internal/provider/openai.go`
- Modify: `internal/provider/claude.go`
- Modify: `internal/provider/resolving.go`
- Modify: `internal/app/model.go`
- Modify: `internal/runtime/modeladapter.go`
- Modify: `internal/runtime/modelbroker.go`

**Interfaces:**

- Consumes: validated Profile and scoped credential resolution.
- Produces: one internal model interface adapted to Eino within the quarantine.

- [ ] Write `TestEveryModelCallUsesModelHost`; expected RED identifies direct
  provider factories outside the Host.
- [ ] Preserve existing OpenAI-compatible and Claude behavior, streaming,
  thinking options, cancellation, and error chains.
- [ ] Assert native endpoints receive the configured raw model ID with no
  automatic `provider/` prefix.
- [ ] Keep Eino imports in `internal/provider` and `internal/runtime` only.
- [ ] Run `go test ./internal/modelhost ./internal/provider ./internal/runtime -run Model`.
- [ ] Commit `refactor(provider): route models through modelhost`.

### Task 4: Register first-party Profiles in the default Generation

**Files:**

- Create: `internal/modules/defaults/providers.go`
- Create: `internal/modules/defaults/providers_test.go`
- Modify: `recipes/default.vivy.yml`
- Modify: `internal/config/config.go`
- Modify: `internal/app/settings/settings.go`

**Interfaces:**

- Consumes: existing provider configuration families with verified adapters.
- Produces: default-on Profile definitions whose instances remain unconfigured
  without Secret/config values.

- [ ] Write baseline parity tests for profile IDs, model selection, endpoint,
  and missing-credential errors.
- [ ] Register only families with verified pinned adapters.
- [ ] Do not activate or network-probe a Profile during Describe, Construct, or
  status inspection.
- [ ] Run `go test ./internal/modules/defaults ./internal/config ./internal/app -run Provider`.
- [ ] Commit `feat(provider): register default provider profiles`.

### Task 5: Project Profile and deferred status

**Files:**

- Modify: `internal/rpc/control.go`
- Modify: `ui/src/components/settings/provider-catalog.ts`
- Modify: `ui/src/components/settings/provider-catalog.test.ts`
- Modify: `ui/src/components/settings/custom-providers.ts`
- Modify: `ui/src/components/settings/GenerationParamsCard.tsx`

**Interfaces:**

- Consumes: ModelHost/Profile instance status and Generation Manifest.
- Produces: UI/RPC truth for compiled, unconfigured, ready, unavailable, and
  deferred capabilities.

- [ ] Write UI tests proving a `DEFERRED-INDEFINITE` family cannot be selected
  as executable and no fake endpoint is generated.
- [ ] Preserve existing configuration forms for supported profiles.
- [ ] Keep Secret values out of RPC output.
- [ ] Run focused Go and UI tests.
- [ ] Commit `feat(provider): project profile capability status`.

### Task 6: Complete Provider conformance

**Files:**

- Create: `internal/modelhost/conformance_test.go`
- Modify: `internal/provider/provider_test.go`
- Modify: `internal/provider/secret_audit_test.go`
- Modify: `internal/app/model_test.go`

**Interfaces:**

- Consumes: supported first-party Profiles and fake adapter boundaries.
- Produces: seven-artifact proof without live-network unit tests.

- [ ] Cover missing Profile, duplicate Profile, unsupported family,
  unconfigured Secret, adapter failure, timeout, cancellation, streaming error,
  raw model ID, Secret redaction, Inspect provenance, and shutdown.
- [ ] Build/inspect default and minimal Generations.
- [ ] Run `just ci` and the repository's authorized provider smoke path with
  fake/local endpoints only.
- [ ] Commit `test(provider): prove profile and modelhost conformance`.

## Phase exit and rollback

Exit requires no public executable Provider, exact pinned API evidence,
preserved raw model IDs, Secret-safe status, and one ModelHost. Rollback selects
the prior Generation. A deferred family remains absent rather than falling back
to custom code.
