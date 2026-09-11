# Plugin v1 P4 Iteration Acceptance

Date: 2026-09-11 (phase closure; iteration opened 2026-09-10)

Status: **COMPLETE**

This is the PLG-P4 phase acceptance record. The following evidence closes the
phase; unavailable environment venues are recorded as limitations rather than
claimed as passed.

| Acceptance item | Evidence | State |
|---|---|---|
| Pinned Skill, Agents.md, and Tool APIs are cited with ADAPT decisions | `docs/research/plugin-v1-eino-capability-check.md` and focused runtime tests | RECORDED |
| Pinned EinoExt MCP `GetTools` use is separated from transport/session ownership | Research note's EinoExt and current-vs-target sections; `internal/runtime/mcpadapter.go` | RECORDED |
| Pinned mcp-go OAuth primitives are acknowledged without claiming governed support | Research note's OAuth section; decision remains DEFERRED-INDEFINITE | RECORDED / DEFERRED |
| Host-owned MCP session cleanup, including the compatibility factory's shared-facade boundary, is documented | Research note and `internal/runtime/mcpadapter_lifecycle_test.go` | RECORDED |
| Default Generation compiles/starts ContextHost, SkillHost, and MCPHost with typed first-party provider inventories | Regenerated `internal/generated/assembly/zz_default.go`; `internal/app/default_generation_test.go` | RECORDED |
| Minimal Generation's generated Assembly omits selected Sources/Hosts, constructors, imports, fields, and Manifest edges | `sdk/internal/assembly/runtime_generate_test.go`; `sdk/internal/frontend_v1_test.go` Pack/Inspect test | RECORDED; shared common app/runtime package symbols are outside this generator boundary |
| Compiler rejects missing/duplicate Providers and incompatible source graphs; lifecycle cleanup remains ordered/idempotent | `sdk/internal/assembly/p4_conformance_test.go`; generated lifecycle integration tests | RECORDED |
| Seven evidence anchors exist for `std/context-source@v1` and `std/skill-source@v1` | `sdk/internal/assembly/evidence.go`; `TestP1P2EvidenceReferencesRepositoryFiles` | RECORDED |
| ContextHost/SkillHost/MCPHost conformance covers timeout, cancellation, unavailable, cleanup, redaction, deterministic identity, budgets, no-System/authority boundaries, explicit MCP bridge, prompt prohibition, and ToolHost traversal | `internal/contexthost/conformance_test.go`, `internal/skillhost/conformance_test.go`, `internal/mcphost/conformance_test.go`, existing Host suites | RECORDED |
| MCP status remains generated/not-compiled when MCP is omitted and compiled/unconfigured when selected | `sdk/internal/frontend_v1_test.go` shipped-recipe Pack/Inspect coverage; generated Manifest network state | RECORDED |
| Full P4 equivalent gate and phase exit evidence | Full regular Go/vet, focused/full scoped race, UI, generator, gofmt, and diff evidence recorded in `verification.md`; `just` is unavailable, full app race has the pinned Eino Claude race, and live-network/browser venues are unavailable | COMPLETE WITH LIMITATIONS |

No unavailable command or smoke is represented as passed.

## Task 6 review-hardening acceptance (2026-09-11)

| Acceptance item | Evidence | State |
|---|---|---|
| Deferred MCP instances cannot activate through RPC Probe or legacy backend tool/resource/prompt/acquire paths | `internal/rpc/control_test.go`, `internal/runtime/mcp_backend_test.go`, `internal/mcphost/host_test.go` | RECORDED |
| Disabled precedence projects a disabled deferred instance as `inactive` | `internal/mcphost/host_test.go`, `internal/rpc/control_test.go`, `internal/runtime/mcp_backend_test.go`, `ui/src/components/mcp/McpView.tsx` | RECORDED |
| Credential-bearing endpoint userinfo/query values are rejected before persistence and never echoed in public endpoint output | `internal/config/config_test.go`, `internal/app/settings/settings_test.go`, `internal/rpc/control_test.go`, `internal/rpc/control.go` | RECORDED |
| Focused Go/UI regular, race, vet, formatting, and diff checks | Task6 report and this verification record | PASS |
| Real split-browser path | Playwright attempt with pinned `GO_EXE` | COMPLETE WITH LIMITATION: Chromium unavailable in this environment |

The phase-level unavailable venues and the upstream race limitation are
recorded in the phase-closure table below; they are not represented as passed.

## Task 7 review continuation (2026-09-11)

| Acceptance item | Evidence | State |
|---|---|---|
| MCP Resource bridging cannot recreate an omitted ContextHost | `internal/app/assembly_sources_test.go`, `internal/rpc/control_test.go`, startup/overlay assembly validation | RECORDED |
| MCP `mcp` capability is compiled only from typed MCPHostProvider + core Host binding | `sdk/internal/assembly/compiler_test.go`, `sdk/internal/assembly/runtime_generate_test.go`, `sdk/internal/frontend_v1_test.go` | RECORDED |
| Full regular Go suite no longer has the legacy marker blocker | `go test ./... -count=1`; `sdk/plugin/doc.go` removal conformance | PASS |

## Task 4 ownership continuation (2026-09-11)

| Acceptance item | Evidence | State |
|---|---|---|
| MCPHost owns initialized HTTP/stdio transports and closes each exactly once | `internal/runtime/mcp_host_bridge_test.go` (`TestMCPHostCloseOwnsInitializedHTTPTransport`, `TestMCPHostReplaceOwnsRetiredHTTPTransport`) | RECORDED |
| A retired Host generation cannot mark a same-named replacement ready during `ReplaceServers` | `internal/runtime/mcp_host_bridge_test.go` (`TestRetiredMCPHostGenerationCannotMarkReplacementReady`), regular and race selectors | RECORDED |
| MCPBackend has no transport ownership, no no-op-close session, or duplicate retry/close authority | `internal/runtime/mcpadapter.go`, `internal/runtime/mcp_backend.go`, `internal/runtime/mcpadapter_lifecycle_test.go` | RECORDED |
| EinoExt `GetTools` remains an adapter conversion seam after Host-approved initialization | `internal/runtime/mcpadapter.go`, MCP Host discovery tests | RECORDED |
| Live replacement, status, probe, resources, prompts, and direct control behavior remain Host-routed | `internal/runtime/mcp_host_bridge_test.go`, `internal/runtime/mcp_backend_test.go`, `internal/mcphost` Host suites | RECORDED |
| Full phase gate | Equivalent scoped/full Go, vet, UI, generator, race, formatting, and diff evidence recorded in `verification.md`; `just`, Chromium, live-network smoke, and the upstream full-app race remain unavailable | COMPLETE WITH LIMITATIONS |
