# Remove Embedded Host and C ABI

## Scope

Remove the deprecated native Embedded Host, its C ABI, and the SDK's `shared`
and `go-host` pack targets. Remove the associated manifest provenance field and
active references. Preserve the ordinary executable pack path, generic
embedded Generation Manifest, and App lifecycle services owned by `App.Run`.

## Implementation

1. Delete the C ABI package, internal and public Embedded Host packages, FFI
   smoke harness, Go host packer/tests/fixture, and host-specific Generation
   provenance.
2. Remove the SDK target/parser/build/inspection branches and host-specific
   tests while retaining executable packing and inspection.
3. Remove `App.StartEmbeddedServices`; exercise lifecycle services through
   `App.Run` in lifecycle and memory-loop tests.
4. Update active README, recipe, and Laputa-bootstrap wording; record the
   delivery under `docs/logs/2026-10-10-remove-embedded-host-c-abi/`.

## Verification

- Run focused SDK and app tests, then the repository `just ci` gate.
- Scan active source/config/docs for removed symbols, packages, target names,
  and ABI files; historical delivery logs remain as historical records.
- Review the complete branch diff for accidental removal of executable
  packing, generic manifest embedding, and non-embedded lifecycle behavior.

## Baseline

The initial `just ci` attempt stopped in `ensure-laputa` because the default
GitHub proxy was unavailable. After retrieving the pinned source, the gate
passed bootstrap and formatting, then stopped in UI staging on compile errors
inside the locked Laputa revision; see the delivery verification log.
