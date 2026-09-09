# agent-vivy

Implementation home of the Vivy species (`vivy.exe`), the independent
VIVY CODE terminal (`vivy-code.exe`), and the first-party Studio overlay.
Canonical product rules: `docs/architecture/VIVY-STUDIO.md` and
`docs/architecture/VIVY-FACE-PACK.md`.

## Architecture decision order (mandatory)

Vivy implementation decisions follow this order. A lower priority must not
silently override a higher one:

1. **Vivy architecture unity first.** Preserve the canonical product
   contracts, the single `Service.Run` / Journal / policy path, existing
   package seams, domain types, durability semantics, and Vivy vs Studio
   scope separation. Do not create a second runtime, second source of truth,
   or feature-specific path around those contracts.
2. **Eino-native capability second.** Before designing or implementing an
   LLM/runtime capability, inspect the repository-pinned Eino and EinoExt
   versions for an existing component, ADK primitive, middleware, compose
   abstraction, schema helper, callback, or transport. If it satisfies the
   requirement without violating priority 1, use or adapt it instead of
   rebuilding the same mechanism in Vivy.
3. **Custom code is the exception.** Vivy-owned implementation is justified
   only when the pinned Eino surface is missing the capability, cannot meet a
   Vivy product invariant, or would force an architecture violation. Keep the
   custom seam minimal and record: the Eino APIs inspected, the concrete gap
   or conflict, why an adapter is insufficient, and the migration/removal
   boundary if Eino later closes the gap.

“Eino-native first” does not mean leaking Eino across the codebase. The
existing import quarantine remains part of architecture unity: only
`internal/runtime/` and `internal/provider/` may import
`github.com/cloudwego/eino*`; other packages consume Vivy domain interfaces.
Prefer a thin adapter at that boundary over either duplicating Eino internals
or exposing Eino types through product, storage, policy, RPC, plugin, or UI
layers.

Every plan and review that touches agent loops, model/tool orchestration,
prompting, streaming, context management, checkpoints, callbacks, RAG, MCP,
or multi-agent behavior must include an **Eino capability check** before code
is added. Naming a custom type `Eino*` is not evidence of Eino reuse; cite the
actual upstream package/API used. Reviewers must reject unexplained parallel
implementations even when tests pass.

## Scope Separation: Vivy vs Vivy Studio

**Default scope is VIVY (the species/kernel).** When the user mentions "Vivy" without "Studio", develop the Vivy kernel/species itself — not the Studio overlay. Only when the user explicitly says "Studio" or "Vivy Studio" should you work on the Studio overlay/shell.

- **Vivy (default)**: Kernel, engine, UI, skills, recipes, plugins, product-contract docs. Use `just ci` for verification.
- **Vivy Studio (explicit only)**: First-party IDE shell, skin, theme, lifecycle. Use `just studio` for builds. See `.agents/skills/vivy-studio-lifecycle`.
- **Studio shell source** lives in the git submodule `studio/` → [`ProjectViVy/vivy-studio`](https://github.com/ProjectViVy/vivy-studio). The host no longer vendors plugin trees. Lifecycle CLI (`cmd/vivy-studio`, `internal/studiocore`) stays in this repo.

This separation prevents accidental cross-contamination between the species runtime and its development environment.

## Studio submodule bootstrap

After a plain `git clone` (without `--recurse-submodules`), `studio/` may be
empty. Before editing Studio shell files or running `.\launch-vivy-studio.ps1`,
ensure the checkout:

```text
just ensure-studio
# or: git submodule update --init --recursive -- studio
# or: git clone --recurse-submodules https://github.com/ProjectViVy/agent-vivy.git
```

`launch-vivy-studio.ps1` calls `scripts/ensure-studio.ps1` on every start
(no-op when `studio/dsh-vivy-studio/package.json` is present). Agents that
touch Studio should run the same ensure first. Do **not** treat missing
`studio/` as a reason to re-vendor trees into the host repo.

## Development environment

**Vivy feature development starts the split pair.** Do not use the embedded
UI in `vivy.exe`, Docker, or `just build-split` as the inner-loop server.

```text
just dev                      # one-click: backend + Vite, Ctrl+C stops both
# or two terminals:
terminal 1: just run          # control plane 127.0.0.1:8787
terminal 2: cd ui; pnpm dev   # Vite UI 127.0.0.1:3015, proxies /rpc
```

Open the app at `http://127.0.0.1:3015` (`.\dev.ps1` / `.\dev.cmd` /
`just dev` also opens it). The Vite `/rpc` proxy keeps the same-origin
browser contract while UI and Go reload independently.

- Embedded UI (`http://127.0.0.1:8787` on the default binary) is the
  release / `just ci` smoke path.
- `just build-split` is the packaged headless-backend + standalone-static-UI
  path; it remains loopback-only.
- Docker (`just docker-up`) packages that same embedded-UI binary as one
  container with SQLite on a volume and host bind `127.0.0.1:8787`; it is
  not a replica set and not a second Journal. Optional Postgres
  (`just docker-up-postgres` / `storage.backend: postgres`) is still one
  organism and one lease; SQLite remains the default.

Vivy Studio is still the first-party lifecycle and Studio-shell product, but
it is not a mandatory venue for Vivy feature development. Authorized developer
tools should edit and verify this workspace directly; do not transfer or
replay work inside Studio merely to satisfy a venue rule. Daily `vivy.exe`
remains a tenant product, not an IDE.

## Air gap (ST-2)

The Studio engine's workspace is this repository root. It is not
`data/` and it is not a tenant install.

Do **not** read or write production journals from a Studio session:

- `data/vivy.db`
- `data/demo/`
- `data/workspaces/`

Studio's own DSH home is `data/studio-home/` (sessions, profile, settings).
That directory is the engine's scratch, not the species Journal.

## How to verify

- Browser UI during development: `http://127.0.0.1:3015` (split Vite), not
  the embedded UI on `:8787`
- Kernel / docs / UI: `just ci`
- User plugin v1: first use `.agents/skills/vivy-plugin` and the manually
  scheduled phase under `docs/plans/plugin-platform/`. The v1 Ports are
  currently specified, not implemented; do not fall back to the rejected v0
  API. Once the v1 SDK phase ships, verify with `vivy-sdk verify
  plugins/<name>`, pack the explicit Recipe, then inspect the artifact.
- Studio lifecycle (pack → eval → release → install → rollback):
  `just studio` builds `vivy-studio.exe`; the ledger lives at
  `data/studio-home/studio.db` (Studio-owned, not the species Journal).
  See `.agents/skills/vivy-studio-lifecycle`.
- Studio shell / skin / console: sources under submodule `studio/`
  (`just ensure-studio` first); launch with `.\launch-vivy-studio.ps1`
- Do not treat a hand-rolled `go test` as the product path when `just ci` exists
- Do not open `internal/runtime/engine.go` to "install" a plugin

Prefabricated skills: `.agents/skills/vivy-plugin`,
`.agents/skills/vivy-plugin-five` (legacy redirect only),
`.agents/skills/vivy-kernel-ci`, `.agents/skills/vivy-studio-lifecycle`,
`.agents/skills/vivy-studio-skin`.

## Plugin development v1 (mandatory)

Normative sources are `docs/architecture/VIVY-MODULE-STANDARD.md`,
`docs/architecture/VIVY-PORT-CATALOG.md`,
`docs/architecture/VIVY-PLUGIN-SPEC.md`, and
`docs/architecture/VIVY-ASSEMBLY.md`. The executable program plan is
`docs/plans/plugin-platform/README.md`. Use `.agents/skills/vivy-plugin` for
any Module, Port, plugin, Recipe, Generation, `vivy-sdk`, pack, or Inspect
work.

- Design in the order **Module -> typed Port -> Provider/Consumer -> Recipe ->
  generated Assembly -> Generation evidence**. Do not recreate a God `Plugin`
  interface, untyped registry, or last-writer-wins composition.
- `vivy.plugin/v0`, `Seam`, the legacy public API, compatibility Adapters, and
  migration commands are rejected. Do not extend or preserve them. Until P1/P2
  ships, v1 implementation requests follow the approved plan instead of using
  v0 as a shortcut.
- External Modules enter only through explicit Recipe source pins. Never scan a
  directory or load Go/UI Module code at runtime. Do not hand-edit generated
  Assembly files.
- Every public Provider uses one cataloged `std/*` Port and one named Host
  Consumer. Public Modules cannot provide `core/*`, access raw Journal/Policy/
  storage/credentials, or bypass `Service.Run`, ToolHost, ChannelHost,
  FaceHost, or ActionHost.
- The protected Tools `ask_user`, `list_dir`, `read_file`, `search_files`,
  `write_file`, `patch`, `multiedit`, `execute`, `bash`, `skills_list`, and
  `skill_view` are T1 implementations with reserved IDs. Public Modules cannot
  shadow, alias, replace, or override them.
- All public Port Hosts and established first-party Providers belong to the
  default Generation. Missing network configuration/credentials means
  unconfigured and inactive, not automatic connection.
- A selected UI Module has complete UI control by default. There is no UI Grant
  or authorization prompt. Backend inputs remain untrusted and server-side
  identity, schema, Policy, Grant, approval, Run, Tool, and Journal checks stay
  authoritative.
- For Provider/model/OAuth/orchestration/RAG/MCP adapter capability, inspect
  and cite the pinned Eino/EinoExt API. Adapt it when present; otherwise mark
  the capability `DEFERRED-INDEFINITE`. Do not build a custom substitute.
- A Port is supported only with Definition, SDK Contract, Host Consumer, real
  Provider, Failure Model, Conformance Suite, and Inspect Projection.

## DSH harness reference source

DeepSeek Harness source lives in `.workspace/deepseek-harness/` (currently:
`deepseek-harness/` working clone + `upstream/` mirror of
`https://github.com/deepseek-ai/deepseek-harness.git`). It is reference
material for engine internals — do not edit it as the implementation path.

If the checkout is missing and you need it, clone it into `.workspace/`:

```text
git clone https://github.com/deepseek-ai/deepseek-harness.git .workspace/deepseek-harness/upstream
```

`.workspace/` is gitignored (see `.gitignore`) — never commit it. The
installed `@deepseek-ai/dsh` npm package is built JS, not source; treat this
tree, not `node_modules`, as the source of truth for harness behavior.

## Documentation placement

Ordinary design notes, research, reviews, plans, reports, and iteration
records belong under `docs/`.

### Documentation language (mandatory)

Write all human-readable documentation in English. Preserve non-English text
only when it is an exact code, UI selector, test fixture, localization value,
protocol payload, or other literal that must remain unchanged.

Keep at the repository root only intentional entry points: `AGENTS.md`,
`README.md`, `LICENSE`, `justfile`, `config.example.yaml`. Product-contract
docs live in `docs/architecture/`. Research dossiers live in `docs/research/`.
Iteration records live in `docs/logs/`. Crate-local or UI-local `AGENTS.md`
files stay beside the code they govern (`ui/AGENTS.md`, `.agents/skills/`).

Do not drop one-off plans on the repository root. Runtime data (`data/`),
Studio scratch (`data/studio-home/`), and `.workspace/` are not documentation.

## Iteration logs (`docs/logs`)

Every deliverable change (kernel, UI, Studio, product-contract docs, or a
closed TODO track) gets a new directory under `docs/logs/`.

Naming: `YYYY-MM-DD-short-slug` (date of the delivery, hyphenated theme).
Do not nest extra version directories unless one folder must hold several
shipped cuts of the same theme.

Required files in that directory:

- `summary.md` — what changed, scope, what was explicitly not done
- `verification.md` — commands run and results (`just ci` at minimum)
- `acceptance.md` — how a human can tell it worked (product/user view)

Optional: `notes.md` (discussion), `release.md` (how it ships; omit with a
reason if not a release), `rollback.md`.

Existing examples: `docs/logs/2026-08-12-hitl-release-closure/`,
`docs/logs/2026-08-16-studio-lifecycle/`. Archives of closed boards also live
here (see `docs/logs/2026-08-25-todo-board-archive/`).

Counterexample: finishing a feature with only chat history and no log.

## Backlog (`docs/TODO.md`)

`docs/TODO.md` §0.1 is the living open board. Root `TODOLIST.md` is not used.

When a bug, gap, or deferred item is found and not fixed in the same
iteration, add it to §0.1 (status, short title, why it waits, related paths).
When it is done, move it to §10 Completion log and/or `docs/logs/` — do not
silently delete it.

Closed tracks (V0, MA, ET, HITL P0, harness H0–H10, Studio ST-*) stay in the
file as archive tables; do not pick new work from them.

## Parallel lanes (worktree isolation, hard requirement)

The shared root working tree hosts at most one active write lane. Parallel
work is isolated by structure, not by coordination — there is no `LOCK.md`.

- A second concurrent lane (another agent session, a Studio session, or
  manual edits) must develop in its own `git worktree` on its own branch
  (`git worktree add ../agent-vivy-<slug> -b feat/<slug>` from this root)
  and must not edit the root tree while another lane is active.
- A new feature started while the root tree is not clean also goes to a
  worktree; do not stack unrelated themes in the root tree.
- A lane lands back through its branch (merge or PR), never by piling
  files into the root tree.
- Ignored runtime state (`data/`, `data/studio-home/`, `.workspace/`) is
  per-checkout; a worktree starts with its own empty scratch.
- If two lanes must touch the same files, they are one lane — sequence
  them on a single branch.

## Validation

Default gate for kernel, UI, Studio overlay, and product-contract docs is
**`just ci`** from the repository root. A hand-rolled `go test` is not the
product path when `just ci` exists.

User-visible or executable behavior also needs a minimum real-path smoke:

- Browser UI: exercise the change at `http://127.0.0.1:3015` (split Vite),
  not only a screenshot and not the embedded UI on `:8787`
- Plugin: `vivy-sdk verify` then `vivy-sdk pack --with <name>`
- Studio lifecycle: `just studio` and the skill `vivy-studio-lifecycle`

Record the commands and outcomes in that iteration's `verification.md`.

Tests should be deterministic (no live network in unit tests). Cover a
representative failure path, not only the happy path. New config fields need
parse/validate tests. Secrets stay out of fixtures, logs, and event payloads
(D-010).

## Secrets, errors, and logs in code

- Never commit tokens. Config holds `env_key` names only.
- Redact secrets in logs, errors, snapshots, and test fixtures.
- Preserve error cause chains; do not discard the source error.
- Structured logs: include run/session ids when useful; never include
  provider keys, bot tokens, or raw Journal blobs.
- Log output contract (init path, config + env overrides, file sink,
  field keys, levels): `docs/architecture/LOGGING.md`. Kernel logging
  goes through `internal/logging.Setup` only — no ad-hoc handlers.

## Provider model IDs

When calling a provider's native OpenAI-compatible endpoint, send that
provider's raw model id. Do not auto-insert a gateway `provider/model`
prefix. Prefix rewriting is only for a true aggregator gateway. Changing
routing requires a test that asserts the outbound `model` field.

## Rulebook (mandatory unless a rule states an exception)

- **architecture-unity-first** — Preserve Vivy's canonical contracts, single
  runtime/Journal/policy path, domain firewall, and existing seams before
  optimizing a local feature. No parallel runtime or source of truth.
  Maintainer: current design and delivery owner.
- **eino-native-second** — After satisfying architecture unity, inspect and
  prefer the repository-pinned Eino/EinoExt capability before writing custom
  LLM runtime or orchestration machinery. A custom implementation must carry
  the comparison and exception evidence required by “Architecture decision
  order”. Maintainer: current design and delivery owner.
- **expert-mode-subagent-supervision** — When the user explicitly asks to
  enable “Expert Mode”, start subagents for the problem-analysis,
  localization/diagnosis, and actual code-writing phases. The main agent is
  the supervisor: it assigns and scopes the work, keeps lanes isolated,
  reviews the findings and changes, and owns the final integration and
  verification. Do not silently enable Expert Mode when the user has not
  requested it.
- **goal-human-intent** — When the user explicitly starts `/goal` or Goal
  mode, treat the human's stated task as the sole objective. Think through
  how to complete that task; do not invent side quests, expand the product
  scope, or assign unrelated work to yourself.
- **minimal-lightweight-scope** — Keep code minimal, the framework
  lightweight, and the design clear and elegant. Do not add requirements or
  unrelated features beyond the human's request. If a separate security,
  safety, or other material finding would require a new plan or scope
  expansion, pause and ask the human before proceeding, unless an immediate
  safety stop is required.
- **iteration-log-required** — Deliverable work writes `docs/logs/<date>-<slug>/`
  with `summary.md`, `verification.md`, and `acceptance.md` before claiming
  done. Maintainer: current delivery owner.
- **just-ci-is-the-gate** — Kernel / UI / Studio / architecture-doc changes
  run `just ci`. Skip a slice only with a reason in `verification.md`.
  Maintainer: current delivery owner.
- **smoke-for-user-visible-change** — UI or executable behavior is not done
  after unit tests alone. Hit `http://127.0.0.1:3015` (or the plugin/Studio
  path above) and record it. Maintainer: current delivery owner.
- **todolist-capture-required** — Unfixed findings go in `docs/TODO.md` §0.1
  in the same iteration. Maintainer: current assistant.
- **air-gap-tenant-journal** — Do not read or write `data/vivy.db`,
  `data/demo/`, or `data/workspaces/` from a Studio or agent session.
  Maintainer: current assistant.
- **no-plugin-via-engine-import** — Do not install a plugin by editing
  `internal/runtime/engine.go`. Use `vivy-sdk pack`. Maintainer: current
  assistant.
- **parallel-worktree-isolation** (hard requirement) — The shared root
  working tree hosts at most one active write lane. A second concurrent
  lane, or a new feature started on a dirty root tree, must develop in its
  own `git worktree` on its own branch and land via merge/PR. See
  "Parallel lanes". Maintainer: current assistant.
- **commit-one-concern-per-deliverable** — Every completed deliverable is
  committed on completion as one focused commit: stage only that
  deliverable's explicit paths, keep unrelated pre-existing dirty changes
  and other lanes' files out, and remove scratch artifacts first. Never
  mix features, docs, and cleanup in one commit. Pushing still requires
  explicit user authorization. Maintainer: current delivery owner.

Not ported from agent-diva on purpose: `LOCK.md` parallel mutex, root
`TODOLIST.md`, `/new-command` index, per-update auto-commit, and the
`[I strictly follow the rules]` reply prefix. Concurrency and commit
hygiene are handled natively by `parallel-worktree-isolation` (structural
worktree separation replaces the lock file) and
`commit-one-concern-per-deliverable` (commits follow deliverables, not
raw updates). Vivy already has air-gap, `just ci`, and Studio venue
rules; those stay as written above.
