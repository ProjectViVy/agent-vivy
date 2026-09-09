# How to Make Vivy a Single-EXE Self-Evolving Gateway

> **Plugin v1 note (2026-09-09):** This document retains the single-EXE / generational-product direction; the
> descriptions of `vivy.plugin/v0`, `vivy.generation/v0`, Seam, the old registry, and compatibility migration are
> historical. New implementations follow only `VIVY-MODULE-STANDARD.md`,
> `VIVY-PORT-CATALOG.md`, `VIVY-PLUGIN-SPEC.md`, and `VIVY-ASSEMBLY.md`.
>
> Status: **direction adopted** (2026-08-14). **Studio shape corrected on 2026-08-15**: an independent app that manages development and distribution; Studio is the first-party daily development IDE, but not the exclusive execution venue.
> It does not replace the V0 ADR. Species-side S1–S6 parts remain usable; the S7 Studio card's product meaning is void.
> Date: 2026-08-15 (Studio correction)
> Source: comparative discussion of `.workspace/deepseek-harness/upstream` (including the paper) and the current state of agent-vivy.
> Readers: people who want to read in one pass "why split, what to split into, how to install plugins, and how much it resembles DSH."
>
> **The canonical Studio source is `VIVY-STUDIO.md`.** The condensed decision table remains in `VIVY-GATEWAY-AND-STUDIO.md` (NG-1..NG-28, S0..S6 + ST-*).
> This document governs the species / kernel / assembly. If it conflicts with `VIVY-STUDIO.md` on Studio shape, that document wins and this one must be updated.

Related:

- `../../AGENT-VIVY-DIRECTION.md` — V0–V3 staging
- `../../prd-agent-vivy-v0.md` — §5.0 philosophy anchors, D-014..D-021
- `../AGENT-VIVY-ARCHITECTURE-V0.md` — assembled kernel
- `../GOAL-AGENT-HARNESS-ROADMAP.md` — harness slices already completed
- `ACP-REMOTE-CONTROL-PROPOSAL.md` — control-plane draft; borrows local and admission principles, not remote hosting
- `VIVY-CHANNEL-PACK.md` — super-channel contract (direction adopted 2026-08-30; Host in the kernel; this batch's adapters are `plugins/` + `seam: channel`)
- `VIVY-FACE-PACK.md` — built-in face cold plug/unplug proposal (FaceHost in the kernel; mouths in `faces/`; one face per generation; Android is a downstream product)
- `VIVY-STUDIO.md` — Studio product identity, lifecycle, and development-environment strategy (canonical)
- `VIVY-WORLDVIEW.md` — why the species/lab split and this name are the same underlying idea
- `.workspace/deepseek-harness/upstream` — evidence, not a species dependency

---

## 0. One Sentence

**Daily Vivy is the `vivy.exe` a person can open by double-clicking.**  
**Vivy Studio is another independent application**: develop, evaluate, release, and install the next-generation body.  
The first development engine may be DeepSeek Harness, but it is a replaceable engine inside Studio—not the root and not a card opened from the gateway.

The resident always faces one gateway. Single EXE means **the daily installation, daily life, and all capabilities are compiled into this body**. Installing a plugin = using the SDK to build a new version, not attaching parts to a live process. Authors face Studio, not an "evolution button in the gateway."

```text
author ──independent app──► Vivy Studio
                      │ modify source / verify / pack / evaluate / release / install
                      ▼
                 vivy.exe at daily install   species
                      │ read-only inspect
                      ▲
                 Studio can view, replace, stop; species does not start Studio
```

---

## 1. Where the Unease Comes From

The discussion stacked five things that cannot live in the same address space or on the same release train:

| Claim | What success looks like | What happens in one process |
|---|---|---|
| Personal gateway | Usable, auditable, recoverable today; keys stay local | Becomes a platform or lab that people will not leave running overnight |
| High-performance single EXE | One-click Windows, no cgo, short hot path | Plugin sprawl, Node, ports, and a mesh |
| Everything is a plugin (DSH) | Clean removal, rebinding dependencies, replaceable loop | The Journal and approvals are removed too |
| Self-evolve toward greater capability | Can try the next generation, kill it, and compare it | Without fitness, it is permanent rewriting |
| Microservices / K8s-like | Stable control plane, killable data plane | The local machine becomes a cluster and measurement breaks first |

The direction document already says that one stage cannot promise a stable product, complete framework replacement, and AGI-OS at the same time.  
This document assigns the three things to three bodies instead of pretending one EXE can be a species, framework, and laboratory at once.

| Body | Mission |
|---|---|
| **Species** `vivy.exe` | V1 Operate: daily gateway |
| **Vivy Studio** | Independent application: development and distribution; V2 Explore happens here, not in the gateway |
| **A later-generation EXE** | V3 Rebuild only if there is evidence |

---

## 2. What Exactly Is DeepSeek Harness

### 2.1 Core Ideas

DSH is not another Claude Code clone. Underneath it is the paper *A Programming Paradigm for Spatiotemporal Composability* (Shi / Zhang / Cui, Peking University + DeepSeek):

- **Temporal composability:** when a component is unloaded, its side effects on the shared environment must be completely and orderly reversed. Every effect carries an inverse, recorded at runtime (`ctx.effect()`).
- **Spatial composability:** components declare dependencies (`inject`) and activate or deactivate as dependencies appear or disappear.
- Both compose into one `ctx`, called the context paradigm. The implementation is the vendor's **Cordis**.

Product slogan: **everything is a plugin**. Model adapters, the tool table, session logs, and **even the agent loop itself** are plugins. There is no privileged product kernel; the real kernel is Cordis (loading + accounting). Extension means attaching a plugin alongside the system, not patching the loop.

Several other engineering disciplines are equally important:

- **Model-visible ≡ logged.** Everything that enters a model request must be reconstructable from the session log.
- **Two event planes.** `session/event` is durable fact; `agent/*` is in-progress coordination.
- **Capability seam** = Service Definition + Provider + Consumer. Switching execution worlds (local / E2B) carries fs, shell, PTY, and LSP with it.
- **Assembly is profile over bundle over patch**, not a hard-coded startup order.

### 2.2 Innovations

1. **Lift** compile-time effect / coeffect **into runtime** (classic systems stop at lexical scope).
2. Turn a self-evolving harness into real tools: `cordis_inspect / define / run / stop / undefine`; the model can inspect and mount plugins it wrote itself.
3. Microkernel loop + waterfall extension points; changing the loop requires changing the architecture document.
4. The seam makes "switching worlds" composition, not a forked toolset.

What it **does not** provide (critical for a gateway): dynamic packages live only in memory and disappear on restart; the official documentation says it is not a security boundary and may affect other sessions in the same process; there is no across-generation fitness; and pre-release correctness takes priority over overnight availability.

### 2.3 Language Facts

No production language has made "reversible effect + reactive coeffect" into syntax.

- TypeScript merely satisfies the **minimum host conditions** (`Proxy`, declaration merging, disposable modules), which is why Cordis lives there.
- Koka / Effekt are ancestors at the type-system level, not runtime loading/unloading.
- Erlang is the production answer of "kill the process to unload."
- WASM is a sandboxed dynamic library where "discard the instance to unload."
- Go provides a fast single binary but **cannot unload** native code.

Therefore: the species uses Go; the lab may use TS (if its backend is DSH); capability plugins are Go source linked into the next-generation EXE only through the SDK. Do not force Go to become Cordis, and do not open another WASM/process plugin world.

### 2.4 What Resembles Today's Vivy

Both are local event-sourced agent harnesses: Journal / session log, ReAct + interruption, tool approval, Ask User, Plan Mode, hooks, parent-governed subagents, JSON-RPC, and a thin UI.

Vivy already has what it does not need to buy from DSH again: Skill, MCP, worker stdio, policy profiles, budgets, isolated workspaces, and recovery.

Their identities are opposite: Vivy is a personal gateway with a curated catalog and Eino isolated in `internal/runtime`; DSH is a plugin OS with an unloadable loop and community discovery.

---

## 3. How Much of DSH's Core Ideas Can Be Realized Under the New Model

Do not use one percentage. Look at the pillars.

| Pillar | Species EXE | Species + Studio (DSH) | Deliberately not added |
|---|---|---|---|
| Model-visible ≡ logged | 90–100% (requires upgrading ADR-009) | Same as left | — |
| Capability seam | 80–90% | Same as left | Runtime `inject` hot rebinding |
| Thin loop / event extensions | 80% | Same as left | In-process waterfall mesh |
| Temporal composability | 30–40% (generation change / kill candidate process) | ~100% in the lab | Fine-grained inverse-operation stack |
| Spatial composability | 35–45% (star topology through the gateway) | ~100% in the lab | Network of inter-plugin calls |
| Everything is a plugin | 30–40% (the product surface can change generation) | ~100% for the Studio engine | Journal / policy / species kernel |
| Modify itself while alive | 15–25% | ~90% in the lab | In-process mounting on a production instance |

- Species only: **about 35–45%**, with a theoretical ceiling of about **60%** (the kernel is never pluginized).
- The human-facing effect of a "self-evolving gateway": **about 70–80%**, because full-strength Cordis lives in the lab.
- Evolution that can accumulate overnight (air gap, EvalRun, promotion): **can exceed DSH out of the box**. The DSH demo disappears on restart.

Pushing Cordis reproduction to 80% = canceling the species/lab split. Do not do it.

---

## 4. Architecture: Species, Studio, and Data Plane

### 4.1 Species — The One a Human Double-Clicks

One installer, one shortcut, and one primary EXE for daily life.

It owns:

- Session / Run state machine
- Journal (product history; source of truth for the model-visible projection)
- Policy、Approval、Ask User
- Budget, cancellation, and recovery
- Secret resolution (env only; values never persist)
- JSON-RPC control plane and resident UI
- The capability list compiled into this generation (frozen at pack time)
- Read-only identity: `inspect` (hash, recipe, tool names, channel plugin names, and seams)

It does not own: pack, the evaluation farm, releases, the installation location, or the Studio main interface. Those belong to `VIVY-STUDIO.md`.

### 4.2 Kernel (Never a Plugin)

```text
Journal writes
Event vocabulary and sequence
Policy admission (deny / prompt / allow, immutable hash)
Secret resolution
Capability list compiled into this generation (frozen at pack time)
Read-only inspect implementation
Process supervision (worker only; no external plugins and no Studio)
ChannelHost (world ingress: admission, session mapping, channel.inbound, outbound; never pluginized)
FaceHost (selection and hosting of this generation's mouth: exactly one face per generation, face is a recipe organ; never pluginized)
```

V3 may replace the kernel later; that is **promotion to a new species generation**, not a hot unload.

### 4.3 Data Plane (Killable)

The control plane for daily life stays in the species process. Studio is another control plane in an independent application. The only short-lived process under the species is:

| Process | Role | Failure |
|---|---|---|
| `vivy worker` | Same-binary child run; tools remain in this EXE | Existing `worker_lost_after_restart` |

Candidate EXEs are started and killed by **Studio** and recorded in Studio's EvalRun; they are not children of the species. Studio itself is also an independent process: killing Studio does not affect an already-open daily gateway.

Do not write the species as kube-apiserver or Studio as a Pod. Studio is not the gateway's data plane.

Microservices would destroy the hot path, single Journal, one-click Windows experience, and the number of mutants that can be killed in a week. If the evaluation farm later needs multiple machines, it is **another system** behind the same object surface, not the personal gateway split into services.

### 4.4 Vivy Studio — Independent Application

For the complete shape, development-environment strategy, bootstrap, and slices, see **`VIVY-STUDIO.md`**. This section keeps only the boundary with the species.

Studio is an independent application, not a role on the species and not "another general-purpose agent." The product metaphor is an IDE for old industrial software: open it to build, burn, and install into the daily location. It is not a card that pops out of `vivy.exe`.

Responsibilities (authority in Studio): manage the project → develop → verify → `vivy-sdk pack` → **evaluate the candidate itself** → human release in Studio → install into the daily location / roll back. Read-only query of the live species uses `inspect`.

The first development engine may be a pinned DeepSeek Harness factory profile. The name the human sees is Vivy Studio. DSH is the engine; replace it and Studio remains.

Studio **must not**: write the production Journal, take resident API keys, or hot-replace a live kernel.  
Studio **must**: provide complete first-party daily development capability; other authorized tools may handle the source workspace directly using their own capabilities (NG-26).

There is no "open Studio" action. The species does not start Studio.

---

## 5. Lineage: Plugins, Generations, Forks, and New Species

| What changed | Name | Relationship |
|---|---|---|
| Skill text | **Behavior package** | Ideas can change within a generation; the compiled body does not |
| Capability source (tool / world / provider implementation) | **Plugin awaiting compilation** | Not a body yet; exists only after `pack` |
| New EXE from `vivy-sdk pack` | **Generation** | The only result of installing a plugin |
| Forked source with the contract intact | Still a Generation | The source is in someone else's git |
| UI skin, model-provider switch, remote MCP address | Configuration | Not a plugin |
| Journal semantics, event vocabulary, or species contract changed | **Another species** | Cannot be fairly compared in an EvalRun |

Rule:

> If it can still speak through the same ledger vocabulary and the same gates, it is a different generation of the same species.  
> It becomes a new species only when the gates or ledger change.

Lineage is recorded in `Generation.parent`. A branch is not a new species. Once the species splits, fitness disappears.

Plugin source is **not the species**. It becomes part of a generation's body only after being compiled into that generation's EXE. Removing a plugin = pack another body without it and switch to it at the next start.

---

## 6. Plugins Are Just "Source Packages"; Installing Means a New Version

Reject: WASM, `.dll`, Go `plugin`, loading an external stdio exe as a plugin, and treating MCP as the plugin system. MCP stdio is available only as an explicitly configured MCP dependency; it is not part of the plugin-loading model.
There is only one path for a capability to enter the world:

> **Source implementing the SDK contract → `vivy-sdk pack` → new `vivy.exe` → eval → promote.**  
> **Installing a plugin = building a new version.**

Code may be pure Go. Built-in units are assembled by their real names (loop / world / tool / provider); see `VIVY-ASSEMBLY.md`.  
**Only user-defined code under `plugins/` is called a plugin**; see `VIVY-PLUGIN-SPEC.md` for the specification.

### Kind A — Behavior (Still Text, Not Compiled)

`data/skills/**/SKILL.md`. Changes how the system thinks, not the compiled body. Not a plugin system.

### Kind B — Capability Source (Compile-Time Package)

**Go source packages** for tools / providers / tool-worlds / **channels**, implementing the `sdk/plugin` contract.  
Before being compiled into a generation by `pack`, they are only source on disk and invisible to the live process.
`seam: channel` is consumed by ChannelHost and does not enter the tool table (`VIVY-CHANNEL-PACK.md`).

### Kind C — Generation (The Only Loading Action)

The output of `pack`. Kind B source is linked into this EXE. There is no intermediate state of "build a plugin exe first and then attach it."

Remote MCP and explicitly configured local MCP stdio are only **configured dependencies** (like a provider endpoint), not plugins, and cannot replace Kind B. Stdio configuration is authorization: accept only a PATH executable name or absolute path, use the dangerous-basename denylist, and do not create a separate execute allowlist; environment values may be injected only through CHILD→HOST name references, and cwd may fall only within `runtime.workspace_root`.
Local stdio starts lazily; after the process dies, the next operation maps to the existing `error` state and does not restart automatically. Raw-frame limits and Windows child-process-tree cleanup are tracked separately in TODO.

---

## 7. Plugin Format (For the Compiler, Not the Loader)

The species runtime does not read the plugin directory. The manifest is a **pack recipe**:

```text
hello-fs/
  vivy-plugin.json      read at pack time
  README.md
  plugin.go             implements the sdk/plugin contract
```

```json
{
  "apiVersion": "vivy.plugin/v0",
  "kind": "capability",
  "name": "hello-fs",
  "version": "0.1.0",
  "seam": "tool-world",
  "module": ".",
  "grants": ["fs.read", "fs.write"],
  "tools": [
    {
      "name": "hello_stat",
      "effect": "read",
      "schema": {
        "type": "object",
        "properties": { "path": { "type": "string" } },
        "required": ["path"]
      }
    }
  ]
}
```

No `runtime`, no `entry` exe, and no wasm. `module` points to the Go package linked in by `pack`. Seams that attach `journal` / `policy` are rejected. `grants` freeze when compiled into the generation; runtime may execute only according to that generation's manifest and cannot widen them through configuration.

---

## 8. Loading = Packaging a New Version

There is no `plugins.allow` to launch an external process. There is no startup preflight followed by spawn.

```text
add / remove a plugin source
    → vivy-sdk verify
    → vivy-sdk pack          link into new vivy.exe, record source_ref + recipe + hash
    → register Generation
    → evaluate candidate (separate data directory)
    → human promote
    → switch body at next start
```

Remove a plugin: take that source out of the recipe and `pack` another version, again through eval / promote.  
A live EXE **does not** dynamically load any plugin code.

`execute` pointing at species source and compiling it directly is still ungated self-rewriting; the formal path permits only `vivy-sdk pack`.

### 8.4 Vivy SDK: Separate Binary, Same Source Tree

The building tool must be available, but it cannot live in the resident's gateway. `vivy-sdk` **is a separate binary**; its code lives under the repository root's `sdk/`, not in `cmd/` and not attached to `vivy.exe`.

It may grow large: later it may bundle a species-source snapshot, a Go distribution, or a complete toolchain. Its size and release train are separate from the daily gateway.

```text
vivy.exe              Species: daily gateway + worker
vivy-sdk.exe          Builder: verify / pack / inspect-artifact
                      Source tree: sdk/   (plugin/ for authors; internal/ for packer)
```

Residents receive only `vivy.exe`. Authors or Studio use the source tree (or a future SDK distribution) to compile again. Without species source or `go` (local or bundled with the SDK), `pack` must fail loudly.

**What It Packages**

| Command | Input | Output |
|---|---|---|
| `vivy-sdk verify` | Plugin source + `vivy-plugin.json` | Whether the contract is satisfied (seam, grants, schema, linkability) |
| `vivy-sdk pack` | Species source + plugin sources to link into this generation | **Only output:** a new `vivy.exe` + Generation manifest (with hashes and provenance) |
| `vivy-sdk inspect-artifact` | Generation EXE / manifest | Whether provenance is complete and it can be rebuilt |

There is no separate `build-plugin` output that produces an external exe. A plugin cannot be a loadable artifact by itself.

**Always Ready Does Not Mean the Resident Machine Can Compile**

- **Prepared:** the packer entry point always lives in this repository's `sdk/`; the contract and species share a module and do not fork into an external distribution package.
- **Compile in place:** happens only in Studio or on a development machine. Daily `vivy.exe` has no `sdk` subcommand.
- **Hot path:** chat, tools, and approvals **must not** call pack.

**Import Surface for Plugin Authors**

Only `agent-vivy/sdk/plugin` is exposed. Authors `import` it and implement the interfaces; `pack` **links** the package into a new `vivy.exe`. Do not create another git module (D-006). Eino is not part of the public surface.

**Relationship to Studio**

Studio (the human or its internal engine) modifies source → calls **`vivy-sdk pack`** → obtains a Generation → **Studio** evaluates / releases / installs it.  
DSH does not own "how Vivy is compiled"; replace DSH and the packer remains in `sdk/`. Authors do not open the SDK directly; Studio calls it.

---

## 9. Communication: Which Pipe

After a capability is compiled into the EXE, tool calls are **in-process functions** (still passing through policy / hook / Journal). There is no plugin-specific pipe.

The only remaining pipes are the ones that are "not plugins":

| Counterparty | Transport | Frames |
|---|---|---|
| `vivy worker` | Same-binary stdio | JSONL JSON-RPC (existing) |
| UI / local control plane | Loopback WebSocket | Same JSON-RPC (existing) |
| Studio → species | Read-only | `inspect` (the species does not call Studio back) |
| Remote MCP (if retained) | HTTP | Configured dependency, not a plugin |
| Local MCP (explicitly configured) | stdio | MCP JSON-RPC; configured dependency, not a worker/plugin channel |
| Kind A Skill | File read | — |

Reject opening another stdio/gRPC/WASM/dll channel for plugins. `vivy worker` is not a plugin channel; it is a same-binary child run.

---

## 10. Reject All Runtime Loading

| Form | Conclusion |
|---|---|
| WASM | **Reject.** Another runtime world, contrary to "installing a plugin = building a new version" |
| `.dll` / Go `plugin` | **Reject.** Cannot be unloaded; worse for Windows / cgo |
| External stdio exe (as a plugin) | **Reject.** An opaque binary with no source, or a second body |
| Explicitly configured MCP stdio command | **Allowed but restricted.** MCP dependency only; command, env references, and cwd all undergo configuration validation and runtime governance |
| Directory scan / plugin marketplace | **Reject.** |

When someone submits something, recognize only two forms:

```text
Skill text              → Kind A, not compiled
Go source + vivy-plugin.json → enters the recipe and waits for pack into a new EXE
Any other binary         → reject
```

---

## 11. The Species Keeps Only Read-Only Identity

"Seamless" is no longer three doors on the species. Development and distribution authority belongs to Studio (`VIVY-STUDIO.md` §4, §8). There is no need to link Node into `vivy.exe`, and Studio is not opened from `vivy.exe`.

### Species `inspect`

Read-only. Version, generation hash, seam, policy hash, and a tool-manifest summary.  
Reject: plaintext secrets, unrelated session bodies, and absolute paths that should not be exposed.  
This is Studio's seam for observing an installed / running body, not an evolution entry point.

### `eval` / `promote` Are Not on the Species

Studio starts candidates for evaluation. Release is an installation triggered by a human in Studio. Existing species-side `evals/start` and Promotion writes are treated as the wrong home, and their product semantics are frozen.

If ACP is approved later, it is only another face of the species' daily-life control plane, not a second evolution channel.

---

## 12. Control-Plane Objects

**Daily-life objects** (Run / Session / Approval) remain in the species Journal.  
**Generation objects** (Generation / EvalRun / Release / Install) live in Studio's own store. The species is no longer authoritative for these objects. See `VIVY-STUDIO.md` §8 for the complete schema.

The following Run shape still belongs to the species:

```text
kind: Run
spec:
  session: ses_...
  loop:    builtin | generation:<hash>
  world:   builtin | plugin:<hash>
  policy:  default | plan | read_only | full_auto
  budget:  { events, models, tools, retries }
status:
  phase: running | suspended | completed | failed | cancelled
  waiting: approval:... | question:...
  seq: 142

```

Generation objects (Generation / EvalRun / Release / Install) do not belong here. See `VIVY-STUDIO.md` §8.  
DSH's own session log is scratch paper; evolution visible to humans is recorded in the Studio ledger, not written to the production Journal.

---

## 13. Air Gap and Fitness

The live species and candidates must not share: SQLite / Journal files, workspace root, listen address, or production secrets.

They may share only the same EXE bytes for configuration-only mutants, the read-only evaluation suite, and the object schema.

Without a named suite, Studio is merely auto-installing plugins. The first suite should be small and local:

- V0/V1 vertical workflow (session → stream → tool → approval → recovery)
- One Plan Mode task
- One Skill or MCP task
- Restart recovery after a suspended approval

Compare classified failures, approval count, event count, and whether the Journal can still replay.  
"Closer to AGI" is not a suite ID. Adding a suite is a product decision.

The honest wording for the grand claim:

> If part of AGI is an intermediary-architecture problem, Vivy is an environment that can search for that architecture without losing measurement.  
> If AGI is almost entirely a model problem, Vivy is still a gateway that can change models without changing sovereignty.  
> The system should survive in both worlds.

---

## 14. Exact Meaning of Single EXE

| Is | Is not |
|---|---|
| One installer, one shortcut | A string of plugin processes attached to it |
| Daily life is that one `vivy.exe`, with all capabilities inside | Linking Node / DSH / WASM into the hot path |
| Kernel, L1, and this generation's capabilities compiled into one file | A microservice mesh |
| New capabilities appear only as a new EXE | People must open a lab in the gateway before they can live day to day |

Daily life remains a single-EXE product. Studio is a second application and is not embedded in `vivy.exe`.  
Embedding DSH in the main binary is what breaks single EXE.

---

## 15. Applied to Existing Code

Do not open a second runtime. Add seams.

| Existing | Next step |
|---|---|
| Species `inspect`, `vivy-sdk`, S1 projection | Keep as parts |
| Species-side Generation / eval / promote / Studio card | **Freeze product semantics** (wrong home; see NG-28) |
| Studio application (not yet present) | Start at ST-1 per `VIVY-STUDIO.md`: independent process, ledger, development venue |
| `internal/worker` | Remains a same-binary child run, not a plugin channel |
| Policy / Hooks / Plan Mode | Admission; philosophy unchanged |
| Skill revisions | Behavior package; capability changes go through pack |
| `mcp_servers` | Retain at most as remote-dependency configuration; do not upgrade it into a plugin |

Eino remains the built-in loop, with isolation unchanged. Only a candidate generation may replace the loop. Do not link Cordis into `vivy.exe`.

---

## 16. Invariants

1. During daily life, the human faces one gateway process at a time. During development, the human faces the independent Studio.
2. The kernel in §4.2 is not a plugin, and Studio cannot hot-modify it on a live instance.
3. Model-visible ≡ logged. Secrets are not logged. Generation-change facts are recorded in the Studio ledger.
4. Workers, plugins, and Studio cannot widen the species policy snapshot.
5. Capabilities are not unloaded at runtime. Removing a capability = pack another version + release it in Studio. Workers remain killable.
6. A new body takes effect only after the next start from the daily installation. Hot swapping is forbidden.
7. The production instance's workspace is not the Vivy source tree. The source tree is a Studio project.
8. If DSH is present, it is only an engine inside Studio; if absent, the installed species still starts.
9. Two generations are compared only through an EvalRun on a named suite. Studio is the evaluation parent.
10. One primary mission per stage. Building Studio cannot stop daily Operate; Studio is the first-party daily IDE, but it does not force other authorized tools to move their execution venue (NG-26).

---

## 17. Non-goals

- Rewriting the species in TypeScript or putting Node in the hot path
- Making DSH a required dependency of `vivy.exe`
- In-process self-modification of the live kernel
- Microservices, a service mesh, or local Kubernetes
- Multi-tenant or hosted Studio (D-016 remains valid unless a separate decision is made)
- Opening Studio from `vivy.exe` or making Studio a card in the gateway
- A plugin marketplace, directory scanning, or `dsh-plugin`-style discovery
- WASM, `.dll`, Go `plugin`, or external stdio plugin exes (the explicitly configured MCP dependency exception is not a plugin)
- Treating remote MCP as "installing a plugin"
- Binding Memory / Laputa / AutoDream into this architecture
- Treating "reaching AGI" as a sprint acceptance criterion

---

## 18. Staging (One Mission per Stage)

S0 is adopted. S1–S6 species-side parts are **done**. The S7 Studio card's **product meaning is void** (NG-28). S8 / S9 proceed after Studio's complete authoring capability is proven; development tools have no exclusive venue.

For workstation slices and development-environment strategy, see `VIVY-STUDIO.md` §3, §10 (ST-0..ST-8). ST-6 is the proof event for Studio's complete authoring capability.

---

## 19. Key Decisions

| ID | Decision | Rationale |
|---|---|---|
| NG-1 | Species and Studio are two bodies and two applications | The environment cannot also be the population |
| NG-2 | Species remains a single Go EXE | Personal gateway, Windows, existing kernel |
| NG-3 | Evolution is air-gapped and takes effect at the daily location on next start | Failed mutants must not take down daily life and the ledger |
| NG-4 | Species exposes read-only `inspect` only. Eval / release / install are Studio protocols | Corrected 2026-08-15: not three doors on the species |
| NG-5 | DSH is Studio's first internal engine, not the trust root or a species dependency | Do not marry a preview framework; the species does not embed Node |
| NG-6 | Behavior is text; capability is source awaiting compilation; the installed form is a Generation | Thought / source / new body |
| NG-7 | Learn DSH discipline, refuse DSH identity | Seam, log, thin loop; no in-process plugin OS |
| NG-8 | The species control plane remains the only daily-life write path; Studio is another application, not a Pod | No local mesh, and do not write Studio as the species data plane |
| NG-9 | Fitness is a named suite | Otherwise Studio is self-inflation |
| NG-10 | Model-visible ≡ logged; upgrade ADR-009 | Otherwise generations cannot be compared |
| NG-11 | Reject WASM, dll, Go plugin, and external stdio plugins. MCP stdio is only an explicitly configured dependency; capability plugins enter a new EXE only through `pack` | Installing a plugin = building a new version; do not create a second body |
| NG-12 | Do not scan directories; the recipe names source packages and artifacts are hashed | Curated catalog, not a marketplace |
| NG-13 | A fork with the same contract is a Generation, not a new species | Preserve lineage and evaluation |
| NG-14 | `vivy-sdk` is a separate binary, with source under the repository-root `sdk/`. Daily `vivy.exe` does not carry the SDK. Compilation is forbidden on the hot path. Studio invokes the SDK | The resident EXE cannot pretend it can compile |
| NG-15 | Installing, removing, or changing a plugin always builds a new version and goes through Studio eval / release | No runtime plugin surface |
| NG-16 | Vivy Studio is an independent application (industrial IDE): sealed engine + Skill + SDK/toolchain, distributed separately from the daily EXE | Authors open Studio; residents receive only the gateway |
| NG-17 | Capabilities are pure Go, but only user plugins use plugin governance | "I am developing a plugin" applies only to `plugins/` |
| NG-18 | Built-in units are named by what they are and assembled by recipe; the word plugin is reserved for the user layer | Learn DSH naming and stacking, not hot loading |
| NG-19 | Remote MCP is a configured dependency, not a plugin | Calling outward is not growing a limb |
| NG-20 | First make Studio a complete development IDE (DSH inside Studio), then reskin it; release is human-gated only | First-party development capability outranks species-side "doors" |
| NG-21..28 | See `VIVY-STUDIO.md` §11 | Independent application, ledger in Studio, non-exclusive development venue, frozen species card |

---

## 20. Confirmed

1. **2026-08-14 adopted as direction.** The species / kernel / assembly principles remain. The V0 ADR remains valid.
2. **Remote MCP remains a configured remote dependency.**
3. **2026-08-15: `vivy-sdk` split out of the daily EXE.** NG-14.
4. **Studio corrected on 2026-08-15:** independent application; owns the full development and distribution lifecycle; the species does not start it; the ledger is not in the species SQLite. Canonical source: `VIVY-STUDIO.md`.
5. **Development-environment strategy on 2026-08-23:** Studio is the first-party daily development IDE; other authorized tools that can read the workspace develop directly using their own capabilities and need not move into Studio (NG-26).
6. **Release can only be triggered by a human in Studio.** It is not `promote` on the species card.
7. **Product name:** "Vivy Studio" / "Studio" refers to the independent application; English `Studio`. It no longer refers to a card in the gateway.

---

## 21. Four-Sentence Contract

Follow `VIVY-STUDIO.md` §12:

- **Gateway:** the only daily body and the only writer of the Journal.
- **Studio:** the first-party daily development IDE and the authoritative application and installer for the distribution lifecycle.
- **Plugin:** source that satisfies the contract. It exists in the world only after the SDK compiles it into a generation's EXE.
- **Human:** the only releaser. Developers (including agents) may modify Vivy directly in Studio or another authorized tool.

If a design makes two of these sentences false at once, it is not this architecture.
