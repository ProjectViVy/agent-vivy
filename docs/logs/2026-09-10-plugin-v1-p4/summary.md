# Plugin v1 P4 Iteration Summary

Date: 2026-09-11 (phase closure; iteration opened 2026-09-10)

Status: **COMPLETE**

## Scope

This iteration records the PLG-P4 Task 7 generator and conformance slice on
top of the accepted Tasks 1–6 work and closes the phase. It is not a release
claim for unavailable environment venues; those limits are explicit below.

## Recorded

- The build-owned catalog now binds `ContextHost`, `SkillHost`, and `MCPHost`,
  their constructors, and typed ContextSource/SkillSource/MCP ToolWorld
  provider contributions. The default Recipe includes all three Hosts and
  first-party provider inventories.
- The compiler recognizes the ContextHost and SkillHost core owners; the
  source Ports have all seven required evidence anchors. SDK frontend binding
  conversion preserves the typed source/MCP binding metadata.
- Runtime Assembly generation emits typed ContextSource/SkillSource fields,
  provider slices, Manifest identities, and lifecycle owners only when a
  selected plan contains those Ports. The default `zz_default.go` was
  regenerated with the official generator; a minimal Pack's generated
  Assembly source omits selected Source/Host constructors, imports, fields,
  and Manifest edges. Common app/runtime Go packages remain shared by the
  packed command, so this P4 generator gate does not claim whole-binary
  package-symbol removal.
- App composition validates generated provider identities and routes typed
  ContextSource/SkillSource values through ContextHost/SkillHost. MCP Resource
  bridging remains explicit and lazy; MCP ToolWorld discovery remains behind
  ToolHost governance.
- ContextHost, SkillHost, MCPHost, compiler graph, default-generation, and
  minimal-generation conformance tests cover the Task 7 failure, authority,
  budget, provenance, cleanup, and inactive-network requirements.
- The stale dingtalk module source digest was synchronized with the canonical
  source tree so the official generator/source-catalog verification is
  reproducible.

## Phase closure and explicit environment limits

The equivalent full evidence required for P4 is complete: the full regular
`go test ./...` and `go vet ./...` suites pass; the focused MCP/app/rpc race
selectors and full `go test -race ./internal/runtime` pass; UI tests,
typecheck, and build pass; the official Assembly generator is reproducible;
and changed Go formatting plus `git diff --check` pass. The P4 plan records
Tasks 1–7 and the missing venue checks precisely.

The following are not claimed as passed: `just ci` is unavailable in this
environment; the split-browser Playwright smoke is blocked because Chromium
is not installed; live-network MCP smoke was intentionally not run; and the
full `go test -race ./internal/app` run remains blocked by the pre-existing
pinned Eino Claude stream race in
`TestLoopbackControlCompletesApprovedConversation`. These are environment or
upstream limitations, not hidden failures in the scoped P4 evidence.

## Task 6 review-hardening continuation (2026-09-11)

The pending review edits are preserved and scoped to MCP state/secret
boundaries. Deferred and disabled instances now fail closed before legacy
backend acquisition, including ListTools, resource, prompt, GetPrompt, and
acquire paths; RPC Probe and resource reads short-circuit the same way.
Disabled state takes precedence over deferred and transport diagnostics in
MCPHost/backend, RPC, sidebar fallback, and UI projections. Config/settings
and the MCP upsert RPC reject credential-bearing HTTP endpoint URLs (userinfo
and credential-shaped query parameters) before persistence; public endpoint
projections omit invalid legacy values as a second boundary.

No live-network MCP smoke was used. The focused Playwright test remains
environment-blocked by the missing Chromium executable.

## Task 7 review continuation (2026-09-11)

The MCP Resource -> ContextHost bridge now checks the sealed generated
Assembly before asking the backend for a Resource Source. A Generation that
omits `vivy/context-host` cannot recreate that Host through config, settings,
or RPC; startup/overlay validation and the live settings write reject a
Resource bridge in that shape. The compiler and both Assembly/Manifest
capability projections now require the build-owned typed `MCPHostProvider`
binding and the `core/mcp-host@v1` owner for the reserved `mcp` ToolWorld.

The stale `sdk/plugin/doc.go` marker was removed after confirming there are no
live Go imports of that path. The root layout no longer advertises the removed
author import window. Minimal-removal evidence is deliberately scoped to
generated Assembly source/constructor wiring and sealed Manifest edges; shared
common runtime packages remain outside this generator boundary.

## Task 4 ownership continuation (2026-09-11)

MCPBackend is now a configuration/status facade: it retains logical server
settings and secret-free status only, while MCPHost sessions create, initialize,
replace, retry, and close their own HTTP/stdio transports. A Host session closes
each initialized transport exactly once; replacing a live configuration retires
the old session before its replacement becomes active. EinoExt `GetTools` and
schema conversion run in the runtime adapter only after Host initialization;
resources, prompts, and direct control calls continue through Host, and
inactive/unconfigured reads remain no-connect.

The final replacement interleaving regression binds status to the exact
configuration generation: an old Host transport opened during the
`ReplaceServers` publication window cannot mark the new same-named record
ready.

The scoped MCP ownership tests, broader regular Go suite, runtime race suite,
focused race selectors, vet, formatting/diff, and generator reproducibility
checks pass. The phase-level `just ci` gate remains unavailable in this
environment, and no live-network MCP smoke was used.
