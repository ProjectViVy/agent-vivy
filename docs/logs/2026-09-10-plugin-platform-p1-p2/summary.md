# 2026-09-10 — Plugin platform P1/P2

## What changed

- Added the v1 Module Descriptor, Port catalog, compiler, grant resolution,
  lifecycle graph, sealed manifest, and generated binder foundation.
- Added focused Tool, ToolWorld, Channel, and Face Provider contracts.
- Converted every first-party Channel, Face, and external ToolWorld source to
  `vivy.module/v1` with typed constructors and standalone conformance tests.
- Replaced the hand-maintained Channel/Face registers with one generated
  executable `RuntimeAssembly` consumed by the app composition root. Its
  generated owners execute Construct/Start/Ready and reverse Stop/Close;
  generated Tool, ToolWorld, Channel, and Face providers are the runtime
  selection authority.
- Cut `vivy-sdk` to v1-only `verify`, Recipe-driven `pack`, and
  `inspect-artifact`. Packing uses a Go overlay so omitted Modules are absent
  from the linked binary, not merely hidden from its manifest.
- Bound the sealed Generation Manifest into the executable and made
  `inspect-artifact` compare it byte-for-byte with the sidecar. The seal covers
  source identities, dependency locks, UI artifacts, compiled I18N catalogs,
  capability states, grants, Port edges, and lifecycle order.
- Added deterministic source-tree/ref verification, Go AST capability and
  import checks, linkability checks, atomic publication, and temporary
  `go.mod` binding for independently versioned `--source` Modules.
- Required every T2 source to carry an authoritative Recipe pin, compiled from
  an immutable source snapshot, and sealed the resolved module build list and
  dependency sums before publishing the executable.
- Kept effective grant constraints in generated runtime bindings. ToolWorld
  filesystem roots and process commands, plus Channel secret names and network
  hosts/schemes/ports, are enforced by focused Hosts. Runtime Provider identity
  is checked against the compiled plan before application construction.
- Routed first-party Channel HTTP and websocket traffic through the granted
  Channel Host. Dingtalk and QQ now use Host-governed websocket/token clients;
  unsupported Telegram proxy replacement fails closed instead of bypassing
  the sealed transport.
- Made generated code explicitly bind one LSP instance into its ToolWorld,
  write-diagnostic, and language-server-status consumers. Artifact inspection
  now reads the framed linker value directly and never runs the target binary.
- Added one executable P1/P2 conformance suite covering registration, missing
  and duplicate Providers, incompatible versions, cycles, denied Grants,
  timeout/cancellation, unavailable instances, redaction, Inspect provenance,
  default behavior, rollback, and representative Host failures for the four
  promoted public Ports.
- Removed the unreleased v0 SDK, runtime registry, descriptors, and fixtures.
  A repository conformance test prevents their return.

## Behavior preserved

The default generation still contains the five established Channels, eleven
protected Tools, the MCP ToolWorld, and kernel-headless `vivy run` posture.
Channel settings, allow lists, envelopes, message limits, lifecycle, and Face
RPC behavior continue through the existing host implementations. ToolWorld
filesystem access remains workspace-contained and writes preserve the existing
file-version Journal seam. Every completed P2 Port now points to seven concrete,
repository-local evidence anchors rather than placeholder references.
