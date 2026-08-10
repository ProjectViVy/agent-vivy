# AGENT-VIVY-ARCHITECTURE-V0

> ADR baseline for the V0 assembly slice (P0-1 / SR-1 / D-035). Records the
> shape that was built and verified across M0..M4; supersedes the
> implementation plan's stand-in architecture description. Each ADR is one
> screen or less and traces to PRD decisions (`prd-agent-vivy-v0.md`
> D-001..D-035) or a philosophical anchor. ADR-009 records the first V1
> capability decision (MA-1, session-memory feed).

## Preamble — the four philosophical anchors (PRD §5.0, quoted)

1. **Personal gateway application (§5.0.1).** "AGENT-VIVY is a personal
   gateway application. Not a multi-tenant platform, not a hosted service,
   not a SaaS." Single-user, local-first; the application is the product;
   the provider is a dependency, not the product; personal control over
   data and approvals.
2. **Logs as first-class citizens (§5.0.2).** "The user must be able to
   trace any past run, any past error, and any past tool call from end to
   end through persisted log and event records." Errors carry structured
   cause categories; "I can't see what happened" is a product bug.
3. **Providers are a curated, pre-baked catalog (§5.0.3).** "Provider
   integration is opinionated and pre-prepared, not anything-goes plugin
   sprawl." V0 ships exactly two pre-baked bundles; new providers enter by
   capability proposal; secrets are never persisted.
4. **Large-modular decomposition (§5.0.4).** "One capability = one
   well-bounded unit." Modules have explicit narrow public surfaces;
   cross-module coupling is intentional and visible.

The anchors are product constraints, not implementation constraints
(§5.0.5, D-021): AGENT-VIVY does not inherit Diva's package graph, Tauri
commands, schema, runtime, or code style.

## ADR-001 — Stack: single Go module, Eino as the only engine door

**Decision.** V0 is one Go module (`agent-vivy`) per D-006. The Eino
framework is consumed exclusively as online module dependencies
(`github.com/cloudwego/eino` v0.9.13, `eino-ext/components/model/openai`);
vendored copies and `replace` directives are prohibited. All Eino types are
quarantined inside `internal/runtime` and `internal/provider` (D-007),
enforced at build time by an AST-level import-lint test. SQLite access uses
`modernc.org/sqlite` (pure Go, no CGO) so the single binary builds and runs
on Windows without a toolchain. The UI is Vite + native TypeScript embedded
via `go:embed`, so one executable is the product.

**Guard tests.** import-lint gate (D-007); secret-boundary audit (E3).

## ADR-002 — Storage: Vivy-owned contracts over a replaceable backend

**Decision.** The durable source of truth is a small set of Vivy-owned
contracts — `Journal`, `SnapshotStore`, `BlobStore`, `LeaseStore`, plus the
session/message/run/approval/notes stores — not a database engine (D-026).
SQLite is the V0 reference backend (D-031); domain code never depends on
SQLite-specific surfaces (D-027). Journal appends are atomic with monotonic
`seq` per run and an exactly-one-terminal guard (D-008); blob writes are
generation-based with an atomic pointer flip so a same-ID overwrite is never
in place (D-030). The backend is trusted only after passing the 16-case
conformance suite (D-032, `CN-01..CN-16`); the filesystem-journal backend
remains a V1+ probe.

**Reconciliation.** This ADR realizes addendum §1–§3, §5 as implemented
code; the addendum stays proposal-only elsewhere per D-035 until superseded.

## ADR-003 — Run lifecycle: six-state machine, one terminal event

**Decision.** A run moves through `pending → running → suspended →
completed | failed | cancelled` with transition validation in
`internal/domain`; terminal states are locked (D-008 pillar). Exactly one
terminal event is appended per run, persisted on a detached context so user
cancellation cannot strand a run, and the journal guard makes double
terminals impossible. Cancellation is formalized per phase (AS-5): mid-tool,
pre-start, and concurrent idempotency; append races with cancel route
through the classified terminal.

## ADR-004 — Events: versioned vocabulary as the single contract

**Decision.** Ten `RunEvent` types form the locked vocabulary
(`schemas/events/run-event.schema.json`); payload schemas per type carry
structured cause categories (`provider_error | tool_error | internal_error |
cancelled`). The same events serve journal persistence, SSE fan-out, and UI
re-rendering (FR-5): the UI rebuilds state from `after_seq` replay, never
from in-memory state. Events carry `session_id` / `run_id` / `seq`
correlation identity per the logs-as-first-class anchor (§5.0.2).

## ADR-005 — Provider: two pre-baked bundles, env-only secrets

**Decision.** Exactly two providers ship as pre-baked YAML bundles adapted
from Diva's `providers.yaml` schema with provenance tags — `openai`
(OpenAI-compatible, wired over `eino-ext`) and `anthropic` (unwired stub in
V0) — per D-018/D-022..D-025. Keys are read from the environment named by
`env_key` at model-construction time inside `internal/provider` only
(D-010); `KeyMissingError` names the variable but never carries a value;
`VIVY_API_BASE` overrides the bundle default base for gateway smoke. A
deterministic mock provider serves tests and offline development and is
reachable only through the same `ProviderRef` seam — never in the
production path (PRD §6.2).

## ADR-006 — Tools & approval: readonly auto-execute, effectful gated

**Decision.** Tools are Vivy-owned `ToolSpec` values resolved by a registry
(unknown name = startup error, consuming `tools.enabled`). Read-only tools
auto-execute; effectful tools interrupt the run (D-012). Suspension uses the
two-layer checkpoint bridge (D-028): `EinoCheckpointAdapter` →
`VersionedCheckpointStore` (engine-version + checksum fail-closed) →
`BlobStore`. Approval write ordering follows the six-step invariant
(D-029); decisions are first-writer-wins with server-side 404/409
(D-009). Checkpoint bytes never define product history: the journal is the
record, the checkpoint only resumes it.

## ADR-007 — UI: thin browser shell over the HTTP/SSE contract

**Decision.** The UI is a zero-runtime-dependency Vite + TypeScript shell
served by the Go binary (`go:embed` + SPA fallback). It consumes only the
JSON API (`{"error":{"code","message"}}` envelope, FR-11) and the SSE
stream with reconnect cursor; all state is rebuildable from the API after
refresh (no hidden client state, no web storage). No Eino or engine type
crosses into the UI contract (D-007, D-013). Smoke is automated with
Playwright against the real Go process.

## ADR-008 — Recovery: restart settles every run to a definitive state

**Decision.** On startup, before the server listens, `Service.Recover`
settles every non-terminal run (FR-8, AS-6): a run suspended on a
still-valid approval with a readable versioned checkpoint rebuilds its
in-memory pending state and resumes through the ordinary decision path;
every other run closes with a definitive `run.failed` carrying the
restart-recovery cause. Recovery is idempotent through the
exactly-one-terminal guard. `/healthz` reports the recovery stage.

## ADR-009 — History rebuild feed (V1 MA-1 decision)

**Decision.** Every run enters the engine via `adk.Runner.Run` with the
session's message history rebuilt from the journal (`user` →
`schema.UserMessage`, `assistant` → `schema.AssistantMessage`, in `seq`
order) plus the new user message. Verified premise: eino v0.9.13
`Runner.Query` starts a fresh execution carrying only the single new
message; the ADK keeps no cross-Query memory, so without this feed every
turn was stateless while the journal already held the full transcript.

**Scope trade-off (minimal tier).** Cross-turn context carries only
user/assistant text pairs; per-turn tool-call/tool-result exchanges are not
replayed into later turns. This keeps the feed bounded and the journal the
sole history source; context compaction, summaries, and RAG remain deferred
(IMPLEMENTATION-PLAN §10) and any change to this rule requires a further
capability proposal (RK-2/RK-4).

## Status

- v0 (2026-08-09): baseline recorded after V0 closure (M0..M4, AS-1..AS-9
  verified on mock and real provider paths). Closes SR-1 / D-035.
</file_content>
