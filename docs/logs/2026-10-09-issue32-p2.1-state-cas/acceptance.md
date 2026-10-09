# Acceptance: P2.1

## Engineering acceptance

C1's targeted contract is implemented and locally verified:

- A stale snapshot version is rejected without losing accepted capture state.
- Concurrent capture, policy and settlement transitions retain their separate
  fields and the expected policy revision.
- Equal-base policy CAS attempts across independent Service instances yield
  exactly one accepted write and one policy conflict.
- Cognitive regression tests pass with Go's race detector.

## Remaining gates

Aggregate `just ci` could not run in the current Linux environment because the
`just` executable is absent and the repository recipe selects PowerShell.
Run that gate in a supported environment before treating the integrated
remediation as accepted. P2.2-P2.4, later phases, and final product/release
acceptance remain open.
