# Default / headless Laputa persona integration

The default development Generation omitted `vivy/diva-cognitive`, while the
Persona page wrote demo localStorage. The existing primary admission path
already supported Garden FrozenCore v2, but that path was unbound in the default
Generation. This delivery selects the existing owner and replaces the demo UI
with its session-bound control actions.

## Design and boundaries

Reuse the T1 `vivy/diva-cognitive` owner, the supported exclusive
`core/cognitive-factory@v1` seam, and `std/control-action@v1` through ActionHost.
The existing generated Assembly is the only composition root; Garden remains
the only persona authority. The existing factory passes
`<data directory>/garden/persona` as Garden's config root; the pinned
`persona.Open` appends `persona`, so authority documents live under
`<data directory>/garden/persona/persona`. This patch preserves that existing
storage layout. No new
Port, grants, resolver, runtime, authority store, or evolution scheduling is
introduced. Cognitive evolution remains disabled by default. Persona writes
are explicitly allowed in the default governance profile; unrelated cognitive
writes are not broadened. Authentication, connection-bound session checks,
strict inputs, and authority revision checks remain in force.

The default recipe is also what `dev.ps1` builds with `vivy_headless`. The build
tag removes embedded web assets; it does not select `recipes/headless.vivy.yml`.
Deliberately reduced recipes and the coding-only product are outside this fix.

Fresh data requires explicit owner initialization through Persona. A turn before
initialization now returns an actionable conflict instead of an opaque internal
error. No browser demo data is imported or silently promoted to authority.
Existing conversation test fixtures explicitly initialize a test persona.

## Eino capability check

Inspected pinned Eino v0.9.13 `adk/handler.go`: `ChatModelAgentContext.Instruction`
and `ChatModelAgentMiddleware.BeforeAgent` already support the required prompt
installation. Vivy's existing `promptMiddleware.BeforeAgent` installs the
committed `RunPromptPayload.Instruction`; `Service.RunWithOptions` obtains the
bound bundle's `Prepare` output before prompt admission. Reuse those paths.
Eino does not own persona revision governance, session FrozenCore persistence,
or Recipe selection; no substitute agent framework or prompt pipeline is added.

## Projection contract

The authority currently has eight document kinds (including optional MISSION).
FrozenCore v2 has seven slots: MISSION, IDENTITY, RELATIONSHIP, REDLINE, USER,
DREAM, DARK. WORLD is available through explicit document reads and is not
injected into primary authority. USER and per-slot bounds retain upstream
projection semantics. A saved edit affects a new session; an existing session
keeps its durable FrozenCore, including after process restart.

The work does not claim to close the broader MEM-3 governance backlog or add
new memory/evolution features. Unsupported UI history must not display demo
history as real authority data.

## Coding-face regression guard

The standalone `vivy-code` launcher also reuses the default generated body but
owns a fresh private instance per launch. Its `FaceCode` admission retains the
existing project/coding instruction path and does not consult companion
FrozenCore. This prevents a newly selected companion module from requiring
persona onboarding on every coding instance; normal Vivy chat remains gated
on initialized authority. A real model-input test covers both face behaviors.

## Review follow-up

Default selection exposed an existing owner requirement for an absolute data
root, while `config.example.yaml` uses a relative SQLite path. The app resolves
the cognitive owner's data root with `filepath.Abs` before calling the factory,
without changing the user's config or relocating data. A default-app startup
regression test reproduces the example's relative path in an isolated directory.

The review suggestion to recheck live persona readiness for every existing
frozen session was not adopted: a persisted FrozenCore is the session authority
by contract. Only creating a new snapshot requires the current persona to be
initialized; an existing durable snapshot must remain stable across edits and
restarts. Corrupt snapshots continue to fail through the existing read path.
