# PROV-P2 — Adapter Table and Sealed Adapter Set

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task.

**Goal:** Move the sealed unit from the vendor to the **protocol adapter**, so a
vendor with two wire protocols is ordinary data and a missing protocol is an
explicitly deferred capability.

**Architecture:** One three-row adapter table in `internal/provider` is the only
list of adapter families. The compiled Generation seals that list; the embedded
vendor data references it. `Catalog` resolves an adapter by family instead of
looking a bundle up by profile id.

**Tech Stack:** Go 1.26, `eino v0.9.13`, `eino-ext openai v0.1.13`,
`eino-ext claude v0.1.25`, generated Assembly wiring, `just ci`.

**Spec:** `docs/plans/provider-registry/DESIGN.md` §2.1, §3.3, §4;
`docs/plans/provider-registry/EINO-CAPABILITY.md` (all of it);
`docs/plans/provider-registry/MIGRATION.md` §1.1, §1.2.

**Depends on:** `PROV-P1`.

---

## Global constraints

- Exactly three adapter families exist: `openai-completions`,
  `openai-responses`, `anthropic-messages`. No fourth is added; `openai-responses`
  is `DEFERRED-INDEFINITE` and backed by no implementation placeholder.
- No new third-party dependency in this phase. `eino-ext/components/model/agenticopenai`
  is **not** imported (`EINO-CAPABILITY.md` §5.3 records why).
- **Keep the Go symbol names** `defaults.ProviderProfiles`,
  `providerprofile.Provider`, and the test name
  `TestDefaultProviderProfilesMatchExistingRuntimeFamilies`. `sdk/internal/assembly/evidence.go:77-80`
  and `sdk/internal/conformance/reproduction_test.go:456` cite them as Port
  evidence anchors; renaming them invalidates conformance evidence and turns this
  refactor into an evidence migration as well.
- Only `internal/provider` and `internal/runtime` may import Eino.
- Never hand-edit `internal/generated/assembly/zz_default.go`.

---

### Task 1: The adapter table

**Files:**

- Create: `internal/provider/adapters.go`
- Create: `internal/provider/adapters_test.go`

**Interfaces:**

```go
const (
    AdapterOpenAICompletions = "openai-completions"
    AdapterOpenAIResponses   = "openai-responses"
    AdapterAnthropicMessages = "anthropic-messages"
)

// Adapter binds one sealed protocol family to its Eino component.
type Adapter struct {
    Family string
    State  modelhost.CapabilityState
}

// Adapters returns the sealed families in stable order. It is the single
// adapter list in the codebase.
func Adapters() []Adapter
```

- [ ] RED: a test that the returned families are exactly the three constants, in
  stable order, with `openai-responses` marked `DEFERRED-INDEFINITE` and the other
  two `SUPPORTED`.
- [ ] RED: a test asserting no fourth family can be constructed by data (i.e.
  `Adapters()` is the only source and its length is 3).
- [ ] Implement the table by binding `AdapterOpenAICompletions` to the pinned
  `eino-ext/components/model/openai` path and `AdapterAnthropicMessages` to
  `eino-ext/components/model/claude`, mirroring what
  `internal/provider/catalog.go:46-53` and `internal/provider/openai.go:83-87`
  already do. Do not add any binding for `openai-responses`.
- [ ] Build `modelhost.Capabilities` in `internal/app/app.go:288-295` **from**
  `provider.Adapters()` so the capability map and the adapter table cannot drift.

### Task 2: Resolve by family instead of by profile id

**Files:**

- Modify: `internal/provider/catalog.go`
- Modify: `internal/provider/catalog_test.go` (or the existing provider tests)
- Modify: `internal/provider/profile.go`

**Interfaces:**

```go
// Adapter resolves a sealed adapter family to a Ref.
func (c *Catalog) Adapter(family string) (Ref, error)

// RefForEndpoint resolves a vendor endpoint (adapter + base URL) to a Ref.
func (c *Catalog) RefForEndpoint(vendor string, ep Endpoint) (Ref, error)
```

- [ ] RED: `Adapter("openai-responses")` fails with a deferred-capability error,
  not a not-found error; `Adapter("nope")` fails as not-found.
- [ ] RED: two vendors on the same family resolve to the same adapter but carry
  their own `env_key` and `base_url` — the case the current design cannot express
  (today `catalog.go:60` looks the bundle up by `profile.ID`, so DeepSeek's
  Anthropic endpoint would need a `deepseek-anthropic` profile).
- [ ] Replace the `Backend`-based switch (`catalog.go:46-53`) and the
  `profile.ID` bundle lookup (`catalog.go:60`) with family-based resolution.
  `ErrAdapterFamilyMismatch` disappears because the mismatch is no longer
  representable: the endpoint's `adapter` **is** the family.
- [ ] Keep `ResolveModelInfo` behaviour: unknown model returns zero
  `ContextWindow` and the caller keeps its conservative default
  (`internal/provider/ref.go:35-39`).

### Task 3: Thinking options from capability + metadata

**Files:**

- Modify: `internal/provider/resolving.go:114-190`
- Modify: `internal/provider/resolving_thinking_test.go`
- Modify: `internal/provider/openai.go` (metadata tables move to data in `PROV-P1`; what remains is the adapter)

**Interfaces:**

Today the decision is `bundle.Name != "deepseek"` (`resolving.go:146`) plus a
dual-purpose `supportsThinking` metadata flag (`openai.go:41-43`). The new rule:

```
adapter anthropic-messages  + model.supports_thinking      -> claude.WithThinking(budget)
adapter openai-completions  + endpoint "deepseek-thinking" + model.supports_thinking
                                                          -> WithExtraFields(thinking) + WithReasoningEffort(high)
adapter openai-completions  + model.supports_thinking      -> WithReasoningEffort(high)
otherwise                                                  -> send nothing
```

- [ ] RED: OpenAI reasoning models (`o3-mini`-class) now receive
  `reasoning_effort`. Today they receive **nothing** — `openai.go:65-67` leaves
  `supportsThinking` false for `o1`/`o1-mini`/`o3-mini`, and `resolving.go:146`
  returns early for any non-DeepSeek vendor. This is a real behaviour fix, not a
  refactor, and it needs its own assertion on the outbound request body.
- [ ] RED: a `deepseek-thinking` endpoint sends both fields
  (`{"thinking":{"type":"enabled"}}` + `reasoning_effort`), preserving today's
  documented canonical request.
- [ ] RED: a model without `supports_thinking` receives neither field.
- [ ] RED: thinking mode `off` on a `deepseek-thinking` endpoint sends
  `{"thinking":{"type":"disabled"}}`, preserving today's behaviour.
- [ ] Implement the rule as a small pure function over
  `(adapterFamily, endpointCapabilities, modelMeta, thinkingMode)`, so it can be
  unit-tested without constructing a model. Keep `domain.ThinkingMode`
  (`auto`/`on`/`off`) as the per-run input; do **not** merge it with the
  capability flag.
- [ ] Keep the cache key in `resolving.go:91-92` hashing the key rather than
  storing it; re-key on the endpoint identity.

### Task 4: Re-seal the Generation on adapters

**Files:**

- Modify: `internal/modules/defaults/providers.go:18-39`
- Modify: `internal/modules/defaults/providers_test.go`
- Modify: `internal/modules/defaults/catalog.go:50`
- Modify: `internal/app/assembly_validate.go` (only if a check needs rewording)
- Generate: `internal/generated/assembly/zz_default.go`

**Interfaces:**

`defaults.ProviderProfiles()` keeps its name and signature but returns three
**adapter** Profiles instead of three vendor Profiles:

```go
providerprofile.Profile{
    ID:            provider.AdapterOpenAICompletions,
    AdapterFamily: provider.AdapterOpenAICompletions,
    EndpointClass: providerprofile.EndpointNative,
    ModelIDs:      <union of the embedded endpoints' model ids for this family>,
    SecretRefs:    <union of the embedded vendors' env keys for this family>,
    OptionsSchema: defaultProviderOptionsSchema,
}
```

- [ ] Keep `TestDefaultProviderProfilesMatchExistingRuntimeFamilies` by name, so
  the evidence anchors in `sdk/internal/assembly/evidence.go:77-80` stay valid.
  Change its body: assert the three adapter ids, that two families are
  `SUPPORTED` and one `DEFERRED-INDEFINITE`, and that `ModelIDs`/`SecretRefs` are
  non-empty for supported families.
- [ ] Verify `providerprofile.Validate` still passes: it requires a non-empty
  `ModelIDs` and rejects native-endpoint model ids carrying a `vendor/` prefix
  (`sdk/port/providerprofile/providerprofile.go:64-86`). The union projection must
  satisfy both.
- [ ] Regenerate:

```powershell
go run ./sdk/internal/cmd/generate-default --repo . --output internal/generated/assembly/zz_default.go
```

- [ ] Confirm the generator is byte-reproducible: regenerate to a temporary file
  and compare.
- [ ] Confirm `internal/app/assembly_validate.go:130-143` (profile identities vs
  sealed manifest) passes with adapter identities.
- [ ] Check whether `sdk/internal/removal_conformance_test.go:78` and
  `sdk/internal/assembly/runtime_generate.go:116,298,408` need value updates
  (names stay, values change).

### Task 5: Focused verification

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test ./internal/provider ./internal/modelhost ./internal/modules/defaults ./internal/app -count=1`
- [ ] `go test ./sdk/... -count=1` (evidence anchors and generated-assembly
  conformance)
- [ ] `go test ./... -count=1`
- [ ] Record commands and outcomes in `verification.md`.

## Phase exit

Exit requires: `Adapters()` is the only adapter list; no vendor name appears in
`internal/provider` production code; a DeepSeek Anthropic endpoint and an OpenAI
reasoning model are both expressible; `openai-responses` is visible but not
executable; the generated Assembly reproduces byte-for-byte.

## Commit

```
refactor(provider): seal protocol adapters instead of vendors
```
