---
name: vivy-plugin
description: Develop a Vivy v1 Module or Port, change Recipe composition, or use SDK verify, pack and Inspect. Reuse established Port contracts for ordinary Providers; prose mentions and instruction-only edits do not trigger the development workflow.
---

# Vivy plugin v1 development

## Core rule

Build one typed contribution to one Vivy body. Preserve the single generated
Assembly, `Service.Run`, Journal, Policy, and Host path.

Read the affected contract, not the entire platform documentation set:
`docs/architecture/VIVY-MODULE-STANDARD.md` for Module boundaries,
`VIVY-PORT-CATALOG.md` for a Port contract, `VIVY-PLUGIN-SPEC.md` for SDK behavior,
and `VIVY-ASSEMBLY.md` for composition/compiler changes. Consult the active
`docs/plans/plugin-platform/` task only when changing that planned capability.
Root AGENTS.md owns shared process, validation and delivery policy.

## Route the work

| Change | Route |
|---|---|
| Verification needs host or Recipe guidance | Consult `vivy-kernel-ci` |
| New/replacement Eino capability or changed upstream interface | Consult the relevant API through `vivy-eino` |
| Web UI implementation needing frontend guidance | Use the applicable `oil-frontend` rules |
| Public Module using a supported Port | Stay in its source boundary; verify, pack the explicit Recipe, and inspect |
| Port is only `SPECIFIED` or phase is `UNSCHEDULED` | Stop implementation and report the unmet Gate; never fall back to v0 |

## Development decisions

For a new public Port, establish the following in its existing design. A normal
Provider using an established Port reuses that contract and records only changed
behavior, Recipe and checks; no duplicate platform dossier is required.

New Port decisions:

1. Module ID, T1/T2/T3 classification, and source pin;
2. existing cataloged Port and exact version;
3. Provider, sole Host Consumer, authority owner, and cardinality;
4. requested Grants and Recipe constraints;
5. lifecycle, timeout, cleanup, and failure result;
6. Definition, SDK, Provider, Consumer, Failure Model, Conformance, and Inspect
   artifacts;
7. meaningful checks for the changed contract and representative failures.

New or replacement runtime adapters follow the Eino decision order in root
AGENTS.md. Small fixes reuse the existing decision and inspect affected APIs;
they do not repeat full framework research.

## Non-negotiable boundaries

- No `vivy.plugin/v0`, `Seam`, God `Plugin` interface, compatibility Adapter,
  migration command, runtime discovery, hand-edited generated wiring, or
  `engine.go` plugin import.
- Public Modules provide only cataloged `std/*` Ports; they never provide
  `core/*` or access raw Journal, Policy, storage, credentials, RPC server, or
  Eino types.
- Protected Tool IDs cannot be shadowed, aliased, replaced, or overridden.
- All Tool sources use ToolHost. Channels, Faces, Context, Skills, MCP,
  Middleware, Observers, Status, and Actions use their named Host.
- Explicit Recipes select first-party features; preserve existing shipped
  selections. Unconfigured network instances stay inactive.
- Selected UI Modules have complete UI control by default. Do not add a UI
  Grant or permission prompt. The backend still distrusts every browser claim.
- Same-process T2 code is trusted, not sandboxed. Untrusted execution is T3.

## Placement rule: universal stays internal

Keep required core capabilities internal. A new Module needs an actual consumer
or variation boundary that justifies its descriptor, source pin and binding
maintenance. Being first-party or currently selected by the default Recipe does
not make a capability mandatory in every Generation.

Plugin categories exist for capability that a real Recipe boundary drops:

| Category | Holds | Examples |
|---|---|---|
| `internal/` | Required kernel ownership | protected tools and shared Hosts |
| `plugins/infra/` | Local daemon discovery/health/lifecycle | infra/llm (ollama, llama.cpp…), future asr/tts/vad |
| `plugins/coding/` | Programming-specialized capability | lsp, session-tree, coding tools |
| `plugins/provider/` | Cloud provider/credential adapters | future OAuth login flows |

If a universal tool later gains a confirmed variation boundary (e.g. an
offline profile that must drop http_request), extract that one tool then —
the door is never welded, but do not pre-pay modularity for hypothetical
flexibility.

## Example decision

Request: “Publish another `read_file` plugin.”

Result: reject the reserved identity. If the intent is a distinct analyzer,
define a namespaced `std/tool@v1` Provider, request scoped `fs.read`, pass the
ToolHost conformance suite, add it to an explicit Recipe, rebuild, and verify
its source/reference/Grant in Inspect.

## Completion

Verify changed behavior and representative failures; add a regression for a
reproducible bug. Instruction/prose-only changes need relevant static checks.
A browser refresh never installs a Module.

From the repository root, the supported quick workflow is:

```text
go run ./sdk verify plugins/hello-fs
go run ./sdk pack --recipe examples/minimal-plus-hello-fs/recipe.vivy.yml --output <new-output-directory>
go run ./sdk inspect-artifact <output-directory>
```

`pack` requires both `--recipe` and `--output`; the output path must not exist.
Add one `--source <module-directory>` for each external source selected by the
Recipe. Inspect consumes the artifact directory, parses the sealed executable
without running it, and reports build-owned Port support and conformance
evidence.

For changes to Generation failure/rollback or shared Host authority, use the
affected cases below. Ordinary Provider changes use their Host/Port checks and
explicit Recipe rather than this entire platform matrix:

```text
go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1
go test ./sdk/internal/conformance -run 'TestGeneration(FailureMatrixEvidence|RollbackRestoresCatalogAndLocaleIdentity)' -count=1
go test ./sdk/internal -run 'Test(GenerationFailureMatrixExecutesEveryCase|MinimalArtifactPhysicallyOmitsOptionalModules)' -count=1
go test ./sdk/internal/assembly -run 'Test(CompilePluginV1GraphFixtures|StartFailureRollsBackEveryConstructedOwner)' -count=1
go test ./internal/toolhost -run 'TestMiddleware(TimeoutFailsClosed|PanicAndInvalidDecisionFailClosed)' -count=1
```

Use root AGENTS.md for validation and proportional records. Verify the actual
selected product path when behavior changes.

`verify` is static and never starts Provider code. `pack` consumes the
checked-in behavior-conformance result bundle; it does not execute
the suites. Unattested third-party Providers remain packable but receive no
conformance record in Inspect. The bundle makes no source-authenticity claim.
Updating that shared bundle requires its own reproduction checks; a Provider
change that leaves it unchanged does not acquire a release-evidence ceremony.
