# Acceptance

- The branch has no active C ABI exports, FFI smoke files, native Embedded
  Host package, public Go Host API, Go host packer, or `shared` pack target.
- `vivy-sdk pack` builds the ordinary VIVY executable; artifact inspection
  expects that executable and validates its sealed Generation Manifest.
- Generation Manifest no longer defines or accepts `hostBuild` provenance.
- Gateway-less background services start and stop under `App.Run`; lifecycle
  and memory-loop tests use that owner.
- The generic embedded Generation Manifest remains available to executable
  packing and inspection.

The mandatory full CI acceptance remains pending because the repository's
pinned Laputa source fails to compile as recorded in `verification.md`.
