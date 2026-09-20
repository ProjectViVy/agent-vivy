# CH-P0-1 contract and conformance foundation

Baseline: `main@9e6db43c81e1d7ce249ee785f1b6efd91a09c5bb`.

This delivery freezes the first implementation slice of issue #42 without
moving Channel runtime ownership.

## Delivered

- `CHANNEL-MODULARIZATION.md` defines CH-01 through CH-12, the canonical
  `vivy/channel-host` T1 owner, `core/channel-host@v1` cardinality `0..1`,
  lifecycle/absence semantics, and the five-slice scope fence.
- The Module, Port, and Assembly normative contracts now describe ChannelHost
  as a removable canonical build-owned T1 Module whose authority remains an L0
  rule.
- `internal/channelcontract` provides the inert Factory/Owned dependency seam,
  including distinct compiled and process-available state.
- `internal/rpc` provides typed method contributions with deterministic
  validation and no dispatcher attachment.
- The closed internal Port catalog records that `std/channel@v1` conditionally
  requires `core/channel-host@v1`.
- Conformance tests cover a Host with zero Providers, Provider without Host,
  duplicate Host, public core Provider, and lossless omitted-Provider settings.
- Backend and frontend types pin the optional, secret-free
  `ui_extensions: [{id, enabled}]` wire shape without populating or consuming
  it.

## Explicitly not delivered

- No real Module-owned Host construction, Provider binding, configuration
  adapter, lifecycle migration, or management-handler relocation (CH-P0-2).
- No reduced Recipe, optional generated import, physical backend omission, or
  Inspect implementation (CH-P0-3).
- No Channel UI move, settings-section registry, visibility behavior, or asset
  omission (CH-P0-4).
- No release-selection matrix, packaged omission proof, or browser/network
  acceptance run (CH-P0-5).

The default runtime paths, generated Assembly, existing Recipes, Channel UI,
and `internal/channelhost` implementation remain unchanged.

