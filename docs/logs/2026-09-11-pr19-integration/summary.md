# PR 19 integration

## What changed

- Integrated the merged PLG-P3/P4 branch into PLG-P5 without creating a second Assembly or runtime path.
- Combined Provider Profile, Context Source, Skill Source, MCP Host, observer, and status records in the build-owned default Source Catalog.
- Registered ObserverHost and StatusHost as build-owned core Port owners, selected both established L2 organs in the default Recipe, and added compiler/default-Generation regression coverage after independent review found they were otherwise unselectable and absent from the generated Assembly.
- Combined their typed Provider inventories and Manifest projections in the single generated Runtime Assembly, then regenerated `internal/generated/assembly/zz_default.go` from the compiler inputs.
- Preserved the shared supported-Port evidence ledger and retained the earlier `P1P2PortEvidence` entry point as a compatibility wrapper.
- Made the P4 generated-source assertion insensitive to `gofmt` column alignment after the P5 Manifest field was added.
- Replaced an asynchronous cron test's atomic-state assumption with a bounded condition wait for the active-run entry to clear after the durable job row settles.

## Scope

This delivery is limited to reconciling pull request 19 (`feat/plugin-p5`) with the already merged pull request 18. It does not add a Provider family, OAuth flow, model adapter, runtime, or source of truth.

The Eino capability decision is unchanged: PLG-P5 continues to adapt only the repository-pinned EinoExt OpenAI-compatible and Claude ChatModel components through the existing internal ModelHost. Deferred families and OAuth remain `DEFERRED-INDEFINITE`; no Vivy-owned substitute was introduced.

## Explicitly not done

- Pull request 20 was not integrated in this delivery.
- The CLI behavior that treats `--help` as a normal server launch was not changed; it is recorded as `CLI-HELP-EXIT` in `docs/TODO.md`.
- No Studio source was touched, and no tenant data was inspected, deleted, or cleaned up. The accidental default-config launch is disclosed in `verification.md`.
