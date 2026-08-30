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
on Windows without a toolchain. The default UI is Vite + native TypeScript
embedded via `go:embed`, so one executable remains the product. An opt-in
`vivy_headless` build omits those assets for a same-machine static-UI split;
both variants expose the same in-process JSON-RPC control plane.

**Guard tests.** import-lint gate (D-007); secret-boundary audit (E3).

## ADR-002 — Storage: Vivy-owned contracts over a replaceable backend

**Decision.** The durable source of truth is a small set of Vivy-owned
contracts — `Journal`, `SnapshotStore`, `BlobStore`, `LeaseStore`, plus the
session/message/run/approval/notes stores — not a database engine (D-026).
SQLite is the V0 reference backend (D-031); domain code never depends on
SQLite-specific surfaces (D-027). Journal appends are atomic with monotonic
`seq` per run and an exactly-one-terminal guard (D-008); blob writes are
generation-based with an atomic pointer flip so a same-ID overwrite is never
in place (D-030). The backend is trusted only after passing the 17-case
conformance suite (D-032, `CN-01..CN-17`); the filesystem-journal backend
remains a V1+ probe. Optional Postgres is ADR-020.

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

**Decision.** The versioned `RunEvent` vocabulary
(`schemas/events/run-event.schema.json`) and its payload schemas carry
structured cause categories (`provider_error | tool_error | internal_error |
cancelled`). The same events serve journal persistence, JSON-RPC
notifications, and UI re-rendering (FR-5): the UI rebuilds state from
`after_seq` replay, never from in-memory state. Events carry `session_id` /
`run_id` / `seq` correlation identity per the logs-as-first-class anchor
(§5.0.2), including durable child-run lifecycle events.

## ADR-005 — Provider: two pre-baked bundles, env-only secrets

**Decision.** Exactly two providers ship as pre-baked YAML bundles adapted
from Diva's `providers.yaml` schema with provenance tags — `openai`
(OpenAI-compatible, wired over `eino-ext`) and `anthropic` (unwired stub in
V0) — per D-018/D-022..D-025. Keys are read from the environment named by
`env_key` at model-construction time inside `internal/provider` only
(D-010); `KeyMissingError` names the variable but never carries a value;
`VIVY_API_BASE` overrides the bundle default base for gateway smoke. Tests use
isolated deterministic model doubles outside the provider catalog; no mock
provider is reachable from the product runtime (PRD §6.2).

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

## ADR-007 — UI: thin browser shell over the JSON-RPC contract

**Decision.** The UI is a zero-runtime-dependency Vite + TypeScript shell
served by the Go binary (`go:embed` + SPA fallback) by default, or served as a
standalone static directory by the opt-in headless backend build. It consumes
only the JSON-RPC control plane and the journal-backed run event stream with
reconnect cursor; all product state is rebuildable from the API after refresh. Only the
non-sensitive locale/theme preferences are persisted in browser storage; no
session, run, prompt, event, or provider data is stored there. No Eino or
engine type crosses into the UI contract (D-007, D-013). Smoke is automated
with Playwright against the real Go process.

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

## ADR-010 — Model-visible ≡ logged (upgrades ADR-009)

**Decision.** Everything that reaches a model request must be reconstructable
from durable product records. Cross-turn context now carries the paired
tool-call / tool-result exchange in addition to user/assistant text. Each
model invocation appends a `model.request` journal event whose message
digests (role, optional tool name, content SHA-256, byte length) match the
exact `[]*schema.Message` handed to `Runner.Run`.

The Message store is a projection of that trajectory (user rows, assistant
rows with tool-call metadata, tool rows). The Journal remains the product
source of truth. Compaction, summaries, and a journal-only rebuild (which
would need a user-message event) stay out of this ADR.

**Guard tests.** Context unit tests keep paired tool turns; a service test
proves the second run of a tool session is fed the prior call/result; a
digest test proves `model.request` matches the captured model input.

## ADR-011 — Studio object plane (S2)

**Decision.** Generation, EvalRun, and Promotion live as sibling SQLite
tables, not as `RunEvent` types. A separate append-only `studio_events`
log records `generation.created`, `evalrun.recorded`, and
`promotion.accepted`. Promote is human-gated, requires at least one
EvalRun on the candidate, and is first-writer-wins on the source
generation (`from_id`). This slice only writes rows; it does not spawn a
candidate process or change the next-launch binary.

**Guard tests.** Empty list; create then list; promote without eval
fails; a second promote from the same `from` conflicts.

**Superseded as product home.** ADR-018: these tables are the wrong
process. Keep the code frozen; Studio owns the ledger.

## ADR-012 — Live species inspect (S3)

**Decision.** `species/inspect` is a read-only JSON-RPC method. It reports
build identity, the latest accepted next-launch promotion (or `builtin`),
the assembly recipe, default policy profile/hash, and tool names with
readonly flags. It never returns secrets, session bodies, or host paths.
This slice does not hash the running EXE and does not write Journal.

**Guard tests.** Empty studio store → `generation_id=builtin` and a
non-empty tool list; after promote → `generation_id` equals `to`;
payload contains no `api_key` and no drive-letter paths.

## ADR-013 — Air-gapped eval (S4)

**Decision.** `evals/start` launches a candidate species process with its
own data directory, listen address, and stripped environment. This slice
may reuse the live EXE bytes (config-only candidate). The only suite is
`airgap.probe`: healthz then kill. Success is `mixed` (no fitness claim);
a dead candidate is `failed_to_run`. The launcher refuses a layout that
would open the production SQLite path. Eval journals stay in the
candidate directory. `evals/record` remains the manual write.

**Guard tests.** Production session/journal rows unchanged after start;
candidate SQLite exists on a successful probe; feeding the production
path is rejected; a missing binary records `failed_to_run`.

## ADR-014 — SDK verify and plugin window (S5)

**Decision.** `vivy-sdk verify` checks one `plugins/<name>/` source
package against `sdk/plugin`. It reads the manifest and AST, optionally
`go list`s the package, and never writes an executable. Authors may
import only `agent-vivy/sdk/plugin`. Pack, generated registers, and live
loading stay out of this slice. The packer is not a `vivy.exe` subcommand.

**Guard tests.** `plugins/hello-fs` verifies clean; testdata cases fail
for internal imports, Eino, forbidden seams, name mismatch, and
`package main`.

## ADR-015 — SDK pack into a new generation (S6)

**Decision.** `vivy-sdk pack --with <plugin>` verifies each named source
package, then `go build -overlay`s a generated `Register()` into a new
species EXE. The live `zz_register.go` stays empty. The only pack
outputs are the EXE and `generation.json` (`source_ref=file:<exe>`).
Pack does not open the production Journal. `evals/start` launches that
artifact when `source_ref` is a present `file:` path. Compiled plugin
tools join the process tool set through `internal/pluginhost`, not
`config.tools.enabled`.

**Guard tests.** Pack hello-fs writes exe + generation.json and leaves
the live register empty; pluginhost can `Run` `hello_stat`; eval of a
`file:` generation uses that EXE; production sessions stay unchanged;
missing `--with` / failed verify / missing `go` do not emit an exe.

## ADR-016 — Studio card (S7) — product meaning void

**Decision (historical, 2026-08-15).** The embedded UI grew a Studio
surface beside Review Center. That implementation may remain in the
tree.

**Superseded.** ADR-018 / `VIVY-STUDIO.md`: Vivy Studio is an
independent application. The gateway card is not Studio. Do not
extend this surface's product semantics (NG-28).

**Guard tests.** Existing Playwright may keep running as a regression
on leftover UI; it is not Studio acceptance.

## ADR-017 — vivy-sdk is its own binary (2026-08-15)

**Decision.** The packer is `vivy-sdk.exe`, not `vivy sdk`. Source lives
at repo-root `sdk/` (`plugin/` for authors, `internal/` for the packer,
`main.go` for the entry). It is not under `cmd/`. The daily `vivy.exe`
has no sdk subcommand. `vivy-sdk` may later ship species sources or a
Go toolchain and is allowed to be large; that is why it cannot ride
inside the personal-gateway install.

**Guard tests.** `go build ./cmd/vivy` has no `sdk` import; `go build
./sdk` is the packer; `plugins/` still imports only `agent-vivy/sdk/plugin`.

## ADR-018 — Vivy Studio is an independent application (2026-08-15)

**Decision.** Vivy Studio is a second product. It owns Studio development and
species distribution (worktree, verify, pack, eval, release, install,
rollback). Daily `vivy.exe` is the installed body. The species never launches
Studio. Lifecycle objects live in Studio's store. ST-6 proved that Studio can
serve as the first-party lifecycle development environment; Vivy feature
authors use the backend + Vite frontend inner loop by default. Other
authorized developer tools may work directly in the same source workspace
with their own native capabilities (NG-21..NG-28).

**Guard tests.** None in this ADR: it records a product constraint.
Acceptance is the ST-* evidence in `VIVY-STUDIO.md` §10.

## ADR-019 — Container packaging is the same organism (2026-08-24)

**Decision.** Docker is a packaging of the default `vivy` binary (embedded
UI, in-process JSON-RPC, SQLite Journal). One container is one organism:
replica=1, one volume-backed Journal at `/data/vivy.db`, listen
`0.0.0.0:8787` inside the container, host publish `127.0.0.1:8787` only.
`server.allowed_origins` stays empty so the origin policy remains
same-origin. The split UI (`vivy_headless`) stays a same-machine
non-container path. Optional Postgres, Redis, and remote origins are not
this ADR.

**Guard tests.** Config accepts `0.0.0.0` only with empty origins;
`docker/config.yaml` loads as a Default overlay; compose must not publish
8787 on all interfaces.

## ADR-020 — Optional Postgres Journal (2026-08-24)

**Decision.** SQLite remains the default one-click Journal. Postgres is
the only optional server backend. Selection is `storage.backend:
postgres` plus `storage.postgres.dsn_env` (an environment variable name;
the DSN never sits in yaml). One process owns one DSN via an instance
lease; a second `Open` returns `ErrLeaseHeld` (CN-14 exclusive mode).
The CN-01..CN-17 suite lives on `storage.Engine` (`internal/storage/conformance`).
Eval air-gap stays SQLite and must not inherit the production DSN.
MariaDB, Redis, GORM, and replica sets are out of this ADR. `just ci`
does not require a Postgres server.

**Guard tests.** Config rejects a missing `dsn_env` and `mariadb`;
sqlite CN suite runs in `just ci`; postgres CN suite runs when
`VIVY_POSTGRES_TEST_DSN` is set.

## Status

- v0 (2026-08-09): baseline recorded after V0 closure (M0..M4, AS-1..AS-9
  verified on deterministic test doubles and real provider paths). Closes SR-1 / D-035.
- 2026-08-14: next-generation write-up lives at
  `docs/architecture/SELF-EVOLVING-GATEWAY.md` (narrative) and
  `docs/architecture/VIVY-GATEWAY-AND-STUDIO.md` (decision table).
  Direction adopted the same day; ADR-010 is the first implementation slice.
- 2026-08-15: ADR-012 live species inspect is implemented (`species/inspect`).
- 2026-08-15: ADR-013 air-gapped eval is implemented (`evals/start`).
- 2026-08-15: ADR-014 SDK verify is implemented (`vivy-sdk verify`).
- 2026-08-15: ADR-015 SDK pack is implemented (`vivy-sdk pack`).
- 2026-08-15: ADR-016 Studio card was claimed; product meaning voided
  the same day by ADR-018.
- 2026-08-15: ADR-017 splits `vivy-sdk` into `sdk/` as its own binary.
- 2026-08-15: ADR-018 — Studio is an independent app and first-party
  lifecycle development environment. Other authorized tools may work directly
  in the workspace; Vivy feature development now recommends the split loop.
  Canonical: `docs/architecture/VIVY-STUDIO.md`.
- 2026-08-24: ADR-019 — Docker packages the same embedded-UI organism
  (replica=1, volume SQLite, host-loopback publish).
- 2026-08-24: ADR-020 — optional Postgres Journal, instance lease,
  CN suite on storage contracts.
</file_content>
