# Provider management and plugin boundaries

Research and implementation checkpoint, 2026-10-26. DESIGN records the accepted
protocol/authentication boundary; README describes the implemented configuration
and commands. The implementation record below supersedes earlier proposals.
Historical reference findings do not expand that scope.

Owner refinement, 2026-10-26: support exactly two provider protocol families,
OpenAI-compatible and Anthropic-compatible. Built-in providers are presets and
user-defined providers are configured instances derived from those families.
Subscription login, OAuth, device/browser login and token refresh are excluded.
Native Gemini and cloud-specific protocols are excluded; a service exposing one
of the two supported formats can be configured through that format.
Keep Rig's execution path. Add management of configured provider instances,
their credentials and model catalogs over its existing Connector. Reuse native
presets and compatible protocols already present in Rig; a vendor does not need
a new local protocol plugin. The recipe selects owned management capabilities
and any additional implementation dependencies. Runtime settings configure
instances of compiled capabilities. A compatible gateway remains one instance.

## Source revisions

| Project | Revision and inspected boundary |
| --- | --- |
| vivy-test | `737b5b6`, current gateway, recipe, model controls, launcher and process environment filtering |
| Rig | `55cd481a2421fd5da468b5bd6afc6ac876883599`, pinned catalog, Connector, provider registry, LoginPlugin, defaults and Connection |
| Pi | Current `d5b3de22d0e24a3e9c38a9410d3329b099717283`; the inspected Provider interface and model/custom-provider documents have no diff against the repository's Pi baseline `f321e5bdd26b3624bfb2ab182a334d0a5edb698a` |
| deepseek-harness | `22ddee3792402c3e70ec5f86a63c3e3e895a2f60`, current default branch: provider guide, LLM registry, pi-ai adapter, API-key and account routes |

Pi was cloned into ignored `.workspace/pi` and left detached at the handoff's
baseline. DSH files were read through GitHub; neither reference is a product
dependency. The evidence below is source inspection and the current generated
package's offline Cargo graph, plus the offline construction probe recorded
below. None is a new live-provider compatibility test.

## What the references establish

### Pi

The [model guide][pi-models] separates login, model selection, compatible
endpoints and executable provider extensions. `/login` and `/logout` manage
stored credentials; `/model` selects an available model. Session model choices
and startup defaults are separate. `models.json` supplies endpoint/model
configuration, and model overrides preserve the rest of a provider's catalog.

The [provider extension contract][pi-custom] supports both a complete native
Provider and a legacy configuration form. A [Provider][pi-provider] owns its
identity, authentication, synchronous model catalog, optional refresh and
credential-specific filtering, and request streaming. A new compatible endpoint
usually needs configuration; a new authentication flow or wire protocol needs
code. Registration can reuse an existing protocol rather than copying it.

Borrow the user's workflow, metadata overrides and registration model. Do not
import the Pi TypeScript extension runtime or pi-ai into the Python worker.

### deepseek-harness

The [LLM service][dsh-llm] is provider-neutral; adapters register named routes.
Its [source contract][dsh-registry] has separate adapter, configurable-provider
directory and discovery registrations. Duplicate routes are rejected, route
replacement is atomic, and registrations are disposed with their owning plugin.

The [pi-ai adapter][dsh-pi] serves many configured providers from one adapter
plugin. Built-in providers supply defaults; custom routes specify endpoint,
protocol and models. Configuration changes affect subsequent operations through
captured snapshots. An explicit credential reference that cannot resolve fails
instead of silently selecting an unrelated ambient key.

The [provider guide][dsh-guide] distinguishes third-party provider presets from
custom APIs and separates stored secrets from settings. Custom provider IDs are
stable because sessions and defaults reference them. Discovery assists editing;
manual model IDs remain available when an endpoint cannot list models.

The [API-key][dsh-key] and [account][dsh-account] plugins share a Messages
transport but own distinct authentication, model admission and account failures.
This demonstrates that transport, credential and commercial account semantics
need not be one large provider implementation. Their special account eligibility
and server-version gates are not general requirements for vivy-test.

### Rig and the current experiment

Rig already has [Catalog and Connector][rig-connector], with ModelSpec,
`Provide`, `SignIn`, `ProviderId::declare` and `Connector::provide`. Provide can
be a small closure returning an ErasedHandler. The existing GatewayPlugin uses
that seam, so a provider supplied by a recipe-selected Rust plugin is already
demonstrated by this repository's assembly and scripted-gateway checks.

The upstream [LoginPlugin][rig-login] includes provider API-key entry and
ChatGPT browser/device login with token refresh. The recipe deliberately omitted
it. Under the final scope, do not enable the whole plugin: reuse the relevant
API-key storage/UI pieces without installing its subscription login commands.
It does not provide a complete editor for multiple custom instances.
Its saved-key helper does not replace a connector already registered by another
plugin, which also matters when preserving GatewayPlugin.

Current gaps are specific: one gateway endpoint/key, shared metadata across its
listed models, no provider-instance CRUD UI, no explicit provider-first picker,
no configuration reload for captured endpoint/key state, and no unified policy
for direct-provider credentials. `/model vendor/model` already exists.

### Rig follow-up: two provider sources, one execution pipeline

The existing chain is `Models -> Connector -> ModelSpec/Provide -> native
ProviderConfig -> completion model/handler -> existing agent execution`.
`Models::default()` already contains `Connector::default()`, which starts with
`Catalog::builtin()`. Native providers are not absent from the current host just
because startup defaults to `gateway/...`; management and deliberate selection
are the missing layer.

| User-facing category | Rig reuse | Owned work |
| --- | --- | --- |
| Built-in provider over OpenAI or Anthropic format | Catalog rows, native presets/dialects, `ConnectOptions`, Connector and API-key/model controls | Expose only supported presets; manage instance/default/credential settings and the provider-first picker |
| User-defined provider over OpenAI or Anthropic format | `ProviderId::declare`, `Catalog::insert(ModelSpec)`, `Connector::provide`, and the corresponding existing `ProviderConfig` | Validate endpoint, credential reference and per-model facts; register each configured instance |

For a built-in identity, `ConnectOptions::new().api_key(...).base_url(...)`
overrides credentials and endpoint while retaining the vendor's dialect. With
no explicit registration, Connector can use the preset's environment lookup.
Use the host's explicit credential policy for configured instances rather than
accidentally inheriting unrelated ambient credentials.

A custom endpoint does not normally require custom protocol code. A new instance
such as `company` can reuse `ProviderId::catalog("deepseek").config(...)` with
its endpoint changed, or a generic OpenAI-compatible config. Insert its own
ModelSpec rows and register a closure calling
`config.completion_handler(spec.reference(), spec.id, http, ModelFacts::new(...))`.
The same seam works with Rig's Anthropic config variant. OpenAI Chat
Completions versus Responses is a route choice on OpenAIConfig within the
OpenAI family. Keep request dialects explicit: a compatible endpoint alone
does not establish vendor-specific reasoning or tool behavior.

One important [API boundary][rig-registry]: `ProviderId::declare("company")`
only creates an identity. Combining it with `ConnectOptions::format(OpenAi)`
does **not** create a protocol preset; `Catalog::connect_with` still rejects it
as `CatalogOnly`. Use `Connector::provide` with the existing configured wire.
Our GatewayPlugin already demonstrates this registration pattern. No second
provider trait, engine or generic adapter registry is required.

The built-in catalog also contains catalog-only entries. Bedrock, Vertex AI and
Gemini gRPC require their companion crates/registrations; Voyage AI's entry is
for embeddings/reranking. Listing a row is not proof that this host can run it
as a coding-agent completion model. Derive exposed availability from the actual
assembled catalog, registered handlers and tool capability, and distinguish
local constructibility from successful remote validation.
Use Rig's canonical IDs rather than guessed display names: for example, this
revision identifies Gemini as `gcp.gemini`, while its protocol family is
`gemini`. The construction probe caught that naming distinction as well.

For the requested first scope, use one small provider-management crate with
built-in and custom configuration branches. If the recipe must independently
select those capabilities, expose two plugin constructors from that crate and
assemble them over the same Models resource. Initialize the selected catalog
once, then append registrations. Do not create one wrapper crate per vendor.
The two protocol constructors reuse Rig's existing wires. Supporting any third
wire or subscription authenticator requires an explicit scope change; runtime
native-library loading remains outside this scope.

Before the two-family decision, an ignored `.workspace/provider-api-probe` was
run with `cargo run --offline`
against the exact pinned Rig revision, using fake keys and `.invalid` endpoints.
The standalone probe explicitly enables rig-core's `reqwest` feature: its
default features alone provide no HTTP client, whereas rig-harness already
enables it. The initial probe exposed `NoHttpClient` before this was corrected.
It checked construction through three built-in protocol families, rejection of
a declared ID with only ConnectOptions, and successful handler construction for
four custom instances over those families, including two instances of DeepSeek
with the same model ID. It also checked that an explicitly empty, populated
catalog rejects an unregistered built-in selection. No HTTP was sent: the probe
does not verify wire serialization, authentication, streaming or tool calls.
Its Gemini construction result is historical capability evidence, not part of
the accepted product scope.

## Recommended ownership

| Concern | Implementation owner |
| --- | --- |
| Streaming, tools, cancellation, usage, compaction and child model calls | Existing Rig execution path through Models and Connector |
| Provider instance ID, display name, adapter, endpoint, credential reference and configured models | One small typed configuration collection in user settings |
| Native protocols and provider request dialects | Rig's OpenAI/Anthropic wires and compatible presets; no third protocol in this scope |
| Model facts | Rig ModelSpec, built-in defaults plus per-model overrides and disclosed discovery facts |
| Credentials | One host-owned store/resolver; settings contain references, never resolved secrets |
| Provider/model selectors and status | Projection of registered adapters, configured instances and their actual catalog/status |

Start with one owned provider-management plugin and small typed adapter
constructors. Use existing Rig types at the call boundary; add no standalone
provider SDK, second model engine, universal Port registry or independent usage
and cancellation path. There must not be another hand-maintained availability
list beside the assembled runtime.

Distinguish an upstream vendor from a configured instance. Two accounts/endpoints
for the same vendor need different instance IDs, such as `company` and `personal`.
The canonical selection `company/org/model-1` identifies instance `company` and
upstream model ID `org/model-1`; the request sends the latter unchanged.

### Proposed user experience

- `/provider` opens configured instances with their model counts and credential
  state. Choosing one opens that instance's model picker.
- `/model` retains global search and the canonical `instance/model-id` argument.
- Provider management offers add/edit/remove, credential entry/removal and model
  editing. An explicit model-list refresh is separate from a connection test.
- Keep the current CLI's `--model instance/model-id`. A separate `--provider`
  convenience option can be added after specifying its interaction with model
  IDs that already contain `/`; it is not needed for the first working slice.
- New-session defaults, a resumed session's recorded choice and an explicit CLI
  override remain distinct. A missing/deleted provider produces a visible repair
  choice, without silently routing the session to a different account.

The following is a proposed runtime configuration shape, not currently accepted
syntax. Keep it in user settings; the recipe selects the management/plugin code.

```toml
[providers.deepseek]
kind = "builtin"
preset = "deepseek"
credential_env = "RIG_SECRET_DEEPSEEK"

[providers.company]
kind = "custom"
protocol = "openai"
base_url = "https://gateway.example/v1"
credential_env = "RIG_SECRET_COMPANY"

[[providers.company.models]]
id = "org/model-1"
context_window = 131072
max_output_tokens = 8192
```

Built-in instances reuse the preset's catalog and allow per-model overrides;
custom instances supply models manually first. A custom instance can name a
known preset instead of a generic protocol when vendor-specific mapping is
needed. The implementation should validate those mutually exclusive choices
and reject presets outside the two supported families. Custom `protocol` accepts
only `openai` and `anthropic`. The first OpenAI acceptance path is the existing
Chat Completions route; adding Responses stays within that family but needs its
own explicit route and acceptance evidence.

Preserve `[gateway]` as a legacy input during migration, mapping it into the same
management/execution path. Do not retain two independently editable stores of
the same endpoint. Update start.ps1, whose current credential-file bootstrap
writes `[gateway]`, with that migration rather than breaking existing starts.

## What plugin support means here

| Extension | Rebuild needed | Suitable mechanism |
| --- | --- | --- |
| Another endpoint/model list using an already compiled protocol | No | Add a provider-instance configuration |
| Another vendor preset over one of the two compiled protocols | No if data-only; yes if it adds code | Configuration or a small recipe-selected preset plugin |
| A third wire protocol or subscription authentication implementation | Outside agreed scope | Requires an explicit new decision |
| Independently downloaded native plugin loaded while the program runs | A separate architecture | Stable FFI, WASM or a process boundary and explicit lifecycle contracts |

The last row is technically possible, but is not the current recipe model. The
[Rust ABI has no stability guarantee][rust-abi]; a Rust trait in a shared library
is not automatically a stable external plugin API. Introduce that boundary only
for a concrete hot-install requirement.

There is an important compilation limit: Rig's [ordinary provider modules][rig-modules]
are part of rig-core, and its pinned manifest has no per-vendor feature gates.
Omitting a local provider wrapper excludes that wrapper's crate/registration;
it does not prove that built-in provider source was excluded. Separate upstream
crates such as rig-bedrock and rig-vertexai can have a real crate boundary.
Claiming complete per-vendor source exclusion would require upstream changes or
a maintained fork, neither justified by this management request alone.

The current Connector defaults to the whole built-in catalog. For deliberate
recipe-selected availability, initialize an empty catalog and populate it from
selected adapters/instances using existing catalog rows. Merely hiding entries
in a picker would leave other selections available through the CLI. This bounds
registered capabilities; it does not strengthen the source-exclusion guarantee.

## Integration details that affect correctness

1. GatewayPlugin captures its key and endpoint during plugin construction, and
   Rig Connection retains its spec/handler. Changing settings or registering a
   new connector alone does not update existing connections. Capture one
   configuration generation per operation; reconnect affected idle agents or
   apply the change at the next model-request boundary. In-flight work keeps its
   original endpoint, credential and metadata.
2. Explicit credential references must fail when missing. API-key entry/removal,
   stored-key versus environment precedence and explicit keyless mode need one
   defined policy. Login/logout, if those command names are retained, mean only
   saving/removing an API key; they never start a subscription authentication flow.
3. The current builder and shell/Python process helper filter RIG_SECRET_*
   variables. Standard names such as ANTHROPIC_API_KEY are not covered by that
   filter. Supporting those names requires filtering the resolved credential
   variables across every owned subprocess path, with tests using fake values.
   Rust plugins remain trusted process code, not a secret sandbox.
4. Model facts belong to individual models: context/output limits, tools,
   images, reasoning and prices can differ within a provider. Unknown facts
   stay unknown; a model-list endpoint is not proof of tool/image/reasoning
   compatibility. Preserve manual entries and user overrides when discovery
   fails. Any remote cache must distinguish endpoints and credentials.
5. Avoid two startup-default owners: the omitted Rig DefaultsPlugin and the
   current gateway initializer would need coordinated ordering and precedence.
   Session restore must retain its recorded provider/model choice.

## Suggested delivery order and acceptance

First deliver OpenAI-compatible and Anthropic-compatible API-key and
keyless-compatible instances, the provider-first TUI,
per-model facts, management persistence, migration from `[gateway]`, and reuse
of the two Rig protocols. Keep custom providers editable even while their
credentials are missing so they can be repaired. The owner selected complete
TUI CRUD for the first implementation, alongside configuration-file editing.
Subscription authentication is excluded, not a scheduled follow-up phase.

Model discovery and metadata refresh can follow once manual configuration works;
preserve usable local data during network failure. A provider catalog alone is
not a reason to add a provider marketplace or automatic package installation.

Acceptance should cover two instances serving the same model ID with different
fake credentials, unchanged wire IDs, add/edit/remove and invalid configuration,
per-model reasoning/context differences, persistence and session restore,
configuration changes during streaming, tool calls and cancellation, malformed
settings, and absence of fake credentials in child environments and logs.
Cover both supported wire formats and reject unsupported protocols/presets
through configuration, TUI and CLI rather than merely hiding them in a picker.
Recipe variants must demonstrate protocol-feature selection and removal of
terminal dependencies at their actual crate boundaries. Rig remains one source
dependency; this does not claim separate protocol crates. Actual selected gateways/providers
still need separate streaming/tool-call verification before compatibility claims.

## Implementation and verification (2026-10-26)

The owner approved complete TUI CRUD, all eligible Rig presets offered when
adding a provider, explicit activation, OpenAI Chat Completions only, and saved
keys plus conventional/custom credential environment references. Implementation
is isolated in sibling worktree `vivy-test-providers`, branch
`feat/provider-management`, based on `3dc38b6`. Reference checkouts are unchanged.

The baseline selects ProvidersPlugin in the existing vivy-gateway crate, both
protocol features, and provider UI with the terminal. GatewayPlugin delegates to
the same manager. The manager starts with an empty catalog, then registers the
configured instances. Native vendor IDs and public instance IDs remain distinct;
wire facts bind to the actual wire identity. OpenAI reasoning uses an explicit
known preset, and generic Anthropic respects the model's output cap.

Configuration uses `[providers]` and `[providers.instances.<id>]`, with model
rows and preset exclusions. Each instance chooses `stored`, `env` or `none`;
missing sources fail explicitly. Built-in variable names prefill the add wizard.
Settings remain editable TOML tables. Writes preserve unrelated sections/comments,
check external edits under a file lock, and roll back key changes if settings
persistence fails. Provider deletion also retains its configuration when key
removal fails. Opening a picker reloads saved credentials changed by another
session. API-key values stay in the user authentication store.

Ruling: the launcher supplies first-run gateway metadata as process hints. The
host imports it only without a managed/legacy gateway and persists it at actual
Startup. This preserves existing provider settings and keeps an empty home
usable. A resource-specific preset such as Azure requires an explicit endpoint;
dotted native vendor names get a valid default instance ID.

A fresh read-only whole-change review found six substantive issues. Corrections
have regression coverage: key/settings conflict handling; persistent Python
worker invalidation when credential filtering expands; rejection/selection of
OpenAI reasoning mappings; binding facts independently of conflicting native
instance names; deterministic model-refresh scheduling; and error redaction.
The HTTP adapter redacts failures before Rig decodes and records them. It reuses
Rig's SSE framer for in-stream errors, covering JSON escapes and split frames;
normal event payloads retain their content. Status and error causes are retained.
Budget-based thinking choices persist their portable named level.

Verification record:

- Baseline before implementation: 20 workspace tests passed.
- Final offline Rust suites pass 132 tests: workspace 22, providers 32, Python 5
  and basics 73 (the Python isolation subprocess also reports its child test).
- Focused tests cover two protocol instances sharing one raw model ID, exclusion
  of unconfigured/native third protocols, credential modes, invalid limits/URLs,
  safe IDs, legacy import, defaults, settings conflicts, saved-key conflicts,
  malformed-file retention and active-parent/idle-child reconnect boundaries.
- Picker-driven tests perform full CRUD, hidden key entry, explicit defaults,
  cancel-without-save, removed-preset-model restoration, budget defaults and
  preset instance IDs. Unit tests remain offline.
- Cross-protocol loopback smoke passes streaming text, Shell/Python tool calls,
  environment filtering, independent headers, unchanged model IDs, output caps,
  resumed choices, HTTP and in-stream authentication failures, error-key
  redaction for saved/environment keys, credential-free persisted records, and
  first-run bootstrap without overwriting an existing instance.
- Real Windows ConPTY at 120x40 and 50x20 passes empty-home setup, both provider
  forms, hidden API-key input, model selection, persistence, explicit defaults,
  resume precedence, file-edit reload, deletion, tool execution and cancellation.
  It also primes Python before adding a credential, then verifies the updated
  worker cannot see that variable. Terminal scheduling diagnostics are checked.
- Single-protocol and no-terminal recipe variants build. The no-terminal normal
  dependency graph excludes rig-tui and ratatui. Initial disposable variants
  resolved fresh dependencies; verification was repeated offline using the
  baseline recipe lock as input. Rig's monolithic source boundary is unchanged.
- Existing scripted gateway, compact terminal, Plan terminal and subagent smoke
  checks pass. Linkers emit the existing MSVC library/export-file message.

All network acceptance uses synthetic loopback fixtures. Actual provider/gateway
compatibility and Linux/macOS runtime execution remain unverified. The feature
delivery was initially kept on its branch. The owner subsequently requested
local integration: `de10ee3` was fast-forward merged into `vivy-test` main on
2026-10-26. All 132 Rust tests were rerun successfully there, followed by a
native launcher/baseline rebuild and the cross-protocol loopback smoke. The
shared `C:/vt` build now belongs to the main checkout. No push is part of that
integration.

[pi-models]: https://github.com/earendil-works/pi/blob/d5b3de22d0e24a3e9c38a9410d3329b099717283/packages/coding-agent/docs/models.md
[pi-custom]: https://github.com/earendil-works/pi/blob/d5b3de22d0e24a3e9c38a9410d3329b099717283/packages/coding-agent/docs/custom-provider.md
[pi-provider]: https://github.com/earendil-works/pi/blob/d5b3de22d0e24a3e9c38a9410d3329b099717283/packages/ai/src/models.ts#L158
[dsh-llm]: https://github.com/deepseek-ai/deepseek-harness/blob/22ddee3792402c3e70ec5f86a63c3e3e895a2f60/packages/llm/llm/README.md
[dsh-registry]: https://github.com/deepseek-ai/deepseek-harness/blob/22ddee3792402c3e70ec5f86a63c3e3e895a2f60/packages/llm/llm/src/index.ts#L208
[dsh-pi]: https://github.com/deepseek-ai/deepseek-harness/blob/22ddee3792402c3e70ec5f86a63c3e3e895a2f60/packages/llm/llm-pi-ai/README.md
[dsh-guide]: https://github.com/deepseek-ai/deepseek-harness/blob/22ddee3792402c3e70ec5f86a63c3e3e895a2f60/docs/user/guide/providers.md
[dsh-key]: https://github.com/deepseek-ai/deepseek-harness/blob/22ddee3792402c3e70ec5f86a63c3e3e895a2f60/packages/llm/llm-deepseek-api-key/README.md
[dsh-account]: https://github.com/deepseek-ai/deepseek-harness/blob/22ddee3792402c3e70ec5f86a63c3e3e895a2f60/packages/llm/llm-deepseek-account/README.md
[rig-connector]: https://github.com/0xPlaygrounds/rig/blob/55cd481a2421fd5da468b5bd6afc6ac876883599/crates/rig-core/src/catalog/connector.rs
[rig-registry]: https://github.com/0xPlaygrounds/rig/blob/55cd481a2421fd5da468b5bd6afc6ac876883599/crates/rig-core/src/providers/registry.rs
[rig-login]: https://github.com/0xPlaygrounds/rig/blob/55cd481a2421fd5da468b5bd6afc6ac876883599/plugins/rig-basics/src/login.rs
[rig-modules]: https://github.com/0xPlaygrounds/rig/blob/55cd481a2421fd5da468b5bd6afc6ac876883599/crates/rig-core/src/providers/mod.rs
[rust-abi]: https://doc.rust-lang.org/reference/items/external-blocks.html#r-items.extern.abi.rust
