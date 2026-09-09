# Vivy Face Pack — Cold Plug/Unplug for the Built-In Face

> **2026-09-09 v1 specification coverage:** The Face product semantics in this document remain valid; all
> descriptions of `seam: face`, `vivy.plugin/v0`, `vivy.generation/v0`, the old ABI, and compatibility migration
> are historical records and must not be used as the basis for new implementations. The only v1 mechanism is
> `std/face@v1` + FaceHost + `vivy.module/v1` + Generation Recipe, and it retains no v0 API.
> The canonical specification is `VIVY-MODULE-STANDARD.md`, `VIVY-PORT-CATALOG.md`,
> `VIVY-PLUGIN-SPEC.md`, and `VIVY-ASSEMBLY.md`.
>
> Status: **product semantics adopted; the plugin assembly mechanism is superseded by the v1 specification**. Android remains a downstream product using the kernel.
> Follows `SELF-EVOLVING-GATEWAY.md`, `VIVY-ASSEMBLY.md`, `VIVY-PLUGIN-SPEC.md`, **`VIVY-STUDIO.md`**, PRD §5.0 / D-016.
> Date: 2026-08-29
>
> **2026-09-04 revision (superseding the old D1/NG-11 product-shape conclusion):** First-party
> `vivy-code.exe` is now an allowed independent TUI artifact. It is not a second kernel: it still reuses the same
> app/runtime/provider/tool/FaceHost, but has an independent process boundary. `vivy.exe` and all
> `vivy-code.exe` instances share config/settings/skills; each code instance uses an independent
> SQLite Journal and run directory, so they do not share session records or compete for the organism lease.
>
> Comparative evidence (read-only, not dependencies): DeepSeek Harness's `dsh-base` + `dsh-web-app` /
> `dsh-headless` layering; the Desktop / Web / TUI surface profile of [oh-dsh](https://github.com/hust-open-atom-club/oh-dsh);
> and the terminal interaction feel of `.workspace/crush`.
> The Web remains the current mainline. This document records an assembly contract for a **coding species that can evolve without web**;
> it is not an immediate Bubble Tea implementation and does not hot-mount a terminal onto daily `vivy.exe`.

Related:

- `VIVY-ASSEMBLY.md` — name things by what they are; built-in units do not live in `plugins/`
- `VIVY-PLUGIN-SPEC.md` — constrains only user-layer `plugins/<name>/`
- `SELF-EVOLVING-GATEWAY.md` — installing a plugin = building a new version; default `Register()` is empty
- `VIVY-GATEWAY-AND-STUDIO.md` — NG-10 model-visible ≡ accounted; NG-11 rejects a second body
- `VIVY-CHANNEL-PACK.md` — super-channel (direction adopted 2026-08-30); face is the mouth. Channel must not replace the local UI
- `ACP-REMOTE-CONTROL-PROPOSAL.md` — cross-process / cross-device remote control; not a face
- `.workspace/deepseek-harness/` — profile / bundle evidence, not a species dependency
- `.workspace/oh-dsh/` — multiple surfaces on the same runtime
- `.workspace/crush/` — TUI interaction reference, not a code source

---

## 0. One Sentence

> **Face is a compilable organ in the recipe, not a tool and not the kernel.**
> The control plane stays in the species; the mouth goes into a built-in module or a user `seam: face` plugin.
> One primary face per generation. True removal = change the recipe line and `pack` again. The web gateway and coding TUI are two generations, not one switch.

This translates DSH's "layer the application on base" into Vivy's cold plug/unplug model: borrow the layering and inspectable registration, not hot mounting, not treating the Journal as a plugin, and not a second EXE.

---

## 1. The Feeling to Solve

Allow a generation's body to have **no web**. Do not let authors feel they are attaching parts to a live gateway, and do not let `config.yaml` conjure a TUI.

The criteria for "feels like building a face":

1. The built-in working directory contains only `faces/<name>/`. User-written code goes in `plugins/<name>/` (`seam: face`). Do not open `internal/` for daily work.
2. The world enters only through the public SDK: face lifecycle and Host capabilities go through
   `sdk/plugin`; the first-party terminal face reuses the only presentation, stream-state, and
   restricted control-plane client state machine in `sdk/tui`.
   `sdk/tui` exposes no Host, Journal, `Service.Run`, policy, or secret capabilities, so the face still
   cannot see these kernel objects.
3. Identity is the name in the manifest and the seam, not a `.go` file referenced by `cmd/vivy`.
4. To change the face, the author changes the **recipe**, not an embed switch or `engine.go`. `pack` generates `RegisterFace()`.
5. Before it runs, it is only source. Once it runs, it is a mouth in a generation's EXE. The committed default gateway generation remains `face: web`.

The Web mainline continues overnight. The coding line grows another body.

---

## 2. What to Borrow from DSH / oh-dsh / crush, and What to Refuse

### 2.1 Borrow

| Source | Idea | Vivy form |
|---|---|---|
| DSH | Shared `dsh-base`, mutually exclusive application layer | Kernel is always present; choose one `face: web \| tui \| headless` per generation |
| DSH | `dsh-web-app` / `dsh-headless` are bundles, not the kernel | Built-in `faces/web`, `faces/tui`, `faces/headless` |
| DSH | TUI can be installed outside the tree: `dsh plugin --profile tui add …` | User `plugins/<name>`, `seam: face`, still changes generation through pack |
| DSH | headless has no Host, HTTP, or browser | An EXE with `face: headless` does not embed UI or listen on a port |
| oh-dsh | Desktop / Web / TUI are surfaces on the same runtime | Same kernel, different recipe; TUI-only matches a "webless distribution" |
| oh-dsh | TUI-only has no Electron / browser UI | Coding-generation artifact contains no `ui/dist` |
| crush | Enter interaction without arguments; `run` uses a pipe | Startup feel of built-in tui / headless |
| crush | Permissions are first-class interaction | TUI overlay uses the same approvals; stderr Yes/No must not masquerade as HITL |
| ADR-015 | Live registry is empty; only pack overlay links entries in | Same shape as `internal/generated/faces/zz_register.go` |
| channel proposal | Name things by what they are; Host is in the kernel | FaceHost is in the kernel; the adapter only draws the mouth |

### 2.2 Refuse

| Idea | Reason |
|---|---|
| Forcibly putting TUI into the existing `tool` / `tool-world` seam | Those are the model's hands. Face is the mouth humans see |
| Putting the built-in TUI in `plugins/` | Pretends to be user-layer code (`VIVY-ASSEMBLY.md`) |
| Hiding the web by changing yaml / flag and pretending to be a coding species | The assets and port remain; it is not "webless" |
| Runtime `vivy plugin add tui` | NG-15; Go cannot unload native code |
| Copying the kernel into an independent `vivy-tui.exe` | NG-11 still forbids a second runtime; the first-party thin launcher `vivy-code.exe` is the explicitly approved 2026-09-04 exception, reusing the same kernel with an isolated Journal |
| Face plugin calling `net.Listen` / starting its own loop | A plugin is not a process; the loop is the kernel |
| Face plugin writing the Journal / changing policy / reading secret values | The environment cannot be a population member |
| Making `--yolo` the default | The human remains in charge |
| Using a channel instead of the local face | Already listed as a non-goal in `VIVY-CHANNEL-PACK.md` |
| Embedding Crush / dsh-TUI / oh-dsh into the species | Borrow the feel and layering, not Node or the entire TUI tree |
| Writing Android as `face: android` or `habitat: apk` | Android is a downstream product using the kernel, not a Vivy compilation target |

---

## 3. Three Layers, Keep Terms Separate

```text
vivy.exe  Kernel (never pluginized)
  Journal · Policy · SecretResolver · HITL arbitration · inspect
  In-process control plane (JSON-RPC semantics; HTTP listening is not a kernel obligation)
  FaceHost          ← new first-class object: select this generation's mouth, deliver events to it, reclaim TTY/HTTP
       │
       ├─ Built-in modules faces/web | faces/tui | faces/headless     recipe key face:
       └─ User module plugins/crush-face                            recipe key plugins:, seam: face
              ▲
              │  all implement only the SDK Face contract
              │  an unnamed face does not exist in this generation
```

| Layer | Where it lives | What it feels like to change | How it appears in the live body |
|---|---|---|---|
| Kernel FaceHost + control plane | `internal/` | Modifying Vivy | Always compiled in |
| Built-in face package | `faces/<name>/` | Building the web shell / TUI / one-shot runner | Recipe `face:` + pack |
| User face package | `plugins/<name>/` | Building a plugin | Recipe `plugins:` + pack; seam must be `face` |
| Configuration | Face-specific knobs in `config.yaml` | Adjusting knobs | May turn only the face **listed by inspect** |

Built-in code must not go in `plugins/`. User code must not go in `faces/`. The ABI is the same; the directories and vocabulary differ.

HTTP listening is an effect of `faces/web`, not a kernel obligation. The `vivy_headless` build tag is today's workaround; after adoption it should become a recipe effect.

---

## 4. Recipe: One Face per Generation

`vivy.generation.yml` (after adoption) adds a first-class key:

```yaml
apiVersion: vivy.generation/v0
loop: eino
world: sandbox
face: web                 # exactly one; pack fails if omitted
providers:
  - openai
tools:
  - notes
  - filesystem
  - execute
  - ask-user
plugins: []               # user seam: face competes with the built-in face for the same key; whichever is named enters
```

Rules:

- **Exactly one `face:`.** A species must have one mouth. An empty list is invalid.
- Built-in names: `web`, `tui`, `headless`. A user plugin uses its directory name, is named through `plugins:` with `seam: face`, and **replaces** the built-in face rather than stacking a second one.
- A built-in face not written into the recipe does not exist in this generation. Do not scan `faces/`.
- Remove a built-in face = change `face:` and pack again. Remove a user face = delete its line from `plugins:` and change back to a built-in name. Both require eval / promote.
- `inspect` lists: `face` name, kind (web / tui / headless), whether it listens, whether it embeds UI, source_ref, and tree_hash.

Two valid generations (illustrative; not an immediate default switch):

**Gateway generation (current mainline)**

```yaml
face: web
world: sandbox
```

**Coding generation (evolution target)**

```yaml
face: tui                 # or plugins/crush-face
world: local              # working directory is the project
# This generation's EXE has no go:embed ui/dist and does not open :8787
```

`world: local` is not a face side effect. A coding species changes worlds by selecting `world:` separately. The face only controls how it speaks with people.

---

## 5. Three Built-In Faces

| Name | Startup | Listen port | embed UI | Interaction |
|---|---|---|---|---|
| `web` | Start the gateway with no arguments | loopback | yes | Browser; Review Center |
| `tui` | Claim the TTY with no arguments | no | no | Sessions, streaming, approval overlay, cancellation |
| `headless` | `vivy run "…"` | no | no | One prompt, terminal state on stdout, then exit |

`headless` is not an incomplete tui. It is the mouth for scripts/CI. Without a TTY, approval must fail loudly; silent allowance is forbidden.

The first TUI cut is thin: session list, streaming conversation, first-class approvals/questions, cancellation, and connection to the same Journal. It does not reproduce the settings page or the full Review Center. Settings remain in the web generation or a future dedicated command.

Success criterion (tui): complete one approval-bearing conversation in the terminal, and replay the same Journal in the web-generation binary. The two bodies need not run simultaneously.

**Current entry point (2026-09-05).** `sdk/tui` is the sole implementation of the full-screen shell, control-plane projection, and Live
state machine; the independent `vivy-code.exe`, `vivy tui`, and built-in
`faces/tui` all delegate to it. `vivy tui --live [--addr host]` connects
to the resident gateway through the WebSocket transport in `internal/tui`; it fails and exits if the gateway is not running.
The old offline `--demo` and line-based `--plain` modes are retired; there is no demo or local-execution
fallback. `faces/tui` still enters an artifact without `ui/dist` through FaceHost and recipe selection.

---

## 6. `seam: face` (User Layer)

It is structurally parallel to channel and does not use the tool `Adapt` path.

```json
{
  "apiVersion": "vivy.plugin/v0",
  "name": "crush-face",
  "version": "0.1.0",
  "seam": "face",
  "module": ".",
  "grants": ["tty", "argv", "rpc.client"],
  "face": {
    "kind": "tui",
    "listen": false
  }
}
```

| Field | Rule |
|---|---|
| `seam` | Must be `face` |
| `tools` | **Forbidden**. A face is not a model tool |
| `face.kind` | `web` \| `tui` \| `headless` |
| `face.listen` | User plugins default to `false`. `true` is rejected in the first cut |
| `grants` | `tty`, `argv`, `rpc.client`. No `journal.write`, `secret.read`, or `policy.write` |

For `verify` with `seam: face`:

- zero tools;
- do not import `internal/`;
- no `net.Listen`;
- no `go:embed` executable files;
- `kind` is valid; if it conflicts with the `face:` named by the recipe, the recipe wins (the user plugin replaces the built-in face).

Face Env (illustrative; to be added to `sdk/plugin` when implemented) permits only listing sessions, starting a run, subscribing to events, answering approvals/questions, and cancelling. Direct Journal writes, changing the policy hash, reading secret values, and starting a loop are forbidden.

`source` provenance: `web` \| `tui` \| `headless` (or the user face's name). It cannot masquerade as another face's `user` row or be written as `channel`.

Crash = this generation's EXE crashes. Isolate it in the next generation: change the face in the recipe and pack again.

---

## 7. Kernel: Face Can Be Absent

Today's obstacle is not the absence of Bubble Tea; it is that the kernel treats the web as the default body:

1. `cmd/vivy` composes an HTTP service when called without arguments
2. UI uses `go:embed` by default; `vivy_headless` is only a build tag
3. The human interface for approvals/questions is rooted in the web by default

The kernel should become:

```text
launcher
  → read the face compiled into the body
  → web        listen on loopback, embed UI
  → tui        occupy TTY, listen on no port
  → headless   run one prompt, then exit
  → no face    pack already rejected; unreachable at runtime
```

The control plane remains in-process. Built-in and user faces are **in-process clients** of the control plane, not a second run model. The web is already a JSON-RPC client; TUI uses the same methods, with transport changed from WebSocket to function calls.

FaceHost is on the "kernel never pluginized" list alongside Journal, Policy, Secret resolver, inspect, and `vivy worker` supervision. It does not draw UI. It guarantees only that this generation has exactly one mouth and that events and approvals remain decided by the kernel.

---

## 8. What Else the Coding Generation Must Change (Beyond the Face)

Changing only the face produces a "personal gateway in a terminal," not Crush. Crush / DSH headless defaults to **the working directory being the workspace**.

| Knob | Gateway generation | Coding generation |
|---|---|---|
| `face` | web | tui (or user face plugin) |
| `world` | sandbox | local (`--cwd` / working directory) |
| persona | gateway / companion | coding agent, cwd in the system prompt |
| Tool density | notes + conservative execute | grep / glob / edit / bash level |
| HITL | Review Center | TTY overlay; same first-writer-wins |
| Listening | loopback | none |

LSP, project skill discovery, and Crush-style `crushrc` are **not in the first cut**. Configuration remains strictly decoded yaml + `env_key`; do not grow another trusted code configuration that executes upon loading.

---

## 9. Android: Downstream Product, Not a Face

An Android app **uses the Vivy kernel**; Vivy does not compile the APK, and it is not `face: android`.

```text
Vivy kernel   Journal · policy · run · approvals · control plane
    │
    ├─ first-party species    vivy.exe (face: web | tui | headless)
    ├─ user face              plugins/crush-face (compiled into a generation's EXE)
    └─ downstream product     an Android app (its own project, its own APK)
```

Studio continues to pack only species bodies. The Android team ships with its own toolchain. Vivy is not responsible for the Android SDK, signing, store submission, or gomobile.

Therefore this proposal **does not add** `habitat:` or make `seam: face` understand Activity.

For Android to work with the kernel, only F1 is required: the control plane must still complete one conversation and approval without embed or listen. That is a shared cut for the Web mainline, TUI, and downstream apps.

If phones later need to "remote-control the `vivy.exe` at home," that is the remote principal in `ACP-REMOTE-CONTROL-PROPOSAL.md`, not a face or channel. Synchronizing a phone Journal with a computer Journal is a separate proposal.

---

## 10. Boundary with Channel / ACP

| | face | channel | ACP / companion |
|---|---|---|---|
| What it is | The mouth of this process | The ear through which the world speaks first | A remote control on another process/device |
| Recipe | Exactly one `face:` | `channels:` list | Not in generation.yml |
| Provenance | `source=web\|tui\|headless` | `channel.inbound` | Control-plane principal, recorded separately |
| Approvals | Local face may be the HITL principal | Not an approver in the first cut | Remote principal must be explicitly approved |
| Examples | Browser, TTY, `vivy run` | Telegram, Feishu | Future Android remote control, editor |

Mixing the three authorities is a bug.

---

## 11. Cold Plug/Unplug Removal

```text
1. Generation replacement (true cold)
   recipe face: tui → pack → the next generation has no web assets and does not listen on 8787

2. Runtime cannot "turn off the web and pretend to be TUI"
   A live EXE does not dynamically load any face code

3. There is no "kill one face while the species uses another"
   One mouth per generation. A crash kills the generation
```

"Clean removal" applies only to (1). `inspect` must prove that an artifact with `web=false` contains no `ui/dist`.

---

## 12. Non-goals (This Proposal)

- V0 delivery or putting TUI into the current mandatory `just ci` path
- Hot mounting / hot unloading / marketplace scanning
- Implementing Bubble Tea / a Crush clone / a TUI settings page now
- An independent `vivy-tui.exe` or treating Crush or dsh-TUI as a dependency
- `habitat: android`, an APK pipeline, or gomobile
- Replacing the local face with channel or ACP
- Changing `sdk/plugin` or adding event types while merging this document (that is a post-adoption implementation PR)

---

## 13. Implementation Slices (After Adoption)

The order is the dependency order. Each cut should be independently evaluable; unfinished work does not appear in the default gateway EXE.

| Slice | Does | Success |
|---|---|---|
| F0 Contract | Adopt this document; add the `face` line to `VIVY-ASSEMBLY.md`; add FaceHost and "HTTP is not the kernel" to the kernel-never-pluginized list; PLUGIN-SPEC declares `seam: face` routing | Documentation consistent, no code |
| F1 Control plane without web | In-process RPC completes one conversation + approval without embed or listen | Prerequisite for upgrading `vivy_headless` from a build tag to a recipe effect |
| F2 Built-in `faces/headless` | `vivy run "…"`, terminal state on stdout, no port | Usable for scripts; approval must fail loudly without a TTY |
| F3 Built-in `faces/tui` | Thin TUI: sessions, streaming, approval overlay, cancellation | Same Journal; gateway binary may still omit this face |
| F4 Coding recipe | `world: local` + coding persona + tool set; pack an artifact without `ui/dist` | `inspect` shows `face=tui`, `web=false` |
| F5 User `seam: face` | `plugins/crush-face` can replace the built-in tui | verify forbids tools, Listen, and import internal |

The Web mainline continues; F0–F1 do not block it. Only after F3 may someone evolve a Crush-style implementation in Studio.

F0 is a documentation PR. The kernel changes only from F1 onward. Before F3, Bubble Tea must not be a required import in the species default `go.mod`.

---

## 14. Unresolved Questions (Require a Decision Before Adoption)

The four questions were decided on 2026-09-02 (user ruling, all recommended values accepted; prerequisites for F2/F3 cleared):

1. **Whether built-in `faces/` should have an independent `go.mod`.** Decided: **independent `go.mod`**—the gateway generation will not see TUI dependencies at compile time and the kernel cannot accidentally import them; the cost is another module to maintain.
2. **Whether the default committed species body should always be `face: web`.** Decided: **always `face: web`**. The coding generation is another recipe and does not replace the gateway opened by the daily double-click.
3. **Whether Web and TUI can share one Journal.** Decided: **no in the first cut** (one mouth per generation). Multi-client coexistence comes later and must first pin down approval first-writer-wins.
4. **Product wording when `headless` encounters approval.** Decided: **fail and exit**—do not suspend and wait or allow via yolo (run-level durable suspension + cancellable semantics are pinned by F1 tests; process-level behavior = fail and exit).

---

## 15. PR Plan (After Adoption)

### PR 1 — Adopt Contract

- Files: change this document's status to direction adopted; add the `face` line to `VIVY-ASSEMBLY.md`; cross-reference seam routing in `VIVY-PLUGIN-SPEC.md`; add FaceHost to the kernel list in `SELF-EVOLVING-GATEWAY.md`
- Dependencies: none
- No runtime code

### PR 2 — Control Plane Without Web (F1)

- Files: launcher / app composition works without embed; test the approval failure path without UI
- Dependency: PR 1

### PR 3 — Built-In Headless (F2)

- Files: `faces/headless/`, `vivy run`, pack overlay
- Dependency: PR 2

Follow-up F3–F5 are one PR each; do not mix them with channel adapters or Android projects in the same deliverable.

---

## 16. One Sentence (Repeated)

> **The face is an organ in the recipe, not a tool and not a second kernel.**
> DSH layers bundles with a profile to remove `dsh-web-app`; oh-dsh uses TUI-only to prove a distribution can omit the browser.
> Vivy's equivalent is: `face: web | tui | headless` + a user `seam: face` + a coding generation whose recipe does not name the web.
> Install = pack into a new EXE. Remove = change the recipe line and pack again. Android apps use this kernel and ship themselves.
