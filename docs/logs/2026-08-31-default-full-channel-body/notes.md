# Notes — 2026-08-31 default-full-channel-body

## Why plugin modules need to be tidied again (reproducible coupling)

The plugins are standalone modules, but each has `replace agent-vivy => ../..`. This causes
**the root module's entire require closure to enter each plugin's MVS graph**. Once the full
body pulled the union of the five plugins' dependencies into the root graph, shared
dependencies were raised (golang.org/x/net → v0.50.0, along with
golang.org/x/crypto → v0.48.0). The old pins in discord / qq's own go.mod files no longer
matched the new resolution, so `go list` (readonly) immediately reported
"updates to go.mod needed", and the linkable check in `vivy-sdk verify` failed as a result.
The pins for telegram / dingtalk / feishu happened to match the new graph and required no changes.

Conclusion: **from now on, whenever a dependency is raised in the root go.mod, run
`for p in plugins/*; do (cd $p && go mod tidy); done`**. This coupling deserves a dedicated
explanation in VIVY-PLUGIN-SPEC or the SDK documentation (it was not written into the contract this time; see the TODO proposal).

## Boundary of pack idempotence

Only plugin-module pairs that the root already both requires and replaces are skipped; third-party
closures are still merged as usual, keeping the overlay self-consistent when root dependencies
are older. fake-channel (not carried by the root) continues through the full append path, with
the original semantics unchanged.

## Known gaps

- `~/.vivy` data root: the first smoke attempt (run without VIVY_CONFIG) opened it. The directory
  already existed on 2026-08-30 (created by an earlier development session); this attempt only
  performed idempotent migrations/read queries, with no sign of data writes or damage. Smoke
  testing was switched to an isolated configuration.
- The root go.mod now carries the third-party closures of the 5 channel plugins, making the dependency tree larger—this is the direct cost of the full default, and the user approved it.
- Named packaging recipes (aliases such as vivy-with-channel / vivy-code) were not implemented; see the "Explicitly not done" section in summary.
