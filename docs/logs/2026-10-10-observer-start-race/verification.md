# Verification

The direct race test fails before the fix with DATA RACE. Full observerhost race regression passes 11 named tests, zero skips, exit 0. Raw task logs: chain-recall-observer-start-red.jsonl and chain-recall-observer-start-green.jsonl. The actual App/ONNX discovery is retained in chain-recall-child-race-reproduction.jsonl, which also exposes the separately repaired native embedding lifecycle race.

```bash
source /workspace/work/memory-loop/tools/environment.sh
go test -race -json ./internal/observerhost -count=1
```

Unit fixture uses existing in-memory Journal/cursor doubles; this is not full process-recovery acceptance. Complete source CI, sealing and whole-phase review remain open.
