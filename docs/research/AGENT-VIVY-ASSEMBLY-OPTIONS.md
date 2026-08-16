# AGENT-VIVY Assembly Options

> Status: proposed direction
> Updated: 2026-08-06
> Scope: V0 fast assembly of a stable, usable Agent
> Related: `AGENT-VIVY-DIRECTION.md`

## 1. Problem Definition

The V0 goal is not to build a general Agent framework or migrate `agent-diva`. It is to assemble a stable new Agent quickly, using mature reference-project code where that reduces delivery risk.

The first product slice must be real and recoverable:

```text
Session -> message -> model stream -> optional tool -> approval -> result
        -> persisted run -> visible events -> cancel/restart recovery
```

The implementation may be assembled and internally replaceable. The user-facing workflow must be stable, testable, and free of production-only mocks.

## 2. Candidate Directions

### A. Eino-first

Use Eino ADK and components as the Agent execution engine. Add a thin Vivy application layer for configuration, sessions, persistence, permissions, events, and UI API.

```text
Vivy UI
  -> Vivy App API
  -> Vivy Run/Application layer
  -> Eino ADK Runner / ChatModelAgent
  -> Provider and Tool components
```

Use Eino for:

- ChatModel and provider components;
- ReAct/tool loop;
- streaming;
- component composition;
- callbacks;
- interrupt/resume experiments;
- future graph/workflow probes.

Do not expose Eino types directly as the Vivy public protocol. Map Eino events and results into Vivy-owned DTOs and persisted records.

Advantages:

- shortest path to a working Agent;
- already supports model, tool, stream, callback, graph, and HITL concepts;
- avoids immediately designing a second agent framework;
- future SystemV probes can be built around real behavior rather than abstract plans.

Risks:

- Eino's Agent lifecycle and checkpoint semantics may not match Vivy's product needs;
- public behavior can accidentally become coupled to Eino event types;
- provider/component dependencies may grow quickly;
- tool authorization and durable recovery still need Vivy code.

Decision: **recommended for V0 execution**.

### B. Crush selective transplant

Do not port all of Crush. Borrow or adapt selected patterns and code for:

- configuration service;
- SQLite schema and migration discipline;
- session/message services;
- permission service;
- pub/sub event broker;
- tool decorator and hook patterns;
- application composition.

Keep Crush's coding-agent-specific features out of V0:

- LSP manager;
- coding-specific file tracker;
- Bubble Tea UI;
- provider catalog breadth;
- coding-agent prompt and session semantics;
- MCP breadth unless a concrete V0 workflow needs it.

Advantages:

- strong reference for a usable Go application rather than a library;
- explicit service composition and SQLite-backed lifecycle;
- useful patterns for permissions, events, queues, and shutdown.

Risks:

- direct copy of internal Crush code creates license, update, and hidden-coupling obligations;
- Crush's center of gravity is an interactive coding assistant, not a general personal Agent;
- porting its mature concurrency behavior may take longer than writing a smaller Vivy version.

Decision: **selective reference and small-module adaptation only**.

### C. Eino + Crush hybrid

Use Eino for the model/tool execution path and Crush as the application-assembly reference.

```text
Vivy UI
  -> Vivy HTTP/IPC API
  -> App services: Config, Session, Run, Event, Permission
  -> Agent runner adapter
  -> Eino ADK
  -> Eino/provider/tool components
```

Advantages:

- fastest route to a real Agent;
- avoids writing an Agent loop from scratch;
- avoids copying Crush's whole application;
- separates execution engine from product-owned lifecycle and API.

Risks:

- two reference idioms must be reconciled;
- adapter and event mapping need discipline;
- careless copying can produce a small version of the old Diva problem.

Decision: **recommended overall**.

### D. Direct Diva port to Go

Translate existing `agent-diva` behavior and modules into Go, preserving broad product scope.

Decision: **reject for V0**. This recreates compatibility pressure, feature inventory debt, and the failed deep-governance migration pattern.

### E. Build a new Agent framework first

Design Vivy-owned model, tool, workflow, context, and memory abstractions before using a mature runtime.

Decision: **defer**. It is appropriate only after V0/V1 usage identifies a real limitation in Eino or the need for a future SystemV probe.

## 3. Recommended V0 Architecture

Use the C hybrid, but keep the implementation small:

```text
agent-vivy/
  cmd/vivy/                 process entrypoint
  internal/app/              composition and lifecycle
  internal/config/           config loading and validation
  internal/domain/           Vivy-owned Session, Message, Run, Event, ToolCall
  internal/runtime/          Run service and Eino adapter
  internal/provider/         provider selection and secret boundary
  internal/tools/            narrow tool registry and approval policy
  internal/session/          SQLite persistence and migrations
  internal/events/           event fan-out and replay cursor
  internal/httpapi/          UI-facing command/query/event API
  schemas/                   JSON Schema or OpenAPI contract
  fixtures/                 provider, event, recovery fixtures
  ui/                        new UI, kept independent of runtime internals
```

The first Go module should remain a single application module. Do not split into many Go modules or imitate either the 17-crate Diva workspace or the SystemV crate graph.

## 4. Vivy-Owned Contract

Eino and copied reference code stay behind adapters. The product-owned minimum model is:

```text
Session
Message
Run
RunEvent
ProviderRef
ToolSpec
ToolCall
Approval
MemoryItem
```

Minimum lifecycle events:

```text
run.started
model.delta
model.completed
tool.requested
tool.approval_required
tool.started
tool.finished
run.completed
run.failed
run.cancelled
```

Every event must have a `run_id`, monotonic sequence, timestamp, and JSON payload version. Terminal events must be unambiguous. The UI consumes these events but does not derive domain state from button success or local optimistic state.

## 5. V0 Capability Scope

### Keep

- one OpenAI-compatible provider;
- one mock provider for unit tests;
- Eino ChatModelAgent or equivalent runner;
- streaming model output;
- one or two controlled tools;
- explicit approval for effectful tools;
- SQLite sessions, messages, runs, and events;
- cancellation via `context.Context`;
- restart recovery;
- structured errors and logs;
- a new UI backed by the Vivy application protocol.

### Adapt

- Eino stream events -> `RunEvent`;
- Eino tool calls -> `ToolCall` and approval records;
- Crush config/session/permission patterns -> Vivy-owned small services;
- Diva provider configuration behavior -> only where useful, with secrets kept out of persisted config;
- existing UI assets -> only after checking whether they fit the new workflow.

### Deferred

- multiple providers and model catalog breadth;
- MCP and plugin loading;
- multi-channel gateway;
- full shell/sandbox system;
- complex context compaction;
- long-running scheduler;
- memory retrieval and knowledge/RAG;
- multi-agent Team and graph Workflow;
- desktop pet/VRM/voice;
- SystemV integration and Rust migration work.

### Drop

- Diva's 134-command compatibility surface;
- direct import of `agent-diva` crate boundaries;
- direct dependency on the archived deep-governance runtime;
- Eino or Crush types in the public UI contract;
- provider breadth copied without a V0 consumer;
- mock data in production paths;
- agno full-framework compatibility.

## 6. Capability Re-entry Rule

V0 deliberately keeps a narrow scope, but `Deferred` does not mean a capability is abandoned. Diva capabilities must be inventoried and then brought into Vivy only through a new proposal and redesign process:

```text
Diva capability inventory
  -> capability proposal
  -> new product and UX design
  -> architecture decision
  -> Vivy implementation
  -> real-workflow validation
```

A proposal must describe the current user problem, desired workflow, acceptance tests, security/approval model, state and event semantics, persistence/recovery requirements, UI consequences, and the explicit decision to `Keep`, `Adapt`, `Defer`, or `Drop`. It must not begin from Diva's package boundaries, Tauri command names, old data schema, or backend implementation.

The intended end state is not necessarily a permanently small Vivy. It is a fuller Agent product whose capability set can re-enter over time, while each capability is rebuilt under a better architecture rather than mechanically ported.

## 7. UI Direction

The fastest safe UI path is a browser-based development shell first, with a thin desktop wrapper only after the API and workflow are stable.

Recommended order:

1. Vite-based UI using the Vivy HTTP API and event stream;
2. real session/run/tool/approval/recovery states;
3. Playwright or browser smoke against the real Go process;
4. optional Tauri or another desktop host after the product workflow is accepted.

The UI should be new rather than a 134-command port. A thin desktop wrapper may be added later without changing the runtime contract.

## 7. Assembly Order

1. Create the Go module and process health endpoint.
2. Add a mock provider and a deterministic run fixture.
3. Add Eino ChatModelAgent behind `internal/runtime`.
4. Normalize model/tool events into Vivy events.
5. Add SQLite session/run/event persistence.
6. Add one read-only tool and one approval-gated effectful tool.
7. Add cancellation, terminal event rules, restart recovery, and error injection tests.
8. Build the new UI against only the Vivy API.
9. Run Windows real-provider and recovery smoke tests.
10. Decide whether the assembled path is ready for V1 operation.

Each step must leave the previous vertical slice passing. Do not add new feature families before the existing slice has a real verification result.

## 8. Go / No-Go Gates

Proceed beyond V0 assembly only if the system can demonstrate:

- real provider streaming;
- deterministic mock-provider tests;
- tool approval and denial;
- cancellation that produces one terminal `run.cancelled` event;
- restart recovery without duplicate terminal events;
- persisted event ordering;
- UI operation without mock domain records;
- bounded shutdown on Windows;
- no secrets persisted in SQLite, event payloads, or UI storage.

Stop or redesign if:

- Eino lifecycle cannot provide the required cancellation/recovery semantics;
- the UI requires direct access to Eino or internal Go objects;
- permissions are implemented as UI-only checks;
- copied Crush code becomes the dominant architecture;
- the project starts accumulating compatibility aliases without a current consumer;
- the main justification for a redesign is language preference without workload evidence.

## 9. Working Recommendation

Start with **Eino-first execution plus a Vivy-owned thin application shell**, using Crush as a selective reference for configuration, SQLite, sessions, permissions, events, and lifecycle wiring.

Do not start by porting Diva. Do not start by implementing SystemV. Do not build a new Agent framework before a real V0 workflow exposes the need.

The first meaningful deliverable is a small, real, restartable Agent with a new UI and an explicit event contract. Its internal parts can be replaced later; its verified product behavior becomes the evidence base for the next Vivy stage.
