# PLG-P4 Context, Skill, and MCP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose Context and Skill Sources through internal Hosts and route T3
MCP Server tools/resources through MCPHost into the same ToolHost and
ContextHost paths.

**Architecture:** Public Sources provide bounded Vivy domain data. Internal
Hosts own authorization, composition, provenance, budgets, lifecycle, and
failure. Only `internal/runtime` adapts approved content and tools to pinned
Eino/EinoExt APIs.

**Tech Stack:** Go, Eino v0.9.13 ADK Skill/agentsmd interfaces, EinoExt MCP
v0.0.9, current MCP transport dependencies, `just ci`.

**Spec:** `docs/architecture/VIVY-PORT-CATALOG.md` sections 7–8 and
`docs/architecture/VIVY-MODULE-STANDARD.md` section 10.

## Global Constraints

- State: `UNSCHEDULED`; depends on P3.
- Context and Skill Sources never write the final Prompt or Eino Message.
- MCP Server instances are T3 and never enter the native Module graph.
- Existing configured/unconfigured semantics and current upstream-backed MCP
  transport behavior remain; no new protocol stack is introduced.
- A missing pinned Eino/EinoExt MCP, OAuth, RAG, or Skill capability is marked
  `DEFERRED-INDEFINITE`.

---

### Task 1: Record the pinned Eino capability decision

**Files:**

- Create: `docs/research/plugin-v1-eino-capability-check.md`
- Modify: phase iteration `verification.md`

**Interfaces:**

- Consumes: `go.mod`, pinned module source, and current runtime adapters.
- Produces: API-by-API `ADAPT` or `DEFERRED-INDEFINITE` evidence.

- [ ] Verify and cite `skill.NewMiddleware`, `skill.Backend`, `agentsmd.New`,
  Eino Tool types, and EinoExt `mcp.GetTools` in the exact pinned versions.
- [ ] Inspect pinned MCP OAuth support rather than relying on latest upstream
  documentation.
- [ ] Record missing OAuth or transport capabilities as indefinite deferrals;
  add no placeholder implementation task.
- [ ] Confirm all adapter imports remain under `internal/runtime`.
- [ ] Commit `docs(eino): verify plugin v1 source adapters`.

### Task 2: Define Context Source and ContextHost

**Files:**

- Create: `sdk/port/contextsource/contextsource.go`
- Create: `sdk/port/contextsource/contextsource_test.go`
- Create: `internal/contexthost/host.go`
- Create: `internal/contexthost/host_test.go`
- Create: `internal/runtime/contextadapter.go`
- Modify: `internal/runtime/context.go`
- Modify: `internal/runtime/file_context.go`
- Modify: `internal/rpc/project_context.go`

**Interfaces:**

- Consumes: namespaced source queries and workspace/session identity.
- Produces: authorized, deduplicated, ranked, redacted, token-budgeted Context
  candidates with provenance.

```go
type Candidate struct {
    SourceID    string
    ContentID   string
    MediaType   string
    Content     string
    SizeHint    int
    Confidence float64
    Version     string
}
```

- [ ] Write `TestSourceCannotInjectSystemMessage`; expected RED is the absence
  of a Vivy-only candidate boundary.
- [ ] Write tests for workspace escape, duplicate content, source timeout,
  result-size overflow, token budget, Secret redaction, and stable provenance.
- [ ] Implement ContextHost without Eino imports.
- [ ] Adapt final candidates to Eino schema only in `contextadapter.go`.
- [ ] Register current project/file Context behavior as first-party default
  Sources with no behavior change.
- [ ] Run `go test ./internal/contexthost ./internal/runtime ./internal/rpc -run Context`.
- [ ] Commit `refactor(context): host typed context sources`.

### Task 3: Define Skill Source and SkillHost

**Files:**

- Create: `sdk/port/skillsource/skillsource.go`
- Create: `sdk/port/skillsource/skillsource_test.go`
- Create: `internal/skillhost/host.go`
- Create: `internal/skillhost/host_test.go`
- Create: `internal/runtime/skilladapter.go`
- Modify: `internal/runtime/skills_backend.go`
- Modify: `internal/runtime/always_skills.go`
- Modify: `internal/runtime/engine.go`

**Interfaces:**

- Consumes: versioned Skill metadata/content from public and first-party
  Sources.
- Produces: validated, conflict-resolved, budgeted Skill selections adapted to
  pinned Eino `skill.Backend` and `skill.NewMiddleware`.

- [ ] Write `TestSkillTextReceivesNoImplicitGrant`; expected RED proves content
  alone cannot authorize Tool/filesystem/network/Secret use.
- [ ] Write tests for duplicate Skill ID, hash change, disabled content,
  read-only source, invalid frontmatter, budget overflow, and provenance.
- [ ] Implement SkillHost without Eino types and preserve existing revision and
  CAS behavior.
- [ ] Adapt the Host behind the current Eino Skill backend interface only in
  `internal/runtime`.
- [ ] Keep `skills_list` and `skill_view` as protected internal Tools.
- [ ] Run `go test ./internal/skillhost ./internal/runtime -run Skill`.
- [ ] Commit `refactor(skill): host versioned skill sources`.

### Task 4: Separate MCPHost transport/governance from Eino conversion

**Files:**

- Create: `internal/mcphost/host.go`
- Create: `internal/mcphost/instance.go`
- Create: `internal/mcphost/host_test.go`
- Create: `internal/runtime/mcpadapter.go`
- Modify: `internal/runtime/mcp_backend.go`
- Modify: `internal/runtime/mcp_process_windows.go`
- Modify: `internal/runtime/mcp_process_other.go`
- Modify: `internal/app/app.go`

**Interfaces:**

- Consumes: configured MCP Server instances and their governed transport.
- Produces: namespaced dynamic ToolWorld entries and explicit Resource bridge
  candidates with server instance ID and schema hash.

- [ ] Write `TestMCPServerNeverBecomesNativeModule`; expected RED is any path
  that can add a T3 server to the native Module graph.
- [ ] Write tests for unconfigured no-connect, lazy activation, process death,
  timeout, retry bound, duplicate remote name, schema change, and cleanup.
- [ ] Move transport/session lifecycle behind MCPHost while preserving current
  configuration and process governance.
- [ ] Keep EinoExt `mcp.GetTools` conversion in `internal/runtime/mcpadapter.go`.
- [ ] Prohibit MCP Prompt automatic conversion to Skill.
- [ ] Mark unsupported pinned OAuth behavior unavailable/deferred rather than
  implementing it locally.
- [ ] Run `go test ./internal/mcphost ./internal/runtime -run MCP`.
- [ ] Commit `refactor(mcp): separate host governance and eino adapter`.

### Task 5: Bridge MCP tools and resources through standard Hosts

**Files:**

- Modify: `internal/mcphost/host.go`
- Modify: `internal/toolhost/catalog.go`
- Modify: `internal/contexthost/host.go`
- Create: `internal/mcphost/conformance_test.go`

**Interfaces:**

- Consumes: MCP tool definitions and resource results.
- Produces: ToolWorld discovery entries and opt-in Context candidates.

- [ ] Write a RED end-to-end test proving an MCP Tool must traverse ToolHost
  schema, Policy, Middleware, approval, bound, and Journal stages.
- [ ] Write a RED test proving an MCP Resource appears only after an explicit
  Context bridge configuration.
- [ ] Attach stable server instance, remote capability, and schema hashes to
  every projection.
- [ ] Reject reserved protected Tool IDs and ambiguous namespaces.
- [ ] Run `go test ./internal/mcphost ./internal/toolhost ./internal/contexthost`.
- [ ] Commit `feat(mcp): bridge tools and resources through hosts`.

### Task 6: Project configuration and status without activation

**Files:**

- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/app/settings/settings.go`
- Modify: `internal/rpc/control.go`
- Modify: `ui/src/components/mcp/McpView.tsx`
- Modify: `ui/e2e/mcp-settings.spec.ts`

**Interfaces:**

- Consumes: runtime instance lifecycle and StatusHost snapshots.
- Produces: distinct not-compiled, unconfigured, inactive, ready, unavailable,
  and deferred states.

- [ ] Write config parse/validation tests before adding any changed field.
- [ ] Write browser RED coverage showing a status read cannot connect or revive
  an MCP process.
- [ ] Preserve Secret references and redact missing-env details appropriately.
- [ ] Use the existing RPC authority; do not add arbitrary Module routes.
- [ ] Run focused Go/UI tests and the real split-browser path.
- [ ] Commit `feat(mcp): project governed instance states`.

### Task 7: Complete Source/MCP conformance

**Files:**

- Create: `internal/contexthost/conformance_test.go`
- Create: `internal/skillhost/conformance_test.go`
- Modify: `internal/mcphost/conformance_test.go`
- Modify: `internal/generated/assembly/zz_default.go` through the generator only

**Interfaces:**

- Consumes: default first-party Sources and MCPHost.
- Produces: Gate B proof for SCX consumers.

- [ ] Test missing/duplicate Provider, timeout, cancellation, unavailable
  instance, cleanup, redaction, provenance, and default inactive state for each
  Port.
- [ ] Inspect the default Generation and verify all three Hosts are compiled.
- [ ] Inspect a minimal Generation and verify omitted Sources and Hosts have no
  code or Manifest edges.
- [ ] Run `just ci` and required MCP real-path smoke without live-network unit
  dependencies.
- [ ] Commit `test(plugin): prove context skill and mcp conformance`.

## Phase exit and rollback

Exit requires one Host per capability, pinned upstream adapter evidence, no
Prompt/Tool/Run bypass, explicit MCP Resource bridging, and default inactive
network state. Rollback selects the prior Generation; missing upstream
capabilities remain deferred.
