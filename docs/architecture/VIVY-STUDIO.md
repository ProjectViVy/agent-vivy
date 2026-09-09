# Vivy Studio

> Status: **direction adopted** (user correction and confirmation on 2026-08-15; development-environment strategy revised on 2026-08-23).
> This document is the canonical source for Studio product identity, lifecycle, and development-environment strategy.
> If this document conflicts with `SELF-EVOLVING-GATEWAY.md` / `VIVY-GATEWAY-AND-STUDIO.md` on Studio shape, this document wins and those two must be updated.
> Date: 2026-08-23
>
> Related:
> - `SELF-EVOLVING-GATEWAY.md` — species / kernel / assembly / no hot loading
> - `VIVY-WORLDVIEW.md` — structural correspondence between product philosophy and worldview (does not replace development-environment strategy)
> - `VIVY-GATEWAY-AND-STUDIO.md` — English decision IDs (NG-*)
> - `VIVY-ASSEMBLY.md` / `VIVY-PLUGIN-SPEC.md` — recipes and user plugins
> - `.workspace/deepseek-harness/upstream` — evidence source for the first development engine (official clone, `47f9438`)

---

## 0. One Sentence

**Vivy Studio is an independent application.** It manages Vivy's full development and distribution lifecycle.  
**Daily `vivy.exe` is its product, not its host.**  
There is no "open Studio from the gateway." The species does not start Studio.

People have two entry points; their purposes must not be mixed:

```text
author / development
  open Vivy Studio.exe
    project → develop → verify → pack → evaluate → release → install / rollback

resident / daily life
  open vivy.exe
    sessions, approvals, Journal, tools
    does not ask Studio, compile binaries, or promote others
```

The first development engine may be pinned DeepSeek Harness. The name the human sees is Vivy Studio, not DeepSeek. Replace the engine and Studio remains.

---

## 1. Why It Must Be an Independent Application

The species must last overnight: keys stay local, the Journal is replayable, and people dare to entrust their lives to it.  
Studio builds during the day: edit source, run tests, compile EXEs, evaluate them, and install the new body into the daily location.

These two things cannot share:

- the same process;
- the same SQLite database;
- the same set of production secrets;
- the same "open the lab from the chat box" entry point.

The old draft described Studio as a role on the species, a client of the three doors, and a card in the gateway. That is the wrong home. `internal/studio`, `evals/start` / `promotions` on species RPC, and the embedded Studio card all put the ledger in the product process. This document freezes their product meaning and adds no more functionality.

---

## 2. Development Environment — Vivy Feature Development Recommends Frontend/Backend Separation

**The recommended inner loop for Vivy feature development is two frontend/backend processes: the backend runs
`vivy.exe`, and the frontend runs Vite. Studio is the first-party Studio / release-lifecycle product,
but it is not a mandatory entry point for Vivy feature development.**

Recommended startup:

```text
terminal 1: just run
terminal 2: cd ui; pnpm dev
```

The backend provides a JSON-RPC control plane at `127.0.0.1:8787`; Vite provides the browser UI at
`127.0.0.1:3015` and proxies `/rpc`. This lets Go and the UI hot-reload independently; the embedded UI
is reserved for the default release shape and `just ci` verification, while `just build-split`
packages the headless backend and standalone static UI.

"Development" includes modifying the species, Studio itself, Skills / recipes / plugins, and architecture documents and tests that serve as product contracts. Any development tool or coding agent authorized to read the current workspace should use its own native editing, testing, and automation capabilities directly; it need not hand work off or reproduce it in Vivy Studio merely to satisfy a venue rule.

Whether work is performed by Studio or another authorized tool, workspace boundaries, product contracts, air gap, and verification commands are exactly the same. Daily `vivy.exe` remains the resident product, not an IDE.

### 2.1 Prohibited

- Develop Vivy in a daily `vivy.exe` session (the species is a resident product, not an IDE)
- Continue adding generation UI, Promote authority, or an evaluation farm to the species process on the grounds that "Studio is not ready yet"
- Require another authorized tool to stop work, hand off the task, or reimplement it in Studio because Studio is the recommended author path
- Bypass the current tool and repository's existing permissions, security boundaries, air gap, or verification commands

### 2.2 Allowed Development Methods

- Daily developers use the complete toolchain, including `pwsh`, `git`, `just`, and `go`, in the independent backend + Vite frontend processes
- Work in Vivy Studio when Studio's own UI, Studio lifecycle, or release/install/rollback is needed
- Cursor, Claude Code, a local coding agent, or another authorized tool may read and modify the current workspace directly, using its own native capabilities for implementation and verification
- Residents continue to use `vivy.exe` for daily life
- Perform operating-system-level installation, process termination, and log inspection

### 2.3 Why Studio Remains First-Party

Studio must have complete capabilities for its own development, evaluation, release, and installation to be
a reliable first-party lifecycle product; ST-6 has already proved this. This capability proof does not restrict
other authorized tools from working directly in the same workspace, nor does it require Vivy feature implementation to move into Studio.

---

## 3. Bootstrap (The Only One-Time Outer Loop)

When Studio does not yet exist, it cannot be created inside Studio. The following work is allowed, and only this work is allowed, outside Studio:

| Allowed | Must not be put into bootstrap |
|---|---|
| Correct this document and related documents (this time) | New species features, tools, or providers |
| Bring up the ST-1 independent process | Continue building a Studio card in the species |
| Pin the ST-2 project to the source tree without touching the production Journal | Continue expanding the Generation ledger in species SQLite |
| ST-3 toolchain: Go / gopls / just / `vivy-sdk` | Reskinning the distribution or building a plugin marketplace |
| ST-4 prefabricated Skills (five plugin steps + `just ci` for the body) | Automatic promote |
| Reach ST-6 and land one real body change **inside Studio** | Any species feature done "while we're at it" |

**Capability gate (ST-6):**

1. Vivy Studio starts as an independent process without going through `vivy.exe`.
2. The opened project is the `agent-vivy` source tree (or a worktree cut from it), not `data/`.
3. Studio can modify one body area (`internal/` / `cmd/` / `ui/` / product documentation) and complete `just ci`.
4. This change proves Studio can independently carry a real author path.

After the gate is met: Studio retains its role as the first-party Studio / lifecycle workbench; ST-5 (Studio's own pack / evaluation), ST-7 (release / install), and ST-8 (rollback) remain under Studio's lifecycle authority. Other authorized development tools may implement and verify Vivy features directly in the source workspace using the frontend/backend-separated inner loop.

There is no second bootstrap. If Studio cannot start for an extended period, use the §2.2 emergency exception and return immediately afterward.

---

## 4. Lifecycle Owned by Studio

Authority belongs to Studio. The species does not host these phases.

| Phase | What Studio does | Object |
|---|---|---|
| Project | Open / manage the source or plugin tree | Worktree |
| Development | Built-in coding agent modifies code (first DSH) | Worktree diff |
| Verification | `just ci`, `vivy-sdk verify` | Check |
| Build body | `vivy-sdk pack` | Generation (artifact + manifest) |
| Evaluation | **Studio itself** starts the candidate EXE in a separate data directory | EvalRun (Studio store) |
| Comparison | Difference between generations, suite conclusions | Report |
| Release | Human clicks release in Studio | Release |
| Distribution | Write the EXE to the daily installation location or export the artifact | Install |
| Rollback | Reinstall the previous released body | Still an Install |
| Observation | Read-only query of an installed or running species | Species `inspect` snapshot |

Evaluation is not "ask the live `vivy.exe` to run `evals/start`." Studio is the candidate's parent.  
Release is not writing an `applies_at: next_launch` row in the species store. Release is Studio completing an installation: the `vivy.exe` in the daily location is replaced with a new file. The body changes on the next double-click. Hot replacement of a live process is forbidden.

The human gate remains: the Studio process must not release automatically. A human clicks release. Automatic release requires a separate future decision.

---

## 5. What the Two Apps Hold

### 5.1 Vivy Studio (Authority: Development and Distribution)

- Source and worktrees
- Development sessions (DSH is only the engine here)
- Toolchain: Go, gopls, just, `vivy-sdk`
- Its own ledger: Worktree / Generation / EvalRun / Release / Install
- Layout and rollback points for the daily installation location
- Evaluation secrets and suites (isolated from resident secrets)

### 5.2 `vivy.exe` (Authority: Daily Life)

- Sessions, Journal, approvals, Ask User, budget, recovery
- Read-only identity: binary hash, current recipe, tool names (`inspect`)
- Resident model keys
- Same-binary `vivy worker`

**Must not enter the species:** pack, evaluation farm, release, installing another body, the Studio main interface, or Vivy source development.

If the species is down, the resident cannot get through the day. If Studio is down, the resident should still be able to open the installed `vivy.exe`.  
If Studio is down, **development stops** (except under §2.2). The species is not a backup development venue.

### 5.3 Keep Only Narrow Seams Between Them

| Direction | Seam | Not |
|---|---|---|
| Studio → installed / running species | Read-only `inspect` | Write Journal, change policy |
| Studio → daily installation location | Write files, start/stop processes | Hot-patch the live kernel |
| Species → Studio | **None** | Open Studio, call back promote |

`vivy-sdk` remains a separate binary under the repository's `sdk/`. It is the building tool in the Studio distribution, not a `vivy.exe` subcommand and not a third product for people to open every day. Authors face Studio; Studio calls the SDK.

---

## 6. Development Still Has Three Tiers, All in Studio

| Tier | What the human is doing | Working directory | Success |
|---|---|---|---|
| A. Build a plugin | Add a capability for oneself | Open only `plugins/<name>/` | `verify` → `pack` → `eval` |
| B. Change a built-in | Change notes / fs / provider / recipe | `tools/`, `providers/`, `vivy.generation.yml` | Pack another version |
| C. Change the body | Change Journal, approvals, runtime, UI, Studio | `internal/`, `cmd/`, `ui/`, Studio itself | `just ci`; pack again if the body must change |

The hard condition "Studio can develop Vivy itself" means C, not just A.  
The worktree points to the full `agent-vivy` tree from the start. Skills route by tier; the engine must be able to see the entire repository.

---

## 7. DSH's Role Here

DeepSeek Harness is the **development engine inside the Studio application**, not the species backend or product name.

Take: `dsh-base` + `dsh-web-app`, along with fs / pwsh / grep / lsp / skill / workflow / subagent / plan / approvals. On Windows, use pwsh. `lsp` connects to `gopls`.

Do not take: the default `tool-cordis` (hot-attached to a live process, disappears on restart, and officially declared not to be a security boundary); the plugin marketplace; or treating DSH session logs as Vivy Journal or the Studio ledger.

The first shell can simply be a DSH Web build pinned to a commit (`.workspace/deepseek-harness/upstream`, currently `47f9438`), but the profile name, window name, project, and Skill must be Vivy Studio. Do not launch it from `vivy.exe`. Apply the re-skin afterward.

---

## 8. The Ledger Lives in Studio

The following objects are Studio's product history, stored in Studio's own database, not side tables in the species SQLite database.

```text
kind: Worktree
spec:
  path: <canonical source tree>
  kind: plugin | first-party | kernel
status:
  dirty: bool

kind: Generation
spec:
  parent:     gen_...
  artifact:   sha256:...
  recipe:     vivy.generation.yml
  source_ref: git:... | worktree:...
status:
  phase: built | eval_pending | evaluated | released | rejected

kind: EvalRun
spec:
  candidate:  gen_...
  baseline:   gen_...
  suite:      <frozen id>
status:
  verdict:    better | worse | mixed | failed_to_run
  journal_ref: <candidate dir, never production>

kind: Release
spec:
  generation: gen_...
  eval:       evl_...
  actor:      human
status:
  phase: accepted
  # A release is not the same as replacing a live process

kind: Install
spec:
  release: rel_...
  target:  <daily install location>
status:
  phase: current | rolled_back
```

The Generation / EvalRun / Promotion tables already implemented on the species side (ADR-011..016) are considered the **wrong home**: keep the code until Studio's ledger can replace them, but product authority has moved. Do not expand the product semantics of these APIs on the species side.

If these objects cannot express “evolution,” Studio treats it as never having happened. DSH session logs are scratch paper.

---

## 9. Daily Install Location

There is one Studio-managed directory on the local machine, separate from the repository and from `agent-vivy/data/`.

Release / distribution = place `vivy.exe` (and the read-only resources it needs) here.  
The resident shortcut points here.  
Rollback = restore the previous Release's files here.  
The running process continues using the old mapping until it exits.

---

## 9.1 Product Identity and Theme (Hard Conditions)

Anyone opening the window must immediately know this is **Vivy Studio**, not DeepSeek Harness or the daily `vivy.exe`.

Community themes mostly change colors, not the product name. The official identity is hard-coded in three places: the sidebar `BrandWordmark` (whale + `HARNESS` badge SVG), `document.title` (`DeepSeek Harness`), and the settings welcome copy (the DSH plugin ecosystem). The theme system recognizes only `--dsw-alias-*` tokens, and third-party registration **does not validate complete coverage**. Therefore, identity and color scheme must be specified as separate conditions.

### What Others Have Already Built (Evidence Only; Do Not Ship in the Product)

| Item | What it actually does | Implication for us |
|---|---|---|
| Official `ui-theme` | `light` / `dark` / `system`; `ThemeRuntime.register()` can layer an alias | A legitimate seam for recoloring |
| [dsh-theme-lab](https://github.com/Ultronen/dsh-theme-lab) | Uses official token overrides: translucent full shell, blur, wallpaper | Learn from its use of the official layer; do not turn the workstation into a glass desktop |
| Aurora Nexus from [dsh-custom-css](https://github.com/AnacondaKC/dsh-custom-css) | One CSS file covers all `--dsw-alias-*` tokens, typography, shadows, scrollbars, and Shiki; supports safe uninstall with `?off=` | **A template for a complete theme checklist**; do not rely on users pasting CSS |
| `dsh-qq2006` / `dsh-deep-whale` / maid-atelier / whale-girl | Full skins, desktop pets, whale-girl characters | Identity conflict; some non-commercial licenses; forbidden as Studio's default face |
| Desktop shells (`harness-desktop`, etc.) | Window controls follow the DSH page theme | A future in-house shell should follow the Studio theme, not the whale |

In community directories, “Themes” are mostly skin hubs and anime-style themes. Studio is an industrial IDE; it does not follow that path.

### Identity (Required; Part of ST-1 Acceptance)

After opening, the following locations must not show DeepSeek / the whale logo / the `HARNESS` badge as the product identity:

1. Window title and taskbar: `Vivy Studio` (the session title may be prefixed: `Session — Vivy Studio`)
2. Sidebar wordmark: an in-house wordmark, not the whale SVG from `BrandWordmark`
3. Settings / welcome / onboarding: Studio copy (development and distribution), not the DSH plugin-ecosystem welcome message
4. Profile name: `vivy-studio` (or `vivy-factory`), visible in `dsh --dump-config`
5. Process / shortcut display name: Vivy Studio

It is permitted to state in small text on the “About” or engine page that the development engine comes from DeepSeek Harness (replaceable). That is provenance, not the product name.

### Theme (Required; Delivered in the Same Slice as Identity)

1. **First-party sealed**, bundled into the Studio profile; not a skin from the `dsh plugin add` marketplace.
2. Use the official `ThemeRuntime.register()` + complete `--dsw-alias-*` set (including scrollbars, shadows, and Shiki). If one piece is missing, it is still DSH color; count it as incomplete. Aurora Nexus's coverage is an acceptance reference; do not copy its files.
3. Provide both **light and dark**, with `system` as the default.
4. Industrial IDE: high contrast and suitable for long coding sessions. No default wallpaper, liquid glass, desktop pets, QQ skins, or whale-girl characters.
5. Accessibility: `prefers-reduced-motion` and readable contrast. Do not load remote fonts or images.
6. Known official gap: third-party themes do not validate completeness. We will maintain our own token checklist and fail when something is missing; do not rely on visual inspection.
7. Do not treat `dsh-custom-css` as a product path: arbitrary CSS enters settings and is shared by all browsers, so the trust model is wrong.

Exact color values can be decided later; they do not block ST-1 work. Before ST-1 closes, all five identity checks must be green.

Current first-party skin: `studio/dsh-vivy-studio/` (fluorite theme + Vivy Studio wordmark / welcome copy). Use `launch-vivy-studio.ps1` (repository root) to start **`dsh --profile vivy-studio`**; `DSH_HOME` is in `data/studio-home/`, and the daily gateway is untouched.

**Source ownership (2026-08-29):** The Studio shell and plugin tree (`dsh-vivy-studio` / `dsh-vivy-console` / `dsh-plugin-hub`, plus community plugin snapshots) are maintained in the separate repository [`ProjectViVy/vivy-studio`](https://github.com/ProjectViVy/vivy-studio), mounted as `agent-vivy`'s `studio/` through a git submodule. The main repository no longer embeds these source blobs. The lifecycle CLI (`cmd/vivy-studio`, `internal/studiocore`, `vivy-studio.exe`) remains in `agent-vivy`. If the first clone did not use `--recurse-submodules`, `just ensure-studio` or `launch-vivy-studio.ps1` runs `git submodule update --init -- studio`.

---

## 10. Slices

The completed species-side seams in S1–S6 (model-visible, inspect, verify, pack) remain usable parts.  
The S7 Studio card **is no longer the mainline**; its product meaning is void.  
S8 (forbidding production instances from rewriting themselves without a gate) still needs to be done, but it is a species safeguard that any authorized development tool can implement in the source workspace.
S9 (the frozen evaluation suite) belongs to Studio's evaluation phase.

Workstation slices:

| ID | Content | Venue | Completion evidence |
|---|---|---|---|
| ST-0 | Correction in this document: two applications, ledger in Studio, Studio has first-party development capability | bootstrap (this change) | Architecture documents are consistent |
| ST-1 | Independent Studio process (pinned DSH + `vivy-studio` profile) + §9.1 identity and theme | bootstrap | Does not go through `vivy.exe`; title / wordmark / welcome copy say Vivy Studio |
| ST-2 | Pin the project to the source tree; isolate it from any production `data/*.db` | bootstrap (2026-08-16) | Startup pins `agent-vivy`; production Journal is not read |
| ST-3 | Toolchain in Studio | bootstrap (2026-08-16) | Startup environment `go version`, `vivy-sdk verify plugins/hello-fs` |
| ST-4 | Skill: five plugin steps + the core `just ci` | bootstrap (2026-08-16) | `.agents/skills/vivy-plugin-five`, `vivy-kernel-ci` |
| ST-6 | Complete one real core change inside Studio + `just ci` | **Capability proof (2026-08-16)** | `internal/buildinfo` + `just ci`; proves Studio can serve as a complete author path |
| ST-5 | Studio packs and evaluates a candidate itself | Studio lifecycle (**2026-08-16 done**) | `cmd/vivy-studio` + `internal/studiocore`: `pack` execs `vivy-sdk`; `eval` starts the candidate EXE with a separate data directory; the ledger is in `data/studio-home/studio.db`; the live species process has zero involvement |
| ST-7 | Release → install to daily location → next start uses the new EXE | Studio lifecycle (**2026-08-16 done**) | `release` only accepts `--actor human --yes`; `install` writes the daily location + `install.json`; no hot replacement of a live process |
| ST-8 | Roll back to the previous Release | Studio lifecycle (**2026-08-16 done**) | `rollback` restores the previous Release files from a Studio snapshot; `data/vivy.db` is untouched |

ST-6 comes before ST-5: first prove that Studio can modify Vivy independently, then give Studio control of the evaluation and distribution lifecycle. This is the minimum evidence for first-party IDE capability, not a venue restriction on other development tools.

---

## 11. Decision Numbers (NG Continued)

The following corrections or additions continue the NG table in `VIVY-GATEWAY-AND-STUDIO.md`.

| ID | Decision |
|---|---|
| NG-21 | Vivy Studio is an independent application that manages the full development and distribution lifecycle. The daily `vivy.exe` is the artifact. |
| NG-22 | The species does not start Studio. There is no “Open Studio” entry point. |
| NG-23 | The authoritative ledger for Generation / EvalRun / Release / Install is in Studio. The species retains read-only `inspect` only. |
| NG-24 | Studio starts the candidate for evaluation. The live species is not the evaluation parent. |
| NG-25 | Release is an installation triggered by a human in Studio. Automatic release and hot replacement of a live process are forbidden. |
| NG-26 | **Studio is the first-party daily development IDE, but not the exclusive execution venue.** Other development tools authorized to read the workspace should use their own capabilities to implement and verify directly, without handing work off to or reproducing it in Studio. |
| NG-27 | If Studio is down, the installed species can still get through the day; if the species is down, that is not an excuse to change code. |
| NG-28 | The species-side Studio card and Promote authority are frozen; do not expand their product semantics. |
| NG-29 | Opening it presents Vivy Studio: title, wordmark, welcome copy, and profile name. The theme is a complete first-party token set, not a community skin or pasted CSS. |

NG-4, NG-16, and NG-20 are revised by this document: the three doors are no longer species-side eval/promote; DSH enters Studio before the re-skin to prove that Studio has complete first-party development capability.

---

## 12. Four-Sentence Contract

- **Gateway:** The only body for daily life, and the only writer of product truth (the Journal).
- **Studio:** The first-party daily development IDE; the authoritative application for the distribution lifecycle and the installer of next-generation bodies.
- **Human:** The only releaser, until a different rule is established.
- **Developer (including agents):** Modify Vivy directly in Studio or another authorized development tool; use the current tool's own capabilities, with no forced venue migration.

If a design would invalidate two of these sentences at the same time, it is not this architecture.
