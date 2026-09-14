# Plugin v1 default parity and minimal-removal proof

Date: 2026-09-14
Scope: PLG-P9 Task 4

## Result

The P9 release baseline preserves the established P2 default body while the
minimal Recipe physically removes every optional Module contribution from the
generated Assembly and sealed Inspect projection. Network-capable Channel and
MCP instances remain compiled but inactive when configuration and credentials
are absent.

## Default inventory authority

`sdk/internal/testdata/default-generation.expected.json` is the exact,
secret-free inventory consumed by
`TestDefaultGenerationBaselineInventory`. It freezes:

- 25 selected first-party Modules;
- five Channel Providers;
- eleven protected Tools;
- the `mcp` ToolWorld;
- the `openai` and `anthropic` Provider Profiles;
- the first-party Context and Skill Sources;
- the `kernel-headless` default Face posture; and
- `UNCONFIGURED` state for Channel and MCP networking.

The P9 expansion adds Module, Action, Context Source, Skill Source, and Run
Observer inventories to the original P2 comparison. UI/control behavior is
represented by the selected `vivy/presentation-host` and `vivy/action-host`
Modules and remains covered by the P6 conformance and browser gates.

## Minimal removal boundary

The minimal Recipe retains only the seven closed-kernel owners required to
construct a Generation: loop, model, ToolHost, storage, checkpoint,
credential, and sandbox. The removal test packs and inspects the real minimal
artifact, then proves that omitted Modules leave no:

- generated import, constructor, or Provider binding;
- Manifest Module or Port edge;
- effective Grant;
- selected-Provider conformance record;
- UI root, extension, source hash, dependency lock, or Provider asset; or
- catalog/localization projection.

Release-wide `portSupport` records remain visible because they describe the
compiler's public Port catalog, not Modules selected into the minimal
Generation. They contain only build-owned repository-relative evidence IDs.

## Executed checks

```text
go test ./internal/app -run 'TestDefaultGeneration(LeavesUnconfiguredNetworkInactive|BaselineInventory|ProviderProfilesAreGeneratedAuthority|ComposesEstablishedOptionalHosts)' -count=1
go test ./sdk/internal -run TestMinimalArtifactPhysicallyOmitsOptionalModules -count=1
```

Both checks pass. Final packed artifact identities and executable smoke results
are recorded in the PLG-P9 release log after the complete product gate.
