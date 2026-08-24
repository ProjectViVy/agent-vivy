# AGENT-VIVY Direction

> Status: recorded direction, not a final architecture specification
> Updated: 2026-08-16 (worldview pointer)
> Scope: product positioning and staged evolution of AGENT-VIVY
>
> Namesake rhyme: `agent-vivy/docs/architecture/VIVY-WORLDVIEW.md`

## 1. Decision Summary

AGENT-VIVY is a new Agent product line, not a Go rewrite of `agent-diva` and not a direct continuation of the current `SystemV` code.

The first version of AGENT-VIVY is intended to be a rapidly assembled, stable, usable Agent built from selected mature reference-project components and newly defined product boundaries.

`agent-diva` remains an independently maintained stable product line. It continues to receive development and maintenance work and is not replaced by AGENT-VIVY.

The current `SystemV` repository is a partially abandoned Rust direction and research container. Future system-level infrastructure is expected to be Rust-based, but the existing SystemV code is not a mandatory future implementation base. Future Rust/SystemV work may be redesigned or rebuilt from later evidence.

## 2. Project Roles

| Project | Role | Constraint |
| --- | --- | --- |
| `agent-diva` | Existing stable product line | Continue maintenance and feature development; do not transfer its architecture wholesale |
| `agent-vivy` | New Agent product and validation platform | Go first; new core and potentially new UI; no forced V0 compatibility, while useful Diva capabilities may be fully reimplemented through proposals |
| `SystemV` | Future Rust/system-level direction and research reference | Current code is non-authoritative and may be replaced |

The relationship is product-capability evidence flow, not source-code or architecture inheritance:

```text
agent-diva
  -> capability inventory, product experience, behavior examples, failure evidence
  -> capability proposal and redesign
  -> agent-vivy
  -> validated concepts and contracts
  -> future Rust/SystemV redesign when justified
```

A Diva capability is not removed merely because it is outside V0. Before any capability returns to Vivy, it must be expressed as a new product proposal with current user value, acceptance criteria, security and persistence implications, UI workflow, and explicit boundaries. Vivy may eventually reimplement the complete useful product capability set, but it must not preserve Diva's internal package graph, Tauri command contracts, data schema, or legacy architecture as the implementation baseline.

## 3. Staged Missions

AGENT-VIVY may have different missions at different stages. A stage must have one primary mission; it must not simultaneously promise stable product parity, complete framework replacement, and AGI-OS implementation.

### V0: Assemble

Build a stable and usable new Agent quickly by combining mature components, a small amount of new code, and explicit adapters.

Focus:

- reliable startup and configuration;
- session and message lifecycle;
- provider calls and streaming;
- controlled tool execution;
- persistence and restart recovery;
- visible errors and events;
- a workable new UI or UI shell.

### V1: Operate

Use AGENT-VIVY in real workflows to validate the product shape and identify which Agent Core concepts are genuinely useful.

Focus:

- stability;
- interaction quality;
- tools and approvals;
- session/context behavior;
- memory behavior;
- observability and recovery.

### V2: Explore

Use Go prototypes or probes to test deeper Agent Core and future SystemV ideas before committing to a Rust implementation.

Possible topics:

- state and event semantics;
- Memory/Context boundaries;
- long-running execution;
- tool permissions and human approval;
- multi-agent coordination;
- workflow and resumable execution;
- system-level abstractions relevant to a future AGI-OS.

A Go probe produces evidence. It does not automatically become a Vivy product API or a Rust/SystemV crate design.

The post-V1 direction is two applications: daily `vivy.exe` (Operate)
and independent **Vivy Studio** (develop + distribute; also the host
for V2 Explore). Canonical Studio text:
`agent-vivy/docs/architecture/VIVY-STUDIO.md` (corrected 2026-08-15).
Narrative: `SELF-EVOLVING-GATEWAY.md`. Decision ids:
`VIVY-GATEWAY-AND-STUDIO.md` (NG-1..NG-28). Studio bootstrap ST-6
proved the first-party daily IDE path. Other authorized developer tools may
work directly in the repository with their own native capabilities. It must not
collapse V1 Operate, V2 Explore, and V3 Rebuild into one sprint. V0
ADRs remain in force. Studio is not opened from the gateway.

### V3: Rebuild

If later evidence justifies it, implement a new Rust/system-level kernel. The target may use selected concepts from SystemV and Vivy, but it is not required to preserve either codebase's internal structure.

## 4. First-Version Positioning

The first version is internally experimental but externally stable:

- internal implementation may be assembled, replaced, and simplified;
- user-facing workflows must be reliable and testable;
- no mock-only production path;
- no hidden compatibility promise to old Diva behavior;
- no requirement to run on SystemV;
- no requirement to preserve the current SystemV crate graph.

The first version should validate one complete vertical workflow:

```text
Create session
  -> send message
  -> build context
  -> call provider
  -> stream model events
  -> optionally call a controlled tool
  -> obtain approval when required
  -> persist tool result and final response
  -> cancel or recover when needed
  -> restart and resume the session
```

## 5. Reuse Policy

Reuse is encouraged for speed, but reuse occurs at the module/pattern level rather than by importing an entire project's architecture.

Potential sources:

- Go reference projects such as Eino and Crush for provider, tool, configuration, session, SQLite, event, permission, and application-wiring patterns;
- `agent-diva` for product behavior, UI assets, provider compatibility details, test fixtures, and known failure cases;
- `SystemV` for future-oriented domain concepts and analysis documents;
- other local reference projects only for implementation-congruent concerns.

Every reused component must be evaluated for:

- license and attribution requirements;
- hidden global state and lifecycle assumptions;
- dependency weight;
- compatibility with Vivy's boundaries;
- testability and replacement cost.

Do not copy the whole Diva crate graph, the whole SystemV crate graph, or the full agno feature/configuration surface.

## 6. Boundary Principles

The first Vivy implementation may be a simple Go monolith, but it must establish a few explicit replaceable boundaries:

- `Session`
- `Message`
- `Run`
- `RunEvent`
- `Provider`
- `Tool`
- `ToolCall`
- `Approval`
- `Memory`

The UI must use an application protocol and must not access domain stores or runtime objects directly. Runtime code must not depend on UI details. Side effects must pass through explicit tool/provider/memory boundaries.

These boundaries exist to keep the product governable and replaceable, not to guarantee mechanical Go-to-Rust translation.

## 7. Explicit Non-Goals for V0

The following are not V0 requirements:

- 134-command Diva GUI parity;
- one-to-one migration of the old Tauri command surface;
- direct reuse of the archived `agent-diva-deep-governance` runtime;
- complete multi-channel parity;
- complete desktop pet, VRM, or voice parity;
- full AutoDream, Evolution, Knowledge, Team, or Workflow systems;
- complete sandbox replacement;
- SystemV integration;
- AGI-OS or systemV implementation;
- agno full-framework compatibility;
- old-data migration before the Vivy schema stabilizes.

The archived deep-governance branch remains failure evidence and architecture research, not a product baseline.

## 8. Decision Classification

| Capability or direction | V0 decision |
| --- | --- |
| Agent run loop | Keep, redesign |
| Provider abstraction | Keep, start with one OpenAI-compatible path plus a mock provider |
| Streaming events | Keep, make them a first-class contract |
| Sessions and persistence | Keep, start with explicit SQLite schema |
| Controlled tools | Keep, with explicit permissions and approvals |
| Memory | Keep as a simple auditable boundary; defer complex policy |
| Knowledge/RAG | Deferred |
| Team and complex Workflow | Deferred |
| Multi-channel adapters | Deferred |
| Full sandbox platform | Deferred; may temporarily use a narrow helper |
| New UI | Re-evaluate and design for Vivy; no old GUI parity obligation |
| Desktop pet/VRM/voice | Deferred |
| AutoDream/Evolution | Deferred |
| SystemV runtime integration | Deferred |
| Existing SystemV code | Reference only; not a required base |
| Diva 134-command parity | Drop |
| Deep-governance `/v1` as implementation baseline | Drop as baseline; keep as failure evidence |
| Old data migration | Deferred |

## 9. Revisit Triggers

Revisit the stage or architecture only when evidence exists, such as:

- a V0 vertical workflow passes real Windows use and restart/recovery tests;
- repeated real usage identifies a missing Agent Core abstraction;
- a Go probe demonstrates a SystemV-level concept with measurable product value;
- current Go implementation becomes a measured bottleneck rather than a speculative concern;
- a future Rust/SystemV implementation has a defined contract and real acceptance tests.

No architecture escalation should be justified solely by the claim that Go, Rust, or a particular framework is inherently faster.
