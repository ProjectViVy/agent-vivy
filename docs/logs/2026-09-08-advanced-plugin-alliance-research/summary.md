# Summary

## What changed

- Added `docs/research/advanced-plugin-alliance-2026-09-08.md`.
- Compared Vivy with pinned Eino v0.9.13, Hermes Agent, DeepSeek Harness/Cordis, OpenFang, ZeroClaw, OpenClaw, and Pi.
- Proposed `Module + Port + Generated Assembly`: a shared assembly model for internal and pluggable modules with different trust ceilings.
- Classified existing Vivy capabilities into ready, consumer-required, internal-only, and kernel-coupled groups.
- Defined an Eino-native reuse boundary and phased implementation path.
- Consolidated the later discussion into four product layers: immutable Kernel, Required Internal, Optional Internal, and Pluggable Alliance Products.
- Added the OpenAI/Anthropic/OAuth boundary, Web/TUI/A2UI extension reservations, the normative standard set, and quantified delivery ranges.
- Collapsed the living-board entry to one proposal sentence: PLUGINS full completion awaits formal scheduling.

## Scope

Documentation and architecture research only. Vivy kernel/species scope; no Studio shell changes.

## Explicitly not done

- No production code or tests changed.
- No plugin architecture contract was silently promoted from proposal to accepted.
- No implementation story was authorized or marked complete.
- No runtime plugin loading, WASM host, or sidecar implementation was added.
