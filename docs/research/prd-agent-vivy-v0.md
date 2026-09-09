# PRD — AGENT-VIVY V0 (Assembly Slice)

> **Status:** Draft v0.1 — first formal PRD for the V0 assembly slice
> **Owner:** 📋 John (Product Manager)
> **Audience:** mastwet and the implementation team
> **Output language:** English (per BMM config)
> **Communication language:** Chinese (per BMM config)
> **Updated:** 2026-08-06
> **Related artifacts:**
> - `AGENT-VIVY-DIRECTION.md` (product positioning and staged evolution)
> - `AGENT-VIVY-ASSEMBLY-OPTIONS.md` (V0 implementation options)
> - `_bmad/bmm/config.yaml` (project + module config)
> - **Pending:** Diva capability inventory document (separate artifact, not part of this PRD)

---

## 1. Background and Purpose

AGENT-VIVY is a new Agent product line that is intentionally independent from `agent-diva` and the current `SystemV` codebase. The first version (V0) exists to validate the product shape: a fast, stable, usable Agent assembled from mature reference-project components and a small amount of new code, with explicit product-owned contracts in between.

This PRD covers **only the V0 assembly slice**. It is not a complete product spec and does not constitute a roadmap for every capability AGENT-VIVY may eventually have.

A separate **capability inventory** (deferred to a later artifact) will list Diva's existing capabilities and tag them as `Keep / Adapt / Defer / Drop`. Each capability that returns to AGENT-VIVY must be reintroduced as a new capability proposal and rebuilt under the V0/V1 architecture — never ported from Diva's old implementation.

## 2. Positioning Statement

AGENT-VIVY is a new Agent product line. It is **not** a clone of `agent-diva`, not a Go translation of Diva, not a continuation of the current `SystemV` code, and not a feature-parity replacement for Diva.

The relationship to Diva is capability evidence flow, not source-code or architecture inheritance:

```text
agent-diva
  -> capability inventory, product experience, behavior examples, failure evidence
  -> capability proposal and redesign
  -> agent-vivy
  -> validated concepts and contracts
  -> future Rust/SystemV redesign when justified
```

Diva capabilities may return to AGENT-VIVY over time, but each must be expressed as a new product proposal with current user value, acceptance criteria, security and persistence implications, UI workflow, and explicit boundaries. AGENT-VIVY may eventually reimplement the complete useful product capability set, but it must not preserve Diva's internal package graph, Tauri command contracts, data schema, or legacy architecture as the implementation baseline.

## 3. Goals

### 3.1 Product Goals

1. Deliver a **stable, usable Agent** end-to-end within the V0 slice.
2. Make **session, run, event, tool, and approval behavior visible and recoverable**.
3. Establish a **new UI** built against the V0 product contract, not a port of Diva's GUI surface.
4. Provide a **reproducible development loop** with fast iteration and short build/test cycles.

### 3.2 Technical Goals

1. Validate the V0 architecture: **Eino as execution framework + Vivy-owned thin application shell** (config, session, run, event, permission, persistence, UI API).
2. Keep the V0 runtime a **single Go module** — do not imitate Diva's 17-crate workspace or SystemV's crate graph.
3. Define a **Vivy-owned contract** (`Session`, `Message`, `Run`, `RunEvent`, `ProviderRef`, `ToolSpec`, `ToolCall`, `Approval`, `MemoryItem`) that hides Eino and reference-project internals behind adapters.
4. Produce a **vertical slice** that proves real provider streaming, cancellation, restart recovery, and tool approval under controlled conditions.

## 4. Non-Goals (V0)

- Becoming a complete Agent framework.
- Maintaining compatibility with `agent-diva` data, APIs, UI commands, or schema.
- Migrating Diva's Rust crates or `agent-diva-deep-governance` runtime.
- Continuing the current `SystemV` Rust code as the implementation baseline.
- Supporting all providers (V0 targets one OpenAI-compatible provider plus a mock provider).
- Multi-channel gateway, voice, desktop pet, VRM, complex memory/RAG, AutoDream, Evolution, Team, full Workflow.
- Production sandboxing, complex shell execution, or full MCP breadth.
- Carrying Diva's 134-command Tauri surface into the new UI.
- Mechanical Go-to-Rust translation guarantees.

## 5. Users and Use Scenarios

### 5.0 Product Philosophy

AGENT-VIVY inherits a small set of philosophical anchors from `agent-diva`. These anchors describe what the product **is** and how it must behave; they do not authorize implementation inheritance. The structural rhyme with the Vivy namesake (species must overnight; the heart stays with the human; evolution happens in another room) is recorded in `agent-vivy/docs/architecture/VIVY-WORLDVIEW.md` and does not add or relax these anchors.

#### 5.0.1 Personal gateway application

AGENT-VIVY is a **personal gateway application**. Not a multi-tenant platform, not a hosted service, not a SaaS.

Concretely:

- **Single-user by default.** One operator, one primary device, one local process. Multi-user or "team" capabilities are not part of AGENT-VIVY.
- **Local-first gateway.** The runtime runs on the user's own machine. Network is a means of contacting providers, not a hosting dependency. No central server is required for the product to function.
- **The application is the product.** The agent is delivered as a desktop / desktop-like application the user runs and configures locally. It is not a cloud service the user logs into.
- **Provider is a dependency, not the product.** Provider selection and configuration exist to serve the user's gateway, not to substitute for it. The user's session, memory, history, and configuration live locally.
- **Personal control over data and approvals.** Sessions, messages, runs, events, and approval records belong to the local user. They are not aggregated, uploaded, or cross-shared by AGENT-VIVY itself.

#### 5.0.2 Logs as first-class citizens for problem tracing

Logs are a **first-class product feature**, not an afterthought. The user must be able to trace any past run, any past error, and any past tool call from end to end through persisted log and event records.

Concretely:

- Every `RunEvent` is persisted and is part of the user-visible history (see FR-5, FR-8, FR-11).
- Logs and events share a common correlation identity: `session_id`, `run_id`, `event_seq`.
- The UI exposes a run-detail / event-log view that the user can read without developer tooling.
- Errors carry structured cause categories, not free-text dumps.
- Log retention is a **local user decision**, not a vendor default.
- "I can't see what happened" is treated as a product bug, not a user education issue.

#### 5.0.3 Providers are a curated, pre-baked catalog

Provider integration is **opinionated and pre-prepared**, not "anything-goes plugin sprawl". V0 commits to a small, deliberate set.

Concretely:

- V0 ships **exactly two providers**:
  - **OpenAI-compatible** (covers OpenAI plus any service exposing an OpenAI-compatible endpoint).
  - **Anthropic**.
- Each provider is delivered as a **pre-prepared YAML configuration bundle**, not as user-written ad-hoc config. The bundle declares model IDs, capabilities, default parameter ranges, and known limitations.
- The **bundle schema is adapted from `agent-diva`'s `providers.yaml`** (see D-022): Vivy borrows the field vocabulary that Diva already validated, then rewrites the two V0 entries (`openai` and `anthropic`) into Vivy's owned schema with explicit provenance. Vivy does **not** copy Diva's other provider entries wholesale, and does not copy Diva's Rust source code.
- Adding a new provider is a **product decision**, not a runtime plugin decision. It enters via a capability proposal and ships as another pre-baked bundle (D-019).
- "Provider breadth" is not a proxy for product quality. Two well-prepared providers are preferable to ten half-prepared ones.
- Provider secrets are still never persisted (D-010); pre-baked bundles describe **shape and defaults**, not credentials.

#### 5.0.4 Large-modular decomposition (module = crate / module)

AGENT-VIVY favors **deliberately large, self-contained modules** rather than fine-grained micro-modules. Each top-level product capability is a single, named, independently understandable unit.

Concretely:

- A "module" (capability module / crate / Go package) corresponds to **one user-facing concept or one product subsystem**, not to a single technical layer.
- Modules have explicit, narrow public surfaces and stable internal evolution.
- Module boundaries are stable enough that contributors can read one module in isolation and understand its purpose, contract, and failure modes.
- Cross-module coupling is intentional and visible; "everything depends on everything" is rejected.
- The single Go module rule (D-006) applies **within V0**; the large-modular principle is a **long-term directional preference** for when V0's thin shell is replaced by a fuller architecture (see OQ-3). It must not be used to justify splitting V0 into many Go modules prematurely.
- Naming: AGENT-VIVY uses Go packages in V0 and may adopt Rust crates / Cargo workspaces in a future rebuild. The principle is **one capability = one well-bounded unit**, regardless of language.

#### 5.0.5 Boundary of philosophy vs. implementation

The above anchors are **product constraints**, not implementation constraints. AGENT-VIVY does not inherit Diva's package graph, Tauri command contracts, schema, runtime, or code style. Each anchor governs what the product **is** or how it must **behave**, not how the code is organized.

This boundary is critical: a future contributor reading this PRD must be able to honor these anchors while writing completely new code under V0 (or a future V2/V3) architecture.

#### 5.0.6 What the philosophy excludes

Capabilities that would push AGENT-VIVY away from any of the above anchors are out of scope unless a future capability proposal explicitly revisits the anchor in question. Examples of exclusions:

- Multi-tenant SaaS, hosted control plane, or "platform for others" features (against §5.0.1).
- Silent failure modes, black-box errors, or "check the dev console" debugging (against §5.0.2).
- User-authored ad-hoc provider plugins, "bring your own model" runtime discovery, or unvetted model catalogs (against §5.0.3).
- Micro-modular decomposition where every layer is its own crate with cross-cutting dependencies (against §5.0.4).

### 5.1 Primary User

A developer / power-user who is willing to live with an early-stage Agent and contribute feedback. The user wants:

- to send a message and see streaming output;
- to invoke one controlled tool per session and approve or reject it;
- to refresh the UI without losing run state;
- to restart the process and recover unfinished runs;
- to inspect structured events, errors, and persisted state.

### 5.2 Secondary User

A future contributor who will read this PRD to understand what AGENT-VIVY V0 guarantees and what it deliberately does not.

### 5.3 Reference User (Out of Scope but Acknowledged)

The existing `agent-diva` user base is **not** a V0 target audience. V0 does not promise to replace any existing workflow.

## 6. Scope

### 6.1 In Scope

- A single Go application module for the runtime.
- One HTTP/IPC API for the UI.
- A new browser-based UI shell using the V0 API and event stream.
- One OpenAI-compatible provider path.
- One mock provider for deterministic tests.
- Eino `ChatModelAgent` (or equivalent) behind a Vivy adapter.
- Session, message, run, and event persistence in SQLite.
- One or two controlled tools: at least one **read-only auto-execute** tool and one **effectful approval-gated** tool.
- Cancellation, restart recovery, structured errors, structured logs.
- Process lifecycle: startup, health endpoint, graceful shutdown on Windows.

### 6.2 Out of Scope

- All features listed under §4.
- Diva capability parity: every existing Diva capability is treated as a **proposal candidate**, not an automatic V0 deliverable.
- Diva GUI parity: the new UI is a fresh surface, not a re-implementation of the 134 Tauri commands.
- Diva data migration: V0 schema is greenfield; legacy data is not imported.
- Deep-governance `/v1` reimplementation.
- agno full-framework compatibility.
- Production-only mocks: any UI path exercised by users must run against real backend behavior.

### 6.3 Boundary Statement

The boundary between AGENT-VIVY V0 and `agent-diva` is **product line, not feature parity**. AGENT-VIVY does not inherit Diva's internal architecture, naming, schema, command contracts, or runtime. Where a Diva capability is desired, it must enter AGENT-VIVY through a separate capability proposal document that re-establishes the user value, design, and acceptance criteria. Until such a proposal is approved and implemented, that capability is **out of scope** for V0, not "deferred silently".

## 7. Functional Requirements

### FR-1 — Session Lifecycle

The system shall create, list, load, rename, and delete sessions. Each session has a stable identifier, a creation timestamp, an optional title, and an ordered list of messages.

Acceptance criteria:

- A new session can be created via the HTTP API and is persisted before the API call returns success.
- Sessions can be listed and loaded; loading a session returns its messages in stable order.
- Renaming a session updates the title without losing messages or runs.
- Deleting a session removes its messages, runs, and events in a single transaction.
- Restarting the process and listing sessions returns the same set with the same ordering.

### FR-2 — Message Persistence

The system shall persist user messages, assistant messages, tool-call records, and tool results as part of a session's message log.

Acceptance criteria:

- Messages are append-only; no silent mutation of historical content.
- Each message carries an `id`, `role`, `created_at`, and ordering within its session.
- Tool-call records and tool results reference the same `run_id` as the parent run.
- Reconstructing context for a new run reads from the same persistence layer, not from in-memory state.

### FR-3 — Provider Integration

The system shall integrate one OpenAI-compatible provider and one mock provider behind a single `ProviderRef` boundary.

Acceptance criteria:

- All provider-specific code (auth, base URL, headers, streaming protocol) is hidden behind the `ProviderRef`.
- Switching from the mock provider to the real provider requires no change in call sites.
- Provider configuration (key, base URL, model id) is loaded from the config store, never embedded in source.
- Secrets are not persisted to SQLite, event payloads, or UI storage.

### FR-4 — Run Lifecycle

The system shall model each user request as a `Run` with explicit lifecycle states (`accepted`, `queued`, `active`, `completed`, `failed`, `cancelled`).

Acceptance criteria:

- A run transitions to `accepted` immediately when the API call accepts the request.
- A run transitions to `active` only when Eino actually begins processing; transitions to `queued` when another run holds the session lock.
- Each run has exactly **one terminal event** (`run.completed`, `run.failed`, or `run.cancelled`); duplicates are rejected by the event store.
- Cancellation produces a `run.cancelled` terminal event regardless of whether the provider was mid-stream, the tool was running, or no work had begun.
- Restarting the process surfaces any non-terminal runs and either resumes or marks them `failed` with a clear error.

### FR-5 — Streaming Events

The system shall emit structured `RunEvent`s during model streaming and tool execution.

Acceptance criteria:

- Every event carries `run_id`, monotonic `seq`, `created_at`, `type`, and a JSON payload version.
- The minimum event vocabulary is:
  - `run.started`
  - `model.delta`
  - `model.completed`
  - `tool.requested`
  - `tool.approval_required`
  - `tool.started`
  - `tool.finished`
  - `run.completed`
  - `run.failed`
  - `run.cancelled`
- The UI consumes events from the public HTTP/SSE endpoint; the event source never depends on Eino's internal types.
- Event ordering is consistent across reconnects: clients can request events with `after_seq=N` and receive exactly the events that follow.

### FR-6 — Controlled Tools

The system shall register at least one read-only tool that runs automatically and at least one effectful tool that requires explicit approval.

Acceptance criteria:

- Tool registration is performed through a `ToolSpec` registry owned by Vivy, not by Eino's internal package.
- Read-only tools execute without UI approval but still emit `tool.started` / `tool.finished` events.
- Effectful tools must emit `tool.approval_required` and shall not execute until an explicit `Approval` record exists for the tool call.
- A denied approval produces a `tool.finished` event with status `denied` and the run may continue with that result fed back to the model.
- Tool calls and tool results are persisted as part of the run record.

### FR-7 — Approval Flow

The system shall expose an approval endpoint and persist approvals.

Acceptance criteria:

- Approvals are bound to a specific `run_id` and `tool_call_id`; reusing an approval for a different call is rejected.
- Approvals have an expiration policy; an expired approval cannot authorize a tool.
- The UI can list pending approvals and act on them without holding open a streaming connection.
- Permissions are enforced on the server; UI-only checks are not authoritative.

### FR-8 — Persistence and Recovery

The system shall persist sessions, messages, runs, events, and approvals to SQLite with explicit migrations.

Acceptance criteria:

- Schema migrations are versioned and applied on startup; a migration failure aborts startup with a clear error.
- On restart, the process enumerates runs in `accepted` / `queued` / `active` and either resumes or marks them `failed`.
- No terminal event is ever written twice for the same `run_id`.
- Crash mid-run results in either a recoverable state (run resumes) or a definitive `run.failed` with reason.

### FR-9 — UI Shell

The system shall provide a new browser-based UI shell.

Acceptance criteria:

- The UI calls only the Vivy HTTP API; it does not import Go types, Eino types, or reference-project types.
- The UI supports session list, message input, streaming display, approval interaction, run status display, error display, and refresh-without-state-loss.
- The UI is exercised end-to-end against a real Go process during smoke tests, not against mocks.
- The UI does not need to match Diva's visual design or Tauri command set.

### FR-10 — Configuration and Secrets

The system shall load configuration from a typed config store and treat secrets as a separate boundary.

Acceptance criteria:

- Configuration is loaded at startup and validated; invalid config aborts startup.
- Secrets (provider keys, etc.) are read from environment or OS keystore at request time, never persisted.
- The config store may be queried for non-secret values (model id, base URL, tool enablement), but never for secrets.

### FR-11 — Observability and Errors

The system shall expose structured logs and visible errors.

Acceptance criteria:

- Provider errors are surfaced as `run.failed` events with a structured error payload, not as silent failures.
- The UI displays the error to the user with a clear cause category (provider error, tool error, internal error, cancellation).
- Logs include `run_id`, `session_id`, and event `seq` for traceability.
- Stack traces are logged at debug level; user-visible errors do not leak internals.

## 8. Non-Functional Requirements

- **Build and test cycle:** incremental Go builds and unit tests complete in seconds on the development machine.
- **Single binary:** the runtime ships as one Go binary plus UI assets; no separate backend process in normal operation.
- **Deterministic test mode:** the mock provider produces reproducible runs so unit tests can assert exact event sequences.
- **Bounded shutdown:** SIGINT / SIGTERM / Windows console close produces graceful shutdown, flushing events and closing SQLite cleanly.
- **Resource limits:** stream buffers, event payloads, and SQLite are bounded; large payloads are truncated or rejected, not unbounded.

## 9. Acceptance Scenarios

The V0 slice is accepted when **all** of the following pass on a real development machine:

1. **AS-1:** A new session is created, a message is sent, the model streams a response, the final answer is persisted, and reloading the session shows the same conversation.
2. **AS-2:** A read-only tool is invoked during a run; it executes without UI approval and emits `tool.started` / `tool.finished`.
3. **AS-3:** An effectful tool is invoked; `tool.approval_required` is emitted; the UI denies; the run continues with the denial fed back to the model.
4. **AS-4:** An effectful tool is invoked; the UI approves; the tool runs and its result feeds back to the model; the run completes.
5. **AS-5:** Cancellation during streaming produces exactly one `run.cancelled` terminal event; no `run.completed` is emitted afterward.
6. **AS-6:** Killing the process mid-run and restarting it surfaces the run as either resumed-and-completed or as `run.failed` with a clear reason; no duplicate terminal event.
7. **AS-7:** Refreshing the UI does not lose visible run state; reconnecting to the event stream resumes from the last seen `seq` without gaps or duplicates.
8. **AS-8:** Mock-provider tests assert exact event sequences for each scenario above.
9. **AS-9:** No secret value appears in the SQLite file, in any persisted event payload, or in the UI's local storage.

## 10. Architecture (High-Level, Subject to Redesign)

> Note: this section captures the **V0 implementation strategy** from `AGENT-VIVY-ASSEMBLY-OPTIONS.md`. It is not a final architecture. AGENT-VIVY is intended to grow toward a better architecture as evidence accumulates; V0 is intentionally a thin shell.

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
  fixtures/                  provider, event, recovery fixtures
  ui/                        new UI, kept independent of runtime internals
```

Reference-project usage in V0:

- **Eino:** execution framework — ChatModel, Tool, ChatModelAgent, stream, callbacks, optional interrupt/resume.
- **Crush (selective):** application-assembly patterns — config store, SQLite discipline, pub/sub broker, tool hook pattern, permission flow. No wholesale copy.
- **Diva:** not a code source. Only capability evidence, after a separate capability proposal is written and approved.

## 11. Milestones

1. **M0 — Spike (Eino viability):**
   - Create Go module.
   - Mock provider + deterministic fixture.
   - Eino `ChatModelAgent` behind a Vivy adapter.
   - One HTTP endpoint that returns a streamed response.
   - Acceptance: spike passes; Eino stream and tool call can be wrapped cleanly.

2. **M1 — Vertical Slice (FR-1, FR-2, FR-3, FR-4, FR-5 minimum):**
   - Sessions, messages, runs, events, one provider, mock provider, SQLite, one read-only tool.
   - Minimal UI shell that consumes events.
   - Acceptance: AS-1 and AS-8 (mock path) pass.

3. **M2 — Tools and Approval (FR-6, FR-7):**
   - Effectful tool with explicit approval.
   - Approval persistence and expiration.
   - UI approval UI.
   - Acceptance: AS-3, AS-4 pass.

4. **M3 — Recovery and Cancellation (extends FR-4, FR-8, FR-11):**
   - Restart recovery: detect non-terminal runs, resume or fail them definitively.
   - Cancellation semantics: exactly one terminal event.
   - Acceptance: AS-5, AS-6, AS-9 pass.

5. **M4 — Polish and Smoke (FR-9, FR-11):**
   - Real provider smoke on Windows.
   - UI smoke against the real Go process.
   - Structured error display.
   - Acceptance: all AS-1 through AS-9 pass against the real provider.

## 12. Risks

- **Eino lifecycle mismatch:** Eino's lifecycle and checkpoint semantics may not match the V0 Run state machine. Mitigation: keep Eino behind an adapter; do not let Eino types leak into Vivy's domain.
- **Public type coupling:** Eino or reference-project types may leak into the UI contract. Mitigation: adapters + JSON contract + lint rule that flags non-Vivy types in HTTP handlers.
- **Provider breadth creep:** early success may tempt adding more providers. Mitigation: only one provider + one mock in V0; new providers require a new capability proposal.
- **Diva feature creep:** pressure to "also bring Diva feature X". Mitigation: every Diva capability enters through a separate proposal document.
- **UI drift:** UI may diverge from the backend contract. Mitigation: real-process smoke tests against every acceptance scenario; UI mock forbidden.
- **Recovery bugs:** restart recovery is the hardest correctness surface. Mitigation: explicit recovery tests, deterministic mock provider, and exact-one-terminal-event invariant.
- **Permission bypass via UI:** approvals checked only in UI. Mitigation: server-side enforcement; tests that deny from UI but assert server still rejects.

## 13. Decision Log (V0)

| ID | Decision | Rationale (verbatim where possible) |
|---|---|---|
| D-001 | AGENT-VIVY is a **new product line**, not a Diva clone, Diva translation, or SystemV continuation. | "The current positioning is a Diva clone, but a different product line, completely independent" — user direction 2026-08-06 |
| D-002 | V0 is intentionally narrow; full product capability comes through later capability proposals. | User direction: "First split Diva's capabilities into proposals, then reimplement them in the new project" |
| D-003 | V0 execution engine is **Eino + Vivy-owned application shell**. | From `AGENT-VIVY-ASSEMBLY-OPTIONS.md` §2 C; Eino covers ChatModel/Tool/stream/ReAct/HITL primitives without rebuilding |
| D-004 | Crush is a **selective reference**, not a wholesale transplant. | Coding-agent-specific subsystems (LSP, filetracker, Bubble Tea UI, MCP breadth) are out of V0 scope |
| D-005 | Diva is **not a code source** for V0. | Capability evidence only; capability re-entry requires a separate proposal |
| D-006 | V0 is a **single Go module**. | Do not imitate Diva's 17-crate workspace or SystemV's crate graph |
| D-007 | Eino and reference-project types **must not leak** into the UI contract. | Adapters + JSON schema boundary |
| D-008 | One terminal event per run is a hard invariant. | Recovery correctness depends on it |
| D-009 | Approval authority is **server-side only**. | UI-only checks are not authoritative |
| D-010 | Secrets are **never persisted**. | Provider keys read at request time, not stored |
| D-011 | V0 ships **one OpenAI-compatible provider + one mock provider**. | Provider breadth is deferred to capability proposals |
| D-012 | V0 ships **one read-only tool + one effectful approval-gated tool**. | Demonstrates both execution paths without scope creep |
| D-013 | V0 UI is a **new browser-based shell**, not a Tauri command port. | Avoid recreating Diva's 134-command surface |
| D-014 | AGENT-VIVY inherits Diva's **personal-gateway product philosophy** (single-user, local-first, application-not-service). | "Diva is a pure gateway individual application, so agent-vivy must retain this philosophy too" — user direction 2026-08-06 |
| D-015 | The gateway philosophy is **a product constraint, not an implementation constraint**. | Implementation is rebuilt from scratch under V0 architecture; the philosophy governs **what the product is**, not how the code is organized |
| D-016 | Any future capability that pushes AGENT-VIVY toward a multi-tenant SaaS or hosted control plane is out of scope unless a future capability proposal explicitly revisits the philosophy. | Preserves the gateway boundary across capability re-entries |
| D-017 | **Logs are a first-class product feature.** Persisted events, structured errors, and run/event correlation are user-visible and non-optional. | "Logs are first-class citizens for tracing problems" — user direction 2026-08-06; original Diva design preserved as philosophy |
| D-018 | V0 provider set is **exactly two: OpenAI-compatible + Anthropic**, delivered as pre-prepared YAML bundles. | "Only do OpenAI and Anthropic providers, with provider YAML pre-prepared" — user direction 2026-08-06; original Diva design preserved as philosophy |
| D-019 | Adding a new provider is a **product decision** through a capability proposal; not a runtime plugin mechanism. | Reinforces D-018 against "anything-goes plugin sprawl" |
| D-020 | AGENT-VIVY favors **large-modular decomposition**: one capability = one well-bounded module/crate/package. | "Large modular decomposition (one module is one crate)" — user direction 2026-08-06; long-term directional principle, not a V0 split-V0-into-many-modules license (D-006 still binds V0) |
| D-021 | The four philosophical anchors (gateway, logs, curated providers, large-modular) are **product constraints**, not implementation constraints. Diva's package graph, Tauri commands, schema, runtime, and code style are not inherited. | Preserves the "philosophy yes, implementation no" boundary established in v0.2 §5.0.5 |
| D-022 | V0 provider YAML bundles are **adapted from Diva's `providers.yaml` schema**, not invented from scratch. Source: `agent-diva/agent-diva-providers/src/providers.yaml` (verified: 1113 lines, 17 active provider entries plus commented-out `azure` / `copilot`). | "Copy one directly from Diva's YAML" — user direction 2026-08-06; copying only the **schema and the two V0 entries** (`openai` and `anthropic`), not Diva's other 15 provider entries |
| D-023 | Only `openai` (OpenAI-compatible) and `anthropic` (Anthropic) entries from Diva's `providers.yaml` are carried into V0. The other Diva entries (`openrouter`, `aihubmix`, `custom`, `deepseek`, `gemini`, `zhipu`, `dashscope`, `moonshot`, `minimax`, `vllm`, `groq`, `xai`, `cherryin`, `302ai`, `ph8`, `burncloud`, `silicon`, `ppio`, `together`, `ocoolai`, `github`, `azure`, `copilot`) are **NOT** carried into V0. Each entry that later wants to enter Vivy must do so via a capability proposal (D-019). | Locks V0 provider set to D-018 and prevents silent copy of Diva's gateway breadth |
| D-024 | The provider YAML schema fields that V0 copies from Diva are: `name`, `api_type`, `keywords`, `env_key`, `display_name`, `default_model`, `gateway_prefix`, `skip_prefixes`, `env_extras`, `is_gateway`, `is_local`, `detect_by_key_prefix`, `detect_by_base_keyword`, `default_api_base`, `strip_model_prefix`, `supports_prompt_caching`, `models`, `model_overrides`. Vivy does not copy Diva's Rust `agent-diva-providers` source code; only the YAML schema and the two selected entries. | Separates "schema reuse" (allowed) from "code reuse" (not allowed under D-005, D-021) |
| D-025 | Adapted YAML entries are **re-derived from Diva's source values but rewritten into Vivy's owned schema**. Vivy does not commit a verbatim copy of Diva's YAML text; provenance is recorded in a `provenance` field per entry citing the Diva source path and the entry name. | Preserves the philosophy boundary while honoring the user's "copy schema" intent |
| D-026 | The product's durable source of truth is defined by a **small set of Vivy-owned storage contracts** (`Journal`, `SnapshotStore`, `BlobStore`, `LeaseStore`), not by a specific database engine. SQLite is the V0 reference backend but is not an architectural invariant. | "AGENT-VIVY Storage Architecture Addendum" §1, §3; reinforces philosophy of "no implicit disappearance" by separating semantic contract from implementation choice |
| D-027 | Vivy's domain code must not depend on SQLite-specific surfaces (SQL transactions as exposed type, partial indexes, FK cascades, `AUTOINCREMENT`, `sqlc` structs, SQLite FTS, WAL-specific behavior). Those remain adapter details. | Addendum §5; protects future backend portability |
| D-028 | Eino's `CheckPointStore` interface is adapted via a two-layer bridge: `EinoCheckpointAdapter` (implements Eino Get/Set/Delete) → `VersionedCheckpointStore` (Vivy-owned: atomic replacement, internal generations, checksum, metadata, listing, orphan detection, engine-version validation) → `BlobStore`. Checkpoint bytes never define product history; product events may reference only an already durable checkpoint. | Addendum §6.1–§6.3; preserves the "logs as first-class citizens" anchor by keeping engine state separate from product state |
| D-029 | The required write ordering for approval / resumable-cancellation paths is: (1) Vivy allocates checkpoint ID, (2) Eino Set + adapter durability, (3) Eino emits interrupt, (4) adapter verifies checksum and generation, (5) Vivy appends one journal commit containing `run.suspended` and `approval.requested`, (6) UI sees approval only after commit is durable. Reverse order is a safety violation. | Addendum §6.4; closes the "UI sees approval before checkpoint durable" crash window |
| D-030 | Same-ID Eino checkpoint overwrite must never be in-place. V0 SQLite backend implements generation via a separate generations table + atomic pointer row; V1+ fsjournal backend implements it via `checkpoints/<id>/000000000N.bin` + manifest atomic rename. Both backends must produce the same observable behavior. | Addendum §6.6; protects resume correctness across backend switches |
| D-031 | V0 ships **one storage backend** (SQLite). The QwenPaw-Creator-style filesystem journal backend is a **V1+ probe**, not a V0 deliverable. The probe scope is the V0 vertical slice only (session / message / run / approval / checkpoint / terminalize / restart-replay) and must not extend to memory / RAG / MCP / multi-agent / channels in the same artifact. | Addendum §4, §8; aligns with D-006 (single Go module) and §11 milestones |
| D-032 | Every storage backend must pass a **backend conformance suite** before being trusted by domain code. V0 ships the suite at minimum 16 cases covering atomic append, monotonic sequence, expected-version conflict, idempotent replay, idempotency payload mismatch, exactly-one-terminal, first-writer-wins approval, restart repair, torn final write, malformed complete data, orphan checkpoint recovery, cancellation/approval recovery after kill, secret redaction, Windows process-lock, monotonic-replay under concurrent writers, and replay-after-disconnect. The full 18-case suite is the V1+ target. | Addendum §9; gives V0 a falsifiable storage contract |
| D-033 | QwenPaw is **not yet verified** as a reference project. Before any fsjournal implementation work begins, (a) QwenPaw repository URL must be confirmed, (b) QwenPaw LICENSE must be inspected and recorded, (c) whether we are authorized to borrow its design patterns must be determined. Until then, QwenPaw-related claims are taken on faith from the addendum author. | Addendum §2; P9 verification protocol applied |
| D-033.r1 | QwenPaw license and source verified 2026-08-07: **Apache-2.0**; URL `github.com/agentscope-ai/QwenPaw`; not vendored locally; addendum §2.1 (Scroll SQLite) and §2.2 (Creator Runtime fsjournal) claims **factually verified** against `src/qwenpaw/agents/context/scroll/manager.py` and `plugins/apps/qwenpaw-creator/backend/services/runtime_files/{atomic_store,jsonl_store,locking,session_store}.py`. **Authorized** to read, study, and independently reimplement the design patterns in Vivy's own Go code, under Apache-2.0 attribution conventions when borrowing patterns. | User-supplied URL; `git clone --depth 1` verification; file-level inspection |
| D-034 | Addendum §6 Eino claims (CheckPointStore interface shape, `Gob` payload type, interrupt/resume integration) **must be re-verified against `diva-go/.workspace/eino/`** before being treated as authoritative. QwenPaw is not an Eino project; its checkpoint system is shadow-Git-based and does not validate addendum §6. | Addendum §6 vs `/tmp/QwenPaw/src/qwenpaw/checkpoints/repository.py`; grep confirms zero `eino` import in QwenPaw |
| D-035 | AGENT-VIVY-ARCHITECTURE-V0.md and ADR-002/003/005/006/007 referenced by the addendum are **not on disk**. The addendum cannot be formally adopted as a superseding artifact until (a) those ADRs are located/authored and (b) the addendum is reconciled against them. Until then, the addendum's substantive content is captured by D-026..D-033 in the PRD decision log, and the storage addendum is treated as **proposal-only**, not authoritative. | `find` in morediva returns zero hits for `AGENT-VIVY-ARCH*`, `*architecture*v0*`, `*ADR-002*` |

## 14. Open Questions (Awaiting User / Future Artifacts)

| ID | Question | Owner | Target artifact |
|---|---|---|---|
| OQ-1 | Capability inventory: which Diva capabilities are candidates for AGENT-VIVY proposals? | 📋 John | Capability inventory document (separate, not part of this PRD) |
| OQ-2 | First capability proposal to draft after V0 ships? | 📋 John + user | Capability proposal document |
| OQ-3 | Long-term architecture redesign beyond the thin V0 shell — when to start, on what evidence? | user + future architect session | Future ADR / direction document |
| OQ-4 | Provider breadth order (OpenAI, Anthropic, Gemini, local, etc.) | 📋 John + user | Provider expansion capability proposal |
| OQ-5 | Tooling breadth (file ops, shell, MCP, web fetch, etc.) | 📋 John + user | Tool expansion capability proposal |
| OQ-6 | Memory and context policy beyond simple context replay | 📋 John + user | Memory capability proposal |
| OQ-7 | Desktop UI / Tauri wrapper — is this in scope at all? | user | Desktop wrapper capability proposal |
| OQ-8 | Provider YAML bundle format and version policy: schema, model-id declaration, capability tags, default parameter ranges, known limitations, and how the bundle is shipped and updated. | 📋 John + user | Provider bundle spec (separate artifact) |
| OQ-9 | Large-modular long-term target: when V0's thin shell is replaced, what is the canonical "module" map (provider, runtime, session, memory, tools, events, ui, etc.) and which are crate-level vs. sub-package-level units? | user + future architect session | Future ADR / architecture direction |

## 15. Glossary

- **Run:** a single user request processed by the model, including any tool calls, from acceptance through terminal event.
- **RunEvent:** a structured event emitted during a run, with a `type`, `seq`, `payload`, and `payload_version`.
- **Approval:** a server-side authorization record bound to a specific `run_id` + `tool_call_id` with an expiration.
- **ProviderRef:** the Vivy-owned boundary that hides provider-specific implementation from call sites.
- **ToolSpec:** a Vivy-owned tool description registered in the tool registry.
- **Capability Proposal:** a separate document required to reintroduce any Diva (or other reference) capability into AGENT-VIVY; not part of this PRD.
- **V0:** the assembly slice covered by this PRD; the first credible stable Agent under the AGENT-VIVY product line.
- **Philosophical Anchor:** a product-level constraint inherited from Diva that governs what AGENT-VIVY **is** or how it must **behave**. Anchors are not implementation inheritance. Anchors in V0 are: personal-gateway (§5.0.1), logs as first-class (§5.0.2), curated provider catalog (§5.0.3), large-modular decomposition (§5.0.4).
- **Provider Bundle:** a pre-prepared YAML configuration document describing one provider's model IDs, capabilities, defaults, and known limitations. V0 ships exactly two bundles: OpenAI-compatible and Anthropic.
- **Module (Plate / Module):** a top-level capability unit in AGENT-VIVY's long-term decomposition. One module = one user-facing concept or product subsystem. In V0 the principle is recorded as a directional preference (D-020); in a future architecture (OQ-9) it becomes the canonical module map.

## 16. Revision Notes

- v0.1 (2026-08-06): Initial draft. Establishes V0 scope, fixes the "not a Diva clone" positioning per user direction, locks in Eino + thin-shell implementation strategy, and reserves capability re-entry to a separate capability-proposal process.
- v0.2 (2026-08-06): Added §5.0 Product Philosophy recording Diva's "personal gateway application" anchor and explicitly distinguishing it from implementation inheritance. Added D-014 / D-015 / D-016 to the decision log to lock the gateway boundary across all future capability proposals.
- v0.3 (2026-08-06): Expanded §5.0 into six sub-anchors covering (1) personal gateway, (2) logs as first-class citizens, (3) curated pre-baked provider catalog, (4) large-modular decomposition, (5) philosophy-vs-implementation boundary, (6) explicit exclusions. V0 provider set locked to OpenAI-compatible + Anthropic (D-018). Added D-017 / D-018 / D-019 / D-020 / D-021. Added OQ-8 (provider YAML bundle spec) and OQ-9 (module map for future architecture).
- v0.4 (2026-08-06): Locked Diva `providers.yaml` as the **schema source** for V0 provider bundles. Verified source: `agent-diva/agent-diva-providers/src/providers.yaml` (1113 lines, 17 active provider entries + 2 commented). Added D-022 (schema adapted from Diva, not invented), D-023 (only `openai` + `anthropic` entries carried into V0; other 15 Diva entries explicitly excluded), D-024 (schema field list reused; Diva Rust source code not copied), D-025 (provenance field per entry). Updated §5.0.3 to reference the adapted-schema provenance rule. OQ-8 narrowed to schema-derivation details and version policy once D-022 lands.
- v0.5 (2026-08-06): Reviewed `AGENT-VIVY-STORAGE-ARCHITECTURE-ADDENDUM.md`. Accepted the storage-contract framework (Journal / Snapshot / Blob / Lease) and the Eino two-layer checkpoint bridge; deferred the QwenPaw filesystem journal backend to V1+ (D-031). Added D-026..D-033. Key protections: domain code may not depend on SQLite-specific surfaces (D-027); checkpoint bytes never define product history (D-028); approval write ordering has a strict 6-step invariant (D-029); same-ID checkpoint overwrite is forbidden in place (D-030); conformance suite is required before any backend is trusted by domain code (D-032); QwenPaw's license and authorization must be verified before any fsjournal implementation (D-033). The addendum references ADR-002/003/005/006/007 in a `AGENT-VIVY-ARCHITECTURE-V0.md` that is not on disk; those ADRs must be located or written before the addendum can be formally adopted.
