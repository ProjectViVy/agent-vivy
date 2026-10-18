# Design Concept — Single Generation, Sealed Binding

**Status:** authored 2026-10-17. Concept document; it changes no code.
**Program:** `docs/plans/provider-agentic/README.md`.
**Supersedes:** the current-generation facts in
`docs/plans/provider-registry/EINO-CAPABILITY.md` (the dated 2026-09-18 check).

This document states the one design pattern this program adopts, the evidence
that makes it viable, why the rejected alternative is worse **for this
repository**, and the boundary conditions under which the concept is false.

---

## 1. The pattern

**One message generation, one sealed binding table, zero conversion code.**

The design rule, in one sentence:

> Protocol message encoding and decoding stay in pinned Eino components. Vivy
> keeps sealed bindings and thin configuration/per-request option projections at
> the existing provider boundary; no Vivy-owned protocol codec or message bridge.

Two consequences follow, and both are the point:

- **No Vivy-owned protocol/message conversion is introduced.** Domain event
  projection and product option policy still have explicit owners and tests.
- **Adding a protocol adds a binding**, not a new conversion layer; required
  configuration and per-call option projection must still be priced and verified.

### 1.1 Vocabulary

| Term | Meaning here |
|---|---|
| **Generation** | Which Eino message type the model seam speaks: `*schema.Message` (the `ToolCallingChatModel` generation) or `*schema.AgenticMessage` (the `AgenticModel` generation) |
| **Sealed binding** | One adapter-table row = (protocol family name, data declarations, pinned Eino constructor). The row binds a name to a constructor; it does not carry a capability *state* |
| **Residue** | A declaration, type, state, wire field, or code path that survives the change and describes something that no longer exists |

### 1.2 What "sealed binding" replaces

Today an adapter row carries a **state** (`internal/provider/adapters.go:42-45`:
`Adapter{Family string; State modelhost.CapabilityState}`), and one of the three
rows is permanently non-executable
(`:51-55`: `openai-responses` → `CapabilityDeferredIndefinite`). The state is a
cross-cutting concept with exactly one member.

Under sealed binding a row is a constructor reference. A protocol that cannot be
constructed is not a row with a sad state; it is absent from the table, and its
absence is expressed in data that is not shipped. That is the difference between
"declared but disabled" and "not declared".

---

## 2. Why the concept is viable now (three verified facts)

All three were read from the module cache, not from online documentation.

### 2.1 The three protocol families all have an `AgenticModel` implementation

| Protocol family | Pinned component | Version | Interface assertion | Reasoning |
|---|---|---|---|---|
| `openai-completions` | `components/model/agenticopenai` | `v0.2.4` | `chat_model.go:30` `var _ model.AgenticModel = (*ChatModel)(nil)` | supported |
| `openai-responses` | `components/model/agenticopenai` | `v0.2.4` | `responses_model.go:38` `var _ model.AgenticModel = (*ResponsesModel)(nil)` | supported |
| `anthropic-messages` | `components/model/agenticclaude` | `v0.1.7` | `model.go:43` `var _ model.AgenticModel = (*Model)(nil)` | supported (`model.go:98` `Thinking *anthropic.ThinkingConfigParamUnion`) |

This is the fact that decides the concept. Without `agenticclaude` the pattern
would be false for Anthropic — Anthropic would have to stay on the old
generation, which reintroduces the two-generation problem the pattern exists to
remove. `agenticclaude v0.1.7` exists, requires `eino v0.9.21`
(`go.mod:10`), and maps Claude thinking blocks to `schema.Reasoning` including
signature round-trip (`convertor.go:286`, `event_convertor.go:126`).

### 2.2 The ADK accepts an `AgenticModel` directly, so no bridge is needed

```go
// eino@v0.9.21 adk/chatmodel.go
:260  type TypedChatModelAgentConfig[M MessageType] struct {
:280      Model model.BaseModel[M]
:508  func NewTypedChatModelAgent[M MessageType](ctx context.Context, config *TypedChatModelAgentConfig[M]) (*TypedChatModelAgent[M], error)
```

`components/model/interface.go:109` defines `AgenticModel = BaseModel[*schema.AgenticMessage]`, so
`NewTypedChatModelAgent[*schema.AgenticMessage]` accepts it with **no
conversion**. The historical objection to this migration — "Eino publishes no
`Message`↔`AgenticMessage` conversion, so a boundary bridge would be a custom
mechanism" — was correct about the model components and wrong about the
assembly layer: the bridge is not needed because the agent layer is generic.

The objection remains correct in its own scope, and the gap is real: in
`eino@v0.9.21` the only `ConvAgentic*` symbols are callback converters
(`components/model/agentic_callback_extra.go:61,79`,
`components/prompt/agentic_callback_extra.go:45,59`) and
`components/prompt/agentic_chat_template.go:34 FromAgenticMessages`. None
converts `*schema.Message` ↔ `*schema.AgenticMessage`. **Therefore this plan cannot mix generations through the existing single typed
model seam without a bridge, and the seam and its consumers move together.** That is a constraint on the migration shape, not
an argument against the pattern; the pattern's answer is to move the seam, not
to write the bridge.

### 2.3 Tools bind at request time, and both agentic components honour it

```go
// eino@v0.9.21 components/model/option.go
:116  func WithTools(tools []*schema.ToolInfo) Option
:143  func WithDeferredTools(tools []*schema.ToolInfo) Option
```

`AgenticModel` deliberately has no `WithTools()` method
(`components/model/interface.go:99,109` — the method belongs to the
`ToolCallingChatModel` interface; the agentic generation takes tools as an
option). Both components consume it:

- `agenticopenai@v0.2.4/responses_model.go:491` — `model.GetCommonOptions(&model.Options{...})`
- `agenticclaude@v0.1.7/model.go:550` — same
- and both `DeferredTools` paths (`agenticopenai/responses_model.go:818-820`, `agenticclaude/model.go:581-582`)
- both components' own examples call `Generate(ctx, in, model.WithTools(...))`

This was the highest-risk unknown in the concept: if either component ignored
`model.Option`, the ADK's tool binding would not reach it and the whole pattern
would fail on the first run. Verified, it does not.

---

## 3. The three structural decisions

### D-A. The model seam moves to `model.AgenticModel`

The seam is exactly one line today — `internal/provider/ref.go:33`,
`Model(ctx, spec) (model.ToolCallingChatModel, error)`. Moving its return type
moves every consumer with it, and that is the migration's real cost
(§5).

### D-B. The adapter table becomes bindings, not states

`Adapter.State`, `modelhost.CapabilityState` on the adapter path,
`ErrAdapterDeferred` (`internal/provider/adapters.go:31`), the deferred branch
in `catalog.go:191-200`, the wire fields `Executable`/`State`
(`internal/rpc/control.go:5571,5573,5612`), and the UI badge path
(`ui/src/components/settings/provider-catalog.ts:26,117-118`,
`ModelSettingsCard.tsx:615,618`) all exist to describe one non-executable row.
Once every row is executable, the adapter deferral state has no subject.
Runtime readiness/unavailability and endpoint/model feature support are separate
facts and must remain derived and enforced (S3's preserved list).

This is not a "cleanup after the feature". It is the feature: the pattern's
claim is that a protocol is either constructible or not declared, so the state
vocabulary that says otherwise is exactly the residue to remove.

### D-C. `openai-responses` enters as the third executable binding

`agenticopenai.NewResponsesModel` (`responses_model.go:165`) is the binding.
There is no fourth adapter, no reasoning-only adapter, and no new abstraction
for reasoning: reasoning arrives as `schema.Reasoning` content blocks on the
agentic generation.

---

## 4. The rejected alternative, and why it is rejected

The upstream project investigated for this decision (`my-llm-py`) solves the
same problem with a different pattern: a **hub-and-spoke codec**.

```
convert = encode(source protocol) -> intermediate -> decode(target protocol)
```

Its advantages are real and are the reason it was worth studying: 2N conversion
functions instead of N², stateless codecs, and an explicitly narrowed lossless
boundary (same-protocol ↔ intermediate only).

It is rejected here for one decisive reason and two measured ones.

**Decisive: its intermediate representation is open.** Field-by-field audit
showed it is not a protocol-neutral core but a hybrid — a normalized semantic
core plus source-shaped residue kept at the original paths and indices, plus
reserved snapshot namespaces. It is also publicly reachable
(`MyCodec.encode/decode/convert`) while documented as internal. A closed, typed
representation is a Vivy invariant (`AGENTS.md`, "Single source of truth": every
run-path fact has exactly one home; a second copy is a deletion task). Adopting
an open intermediate representation would create a second, lossy home for every
protocol fact.

**Measured: the per-protocol cost is far higher than the pattern it replaces.**
Adding one protocol to the hub costs 11–12 new files plus 7 mandatory edits in 6
existing files, plus up to 5 conditional edits — 16–19 files, against one
constructor reference and one `case` here.

**Measured: it has live silent-fallback hazards.** Three were verified
unfixed upstream: a request path that falls through to a default parameter
family, a usage path whose `else` emits another protocol's counters, and an
invocation path whose `else` selects a different SDK. A dispatch seam with an
`else` that guesses is the opposite of fail-closed.

**What is transferred is discipline, not architecture:**
hand-written native fixtures (never generated by the converter under test);
"the SDK parsed it" ≠ "the protocol conformed"; fail-closed at every dispatch
seam with no defaulting `else`; explicit lossless boundaries; and
concurrency/lifecycle verification. These become acceptance criteria in
`S2-local-httptest-loop.md`.

---

## 5. "No residue" as three verifiable rules

The requirement "no residual data" is read on two axes at once: no leftover
deferral scaffolding after the introduction, and a representation that is closed
and typed rather than an open residue-carrying structure. Both reduce to three
rules, each checkable by a command.

| Layer | Rule | How it is checked |
|---|---|---|
| **No residual declarations** | Nothing is declared that cannot be constructed. After the change, `internal/provider/data/vendors.yaml` declares no endpoint on a protocol with no binding | Bidirectional consistency gate between the embedded data and the binding table (already the model of `D15` in the provider-registry ledger) |
| **No residual generation** | `schema.Message` is 0 on the run path. One generation exists in `internal/provider` and `internal/runtime` | Counting: today `internal` non-test is **145 occurrences / 24 files**; target is 0 in production paths. `schema.AgenticMessage` is **0** today |
| **No residual machinery** | No hub, no codec, no residue/snapshot namespaces, no second message type, no adapter capability state | Structural: the deleted symbol list in `S3-migration-inventory.md` has no surviving consumer |

Explicitly **preserved, not deleted**: `sdk/port/support.go:12-16` (the generic
5-state port support vocabulary) and
`sdk/port/providerprofile/providerprofile.go:34-41` (the declarative public
Profile, which has no capability field and must not gain one). Those belong to
the plugin platform's axis, not to the provider path. Deleting them would be
scope contamination in the other direction.

---

## 6. Boundary conditions: when this concept is false

Honest limits, each of which would change the plan rather than be worked around.

1. **`AgenticModel` is upstream-marked Beta.** A breaking change in a patch
   release is a live possibility. Mitigation is architectural, not optimistic:
   pin exact versions and re-run semantic regression gates before upgrades.
   Import quarantine bounds the packages, not the work: S3 inventories 24
   production files, seven owned middlewares, and durable checkpoint effects.
2. **Moving Anthropic is an all-or-nothing constraint of this route, not an
   irreversible operation.** `anthropic-messages` currently works on
   `components/model/claude v0.1.25`. If `agenticclaude v0.1.7` cannot preserve
   required behavior, stop this route rather than silently weakening Anthropic.
   S4 decides adoption explicitly and specifies upgrade/rollback handling for
   persisted runs; reverting code alone does not prove state compatibility.
3. **Tool binding changes from a method to an option.** 33 `WithTools(` call
   sites exist across `internal/runtime` and `internal/provider`. Each becomes a
   request-time option. This is mechanical but not optional.
4. **Streaming chunk types change.** `ContentBlockChunk` / `StreamingMeta`
   replace the `*schema.Message` stream. Journal projection and compaction are
   **semantic** changes, not renames.
5. **A second OpenAI client enters the binary.** `agenticopenai` requires
   `github.com/openai/openai-go/v3` (`go.mod:12`). Whether this is a net
   addition depends on whether the current stack's components are removed; that
   is a measurement, not an argument — `S0-dependency-truth.md` requires the
   number. Note that `agenticclaude`'s transitive dependencies
   (`anthropic-sdk-go v1.56.0`, the AWS SDK block, `google.golang.org/api`) are
   **already present** in this repository (`go.mod:34-36,41-55,135`), so that
   side is predicted near-zero and must be confirmed, not assumed.
6. **A known upstream race must be re-checked after the version bump.**
   `docs/DEFER.MD:118` records `EINO-TOOLSNODE-ERR-RACE` (a shared `err` written
   by parallel tool goroutines). A version bump is an opportunity and a risk
   for that row; the migration must assert the state of it, not inherit an
   unverified assumption.
7. **A vendor-compatibility question remains open.** The catalog ships 45
   vendors with custom base URLs, and the DeepSeek path depends on the
   OpenAI-compatible request carrying a `thinking` extra field. Whether
   `agenticopenai.NewChatModel` preserves that expressiveness is an `S1` item.
   If it does not, the response is a finding for `S4`, not a Vivy-owned
   workaround: `AGENTS.md` forbids building a custom substitute for a missing
   pinned capability.

---

## 7. Why this is the frugal choice

The user's standing constraint is that the existing mode should not be changed
for its own sake. This concept does not redesign the provider trunk. The trunk —
one sealed adapter table, one embedded data source, derived projections,
fail-closed resolution — is sound and stays.

What changes is one generation, and what is deleted is one state vocabulary. The
concept introduces no new mechanism: no hub, no codec, no bridge, no adapter
interface, no capability matrix, and no new port. It avoids the owned conversion layer of the compared hub design: pinned
components perform protocol conversion, as `AGENTS.md`'s "Eino-native capability
second" asks for. This does not yet prove lower total lifecycle cost than
keeping the current behavior and deferring Responses. S4 must weigh measured
cost, preserved behavior, and the user-visible benefit before scheduling migration.
