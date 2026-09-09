# Vivy Port Catalog v1

> Status: **Normative**
> Decision date: 2026-09-09
> This file is the sole list of selectable v1 Ports. Implementations may not
> add a Port by extending an enum or registering an unreviewed string.

## 1. Catalog rules

- `std/*` Ports are public contracts.
- `core/*` Ports are internal contracts and never appear in the public SDK.
- Every Provider has exactly one Host Consumer named below.
- `0..n` means multiple Providers may coexist under namespaced identities.
- `0..1` means exclusive; a duplicate is an Assembly compile failure.
- Ordered multi-Provider Ports derive their order from the Generation Recipe.
- A selected Provider without its Host Consumer is an Assembly compile error.
- A Port is selectable only after the seven-artifact completion standard in
  `VIVY-MODULE-STANDARD.md` reaches `SUPPORTED`.

## 2. Public Ports

| Port | Cardinality | Consumer | v1 state |
|---|---:|---|---|
| `std/tool@v1` | `0..n` | ToolHost | `SPECIFIED` |
| `std/tool-world@v1` | `0..n` | ToolHost | `SPECIFIED` |
| `std/channel@v1` | `0..n` | ChannelHost | `SPECIFIED` |
| `std/face@v1` | `0..1` | FaceHost | `SPECIFIED` |
| `std/provider-profile@v1` | `0..n` | ModelHost | `SPECIFIED` |
| `std/context-source@v1` | `0..n` | ContextHost | `SPECIFIED` |
| `std/skill-source@v1` | `0..n` | SkillHost | `SPECIFIED` |
| `std/middleware/pre-tool@v1` | `0..n`, ordered | ToolHost | `SPECIFIED` |
| `std/observer/run@v1` | `0..n` | ObserverHost | `SPECIFIED` |
| `std/observer/diagnostic@v1` | `0..n` | ObserverHost | `SPECIFIED` |
| `std/status-source@v1` | `0..n` | StatusHost | `SPECIFIED` |
| `std/ui-extension@v1` | `0..n`, ordered | PresentationHost | `SPECIFIED` |
| `std/ui-root@v1` | `0..1` | PresentationHost | `SPECIFIED` |
| `std/control-action@v1` | `0..n` | ActionHost | `SPECIFIED` |

The state is intentionally `SPECIFIED`: this documentation delivery adds no
functional implementation. No v1 Port may be advertised as shipped until its
later implementation phase supplies the full evidence set.

## 3. Tool and ToolWorld

### `std/tool@v1`

Provides a statically known model-visible Tool Definition and implementation.
ToolHost owns registration, schema validation, dispatch, policy, approval,
result bounds, and Journal projection.

Tool IDs are namespace-qualified. Duplicate IDs fail. A Tool Provider cannot
execute another Tool directly or add itself to the model without ToolHost.

### `std/tool-world@v1`

Provides a dynamic catalog whose entries may depend on a workspace or runtime
instance. ToolHost owns discovery deadlines, cache invalidation, schema hashes,
visibility, and dispatch. MCP-discovered tools enter through this Port and then
follow exactly the same ToolHost path as static and internal tools.

Discovery is not execution approval. A schema hash change that cannot be
reconciled marks the instance unavailable; it never silently changes a live
call contract.

### Protected internal Tools

The following IDs are implemented only by T1 Modules and are included in the
default VIVY CODE Generation:

```text
ask_user
list_dir
read_file
search_files
write_file
patch
multiedit
execute
bash
skills_list
skill_view
```

They still provide `std/tool@v1` and use the one ToolHost. `internal` describes
source and Trust, not a parallel execution Port.

Public Modules MUST NOT define, shadow, alias, replace, or override these IDs.
A public Module with `fs.read` or `fs.write` may use its scoped Host facade; it
does not become the protected `read_file` or `write_file` Tool. A deliberately
minimal Recipe may omit a protected Tool explicitly, but no public Provider may
fill the reserved ID.

## 4. Unified Tool execution

All internal, public, and MCP-derived Tools follow:

```text
lookup
  -> argument schema validation
  -> Kernel Policy evaluation
  -> ordered pre-tool Middleware
  -> schema + Policy + Grant recheck after every rewrite
  -> final approval
  -> execution
  -> bounded and redacted result
  -> Journal projection
```

There is no fast path for a built-in Tool and no privileged bypass for a
public Provider.

## 5. Channel and Face

### `std/channel@v1`

Provides an inbound/outbound messaging adapter. ChannelHost owns listener
binding, normalized envelopes, identity, replay/deduplication, admission,
`Service.Run`, outbound Journal state, and lifecycle deadlines. A Channel is
never adapted into a model Tool.

Multiple Channels coexist under namespaced instance IDs. An unconfigured
Channel is inactive and performs no network action.

### `std/face@v1`

Provides one complete human control-plane client. A Headless Generation has
zero Face Providers. An Interactive Generation has exactly one. FaceHost owns
RPC authentication and event delivery. A Face cannot invoke Runtime internals
directly.

UI contributions modify a selected Face; they do not create a second Face.

## 6. Provider Profile

### `std/provider-profile@v1`

Provides declarative provider configuration: profile identity, model IDs,
endpoint class, supported user-facing options, and Secret references. It does
not provide executable model code.

ModelHost and internal Provider adapters own execution. They MUST use a
suitable pinned Eino/EinoExt component when one exists. If the pinned surface
does not provide the needed capability, that provider capability is
`DEFERRED-INDEFINITE`; Vivy does not add a public `std/model-provider` Port or a
custom parallel provider stack.

## 7. Context and Skill Sources

### `std/context-source@v1`

Provides bounded content candidates with stable source identity, content type,
timestamps, confidence metadata, pagination, and size estimates. ContextHost
owns authorization, query fan-out, deduplication, ranking, token budgets,
redaction, provenance, and final Runtime projection.

A Context Source cannot write the final Prompt, inject a System Message, call
a Model, start a Run, or expose Eino types.

### `std/skill-source@v1`

Provides Skill metadata and content by stable ID with version, source hash,
dependency declaration, and availability. SkillHost owns validation, conflict
resolution, Trust provenance, activation scope, budget, and Runtime projection.

Skill text is governed instruction data. It receives no Tool, filesystem,
network, or Secret authority by being loaded. `skills_list` and `skill_view`
remain protected internal Tools.

## 8. MCP ownership

MCPHost is an optional T1 organ included in the default Generation. MCP Server
instances are always T3 external-untrusted. MCPHost owns transport lifecycle,
credentials, session isolation, discovery, schema conversion, namespace,
timeouts, retries, and circuit state.

```text
MCP Server
  -> MCPHost
  -> std/tool-world dynamic catalog
  -> ToolHost governance
```

MCP Resources may enter ContextHost only through an explicit bridge. MCP
Prompts do not automatically become Vivy Skills; import requires SkillHost
validation. No configured instance means no connection attempt.

MCP adaptation MUST use the pinned EinoExt capability when it meets the
requirement. A missing upstream transport or OAuth capability is
`DEFERRED-INDEFINITE`, not a custom Vivy protocol implementation.

## 9. Middleware

### `std/middleware/pre-tool@v1`

This is the only public execution-changing Middleware Port. A Provider returns
one of four typed results:

```text
Pass
Deny(reason code, safe message)
RequireApproval(reason class)
RewriteArgs(new arguments, rationale)
```

Middleware cannot change Tool identity, invoke the Tool, register a Tool,
write the Journal, start a Run, or expose Eino types. Ordering comes only from
the Recipe and is embedded in the Generation Manifest.

After every rewrite, ToolHost repeats schema, Policy, and Grant validation
before the next Middleware. Timeout, panic, invalid output, or unavailable
Middleware fails the Tool call closed. Advisory work belongs in an Observer.

## 10. Observers and Status

### `std/observer/run@v1`

Receives ordered, redacted projections after the source Journal event commits.
Delivery is at least once; Providers deduplicate by stable event ID and persist
their Host-managed cursor. Observer failure cannot roll back a completed Run.

### `std/observer/diagnostic@v1`

Receives bounded logs, metrics, and ephemeral diagnostics. Delivery is best
effort through bounded buffers. Dropped data is allowed only when a visible
drop counter is incremented. It is not an audit or accounting source.

### `std/status-source@v1`

Provides read-only status snapshots for owned Module or runtime instances. A
status read MUST NOT start, probe, revive, or reconfigure the subject. StatusHost
applies deadline, item-count, redaction, and namespace limits.

Observers and Status Sources cannot mutate Run state. A follow-up action must
enter through a fresh Control Action or `Service.Run` request.

## 11. Full-code UI

### `std/ui-extension@v1`

Provides arbitrary frontend code that can extend or modify all existing Web
Face UI: routes, navigation, pages, components, state, styles, themes,
shortcuts, commands, and interaction flows. Multiple Providers compose in the
exact Recipe order.

### `std/ui-root@v1`

Provides an exclusive replacement for the Web Face root UI. A duplicate is an
Assembly compile failure.

There is no UI Grant, approval prompt, component allow-list, CSS isolation, or
per-DOM audit. Selection into the Generation gives a UI Module complete browser
UI access by default, including browser APIs and client-visible state. It is T2
trusted code.

UI Modules are compiled and content-hashed during pack. Runtime remote-code
download and T3 UI injection are forbidden. Composition uses explicit
`before`, `after`, and `replaces`; loading order and last-writer-wins are not
valid conflict resolution.

Backend boundaries remain authoritative. A UI can hide or replace an approval
view; it cannot forge server approval, read raw Secrets, write the Journal,
execute a Tool, or start a Run without the server-side path.

## 12. Control Action

### `std/control-action@v1`

Provides a namespaced, schema-typed backend operation invoked through one
ActionHost RPC method. It supports plugin settings, connect/disconnect,
refresh, and other Module-owned control actions without arbitrary RPC routes.

Every Action declares:

- input and result schemas;
- `read`, `write`, or `external-effect` effect;
- owning Module and runtime instance;
- required Grants and failure model;
- whether a human approval is required.

ActionHost authenticates the caller, verifies instance state, validates both
schemas, applies Policy and Grants, invokes the Provider, bounds the result,
and emits the required audit or state projection. The UI needs no separate UI
authorization. Server-side checks cannot trust identity or permission results
sent by the browser.

An Action cannot expose a raw RPC/HTTP route or directly mutate Kernel Run
state. Starting an Agent Run re-enters `Service.Run`; model-visible execution
re-enters ToolHost.

## 13. Internal Ports and Hosts

| Internal Port/Host | Cardinality | Requirement |
|---|---:|---|
| `core/loop-driver@v1` | `1` | Every Agent Generation |
| `core/chat-model-host@v1` | `1` | Every Agent Generation |
| `core/tool-host@v1` | `1` | Default product Generation |
| `core/storage-engine@v1` | `1` | Every Generation |
| `core/checkpoint-store@v1` | `1` | Every Agent Generation |
| `core/credential-resolver@v1` | `1` | Every Generation |
| `core/sandbox-backend@v1` | `1` | Every Agent Generation |
| `core/context-host@v1` | `0..1` | Required when a Context Source exists |
| `core/skill-host@v1` | `0..1` | Required when a Skill Source exists |
| `core/mcp-host@v1` | `0..1` | Default-on; inactive without instances |
| `core/observer-host@v1` | `0..1` | Required when an Observer exists |
| `core/status-host@v1` | `0..1` | Required when a Status Source exists |
| `core/presentation-host@v1` | `0..1` | Required when the selected Face consumes UI |
| `core/action-host@v1` | `0..1` | Required when a Control Action exists |

L0 owns ChannelHost and FaceHost authority; they are not replaceable Provider
slots even though they consume public Ports.

## 14. Closed surfaces

The following are not public Ports:

- executable Model Provider;
- arbitrary RPC or HTTP route;
- arbitrary Journal reader, writer, or backend;
- Policy final evaluator;
- Credential backend;
- raw Eino Graph, Lambda, callback, or schema type;
- runtime Module loader;
- Kernel Service replacement;
- untyped global event listener;
- Tool executor that bypasses ToolHost.

Opening one requires a new architecture decision and a catalog version change.
