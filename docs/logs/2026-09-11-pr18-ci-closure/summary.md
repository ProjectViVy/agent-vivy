# PR 18 CI closure

## What changed

- Classified the MCP resource-bridge and capability-state translation keys as intentional Web-only strings in the cross-face i18n contract.
- Updated the public plugin import lint test to inspect the v1 `sdk/module` and `sdk/port` surfaces instead of the removed v0 `sdk/plugin` directory.
- Made the approval-resume regression test wait for the durable approval Journal event before submitting its decision.
- Centralized source-tree hashing in `internal/sourcehash` and normalized NUL-free UTF-8 text line endings so the same Module source has the same digest on Windows and Unix. Invalid UTF-8 or NUL-containing binary content remains byte-exact, symlinks remain rejected, and generated assembly output remains excluded.
- Made the SDK executable-bit assertion conditional on platforms that expose POSIX executable mode bits.
- Raised the repository Go test package timeout to 20 minutes. The Windows runtime and SDK integration suites exercise hundreds of SQLite-backed real paths and complete successfully beyond Go's default 10-minute package timeout.

## Scope

This delivery closes the CI failures on pull request 18 (`feat/plugin-v1-p3-p4`). It does not add a product capability, change visible UI behavior, implement OAuth, or alter the approved plugin-v1 architecture.

No new model, tool-orchestration, prompting, streaming, context, checkpoint, RAG, MCP, or multi-agent mechanism was introduced. The pull request continues to use the pinned Eino and EinoExt adapters documented by the PLG-P4 capability check; this closure required no new Eino API or Vivy-owned runtime substitute.

## Explicitly not done

- Pull requests 19 and 20 were not rebased or merged as part of this commit.
- No Studio source or tenant Journal data was touched.
- No browser visual smoke was run because this closure changes no user-visible UI behavior.
