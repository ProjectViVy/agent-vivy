# VIVY CODE TUI parity

## Changed

- `vivy tui` now starts the real in-process Vivy code face in the current project. Demo data is available only through the explicit `--demo` flag.
- Added the `runtime.world: local` composition mode so this face governs the current project while reusing the normal Vivy runtime, Journal, tools, approvals, questions, sessions, skills, and provider configuration.
- Restored the colorful VIVY CODE presentation, including explicit true-color selection on interactive terminals, colored reasoning, tool states, and unified diff additions/deletions with change counts.
- Connected permission switching, queued messages, two-stage Escape behavior, streaming reasoning, tool arguments/results, and mutation diagnostics to real RPC state.
- Kept the packed `faces/tui` organ behavior aligned with the built-in face.
- Added a Windows-native embedded POSIX shell fallback using the repository's existing `mvdan.cc/sh` dependency, so the real `bash` tool does not accidentally invoke the Windows WSL launcher stub.

## Scope

This delivery changes Vivy only. It does not modify the Studio submodule, Studio shell, Studio lifecycle, or Studio data.

## Not included

The remaining secondary Crush presentation controls are tracked as `TUI-PARITY-2` in `docs/TODO.md`; they are not represented as fake/demo controls in this delivery.
