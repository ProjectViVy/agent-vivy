# agent-vivy

Implementation home of the Vivy species (`vivy.exe`), the independent
VIVY CODE terminal (`vivy-code.exe`), and the first-party Studio overlay.
Canonical product rules: `docs/architecture/VIVY-STUDIO.md` and
`docs/architecture/VIVY-FACE-PACK.md`.

## Communication and instruction priority

- Match user-facing conversation to the language of the user's first message
  in the current chat, unless they explicitly request a change. Keep code,
  commands, and technical identifiers in English.
- Write documentation and durable records in English, including plans,
  reviews, iteration logs, commit messages, and issue/PR descriptions.
  Preserve exact literals and localization values when required.
- Lead with outcomes and impact, then necessary actions, decisions, and
  evidence. Use concise paragraphs, concrete words, and lists only when useful;
  omit boilerplate, repeated summaries, and unrequested comparisons.
- Follow system, platform, and security constraints. Within those constraints,
  current explicit user instructions take precedence over skills, memory, and
  defaults. Project instructions apply within their directory scope.

## Development principles: More, Fast, Good, Frugal

- Deliver useful capability with the smallest coherent implementation. Optional
  features use existing Module/Port boundaries and an explicit Recipe; adding
  and removing a feature must preserve the basic production loop.
- Shorten time to real feedback. Reuse existing code and checks; remove repeated
  I/O, state and process before adding caches, concurrency or frameworks.
- Judge quality through user behavior, clear failure ownership and readable code.
  A mechanism needs an actual consumer, confirmed requirement or observed failure.
- Minimize total cost: code, runtime, maintenance, user effort and agent time.
  Complexity needs justification; simplicity does not need extra paperwork.
- Do ordinary work directly. Plans, delegation and independent reviews are useful
  when uncertainty or risk warrants them, not mandatory stages for every change.
- Do not introduce Vivy-owned hash/digest/fingerprint/checksum validation or
  content-addressed identities. Preserve the existing hash-retirement scope.
  A final distributable may have an optional informational digest that never
  gates business behavior. Native security protocols and dependency integrity
  retain their own mechanisms.

Keep the single Service.Run/Eino/Journal/Policy path, approved intent, durable
execution ownership, no blind effect replay, data upgrades and secret isolation.
Report implemented, verified and unverified results separately. Stop expanding
checks once affected risks and required acceptance are covered.

## Tools, skills, and delegation

- Use `rg` / `rg --files` for search; batch independent reads and queries.
  Prefer CLI/API tools; use an authenticated browser when no suitable interface
  exists. Prefer `lark-cli` for Feishu when available.
- Complete small tasks directly. For complex workflows, prefer delegation
  when independent subtasks save time or improve quality. Give each sub-agent
  clear inputs, outputs, and completion criteria; the lead agent integrates
  and verifies results. Prefer `gpt-5.6-luna` with `max` reasoning when available
  and suitable.
- Keep shared-state work and sequential decisions in one lane. Follow the
  worktree isolation rules below for concurrent edits.
- Choose optional external skills by task difficulty and relevance. Use
  Superpowers for complex work, not routine edits; do not load workflows just
  because they are available. In this project, ordinary authorized work does
  not acquire extra approval, document or delegation stages from optional
  external workflows. Do not edit their installed caches or personal settings.
  Follow applicable project-specific skills below.
- Before environment diagnosis or CI/race/Postgres/Docker/Windows acceptance,
  read `.agents/skills/vivy-advance-dev-env/SKILL.md`. Prefer a Linux-filesystem
  WSL checkout for Go/Node verification and an independent Windows checkout for
  native acceptance. Check the skill's restored-baseline boundary and actual
  recipe availability; follow the owner's requested verification scope.

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

New or replacement runtime capabilities require a pinned **Eino capability check** before code is added. Small fixes to an existing adapter reuse its established decision and inspect only the affected APIs. Naming a custom type `Eino*` is not evidence of Eino reuse; cite the
actual upstream package/API used. Reviewers must reject unexplained parallel
implementations even when tests pass.

## Single source of truth

Each runtime fact has one authority; other layers read or derive from it.
Remove redundant hand-maintained copies instead of adding synchronization.
Immutable admitted snapshots may capture an authority at a specific revision.
Displayed capabilities derive from the backend, with unavailable features
explicitly marked. Shipped runtime data belongs to declared artifact inputs,
never a fixtures/samples tree. Keep evidence only where a real decision consumes
it; a digest or test record cannot substitute for observed product behavior.

## Commit Rule for AI agents

**important!** :AI tools may assist development, but must never appear as commit authors, committers, co-authors, PR authors, or repository contributors. All contributions must be attributed to the human contributor responsible for the change.
**UNLESS YOU ARE INDIVIDUAL,HAVE YOUR OWN NAME,YOUR OWN GITHUB IDENTITY,ACCOUNT,NOT A COMPANY'S PRODUCT.**

## Scope Separation: Vivy vs Vivy Studio

**Default scope is VIVY (the species/kernel).** When the user mentions "Vivy" without "Studio", develop the Vivy kernel/species itself — not the Studio overlay. Only when the user explicitly says "Studio" or "Vivy Studio" should you work on the Studio overlay/shell.

- **Vivy (default)**: Kernel, engine, UI, skills, recipes, plugins, product-contract docs. Use the scoped validation policy below; full CI belongs to Epic closeout.
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

**Browser-facing feature development uses the split pair.** Backend-only work uses its real API, CLI or package entry without starting an unrelated UI. Do not use the embedded
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

Do **not** read or write production journals from a Studio or agent session:

- `data/vivy.db`
- `data/demo/`
- `data/workspaces/`

Studio's own DSH home is `data/studio-home/` (sessions, profile, settings).
That directory is the engine's scratch, not the species Journal.

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
  migration commands are rejected. Do not extend or preserve them. Follow the
  approved v1 plan and its current phase status; never use v0 as a shortcut.
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
- The explicit Recipe selects Hosts and Providers. Existing shipped Recipes
  preserve their selected product behavior; new first-party capabilities need
  not enter every default Generation. Missing credentials means inactive.
- A selected UI Module has complete UI control by default. There is no UI Grant
  or authorization prompt. Backend inputs remain untrusted and server-side
  identity, schema, Policy, Grant, approval, Run, Tool, and Journal checks stay
  authoritative.
- For Provider/model/OAuth/orchestration/RAG/MCP adapter capability, inspect
  and cite the pinned Eino/EinoExt API. Adapt it when present; otherwise mark
  the capability `DEFERRED-INDEFINITE`. Do not build a custom substitute.
- A Port is supported only with Definition, SDK Contract, Host Consumer, real
  Provider, Failure Model, Conformance Suite, and Inspect Projection.

## Database schema ownership (mandatory)

- Core Storage is the sole owner of core database initialization, schema, and
  migrations. DDL belongs only in `internal/storage/migrations` as paired,
  embedded SQL files; it must not be hidden in repositories, feature modules,
  plugins, or package `init()` functions.
- Migration files are append-only and immutable after release. SQLite and
  PostgreSQL use the same logical numeric ID and filename stem; dialect-specific
  SQL is allowed, but missing pairs, name drift, and checksum drift fail closed.
- The shared migration runner owns ordering, manifest validation, metadata,
  checksums, per-migration transactions, and startup failure behavior. Runtime
  artifacts must not require an external SQL directory.
- Public plugins must not access or migrate the core database directly. Plugin
  persistence requires an explicit reviewed Storage Port with its own ownership,
  lifecycle, and conformance contract.
- Any schema change must prove fresh install, every supported upgrade shape,
  reopen idempotence, rollback on failure, SQLite/PostgreSQL logical parity, and
  the repository CI gates before it is considered complete.

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

Keep at the repository root only intentional entry points: `AGENTS.md`,
`README.md`, `LICENSE`, `justfile`, `config.example.yaml`. Product-contract
docs live in `docs/architecture/`. Research dossiers live in `docs/research/`.
Iteration records live in `docs/logs/`. Crate-local or UI-local `AGENTS.md`
files stay beside the code they govern (`ui/AGENTS.md`, `.agents/skills/`).

Do not drop one-off plans on the repository root. Runtime data (`data/`),
Studio scratch (`data/studio-home/`), and `.workspace/` are not documentation.

## Delivery records

Use English under docs/. A minor editorial or instruction change needs a focused
commit describing the change and checks. An ordinary deliverable uses one record
under docs/logs/YYYY-MM-DD-slug/ covering changes, actual verification, user
acceptance and remaining limitations. Split summary/verification/acceptance,
release or rollback files only when a complex migration or release benefits.
Preserve historical records. Do not create repeated records for every small fix.

## Backlog (`docs/TODO.md`)

`docs/TODO.md` §0.1 is the living open board. Root `TODOLIST.md` is not used.

When a bug, gap, or deferred item is found and not fixed in the same
iteration, add it to §0.1 (status, short title, why it waits, related paths).
When it is done, move it to §10 Completion log and/or `docs/logs/` — do not
silently delete it. When it is deliberately deferred, superseded, or declined,
move the row to `docs/DEFER.MD` with its status and the record that would
restart it; do not leave non-open rows on the open board.

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

For skill, instruction or documentation changes that do not affect product code
or behavior, review the diff and run only relevant static checks. Do not change
CI wiring or run product CI as a side effect of that work. Changes to CI itself
require an explicit CI task; an instruction update alone does not authorize them.

Development uses `just ci-small --base <ref> [--scope <id> ...] [--plan]`.
.github/ci-scopes.json is the sole path/Recipe/test association table. Explicit
scopes add coverage; unknown executable inputs must be mapped. Document-only
changes use static checks. Feature work verifies its selected Recipe and actual
behavior. Shared runtime, protocol, SDK, storage or concurrency changes add their
owning checks; a narrow Recipe never excuses affected shared contracts.

Use the environment guidance in vivy-kernel-ci and vivy-advance-dev-env. Kernel
lanes run in a Linux-filesystem WSL/Linux checkout with its own pinned Laputa;
Windows native acceptance remains separate. UI behavior uses the isolated split
Vite at :3015. No UI dependencies are required for a backend-only small CI.

At Epic closeout, the final integrated candidate runs full product CI after
small CI. The maintainer applies `epic-final` to its PR; updates rerun checks.
Keep `just ci` and all existing product checks. The aggregate reports scoped
results separately from full CI not requested/not verified. Missing services,
failed commands and zero required executed tests never count as success.
Tests are deterministic and use disposable data. Add meaningful failure-path
coverage for changed behavior; do not add tests that merely restate reversible
edits. Record actual commands and outcomes, then stop repeating unchanged checks.

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

## Delivery

Commit each completed deliverable as one focused change, including its related
code, documentation, and verification evidence. Stage explicit paths only;
leave unrelated changes untouched. Push when authorized by the user, including
existing authorization in the current conversation.
