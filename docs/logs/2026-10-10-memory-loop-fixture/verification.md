# Verification

Raw evidence is under the paired DIVA branch at `docs/logs/2026-10-memory-loop-chain/v0.1.0-reflection-and-restart/raw/`.

- Automatic reflection: unsupported mode and missing observer were observed RED; the real workflow then exposed the output-budget defect recorded in the preceding iteration. GREEN verifies a raw canonical source plus one ordinary memory effect, matching receipt, actual inference requests and processed watermark.
- Reusable Restart: unsupported operation RED; GREEN proves different OS PIDs, identical accepted/canonical identity and content, one canonical source and a fresh model counter.
- Repeated Close: `TestMemoryLoopFixtureCloseAfterRestartIsIdempotent` RED with `context deadline exceeded`, GREEN after recording the first close outcome.
- Post-restart actions: the extended `TestMemoryLoopFixtureRestart` reproduced a nil old peer, then passed after forwarding through the active fixture RPC.

The integration overlay is the previous SDK-generated development composition. The selected-file invocation excludes `default_generation_test.go`, whose default-generation inventory is incompatible with the DIVA recipe, and runs that contract separately through the unmodified default package suite. It is diagnostic integration evidence, not an artifact tied to the final new source tree.

The first combined run passed nine tests and skipped the opt-in action capture. A subsequent recorded run enables the capture output explicitly; use its actual results in the paired checkpoint. No skipped test is promoted to an acceptance pass. Child helper executions are identified separately from parent Go test counts.

Recorded combined regression: ten named parent tests pass, zero fail, zero skip, exit 0. The unmodified default App package: 133 named tests/subtests pass, zero fail, nine skip, exit 0. The latter's DIVA-composition tests skip because the default recipe does not select DIVA; opt-in capture and child helper entrypoints also skip when not invoked. Those skips provide no evidence for the corresponding product scenarios. Default-generation inventory assertions are included in that package run.

Full `just ci`, conformance reproduction and source-bound candidate rebuilding follow the remaining authorized local chain work. The preceding checkpoint's full CI does not cover this new tree.
