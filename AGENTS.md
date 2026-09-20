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

### Development principles: More, Fast, Good, Frugal

Build capability with smallest coherent system meeting product contract. Decision criteria, not workflow. Apply independently or inside Supermanagement/Superpowers. Do not activate either merely to apply this skill.

### Scope, precedence

Apply to work already required. Simple edits leaving behavior, interfaces, state, dependencies unchanged need focused checks, not full assessment. Unrelated questions, ordinary prose edits do not trigger this skill.

Follow host instruction hierarchy, authorization boundaries. Within limits: explicit authorized user requirements > repository constraints > this skill defaults. Surface material conflicts. Change repository constraints only when explicitly authorized. General request to work faster not waive mandatory gates.

Correctness, security, data integrity, agreed product contract = acceptance conditions. Make intentional contract changes explicit. Exceptions below concern design defaults, never these conditions. Among acceptable solutions prefer minimum total lifecycle cost, then speed within that frugal design.

### Four principles

Use defaults routinely. Explain exceptions only when consequential. Evidence = confirmed requirement, inspected behavior, or measurement. Fields are decision aids, not form to fill per task.

**More: Less is more**
Default: small cohesive capabilities, clear boundaries, useful defaults, meaningful composition. Not fixed workflows or feature counts.
Exception: confirmed variation or independently supplied capabilities justify targeted extension interface. Not automatically plugin framework.
Evidence: name actual consumer, requirement, or variation boundary supports. Hypothetical future flexibility insufficient.

**Fast: Frugality before optimization**
Default: keep execution paths direct. Cut unnecessary I/O, copying, allocation, repeated work before adding machinery.
Exception: add caches, concurrency, or services when measurements or established runtime constraint show simpler approaches insufficient and benefits justify lifecycle costs.
Evidence: relevant latency, throughput, memory, or startup measurements, or documented constraint. Label estimates. Invent neither targets nor performance gains.

**Good: Lean development, accountable design**
Default: keep code readable, work small. Clarify scope, boundaries, ownership, failure behavior, acceptance before consequential delegation. Resolve shared decisions before parallel work. Limit active work to review capacity.
Exception: cross-component, persistence, concurrency, or security risks require broader design and verification than isolated changes.
Evidence: behavior, failure-path, integration checks proportionate to actual risks, plus required gates. Supervisors retain architectural and end-to-end acceptance responsibility.

**Frugal: Minimum sufficient total cost**
Default: deliver requested behavior plus necessary implementation details only. Reuse or safely delete before adding code, dependencies, configuration, or process.
Exception: extra machinery must address identified requirement or reduce total lifecycle cost versus viable alternatives.
Evidence: account maintained LOC across solution, runtime, maintenance, operations, user effort, agent time/tokens. LOC = design pressure, not quota. Preserve readability, meaningful verification.

### Red flags

Investigate signals. Not automatic findings:

- Performance complexity without measurements or established constraint.
- Hypothetical abstractions, or coupling blocking required composition.
- Unrequested features, configuration growth without concrete need, unrelated cleanup.
- Apparent LOC savings hiding complexity in dependencies, generated code, user effort.
- Contract regressions, silent failures, claims beyond available verification.
- Duplicated plans, mandatory delegation, process without demonstrated benefit.

Complexity signals, not automatic violations. Inspect hand-maintained code around thresholds: function nesting >4; conditionals (if/ternary) >8 per function; parameters >6; classes >20 methods; files >600 lines or sprawling exports with mixed responsibilities; duplicated blocks 10+ lines occurring twice; direct access to internals of >3 unrelated objects.

Report finding only when signal reveals concrete comprehension, coupling, change, or correctness risk. Prefer smallest in-scope fix. Do not split code, wrap parameters, or invent abstractions merely to satisfy counts. Respect repository-specific thresholds. Account for generated code, declarative tables, intentional repetition. Routine work: inspect only change plus direct impact. Broaden review only when requested.

### Output contracts

Use only relevant format inside existing workflow:

**Project rules:** normally 5–10 actionable project-specific rules. Fewer when sufficient. Read existing instructions and relevant architecture first. State defaults, exception conditions, evidence where applicable. Modify only established or user-selected instruction file when requested. Keep one authoritative version.

**Design:** one paragraph: simplest viable choice, material alternative, tradeoff, supporting evidence. Summarizes decision, not entire architecture. Keep interfaces, ownership, failure behavior, acceptance details in existing plan. No parallel paperwork.

**Review:** findings ordered by impact. Each: location, evidence, consequence, smallest useful correction. No substantiated issues = say so. Do not invent findings or unrelated rewrites.

Distinguish implemented, verified, unverified outcomes. Stop expanding validation when acceptance evidence sufficient, concrete risks covered, required gates pass. Disclose remaining limitations.

Replace or remove rules before appending more. Add rules for recurring decisions or demonstrated failures, not to grow this skill.

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
  because they are available. Follow applicable project-specific skills below.

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

## Single source of truth (mandatory)

Priority 1 says "do not create a second source of truth". This is how that is
checked, for any fact entering the run path (provider metadata, model ids,
defaults, credentials, addresses, capabilities).

- Every such fact has exactly one home, and it answers three questions: is it in
  the artifact, is it in the evidence (source digest / conformance), and who
  changes it. Two out of three is a defect, not a follow-up.
- A second copy is a deletion task, not a synchronization task. Derive it or
  delete it; a hand-edit kept in step by a checklist is already broken.
- Displayed truth derives from capability truth: every row, model or option a
  face renders is declared by the backend or explicitly marked deferred. A UI
  list never stands in for what the runtime can construct.
- Ownership is "is it in the artifact", not "is it in the repository": data that
  ships beside the binary, is copied by a packer, or sits outside the digest has
  no owner. Never put run-path data under a directory named `fixtures/` or
  `samples/`; if the live path reads it, its name is part of the contract.
- A ported schema or vendored tree is audited field by field ("do we have this
  behaviour?") before use; record the source and the dropped fields.
- Deleting a redundant copy is a deliverable, reviewed like code. If removing a
  copy earns no credit, copies accumulate.
- Stop and decide on any of these signals: two layers report different counts
  for the same fact, a field nothing reads, a workaround performed every time.
  After a third delivery in one domain, re-ask what that domain's core word
  means.

Worked example: `docs/logs/2026-09-18-provider-registry/notes.md`.

## Commit Rule for AI agents

**important!** :AI tools may assist development, but must never appear as commit authors, committers, co-authors, PR authors, or repository contributors. All contributions must be attributed to the human contributor responsible for the change.
**UNLESS YOU ARE INDIVIDUAL,HAVE YOUR OWN NAME,YOUR OWN GITHUB IDENTITY,ACCOUNT,NOT A COMPANY'S PRODUCT.**

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

Keep at the repository root only intentional entry points: `AGENTS.md`,
`README.md`, `LICENSE`, `justfile`, `config.example.yaml`. Product-contract
docs live in `docs/architecture/`. Research dossiers live in `docs/research/`.
Iteration records live in `docs/logs/`. Crate-local or UI-local `AGENTS.md`
files stay beside the code they govern (`ui/AGENTS.md`, `.agents/skills/`).

Do not drop one-off plans on the repository root. Runtime data (`data/`),
Studio scratch (`data/studio-home/`), and `.workspace/` are not documentation.

## Iteration logs (`docs/logs`)

Every deliverable change (kernel, UI, Studio, product-contract docs, or a
closed TODO track) gets a new directory under `docs/logs/`. For minor
editorial or agent-instruction changes with no product behavior or contract
change, a focused commit describing the change and checks is sufficient.

Naming: `YYYY-MM-DD-short-slug` (date of the delivery, hyphenated theme).
Do not nest extra version directories unless one folder must hold several
shipped cuts of the same theme.

Required files in that directory:

- `summary.md` — what changed, scope, what was explicitly not done
- `verification.md` — commands run, results, and reasons for any skipped checks
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

Default gate for kernel, UI, Studio overlay, and product-contract docs is
**`just ci`** from the repository root. A hand-rolled `go test` is not the
product path when `just ci` exists.

For minor editorial or agent-instruction changes, review the diff and run
`git diff --check`; full product CI is unnecessary. Do not add tests that
merely restate reversible, low-impact edits. After required checks pass,
repeat or expand verification only for new changes, failures, or unresolved
risks. Remove temporary artifacts created by the task before finishing.

User-visible or executable behavior also needs a minimum real-path smoke:

- Browser UI: exercise the change at `http://127.0.0.1:3015` (split Vite),
  not only a screenshot and not the embedded UI on `:8787`
- Plugin: follow `.agents/skills/vivy-plugin` for SDK verification, explicit
  Recipe packing, and artifact inspection. Never install plugins by editing
  `internal/runtime/engine.go`.
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

## Delivery

Commit each completed deliverable as one focused change, including its related
code, documentation, and verification evidence. Stage explicit paths only;
leave unrelated changes untouched. Push when authorized by the user, including
existing authorization in the current conversation.
