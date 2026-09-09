# Deep dive on the plugin model: DSH Cordis vs. the Vivy seam model (2026-09-02)

> Trigger: maintainer observation—"The plugin capability on the Vivy side seems underdeveloped and category-limited; I imagined it should be fully customizable, exactly like DSH."
> This is a focused deep dive into the plugin model. A prior document covers the strategic full-capability comparison:
> `docs/research/DSH-VS-AGENT-VIVY-CAPABILITY-GAP.md` (its conclusion: most gaps are **intentional gaps**).
> This document answers one question only: **At the plugin layer, where exactly is the gap, which parts are by design, and which are worth narrowing?**
> Leading with the conclusion: the maintainer's impression is half right—Vivy plugins are indeed category-limited (a closed seam set + closed grants + a single Plugin interface),
> but this is not "underdevelopment"; it is the compile-time governance model decided for v0. DSH's "fully customizable" model assumes
> **fully trusted plugins + runtime hot composition**, which is opposite to Vivy's single-binary / provenance / generation-audit philosophy.
> What is actually worth narrowing is the **number of extension points** (events, provider-consumer), not the loading timing.

---

## 1. Facts on the DSH side: Cordis, "everything is a plugin"

Source: `.workspace/deepseek-harness/deepseek-harness/` (working clone, 0.1.2-alpha.1, ahead of the upstream mirror).
Below, `<D>` = the root of that clone.

### 1.1 What a plugin is: code only, with no category restriction

A plugin is a JS/TS module that implements Service, in one of three forms (`<D>/docs/cordis-primer.md:9`,
`<D>/docs/cordis-tutorial/01-first-plugin.md:55-75`):

```ts
// 1. Function plugin (canonical form)
export const name = 'hello'
export const inject = ['tools']          // dependent service key
export function apply(ctx: Context, config: Config) { ... }
// 2. Object plugin { name, apply(ctx) }
// 3. Class plugin class MyService extends Service
```

- The optional module exports are limited to four: `name` / `inject` / `apply` / `Config` (Schemastery schema).
- **There is no category/type enumeration.** A plugin can be a tool, LLM adapter, sandbox backend, storage backend,
  UI feature, or even an entire subsystem. Classification is **factual** ("tool-pipeline events belong to `ctx.tools`, model streaming belongs to
  `ctx.llm`", `cordis-primer.md:56`), not enforced by a validator.

### 1.2 Lifecycle: declarative composition, runtime mounting

- **Discovery is declaration**: a plugin is a line in the YAML composition file (`cordis.patch.yml`); bundle patch →
  profile patch → `--patch` overlay are layered by last-write-wins
  (`<D>/packages/bundle/base/cordis.patch.yml`).
- Loader (`<D>/vendor/loader/src/config/entry.ts`) imports modules and mounts them into Fiber through
  `ctx.registry.plugin(...)`; entries start concurrently, with ordering expressed only by `inject` dependencies.
- Inline `!!js` scalars can be evaluated against the loader context during activation (config interpolation / `disabled` condition).
- **Registration is reversible**: everything registered through `ctx.effect()` / `ctx.on()` automatically holds a disposer;
  unload/HMR rolls it back in order (`cordis-primer.md:13-16`).
- State machine `pending|loading|active|failed|unloading` (`<D>/packages/host/plugin-inventory/src/types.ts:7-13`);
  changes to `name`/`inject`/`group` force dispose + restart, with rollback on failure; config changes are hot-patched.
- Installation: `dsh plugin add <pkg>` forwards to pnpm; `--patch ./overlay.yml` mounts a local plugin.
- **Model-authored dynamic plugins**: after mounting `tool-cordis` + `cordis-host-runner`, the model itself can use
  `cordis_define / cordis_run / cordis_stop / cordis_undefine` to define runtime plugins in memory
  (in-process, versioned, not persisted); `cordis_inspect_*` provides read-only introspection
  (`<D>/packages/extensions/tool-cordis/README.md:40-56`).

### 1.3 Extension surface: ~55 ctx service keys + typed events

Service keys (collected from package READMEs): `tools, llm, sessions, sessionQuery, sessionPersistence,
sessionProjections, sessionTitle, fs, shell, subprocess, sandbox, sandboxPolicy, terminals,
codeRuntime, jobs, workflowEngine, subagents, agents, agentPresets, commands, skills,
systemPrompt, attachments, userQuestions, approval, credentials, settings, storage,
compaction, web, webServer, webhookRuntime, lsp, tokenMeter, goals, planMode,
permissionPresets, theme, locale, layout, slots(client), cordisInspect, ...`

Key capability forms:

| Form | API | Source |
|---|---|---|
| Model tool | `ctx.tools.register(defineTool({name, parameters, execute}))` + `tools/pre-execute|execute|post-execute` waterfall middleware | `<D>/packages/core/tools/src/index.ts:142-183` |
| Prompt injection | Ordered sections, dynamic context, `{{var}}` through `ctx.systemPrompt` | `<D>/packages/core/system-prompt/src/index.ts` |
| Human command | Register CommandDefinition with `ctx.commands`; UI executes directly, not through the model | `<D>/docs/subsystems/commands.md:19-46` |
| Skill provider | Register SkillProvider {list, get} with `ctx.skills` | `<D>/docs/subsystems/skills.md:30-77` |
| Subagent | Named spawn registry in `ctx.subagents` | `<D>/packages/subagent/subagent/README.md:32` |
| UI slot | Typed React slot tree through `ctx.slots.register/inject` | `<D>/docs/subsystems/slots.md:44-64` |
| Infrastructure replacement | LLM adapter, sandbox backend, storage, credentials (Service-Definition/Provider/Consumer trio) | `<D>/docs/user/develop/practice/index.md:9-49` |

Five event-dispatch modes: `emit` (observe) / `waterfall` (around middleware, can short-circuit) / `parallel` /
`serial` / `bail` (the first non-undefined result wins). Dispatch mode is part of the event's public contract
(`cordis-primer.md:15-27`).

### 1.4 Trust model: plugins fully trusted, permissions at the tool-execution layer

- Regular plugins are **unsandboxed**: "the preset and the plugins it names have equal privileges … equivalent to shell access"
  (`<D>/packages/preset/agent-presets/README.md:12`).
- Model-authored dynamic plugins have a `node:vm` sandbox + `vmTimeoutMs` (default 5000), and the browser side requires manual approval—
  but the documentation explicitly says "the sandbox isolates global variables, **it is not a security boundary**, treat it as bash access"
  (`tool-cordis/README.md:60`).
- The real permission gate is the `tools/pre-execute` allow/deny/ask waterfall, the approval subsystem,
  and the process sandbox (`read-only | workspace-write | danger-full-access`).
  **Plugin loading itself does not pass through the permission gate.**

---

## 2. Facts on the Vivy side: compile-time seam governance model

Source: `sdk/`, `internal/pluginhost/`, `internal/channelhost/`, `docs/architecture/`.

### 2.1 What a plugin is: governance unit = manifest + Go source + generation

```
plugins/<name>/
  vivy-plugin.json   identity + contract (read by verify/pack)
  plugin.go          implements sdk/plugin.Plugin
```

The manifest is "a recipe card for the SDK compiler, not for a runtime loader"
(`VIVY-PLUGIN-SPEC.md:87`). Schema: `sdk/internal/manifest.go:21-44`
(apiVersion (only `vivy.plugin/v0`) / name / version / seam / module / grants / tools /
channel{transport, max_message_runes}).

Pack flow (five steps, `.agents/skills/vivy-plugin-five/SKILL.md`):
`vivy-sdk verify` → `vivy-sdk pack --with <name>` → generate a new `zz_register.go` →
`go build -overlay` produces a new-generation EXE + `generation.json` (tree_hash, per-plugin provenance).
**Installation = pack; uninstall = remove the recipe line and pack again. A running process never reads the plugins/ directory.**

### 2.2 Category restriction: yes, three closed sets

1. **Four-value seam enumeration** (`sdk/plugin/plugin.go:14-31`): `tool` / `tool-world` /
   `provider` / `channel`. The specification explicitly prohibits `journal`, `policy`, `sdk`, and `studio`
   (`VIVY-PLUGIN-SPEC.md:82`: "allow only … prohibit …"). Validation is in
   `sdk/internal/manifest.go:73-76`.
2. **Eight-word closed grants vocabulary** (`plugin.go:35-76`): `fs.read / fs.write /
   channel.poll / channel.webhook / channel.listen / channel.a2a / secret.read /
   proc.spawn`; further seam-level restrictions apply (the channel grants are only for `seam: channel`;
   `proc.spawn` is only for `tool-world`, `manifest.go:90-95`).
3. **Single Plugin interface**: `Plugin { Name, Seam, Grants, Tools }`
   (`plugin.go:95-100`)—a plugin can contribute **only a tool list**, plus two side channels:
   - with `seam: channel`, implement the `plugin.Channel` ABI (`sdk/plugin/channel.go:14-30`,
     plus 11 optional capability interfaces + 2 reserved slots, `channel.go:138-225`);
   - tool-world may optionally implement `DiagnosticObserver` (`plugin.go:119-122`).

Note: `SeamProvider` is legal in the vocabulary, but has **zero kernel consumers** (grep confirms it appears only at its definition)—
it is a nominal seam. Today, every non-channel plugin is effectively a "toolkit."

### 2.3 What plugins cannot do (verify fails directly, `sdk/internal/inspect.go:58-219`)

- import the kernel (`agent-vivy/internal/...`), Eino, pion, or `.workspace` paths;
- call `os.Open/Create/StartProcess` or `exec.Command` directly (must go through `Env`);
- open any listening socket; executable suffixes with `go:embed`; or `package main`.
- The Env surface is intentionally tiny: `Secret / outbound HTTP / Settings / PublishInbound / Media / Spawn`—
  "no Journal, no Policy, no raw OS, no Eino" (`VIVY-PLUGIN-SPEC.md:139`).
- no UI, commands, event/hook registration, or hot reload. The kernel does have ToolHookChain
  (`internal/runtime/hooks.go:58`, assembled in `internal/app/app.go:312`), but its source is
  **config scripts**, not a plugin registration point.

### 2.4 Design intent (why it looks this way)

- "Allow pure Go code. Do not let authors think they are modifying the kernel" (around `VIVY-PLUGIN-SPEC.md:28`).
- "Plugin crash = this generation's EXE crash. Isolation is not in the process, but in the next generation" (`:263`).
- `SELF-EVOLVING-GATEWAY.md:366-388` **explicitly rejects** all runtime loading: WASM, DLL/Go
  plugin, external stdio exe, directory scanning/plugin marketplace are all "rejected." Only two forms are accepted:
  Skill text (Kind A, not compiled), and Go source + manifest (Kind B, enters the recipe and waits for pack).
- Three-way division: Kind A Skill (behavior text) / Kind B Plugin (capability source) / Kind C
  Generation (recipe + built EXE, the only "installation" action) (`SELF-EVOLVING-GATEWAY.md:236-256`).
- AGENTS.md hard rule `no-plugin-via-engine-import`: install plugins only through `vivy-sdk pack`.

---

## 3. Dimension-by-dimension comparison

| Dimension | DSH (Cordis) | Vivy | Assessment |
|---|---|---|---|
| Plugin form | JS/TS module (code is the plugin) | Go source + JSON manifest | Different paradigm, not better/worse |
| Category restriction | None (factual classification) | Four-value closed seam set + eight-word closed grants set | **Real gap** (user-visible point) |
| Loading timing | Runtime (YAML composition, HMR, hot uninstall) | Compile time (pack a new-generation EXE) | **Intentional gap** (NG-11, required for provenance/audit) |
| Number of extension points | ~55 ctx service keys + five-mode typed events | 1 interface (Tools) + channel ABI + 1 observer | **Real gap, largest one** |
| Middleware/events | Waterfall around, bail, short-circuitable; tool pipeline pre/execute/post | None (ToolHookChain is config-script-only) | Real gap |
| UI/command/skill registration | Plugins can register slots/commands/SkillProvider | None; skills are Kind A plain text and orthogonal to the plugin system | Real gap (partly philosophy: UI is not part of the species body) |
| Infrastructure replacement (LLM adapter/storage/sandbox backend) | Provider trio freely registered | SeamProvider exists in name only; provider is configuration, not a plugin | Real gap (nominal seam reserved) |
| Model-authored plugins | tool-cordis (VM sandbox, manual approval, not a security boundary) | Rejected (S8 "ungated self-rewrite" remains sealed) | **Intentional gap** (NG-11) |
| Trust/sandbox | Plugins fully trusted; permission gate at tool-execution layer | Source-level static checks + grants fail-closed + path sandbox + writes recorded in file_versions ledger | Different paradigms; Vivy is stricter before loading, while DSH is looser but has a finer runtime gate |
| Failure semantics | One plugin fails, the rest survive | Plugin crash = this generation's EXE crash, fixed in the next generation | Paradigm difference (`VIVY-PLUGIN-SPEC.md:263`) |
| Install/uninstall | pnpm add / delete patch line, immediate | Re-pack a new-generation EXE | Intentional gap (generation audit) |

In one sentence: **DSH makes "composability" a product (a runtime plugin OS), while Vivy makes "governance" a product
(compile-time generation). The user's imagined model = DSH paradigm; Vivy v0's decision = anti-DSH paradigm.**

---

## 4. Gap assessment: what not to pursue and what is worth narrowing

### Do not pursue (intentional gaps; changing them would damage the species philosophy)

1. **Runtime hot loading**. `SELF-EVOLVING-GATEWAY.md` has rejected it three times (WASM/DLL/stdio exe).
   Compile-time pack is the foundation for provenance (tree_hash, per-plugin origin) and the "uninstall = re-pack" audit semantics;
   changing to runtime loading would mean abandoning the Generation model.
2. **Plugins in Journal/Policy**. The kernel will never be pluginized (`SELF-EVOLVING-GATEWAY.md:159-169`).
   DSH only makes permissions into plugin-mountable events; it does not hand over the storage engine—on this point the two systems actually align.
3. **Model-authored plugins**. DSH itself states that its VM sandbox is not a security boundary; Vivy S8 lists this as rejected.
   If this is ever built, it must use Kind A text form or an independent gate and is not part of this document's proposal.

### Worth narrowing (real gaps compatible with the governance model)

Ordered by cost-effectiveness:

1. **Land SeamProvider** (existing hook point, no new concept): allow `seam: provider` plugins to register
   LLM adapter / embedding / storage backend implementations, while consumers continue to use kernel-defined interfaces.
   This is the Go counterpart of DSH's Service-Definition/Provider/Consumer trio,
   upgrading "provider is configuration" to "provider can be supplied by a plugin" without disturbing compile-time loading.
2. **Event/hook seam** (largest extension-surface gap): add `seam: hook` (or an optional tool-world interface),
   allowing plugins to attach `pre-execute / post-execute / around` tool middleware and run-lifecycle observers.
   The kernel already has the `ToolHookChain` foundation (`internal/runtime/hooks.go`); only the "hook source"
   needs to expand from config scripts to plugins—the permission gate remains in the kernel (each hook must also pass grants validation).
3. **Do not change commands/UI**: UI and human commands are Studio/frontend surfaces, not part of the species body;
   Vivy's dual-event-surface architecture intentionally keeps UI outside. Keep it that way.
4. **Expand the grants vocabulary as needed** (for example, keep `net.listen` prohibited but consider `timer`): the closed set is correct,
   but the vocabulary should grow with extension points rather than forcing capabilities into tool semantics.

### If the decision is "we want DSH-style full customization"

That is not a plugin-system change; it is a product-philosophy change: accept fully trusted plugins, abandon single-binary generation auditing,
add a runtime composition layer (YAML patch / pnpm equivalent), and rewrite the channel/tools loading paths.
The cost is at least one CH-channel EPIC and conflicts with the single-organism contract in `prd-agent-vivy-v0.md`.
Not recommended. The compromise is items 1 and 2 in §4 above (keep compile-time loading unchanged and widen the seams).

---

## 5. Conclusion

1. The user's perception is accurate: Vivy plugins are constrained by **three closed sets** (seam enumeration, grants vocabulary, and a single Plugin
   interface), with an extension surface approximately 1/50 the size of DSH's (1 interface + channel ABI + 1 observer vs. ~55 service keys
   + event system).
2. But "underdeveloped" is not accurate: the closed sets were explicitly decided for v0 (the spec says "allow/prohibit" in black and white), and the accompanying
   verify static checks, grants fail-closed behavior, and generation audit are strengths DSH does not have;
   DSH's openness comes at the cost of "plugins equal shell access, with no security boundary."
3. The real action is not "change it to DSH," but **widen the seams within the compile-time paradigm**: land SeamProvider +
   hook/event seam. Neither breaks the Kind A/B/C division or generation audit.
   This is tracked as **PLG-1** in `docs/TODO.md` §0.1, pending a decision.
