# Vivy Next Architecture — Gateway and Studio

> Status: **direction adopted** 2026-08-14. **Studio shape corrected
> 2026-08-15**: independent app; owns develop + distribute; development
> venue is Studio after bootstrap.
> Canonical Studio product text: `VIVY-STUDIO.md` (wins on Studio shape).
> Species / kernel / packing narrative: `SELF-EVOLVING-GATEWAY.md`.
> This file keeps the English decision table (NG-1..NG-28) and staging.
> Date: 2026-08-15
> Scope: next-generation product architecture (post-V1 Operate)
> Language of record: English (identifiers and contracts). Chinese names in
> parentheses are aliases, not a second spec.
>
> Related:
> - `../../AGENT-VIVY-DIRECTION.md` — staged missions V0–V3
> - `../../prd-agent-vivy-v0.md` — philosophy anchors §5.0, D-014..D-021
> - `../AGENT-VIVY-ARCHITECTURE-V0.md` — current assembled kernel
> - `../GOAL-AGENT-HARNESS-ROADMAP.md` — harness slices already built
> - `ACP-REMOTE-CONTROL-PROPOSAL.md` — unused control-plane draft; Studio
>   reuses its local-first and admission stance, not its remote scope
> - `.workspace/deepseek-harness/` — evidence source, not a dependency

---

## 1. Thesis

Daily Vivy is a **one-click single-EXE personal gateway**. It is the
living species: journal, approvals, secrets, resident UI, and the run.

**Vivy Studio is a second, independent application.** It owns the full
lifecycle of developing and distributing the next body. The first
coding engine inside Studio may be DeepSeek Harness. The engine is
replaceable. Studio is not a door on the gateway. The species never
launches Studio.

The resident faces the gateway. The author faces Studio. Mixing those
entries is a bug.

This split is the architecture. Everything else is a consequence.

```text
author
  │  independent app
  ▼
Vivy Studio                      DEVELOP + DISTRIBUTE
  worktree · sdk · eval · release · install
  first engine: sealed DSH profile (replaceable)
  own ledger; never the production Journal
  │
  ▼
daily install location
  │
  ▼
vivy.exe                         SPECIES — daily life
  Journal · policy · secrets · resident UI
  read-only inspect only
  does not start Studio
```

---

## 2. Why this shape

Three missions were being forced into one process identity:

| Mission | Success | Failure cost |
|---|---|---|
| Daily gateway | Usable today; recoverable; auditable | The human stops trusting the app |
| Framework | A seam can be replaced without rewriting the product | Protocol rot kills every mutant |
| Architecture search | A generation is comparable and killable | Months of rewrites with no fitness |

Those missions cannot share one release train or one address space. They
can share one **kernel**, if the kernel is the environment and the
framework is the population.

DeepSeek Harness proves a self-modifying runtime can exist. It does not
prove a self-modifying runtime is something a human should hand API keys
and life history to. Vivy's empty slot is the second proof: **a species
that can be evolved without ceasing to be a gateway.**

The V0 direction already said a stage must not simultaneously promise
stable product, complete framework replacement, and AGI-OS. This
document keeps that rule by giving each promise a body:

- V1 continues to operate the species.
- Studio is how V2 exploration is hosted.
- A later V3 kernel is a new species generation, not a live patch.

---

## 3. What is stolen from DeepSeek Harness

Steal discipline. Do not steal identity.

### 3.1 Take

| DSH idea | Vivy form |
|---|---|
| No privileged *product* kernel; the loop is thin | New behavior hangs on events, policy, prompt sections, and seams. Changing `engine.go` requires an architecture update. Eino remains behind the existing quarantine. |
| Model-visible ≡ logged | Anything that reaches a model request must be reconstructable from the Journal. New model-visible input requires a new event type. Compaction edits a projection, never the raw log. This upgrades ADR-009 (current feed is user/assistant text only). |
| Two event planes | Durable facts stay `RunEvent` / Journal. Live coordination (inbox, pre-step, steer, studio progress) must not be smuggled in as forged history. |
| Capability seam = Definition + Provider + Consumer | `provider`, `tool-world`, later `memory` / `loop`. Replacing a world provider moves fs+exec+related tools together. |
| Registrations are effects | A capability plugin's grant, tool schema, and process handle are tracked. Unload is defined (kill + classified terminal + revoke grant). |
| Waterfall interception | Policy, hooks, Plan Mode, compaction stay on the tool/request pipeline. They do not fork the loop. |
| Session log as genetic material | Journal, event vocabulary, and eval transcripts survive a generation change. A rewrite that orphans the log is not evolution. |
| Inspectable runtime | `inspect` is a first-class door. Studio reads a curated report, not private memory. |

### 3.2 Refuse

| DSH idea | Why refused |
|---|---|
| Everything is a plugin, including the loop and the log | The environment cannot be a member of the population. |
| In-process TypeScript / Cordis as Vivy's kernel | Wrong language for unload, wrong packaging for one-click Windows, wrong trust for secrets. |
| `tool-cordis` default (model mounts plugins in the live process) | DSH states this is not a security boundary and can affect other sessions. Unacceptable for a daily gateway. |
| Community plugin discovery | Conflicts with the curated-catalog anchor (PRD §5.0.3). Studio may author a plugin; the species does not scan a marketplace. |
| Hosted control plane / microservice mesh | Conflicts with personal-gateway and D-016. K8s is a metaphor for objects and admission, not a deployment target. |
| Go `plugin` / native `.dll` hot-load | Cannot unload cleanly; Windows is a first-class host. |

### 3.3 Language fact (constraint, not a preference)

No production language implements revertible effects plus reactive
coeffects as native syntax. TypeScript is the only host where Cordis
exists. Erlang and WASM give *killable* worlds. Go gives a fast
single-binary host and cannot retract native code.

Therefore:

- **Species host language:** Go (unchanged).
- **Studio guest language:** whatever the Studio backend already is
  (TypeScript if the backend is DSH; later a thinner Go worker).
- **Capability plugins:** out-of-process binaries or WASM instances.
- **Never:** in-process native modules as the plugin mechanism.

---

## 4. Bodies

### 4.1 Species — `vivy.exe`

The thing the human double-clicks. One installer, one shortcut.

Owns:

- Session / Run state machine
- Journal (product history and the source of model-visible projection)
- Policy, Approval, Ask User
- Budget, cancel, recover
- Secret resolution (env only; values never persist)
- JSON-RPC control plane and the UI shell
- Plugin allowlist, grants, and process supervision
- Read-only `inspect` (identity only)

Does not own: the next generation's source edits, DSH's internal
session log, or any plugin marketplace.

### 4.2 Kernel inside the species (never a plugin)

These objects are not replaceable at runtime and are not writable by
Studio against a live instance:

```text
Journal writer
Event vocabulary and sequence
Policy admission (deny / prompt / allow, immutable hash)
Secret resolver
Grant table and allowlist
inspect implementation (read-only identity)
Process supervisor for `vivy worker` only
```

A future V3 may rebuild this kernel as a new generation. That is
Promotion of a new species, not a plugin unload.

### 4.3 Data plane (replaceable, killable)

Short-lived processes the kernel supervises:

| Process | Role | Fate on failure |
|---|---|---|
| Built-in worker (`vivy worker`) | Child run; tools still brokered by parent | `worker_lost_after_restart` (already shipped) |
| Capability plugin | `provider` or `tool-world` | Kill; one classified tool/run failure |
| Candidate species (`vivy.exe'`) | Eval only — **parent is Studio, not the live species** | Kill; Studio EvalRun records the death |

Control plane stays in-process in the species. Data plane may be many
processes on one machine. This is the K8s shape without a cluster:
the species is `kube-apiserver` + etcd in one process; plugins and
candidates are pods; CRI-style binaries are local, not HTTP services.

### 4.4 Studio — the independent application

Canonical text: `VIVY-STUDIO.md`.

Studio is a **product**, not a role on the species and not a card in
the gateway UI. It owns develop + distribute. DeepSeek Harness is an
engine *inside* Studio, replaceable.

Responsibilities (authority in Studio):

1. Open a Vivy worktree (never production `data/`).
2. Develop (coding agent), verify (`just ci`, `vivy-sdk verify`).
3. Pack (`vivy-sdk pack`).
4. **Spawn the candidate itself** and record `EvalRun` in Studio's store.
5. Human-gated **Release** + **Install** into the daily location.
6. Read-only `inspect` of an installed or running species.

Studio does not write the production Journal, hold resident API keys,
or hot-swap a live kernel. If Studio is down, the installed `vivy.exe`
still opens. If Studio is down, **development stops** (restore-only
exception in `VIVY-STUDIO.md` §2.2).

The species never launches Studio. There is no “open Studio” door.

---

## 5. Objects the control plane speaks

Daily-life objects stay species-owned (Run / Session / Approval).
Generation / EvalRun / Release / Install are **Studio-owned**. The
species-side tables from ADR-011..016 are the wrong home; freeze their
product meaning. Full Studio schema: `VIVY-STUDIO.md` §8.

```text
kind: Run
spec:
  session: ses_...
  loop:    builtin | generation:<hash>
  world:   builtin | plugin:<hash>
  policy:  default | plan | read_only | full_auto
  budget:  { events, models, tools, retries }
status:
  phase:   running | suspended | completed | failed | cancelled
  waiting: approval:... | question:...
  seq:     142
```

Studio's lifecycle objects (product history, not DSH scratch; stored
in Studio, not the species SQLite):

```text
kind: Generation
spec:
  parent:     gen_<current>
  artifact:   sha256:...          # exe and/or plugin bundle
  seams:      [loop?, world?, ...]
  source_ref: git:... | worktree:...
status:
  phase: built | eval_pending | evaluated | released | rejected

kind: EvalRun
spec:
  candidate:  gen_...
  baseline:   gen_...             # usually the live species
  suite:      <frozen task set id>
status:
  verdict:    better | worse | mixed | failed_to_run
  journal_ref: eval_...           # candidate journal, not production
  notes:      bounded summary

kind: Release
spec:
  generation: gen_...
  eval:       evl_...
  actor:      human
status:
  phase: accepted

kind: Install
spec:
  release: rel_...
  target:  <daily install location>
status:
  phase: current | rolled_back
```

A generation change that cannot be expressed as these Studio objects
did not happen. Species-side Promotion rows are not authoritative.

---

## 6. Seams between the two apps

The species does not host an evolution protocol. Studio does.

### 6.1 Species `inspect` (read-only)

Identity only: version, generation hash, seams, policy hash, tool
digest. Refuses secrets, session bodies, and host paths.

### 6.2 Studio `eval`

Studio spawns the candidate with a separate data directory. The live
species is not the parent and does not record the EvalRun.

### 6.3 Studio `release` + `install`

Human-gated in Studio. Effect: files in the daily install location
change. The next double-click runs the new body. No live-process
hot-swap.

ACP, if later approved, is another face of the *resident* control
plane — not a second evolution channel.

---

## 7. Plugins the species may load

Three kinds. Mixing their powers is a bug.

### 7.1 Kind A — Behavior (already shipped)

Skills under `skills_root`. Markdown. Untrusted as data. `skill_manage`
already uses a durable revision (`pending → applied | rejected`) with
HITL. This remains the way the species evolves *how it thinks* without
evolving its body.

Production workspace is not the Vivy source tree. Kind A must not
become an ungoverned path to rewrite the EXE via `execute`.

### 7.2 Kind B — Capability (MCP today, signed process next)

Adds a tool, a provider, or a tool-world. Shape:

1. Manifest: name, seam, required grants, artifact hash, protocol
   version.
2. Allowlist in config (no directory auto-load).
3. Transport: current MCP HTTP, or stdio JSON-RPC extending the worker
   protocol. WASM is allowed later for pure compute. Go `plugin` is
   not.
4. Every invocation returns to the parent broker: schema, safety,
   policy, hooks, redaction, Journal.
5. Unload = kill + revoke grant + one classified event.

First seams allowed to leave the binary: `provider`, `tool-world`
(filesystem and subprocess stay one world). Loop, Journal, policy, UI
are not Kind B.

### 7.3 Kind C — Generation (the only self-rewrite)

Kind C is a **Generation artifact**, not a plugin and not “Studio
itself”. Studio (the independent app) is the only author of Kind C.
Its legitimate outputs are a Generation, an EvalRun, a Release, and
an Install. See `VIVY-STUDIO.md`.

Studio must not:

- patch the live process
- `append` the production Journal
- widen policy
- read production secrets

Lifecycle: `built → evaluated → released | rejected`, then `install`
into the daily location. Applied means “next start of that install,”
not “this address space.”

`execute` / `commandline` on a production species pointing at Vivy
source is the ungoverned ancestor. After Studio exists, that path is
denied on a production instance (S8, implemented inside Studio).

---

## 8. Air gap

Live species and candidate species never share:

- SQLite path / Journal file
- workspace root
- listen address
- secret material (eval credentials are a separate, optional env)

They may share:

- the same executable *bytes* when the candidate is a config-only
  mutant
- the frozen eval suite (read-only)
- the object schema (`Run` on the species; Generation / EvalRun /
  Release / Install on Studio)

Studio's own transcript (DSH session log, editor buffer, model CoT)
is laboratory scratch. Evolution facts land in the **Studio** ledger.
They do not append the production Journal. “I cannot see why I became
this” is answered by opening Studio, not by mining the resident log.

---

## 9. Fitness (without it, Studio is auto-install)

A generation is better only against a named suite. The first suite is
small and local:

- the V0/V1 vertical workflow (session → stream → tool → approval →
  recover)
- a bounded Plan Mode task
- a bounded Skill/MCP task
- restart recovery of a suspended approval

Verdicts compare classified failures, approval count, event count, and
whether the Journal still replays. “Closer to AGI” is not a suite id.

Studio may propose suite extensions. Adding a suite is a product
decision, not a live plugin.

---

## 10. Mapping onto code that already exists

Do not grow the species-side studio surface. Build the Studio app.

| Existing | Next use |
|---|---|
| Species `inspect`, `vivy-sdk`, S1 projection | Keep as parts |
| Species Generation / eval / promote / Studio card | **Frozen product meaning** (wrong home, NG-28) |
| Studio application (does not exist yet) | `VIVY-STUDIO.md` ST-1 onward |
| `internal/worker` | Same-binary child run, not a plugin channel |
| Policy / Hooks / Plan Mode | Admission, unchanged philosophy |

Eino stays the built-in loop provider, quarantined. A later Kind B/C
`loop` seam may replace it in a *candidate*, never by linking Cordis
into `vivy.exe`.

---

## 11. Non-goals

- Rewriting Vivy in TypeScript or embedding Node in the hot path.
- Making DSH a required runtime dependency of `vivy.exe`.
- Opening Studio from `vivy.exe`, or treating the embedded Studio card
  as the product.
- In-process self-modification of the live kernel.
- Microservices, service mesh, or a local Kubernetes.
- Multi-tenant or hosted Studio (D-016 stands until explicitly
  revisited).
- Plugin marketplace or `dsh-plugin`-style discovery.
- Memory / Laputa / AutoDream as part of this architecture.
- Treating “reached AGI” as a sprint acceptance criterion.

---

## 12. Invariants

1. Daily life is one gateway process at a time. Development is the
   Studio application.
2. The kernel listed in §4.2 is not a plugin and is not hot-patched
   by Studio on a live instance.
3. Model-visible ≡ logged. Secrets are not logged. Evolution facts
   live in the Studio ledger.
4. Policy snapshots cannot be widened by workers, plugins, or Studio.
5. Capabilities are not unloaded at runtime. Remove = pack + Studio
   release. Workers remain killable.
6. A new body applies at the next start of the daily install.
   Mid-session kernel swap does not exist.
7. Production workspace is not the Vivy source tree. The source tree
   is a Studio project.
8. DSH, if present, is an engine inside Studio. If it is absent, the
   installed species still starts.
9. Two generations are comparable only through a Studio `EvalRun`
   against a named suite.
10. After the ST-6 venue switch (`VIVY-STUDIO.md` §3), all further
    Vivy development happens in Studio (NG-26).

---

## 13. Staging (one mission per slice)

S0 adopted. S1–S6 species-side parts are done. S7 card product
meaning is void (NG-28). S8/S9 move after the Studio venue switch
and are implemented *inside* Studio.

Studio slices ST-0..ST-8, bootstrap exception, and the ST-6 switch
gate live in `VIVY-STUDIO.md` §3 and §10. Do not add species-side
studio work during bootstrap.

---

## 14. Key decisions

| ID | Decision | Rationale |
|---|---|---|
| NG-1 | Species and Studio are different bodies and different apps | One process cannot be both the environment and the population. |
| NG-2 | Species remains a single Go EXE | Personal gateway, Windows one-click, existing V0/V1 kernel. |
| NG-3 | Evolution is air-gapped; the daily install changes at next launch | Failed mutants must not take down daily life or the log. |
| NG-4 | Species exposes read-only `inspect` only. Eval / release / install are Studio protocols | Corrected 2026-08-15. |
| NG-5 | DSH is the first engine *inside* Studio, not a species dependency | Do not marry a preview framework; do not embed Node in `vivy.exe`. |
| NG-6 | Behavior is text; capability is source; installed form is a Generation | Think / source / new body. |
| NG-7 | Steal DSH discipline, refuse DSH identity | Seams, log invariant, thin loop. No in-process plugin OS. |
| NG-8 | Species keeps the unique daily-life write path. Studio is another app, not a pod | No mesh; do not describe Studio as species data plane. |
| NG-9 | Fitness is a named suite, not a slogan | Without EvalRun, Studio is self-inflation. |
| NG-10 | Model-visible ≡ logged becomes an invariant, upgrading ADR-009 | Otherwise generations cannot be compared or replayed. |
| NG-11 | Reject WASM, dll, Go plugin, and stdio drop-in plugin binaries. Capabilities enter only by `vivy-sdk pack` into a new EXE | Installing a plugin is cutting a new version |
| NG-12 | No directory scan; the pack recipe names source packages; artifacts are hashed | Curated catalog, not a marketplace |
| NG-13 | A source fork that keeps the contract is a Generation, not a new species | Preserves the log as genetic material |
| NG-14 | `vivy-sdk` is a separate binary under repo-root `sdk/`. Daily `vivy.exe` has no sdk face. Studio invokes the sdk | Packing needs source; the resident install stays small. |
| NG-15 | Adding, removing, or changing a capability always produces a new Generation and goes through Studio eval / release | No runtime plugin surface |
| NG-16 | Vivy Studio is an independent industrial IDE, shipped separately from the daily EXE | Authors open Studio; residents get one gateway. |
| NG-17 | User capabilities are plain Go under plugin governance; first-party units are not called plugins | “I am developing a plugin” applies only to `plugins/` |
| NG-18 | First-party units are named by what they are and assembled by recipe | Learn DSH naming and stacking, not live unload |
| NG-19 | Remote MCP is a configured dependency, not a plugin | Calling out is not growing a limb |
| NG-20 | Make Studio a usable development venue first (DSH inside Studio); skin later; release is human-only | Venue hard-constraint outranks species-side “doors” |
| NG-21 | Studio owns the develop + distribute lifecycle. `vivy.exe` is the product, not the host | User correction 2026-08-15 |
| NG-22 | The species never launches Studio. No “open Studio” entry | Inverts the previous door metaphor |
| NG-23 | Generation / EvalRun / Release / Install authority lives in Studio | Species tables are the wrong home |
| NG-24 | Studio parents eval candidates. The live species does not | Air gap |
| NG-25 | Release is a human-triggered install in Studio. No auto-release, no live hot-swap | Human remains the promoter |
| NG-26 | **Development venue is Studio.** Bootstrap (ST-1..ST-4) is the only outside work; ST-6 closes the outside loop | Otherwise Studio never becomes real |
| NG-27 | Studio down does not block an installed species. Species down is not a reason to develop outside Studio | Two trust roots for two jobs |
| NG-28 | Freeze species-side Studio card and Promote authority | Stop extending the wrong home |
| NG-29 | Chrome is Vivy Studio: title, wordmark, onboarding, profile id. First-party complete token theme, not a community skin or pasted CSS | DSH hard-codes whale + “DeepSeek Harness”; community themes recolor only |

---

## 15. Closed on 2026-08-15 (were open)

1. Adopted as direction (2026-08-14), Studio shape corrected
   (2026-08-15). Canonical: `VIVY-STUDIO.md`.
2. First Studio engine is pinned upstream DSH
   (`.workspace/deepseek-harness/upstream`). Not a species dependency.
3. Release is always human in Studio. No auto-release until a later
   decision.
4. Lifecycle objects live in Studio's store, not the species SQLite.
5. Product name: 「Vivy Studio」 / 「工作室」 means the independent
   app. English `Studio` in logs. It does not mean a gateway card.

Remaining implementation choices (install path layout, first-period
shell = DSH Web vs custom) are recorded as defaults in
`VIVY-STUDIO.md` and do not block ST-1.

---

## 16. One-sentence contracts

- **Gateway:** the only daily body and the only writer of the Journal.
- **Studio:** the only develop+distribute application; the only author
  and installer of the next body.
- **Plugin:** source that exists in the world only after `pack` into a
  Generation.
- **Human:** the only releaser. After the venue switch, developers
  (including agents) change Vivy only inside Studio.

If a design needs two of those sentences to be false at once, it is
not this architecture.
