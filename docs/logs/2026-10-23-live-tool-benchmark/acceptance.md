# Human acceptance

Read `summary.md` and the per-run projections in `results.json`. Every passing
row must have a completed run, one terminal event, the expected JSON answer,
and successful file reads for every required input. The nonce answers are
generated outside the model prompt, so a conversational answer alone cannot
pass. The recovery case additionally requires an attempted missing-file read.

The six cases cover a nonce read, a three-file reference chain, paid-order CSV
aggregation, an off-by-one diagnosis, missing-file recovery, and a follow-up
that combines prior session context with a new file. Each runs twice. The
follow-up shares only the nonce-read session in its own round; other cases
have fresh titled sessions and synthetic temporary workspace inputs.

To repeat against an already configured disposable Vivy backend, use Node 24:

```powershell
$env:BENCH_URL = 'http://127.0.0.1:8787'
$env:BENCH_OUTPUT = Join-Path (Get-Location) '.workspace/benchmarks/live-tools-repeat'
$env:BENCH_ROUNDS = '2'
node docs/logs/2026-10-23-live-tool-benchmark/benchmark.mjs --preflight
node docs/logs/2026-10-23-live-tool-benchmark/benchmark.mjs
```

Configure credentials through the existing product flow. Do not put a key in
this script or its command line. Use an isolated backend home, never a tenant
Journal. The benchmark does not change model selection, enabled tools, policy,
or permissions. Its only inputs are public RPC calls and synthetic files.

This is bounded read/list task acceptance. It does not certify file mutation,
shell execution, approval UI, concurrent runs, cancellation, restart recovery,
full-product composition, or the remaining D14 human acceptance checklist.
