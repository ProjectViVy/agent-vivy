# Suite Content-Pack Research: Codex Primary, ZCode Secondary

**Date:** 2026-10-11 (Asia/Shanghai)\
**Status:** Preliminary source-research spike for [ProjectViVy/agent-vivy#20](https://github.com/ProjectViVy/agent-vivy/issues/20). This report records evidence and design questions; it does not authorize implementation or settle the open decisions in #20.

## Executive summary

This spike compares the suite proposal in #20 with two pinned upstream repositories. Codex is the primary reference for portable plugin-manifest compatibility, skill namespacing, and hook admission. ZCode is the closer product-level comparison for a content package with installation, enablement, update, uninstall, marketplace identity, diagnostics, and component projection.

The main findings are:

1. **Neither project has a first-class plugin-manifest component named Rules.** Codex receives project instructions through its separate AGENTS.md instruction chain. ZCode also has a separate user/workspace instruction path; its plugin manifest component schema does not define rules. A Vivy suite would need an explicit adapter into Vivy's existing instruction owner, with defined scope and precedence.
2. **Codex and ZCode have different hook trust defaults.** Codex records hook hashes and does not add ordinary untrusted/modified handlers to the executable handler set; a matching trusted hash, managed status, built-in allowlist, or explicit bypass is required. At the pinned ZCode source snapshot, enabled third-party marketplace hooks are admitted and its adapter test explicitly expects them to be runnable. A ZCode compatibility plan says the opposite for third-party marketplace hooks. Treat the implementation and documentation as inconsistent until ZCode reconciles them.
3. **A content-only package can still cause local side effects.** Hook command declarations can execute local commands, and MCP declarations can launch local processes or connect to remote endpoints. “No new Vivy Go/UI code” is not itself a sufficient security boundary for hooks or MCP.
4. **ZCode has the fuller lifecycle model:** stable IDs in the form plugin-name@marketplace, versioned cache, per-plugin data, atomic cache activation, explicit enablement, update diagnostics, uninstall cleanup, and an orphaned-installed state when a source is removed. This is useful lifecycle evidence, but the product surface is broader than #20's proposed Skills/Rules/Hooks/MCP package.
5. **Namespacing does not always equal full package identity.** Both systems derive skill aliases from the manifest plugin name. ZCode derives MCP runtime names from plugin name plus server name, while its persistent plugin ID also includes marketplace. Codex plugin MCP names remain global server names; duplicate active names are diagnosed and the effective map retains the first encountered definition. These are concrete reasons to define suite identity, child namespace, and collision policy separately.
6. **Neither upstream implementation establishes #20's proposed canonical whole-package digest and coordinated run snapshot as a ready-made contract.** Codex has versioned installation and hook-content hashes. ZCode pins some remote source identities and requires a SHA-256 for ZIP sources. Those mechanisms have narrower meanings than one canonical digest over all suite content.

For Vivy, these findings support keeping the proposed suite as a Host-owned content contract that feeds existing owners, while explicitly deciding rule projection, identity and child names, hook trust, source pinning, digest scope, and snapshot activation. They do not support copying either project's plugin runtime wholesale.

## 1. Scope, method, and terminology

### 1.1 Scope

The comparison follows #20's proposed content set: Skills, Rules, Hooks, and MCP servers; one manifest and lifecycle unit; no new Vivy Go/UI code or Ports; source identity/provenance; component namespacing; and coordinated enable/update behavior. #20 is currently an open, unscheduled proposal. The corrected issue body distinguishes its existing skill, instruction, MCP, and hook foundations from the still-missing unified suite lifecycle.

The Codex snapshot is the primary reference because it was already cloned for this work. ZCode is the secondary reference and was cloned from the URL supplied in the request.

### 1.2 Pinned source snapshots

| Repository | Commit inspected | Commit date/message |
| --- | --- | --- |
| [openai/codex](https://github.com/openai/codex/tree/cc7ba33601286a751477832b01de7b6f8de0dd9c) | cc7ba33601286a751477832b01de7b6f8de0dd9c | 2026-10-10; searchable /config preferences panel |
| [zai-org/ZCode](https://github.com/zai-org/ZCode/tree/aac4755666d09fdcd70272fcf063c077a639015f) | aac4755666d09fdcd70272fcf063c077a639015f | 2026-10-10; update to v3.15.1 |

All source links below point to these commits, so later upstream changes will not silently alter the cited evidence.

### 1.3 Method and limits

This was read-only inspection of manifests, loaders, hook admission, plugin identity, lifecycle code, selected design documents, and relevant test source. Tests were read as evidence of intended behavior but were not run. No build, install, or runtime experiment was performed. The ZCode hook finding is therefore source-backed, not a fresh end-to-end execution.

The Vivy baseline is the current #20 proposal and its 2026-10-11 source-review correction; this spike extends the upstream comparison and does not repeat a complete Vivy code audit.

### 1.4 Terminology

“Suite” means the proposed Vivy artifact in #20. “Plugin” refers to the upstream Codex or ZCode artifact and its own loader/lifecycle. Similar folder names and component concepts do not imply schema or trust-policy compatibility.

“Content-only” describes what the package ships. It does not mean the declared capabilities have no execution effects. A skill can influence model behavior; a hook can run a process; an MCP entry can start a process or connect to a service.

## 2. Codex findings — primary reference

### 2.1 Codex accepts two manifest lineages

Codex has a legacy Codex plugin manifest and a standard Agent Plugins manifest path. The standard root plugin.json must declare a supported Agent Plugins schema. The parser accepts a limited set of standard metadata fields, warns about unknown fields, and drops those unknown fields before resolving Codex capabilities. In that standard format Codex supplies default paths for ./skills and ./mcp.json. Codex-specific behavior is carried under extensions.com.openai or through Codex compatibility overlays; it is not part of the standard field list. [Codex Agent Plugin parser](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/agent_plugin_manifest.rs#L17-L29) [field filtering and extension lookup](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/agent_plugin_manifest.rs#L64-L104) [standard defaults and extension application](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/agent_plugin_manifest.rs#L129-L195)

The legacy Codex parser models skills, MCP servers, apps, hooks, and interface metadata. Skills and hooks accept path declarations; MCP accepts a path or inline object; hooks accept a path, a list of paths, an inline hooks file, or a list of inline hooks files. These are Codex implementation fields, not the same schema as the standard Agent Plugins manifest. [Codex legacy manifest fields](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/manifest.rs#L48-L73) [hooks and MCP manifest shapes](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/manifest.rs#L134-L158)

The loader recognizes the standard root plugin.json only when its schema is recognized. If that is unrelated or absent, it falls back to discoverable .codex-plugin/plugin.json, .claude-plugin/plugin.json, and .cursor-plugin/plugin.json paths. This is compatibility discovery, not evidence that every accepted format has identical behavior. [manifest discovery paths](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/exec-server-protocol/src/protocol.rs#L48-L55) [root-schema and fallback discovery](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/utils/plugins/src/plugin_namespace.rs#L43-L79)

**Implication for #20:** choosing an existing manifest lineage is a real compatibility decision. A clean vivy.suite/v1 format could be simple and explicit, while an Agent Plugins-compatible shape could improve exchange of some content. Neither option automatically supplies Vivy Rules semantics, Vivy trust policy, or Vivy lifecycle guarantees.

### 2.2 The component model is not a Rules bundle

The resolved Codex manifest has component paths for skills, onboarding skills, MCP servers, apps, and hooks, plus interface metadata. It does not expose a first-class rules or general instructions component. [resolved manifest construction](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/manifest.rs#L380-L395)

Codex project instructions are handled through the separate AGENTS.md mechanism and included in the base instructions. This means an upstream plugin package can add skills and hooks without becoming the owner of project instructions. [Codex AGENTS.md instruction entry point](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/protocol/src/prompts/base_instructions/default.md#L1-L30) [AGENTS.md documentation pointer](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/docs/agents_md.md#L1-L3)

**Implication for #20:** Rules need their own contract: are they plain instruction files, a scoped set of files, or a distinct rule language? The package must define when they are read, where they are injected, what project/user boundaries apply, and how they combine with existing instructions. Naming a directory rules does not answer those questions.

### 2.3 Identity and skill namespace

Codex plugin IDs are the pair plugin name plus marketplace name, serialized as plugin@marketplace. Both segments are validated before use as identity/path segments. [Codex PluginId parsing and serialization](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugin-common/src/plugin_id.rs#L9-L48) [segment validation](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugin-common/src/plugin_id.rs#L50-L79)

Plugin skill names are qualified with the plugin namespace and colon, for example sample:search. The resolver derives a namespace from the nearest valid plugin manifest above the skill path; deeper nested roots take precedence. The manifest's name is the child namespace. [namespace resolution contract](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/ext/skills/src/loader/namespace.rs#L11-L24) [nearest-root namespace selection](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/ext/skills/src/loader/namespace.rs#L118-L163) [qualified name construction](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/ext/skills/src/loader/namespace.rs#L166-L181)

The runtime plugin ID includes marketplace, but the skill namespace shown in the loader is derived from manifest name. Therefore, two separately installed plugins with the same manifest name from different marketplaces can have the same child skill namespace. The inspected code does not establish a marketplace-qualified child namespace.

**Implication for #20:** distinguish at least three values: stable installed suite identity, human-facing namespace, and a component's local name. If the user expects two sources to install independent packages with the same name, the collision policy must include source identity or reject one deterministically.

### 2.4 Codex hooks: broad event coverage with content-hash trust

Codex currently defines twelve hook events: PreToolUse, PermissionRequest, PostToolUse, PreCompact, PostCompact, SessionStart, SessionEnd, UserPromptSubmit, SubagentStart, SubagentStop, Stop, and Interrupt. [Codex event labels](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/hooks/src/lib.rs#L94-L109)

Plugin hooks can come from manifest paths, inline entries, or the default hooks/hooks.json file when there is no manifest hooks entry. Loaded hook source records retain plugin ID, root, source path, and relative path for attribution. [Codex plugin hook source discovery](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/loader.rs#L1202-L1309)

Codex does not treat visibility as execution admission. Discovery hashes the normalized handler and associates it with a persisted hook state. A built-in allowlisted hook is Trusted; managed hooks are Managed; other hooks are Trusted only when the saved trusted hash matches current content. A changed hash is Modified; no saved hash is Untrusted. The engine adds an enabled handler to the executable set only when trust is bypassed or the trust status is Managed/Trusted. [trust calculation and executable admission](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/hooks/src/engine/discovery.rs#L666-L735) [trusted-hash behavior](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/hooks/src/engine/discovery.rs#L796-L823)

Plugin handlers are marked as non-managed and carry plugin attribution into hook discovery. The same trust-state machinery is applied unless the caller explicitly enables bypass. [plugin hook source metadata](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/hooks/src/engine/discovery.rs#L244-L295) Executor-discovered plugin hooks are even more constrained in this snapshot: a code comment says manifests are not signed yet and only a known cleanup allowlist and remote Browser plugin are admitted. [executor hook admission note](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/executor_hooks.rs#L20-L30) [temporary allowlist filter](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/executor_hooks.rs#L86-L117)

The hooks registry exposes a reconfigured method that builds a refreshed hook configuration while preserving the command runtime and its in-flight background work. This demonstrates a runtime snapshot replacement seam for hooks. It does not establish a whole-plugin hot reload contract for skills, MCP, and other components. [Codex hook registry snapshot rebuild](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/hooks/src/registry.rs#L60-L100)

**Implication for #20:** a hook's package membership, enabled state, and trust admission are distinct concepts. Codex's evidence supports recording content identity and source attribution, but a Vivy suite still needs to decide whether arming is per hook, per suite, or both, and at what lifecycle boundary changes become effective.

### 2.5 MCP naming and collision behavior

Codex plugin manifests can contribute MCP configuration from a file or an inline object, and the standard Agent Plugin adapter defaults to ./mcp.json. The declared server names are not rewritten into a plugin namespace in the loader paths reviewed here. [Agent Plugin MCP default](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/agent_plugin_manifest.rs#L160-L181) [MCP loading and merge within one plugin](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/loader.rs#L1538-L1592)

Across plugins, loader discovery warns when it sees a duplicate MCP server name. The effective MCP map inserts active plugin definitions with first-writer-wins behavior. Configured plugin entries are sorted by key before loading, so the retained winner follows that deterministic order. This is collision handling, not isolation through namespacing. [duplicate-name warning](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/loader.rs#L188-L208) [effective MCP map](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/plugin/src/load_outcome.rs#L164-L174)

An MCP declaration can launch an external server process or point to a network server. The plugin loader is therefore a capability/configuration loader, not proof that suite contents are inert data.

**Implication for #20:** a suite namespace can prevent accidental cross-suite MCP replacement, but it must be applied before merging into the host-wide MCP registry. The namespace should be stable and traceable to suite identity; configuration and tool names should retain source attribution for inspection and permission handling.

### 2.6 Installation, versioning, and atomic activation

Codex uses PluginId-based storage and chooses an active installed version from the plugin's version directories. Its store keeps plugin data separately from versioned plugin contents. [active version lookup](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugin-common/src/installed.rs#L47-L68) [plugin store identity and data roots](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/store.rs#L120-L174)

Installation verifies that the manifest name agrees with the selected marketplace plugin name, validates the version path segment, and stages a copy before replacing the target. When replacing an existing root, it moves the old root to a backup, activates the staged root, and attempts rollback if activation fails. [install validation and activation](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/store.rs#L293-L340) [atomic staging and rollback](https://github.com/openai/codex/blob/cc7ba33601286a751477832b01de7b6f8de0dd9c/codex-rs/core-plugins/src/store.rs#L554-L690)

This is a useful failure-handling precedent for package replacement. The inspected code does not expose a single canonical digest over the full package tree as #20 proposes. Codex hook hashes cover hook identity/trust; they are not whole-package provenance digests.

### 2.7 Codex summary for #20

Codex's strongest relevance is component interoperability and safe admission patterns:

- A standard manifest can provide conventional skills and MCP locations, while an extension namespace carries product-specific additions.
- Plugin skills have an explicit qualified form.
- Hooks have per-handler identity, source attribution, hash-based trust, and a reconfiguration seam.
- Plugin installation is versioned and atomically staged.

Codex does not supply the proposed Vivy suite as a ready-made type. There is no first-class Rules field in the resolved manifest; MCP names are not isolated by plugin namespace; and the whole content set is not shown with one canonical provenance digest.

## 3. ZCode findings — secondary reference

### 3.1 Manifest shape and component surface

ZCode's own authoring spec uses .zcode-plugin/plugin.json. It documents skills and commands directories, hooks files, and MCP configuration by path or inline object, with paths constrained to the plugin root. [ZCode plugin authoring spec](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/plugin-creator-plugin/skills/plugin-creator/references/plugin-json-spec.md#L1-L17)

The plugin loader prefers .zcode-plugin/plugin.json, then .claude-plugin/plugin.json, then .codex-plugin/plugin.json. This is a ZCode compatibility policy: when manifests coexist, ZCode's own format wins, then Claude, then Codex. [ZCode manifest priority](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/docs/plugin-hooks-compat.md#L9-L17)

ZCode's component inventory can enumerate agents, commands, skills, hooks, and MCP. This inventory is not a promise that every group executes: the compatibility document labels manifest-declared agents diagnostic-only. UI surfaces and userConfig also exist in the broader ZCode ecosystem. That scope is substantially wider than #20's four proposed component families. [component groups](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/plugin-components.ts#L25-L48) [broader ZCode plugin compatibility model](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/docs/design/v2/plugin-compat.md#L249-L286)

ZCode enumeration is intentionally descriptive and read-only: listing component names does not start MCP servers or execute hooks. It avoids following symbolic links while scanning skill files, deduplicates files and names within the plugin component list, and reports diagnostics for invalid paths/events. [component enumeration boundary](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/plugin-components.ts#L20-L48) [skill component scan and local de-duplication](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/plugin-components.ts#L101-L124)

### 3.2 Identity and skill/MCP namespaces

ZCode persists plugin identity as plugin-name@marketplace. It stores settings such as enablement and options keyed by that ID. Explicit local plugin directories and installed marketplace plugins have different default-enable rules. [ZCode plugin identity](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/docs/design/v2/plugin-compat.md#L192-L243)

Plugin skills expose a qualified alias in the form plugin-name:skill-name while retaining the skill's own name. The alias derives the plugin name from the plugin root/manifest. [ZCode skill alias construction](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/skills/index.ts#L250-L262) [qualified names in adapter coverage](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/tests/plugins.test.ts#L4155-L4164)

ZCode rewrites a declared MCP server key to plugin:<manifest-name>:<server-name>. MCP configuration from .mcp.json and manifest.mcpServers is merged, with the manifest form overriding a same-name key from the file. [ZCode MCP namespace and loading](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/mcp.ts#L22-L69)

The plugin ID includes marketplace, but the component namespace is derived from manifest name. Two different marketplaces can therefore host different plugin IDs that share one manifest name; the shown skill and MCP component aliases do not include marketplace. The plugin adapter de-duplicates exact IDs, not all equal manifest names. That means the child namespace can collide even when persistent installation identities differ. [duplicate plugin ID check and runtime projection](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/index.ts#L151-L225)

This resembles #20's proposed suite:skill and suite-owned MCP namespace. The useful lesson is to make the child namespace explicit. The unresolved lesson is whether it is based on the artifact's stable ID, a display name, source identity, or a user-selected alias.

### 3.3 Hooks and the source/documentation discrepancy

ZCode hook discovery reads hooks/hooks.json and manifest hooks paths or inline definitions. It resolves file paths inside the plugin root, ignores duplicate paths, validates supported events, and emits warnings/diagnostics for invalid declarations. [ZCode plugin hook sources](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/hook-sources.ts#L22-L101)

The current adapter's canRunPluginHooks function returns true for all loaded plugins. Its comment says third-party marketplace hooks are allowed by default. Hook events are projected only in the enabled-plugin branch. [ZCode plugin enablement and hook projection](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/index.ts#L175-L210) [current hook admission function](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/index.ts#L371-L376)

The adapter test source creates an enabled plugin from a third-party marketplace with a command hook and expects the hook to be present and marked runnable. This corroborates the code path, although the test was not executed during this research. [third-party marketplace hook test](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/tests/plugins.test.ts#L3610-L3677)

This conflicts with ZCode's Claude marketplace compatibility plan, which says third-party marketplace hooks are diagnostic-only by default and must wait for an explicit trust/permission gate. [compatibility plan V1b](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/docs/claude-code-marketplace-plugin-compat.md#L27-L36) [plan component matrix](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/docs/claude-code-marketplace-plugin-compat.md#L165-L180) [plan trust boundary](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/docs/claude-code-marketplace-plugin-compat.md#L245-L253)

Other ZCode hook documents describe enabled marketplace command hooks as executable and warn that this grants local command execution and access to inherited environment variables. [plugin hooks trust note](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/docs/plugin-hooks-compat.md#L19-L38)

The current runtime's project/workspace hook boundary is separately documented: workspace hooks are isolated as a candidate and require digest-backed exact Trust/policy admission. Plugin hooks share plugin enablement and are injected for new sessions. [ZCode hook source ownership and trust model](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/docs/hooks-runtime-contract.md#L31-L78)

The best-supported description of the pinned runtime behavior is therefore: **enabled third-party marketplace plugins can have runnable hooks in the current adapter; separate project/workspace hook declarations have a trust-admission path; internal compatibility documents disagree about whether marketplace plugin hooks should be allowed.** This discrepancy should be resolved before treating ZCode as a security-policy reference.

For #20, Codex's hash-based hook trust is a more conservative precedent than the current ZCode plugin adapter. ZCode is still useful as a lifecycle and attribution reference. Either way, suite enablement and hook execution authorization should not be assumed to be the same decision.

### 3.4 MCP configuration and external effects

ZCode accepts MCP configuration from .mcp.json and manifest.mcpServers. The manifest can contain the server object directly or point to a file. Manifest definitions override same-name file definitions within one plugin. Each effective server is renamed into the plugin namespace before injection. [MCP definition merge and runtime names](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/mcp.ts#L22-L69)

The loader supports server process and network configuration shapes, variable interpolation, and plugin option defaults. Its design document distinguishes sensitive secret sinks from model-visible command/argument fields and says starting an MCP process remains subject to the existing MCP trust/permission system. [MCP variable and secret boundary](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/docs/design/v2/plugin-compat.md#L418-L445)

The broad principle for #20 is that the suite can own a declared MCP configuration without owning the MCP process lifecycle, credentials, transport, or permission model. Those remain host concerns. Suite provenance should survive the adapter so diagnostics and permissions can name the owning suite.

### 3.5 Marketplace lifecycle, source identity, and data

ZCode installs dependency closures, resolves plugin sources, stages cache activation, persists installed records, and rolls back staged directory activations if installation fails. The record includes the stable plugin ID, version, install path, scope, dependencies, and source description. [marketplace install and rollback](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/marketplace.ts#L790-L849) [atomic cache activation and installed record](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/marketplace.ts#L1325-L1387)

ZCode accepts local directories, git/GitHub sources with optional ref/path and source identity pin, and URL-backed ZIP sources. ZIP sources require a 64-character SHA-256 value, verify the downloaded archive hash, and enforce download/extraction limits. [source resolution](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/marketplace.ts#L1431-L1478) [ZIP size/hash constraints](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/zip-source.ts#L14-L41) [ZIP SHA-256 verification](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/zip-source.ts#L86-L129)

The installation cache and per-plugin durable data are separated. Removing a plugin deletes the install record and cache; by default it also removes the plugin data directory, with a keep-data option. Discovery recomputes component registrations from installed records, so removing the record and cache is the unregister operation. [cache/data layout and teardown](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/docs/design/v2/plugin-compat.md#L93-L129) [uninstall behavior](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/packages/adapters/src/plugins/marketplace.ts#L851-L872)

Removing a marketplace source does not remove its installed plugins. The ADR explicitly retains the installation, configuration, enablement, cache, and data as an orphaned install; update is unavailable until the same source is re-associated, and the user can still uninstall it. [orphaned install decision](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/docs/adr/0003-marketplace-source-removal-keeps-installed-plugins.md#L12-L39)

ZCode's ZIP checksum and remote source pin support integrity of particular sources. They do not by themselves prove one canonical digest of all normalized suite files, including a local directory install. The identity and provenance contract in #20 remains a separate decision.

### 3.6 Rules and effective reload boundary

The ZCode manifest/component schema inspected here lists skills, commands, hooks, MCP servers, userConfig, and broader plugin-specific surfaces; it does not provide a first-class rules component. ZCode's user/workspace instruction handling is separate. [supported plugin fields](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/docs/design/v2/plugin-compat.md#L249-L286)

ZCode plugin enablement recomputes loaded components, but its plugin compatibility document says the current session does not hot-reload plugin-provided skills, commands, or MCP servers; a changed set applies on the next session. The hooks runtime contract likewise says each session captures a hook configuration snapshot, and a configuration edit is guaranteed for new sessions. [plugin next-session activation](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/apps/zcode-cli/docs/design/v2/plugin-compat.md#L484-L492) [hook session snapshot](https://github.com/zai-org/ZCode/blob/aac4755666d09fdcd70272fcf063c077a639015f/docs/hooks-runtime-contract.md#L50-L54)

This is an explicit, understandable update boundary. It is not the same as live mutation in an already-running agent turn. For #20, “hot reload” should say which inventory is frozen for a run, when staged updates activate, and how multiple component owners switch together without exposing a mixed version.

### 3.7 ZCode summary for #20

ZCode is the closer reference for a whole-pack product surface:

- A stable marketplace-scoped plugin identity, persisted enablement, and options.
- Multiple source types, source identity metadata, and ZIP integrity checking.
- Versioned install cache, atomic activation, dependency closure, rollback, uninstall cleanup, and orphaned installs.
- Component enumeration and user-visible diagnostics separated from component execution.
- Namespaced skill aliases and MCP server keys.

Important qualifications are that ZCode's child namespaces use manifest name rather than the full marketplace-scoped ID, rules are not a plugin component, current plugin hook trust behavior conflicts with its compatibility plan, and component changes generally apply to new sessions.

## 4. Side-by-side mapping to #20

| #20 concern | Codex at pinned commit | ZCode at pinned commit | What remains a Vivy decision |
| --- | --- | --- | --- |
| Manifest lineage | Standard Agent Plugins schema plus legacy Codex manifest and Codex extension namespace | ZCode manifest with Claude/Codex discovery fallback and ZCode-specific compatibility rules | Choose one canonical Vivy schema; define whether/how other manifests translate |
| Skills | Conventional skills directory; plugin-name:skill-name; nearest manifest namespace | Skills roots; plugin-name:skill-name qualified alias; component enumeration de-duplicates within a plugin | Define logical name versus directory name, and collisions between same-named suites |
| Rules | Not a plugin-manifest component; AGENTS.md instruction path is separate | Not a plugin-manifest component; user/workspace instructions are separate | Define content format, instruction owner, scope, precedence, and read timing |
| Hooks | Twelve events; plugin source attribution; trust hash/managed/builtin/bypass admission; reconfigure seam | Manifest/default hook discovery; enabled plugins inject hooks; current adapter allows third-party hooks; docs conflict | Decide supported events, wire format, trust/arming, source attribution, and run-boundary activation |
| MCP | File or inline; declared names global; duplicates warned; first effective definition wins | File or inline; runtime key plugin:name:server; config variables/options; existing MCP trust boundary | Namespace by stable suite identity and preserve host permission/credential ownership |
| Install identity | plugin@marketplace plus versioned store | plugin-name@marketplace plus version/cache/data state | Separate suite identity, source identity, and user-facing alias |
| Update/atomicity | Version selection and staged replacement with rollback | Dependency closure; staged atomic activation and rollback | Define install/update/disable/uninstall state transitions for local/git sources |
| Orphan source | Store/source behavior is marketplace-driven; no same orphan contract established in inspected excerpts | Removing source leaves installed package manageable and usable, but not updatable | Decide whether deleting source uninstalls, or preserves orphaned suite |
| Hot reload | Hook registry can reconfigure; no evidence here of one all-component hot-reload guarantee | Plugin skills/commands/MCP changes apply to next session; hook config is a session snapshot | Define suite/run snapshot, idle activation boundary, rollback and in-flight semantics |
| Provenance | Hook content hash; versioned install; no whole-package digest established here | Git/source identity pins and ZIP SHA-256; no universal canonical directory digest established here | Define exact files, normalization, symlink handling, and digest algorithm/meaning |
| Executable effects | Hooks and MCP can invoke host capabilities; hooks require admission | Command hooks and MCP launch/connect effects; marketplace hooks currently admitted when enabled | State plainly that no new Go/UI code does not erase local command or network authority |

## 5. Implications for the Vivy proposal

### 5.1 Preserve Vivy's owner boundaries

The useful commonality is not “Vivy should become a Codex/ZCode plugin host.” Both upstreams show packages projecting content into existing host systems: skills are read by a skill loader; MCP configs feed an MCP manager; hooks enter a hook runtime. That matches #20's stated direction of reusing Vivy's existing component owners.

The corrected #20 describes a missing bundle/lifecycle layer rather than missing underlying Skills, instructions, MCP, or hook capabilities. Keep the suite loader and lifecycle as an adapter/orchestrator. Do not create a second runtime, policy path, MCP registry, skill source of truth, or hook executor to make the bundle convenient.

### 5.2 Treat namespaced identity as a contract

The upstream evidence supports qualified child names. It also shows a weakness to avoid: both projects can use a plugin manifest name for the visible component namespace even when the persistent plugin ID includes marketplace. For Vivy, decide:

- What uniquely identifies an installed suite when two sources provide the same declared name.
- Whether the logical child form is suite:skill, suite:rule, suite:hook, and suite:server, or whether only some components need qualified names.
- How bare-name compatibility aliases work and what happens when two suites expose the same bare name.
- Whether renaming the display name changes runtime IDs. It should not be assumed to.

The #20 correction already notes that Vivy's current skill-name validator rejects colons. The Codex and ZCode examples therefore imply an adapter between logical qualified ID and physical/backend identity, not permission to change the backend validator or directory syntax without a separate decision.

### 5.3 Keep hook authorization independent from package enablement

Codex demonstrates that a handler can be discovered and visible while still untrusted. ZCode demonstrates that conflating enablement and execution can expose local command execution to any enabled third-party package; at the pinned snapshot, this is the actual adapter behavior despite a contrary compatibility plan.

Before a suite hook is admitted, define the trust principal and gesture: install source, content digest, explicit suite enable, per-suite hook approval, per-handler trust, or a combination. Define what invalidates approval after file edits or an update. The #20 proposal's “arming stays a separate human gesture” aligns with the separation found in Codex, but the exact Vivy Hook approval mechanism and hot-reload boundary still belong to #20.

The #13 follow-up is closed as not planned; #20 now owns suite-specific hook decisions. This research does not revive #13's cancelled implementation scope or webhook work.

### 5.4 State the real side-effect boundary

If the suite promises not to ship Go/UI code, it can still carry:

- A hook command that starts a local program.
- An MCP process configuration that executes a program.
- A URL/server configuration that contacts a remote service.
- Skills and rules that alter model behavior and may influence tool use.

The manifest should make these capabilities inspectable before activation. The host should continue to own process launch, network access, secrets, permission checks, and hook trust. Whether v1 permits command hooks at all is an open scope/security decision, not something implied by “content-only.”

### 5.5 Define a real package digest

Codex's trust hash is per hook identity. ZCode's ZIP hash verifies an archive, and its Git source pin identifies a source revision. A Vivy whole-suite digest would need a written canonicalization rule: included component files, manifest inclusion, path normalization, file ordering, line-ending treatment, symlink policy, generated/cache exclusion, and whether source metadata is covered.

Keep the digest's purpose precise. Integrity of installed bytes, trust approval, provenance display, and run-level identity are related but distinct. A single digest can be reused as an input to multiple decisions, but that does not mean the decisions have the same lifecycle.

### 5.6 Define coordinated activation, not just file watching

Codex exposes hook registry reconfiguration but does not show one transaction for every plugin component. ZCode documents next-session activation for plugin components. Vivy's #20 identifies hook-chain replacement as the known integration gap and proposes run-frozen suite identity.

The suite contract should define when a complete new component snapshot becomes active, which operations are allowed mid-run, how a partially failed projection rolls back, and whether the next run gets either the full old suite or the full new suite. A filesystem watcher alone would not specify those semantics.

### 5.7 Keep the first research-to-implementation handoff narrow

This spike does not authorize implementation. When #20 is ready for a plan, the narrowest decisions to settle first are:

1. Canonical manifest and supported local/git source forms.
2. Stable suite identity, logical component namespace, and collisions.
3. Skill and MCP adapters into existing owners, with source provenance preserved.
4. Whether Rules are in v1 and, if so, their exact mapping into existing instruction ownership.
5. Whether Hooks are in v1; if so, the event/schema subset, trust admission, and snapshot activation contract.
6. Digest definition and install/update failure semantics.

Marketplace discovery, userConfig UI, and broad compatibility with multiple upstream plugin formats can remain separate decisions unless a concrete v1 consumer requires them.

## 6. Open questions to carry in #20

1. **Manifest lineage:** Is vivy.suite/v1 canonical, or does Vivy accept a standard plugin manifest and translate it? Which standard version is supported, and which unknown fields fail versus diagnose?
2. **Identity:** Does suite identity include source, or does the same suite ID intentionally resolve to a mutable source? Is source revision part of installed identity or provenance only?
3. **Child naming:** Does the namespace use stable suite ID, manifest name, marketplace/source, or a user alias? Can display names differ safely?
4. **Skill collisions:** How does logical suite:skill map through the current validator and skill host? Are bare-name aliases allowed? What is the deterministic behavior on conflict?
5. **Rules semantics:** Plain instruction documents or a separate rule model? Which roots and precedence? Per-project, per-user, or both? When are contents read?
6. **Hook contract:** Which events beyond the current Vivy PreToolUse surface are included? Which wire protocol? What trust gate? Does approval pin a whole suite digest or individual hook content?
7. **MCP contract:** Are server keys always namespaced? Which source types and transports are allowed? What host-owned secret and permission checks remain authoritative?
8. **Lifecycle:** Which v1 sources are supported? Are source-removal installs orphaned or uninstalled? What data survives updates and uninstalls? What is the atomic activation point?
9. **Hot reload/run consistency:** Are changes visible only to new runs? How does hook-chain replacement align with skills, instructions, and live MCP ReplaceServers?
10. **Canonical digest:** What files and normalization rules are included, and is the digest used for display, integrity, trust, or all three with separately stated semantics?
11. **User configuration:** Is a suite-level options surface necessary for the target use cases, or does it expand v1 beyond the minimum content pack?

## 7. Source notes

Primary sources are the pinned code and repository documents linked inline above. Specific behavior claims about ZCode third-party hooks were cross-checked against the source comment and test source, and against both sides of the conflicting documentation. No claim in this report should be read as an endorsement of the permissive ZCode hook policy.

No source code was changed. No builds or tests were run. The Codex and ZCode clones are research checkouts under the shared workspace’s .workspace directory, outside the agent-vivy Git worktree. The report is an uncommitted documentation addition in agent-vivy, consistent with the repository's attribution rule for AI-assisted contributions.
