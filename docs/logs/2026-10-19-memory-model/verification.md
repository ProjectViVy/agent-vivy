# Verification evidence

Environment: Linux amd64, Python 3.10.12, Go 1.26.4 (`GOTOOLCHAIN=go1.26.4`).
Vivy base: `9cd2801f8090723a652d3e835f72d83715b82121`.
Laputa: clean `0e0027c5f60ad67614d3613c2d5e21d567c4b412`.

## Focused checks

- `python3 -m unittest discover -s scripts -p test_memory_model.py -v`: 15 pass.
  Initial missing module, strict input/protocol tests and changed-replay generation
  were observed red before their implementations, then green.
- `python3 -m py_compile scripts/memory_model.py scripts/test_memory_model.py`: pass.
- Raw BML retention is checked read-only inside the native package test, separately
  from visible search. An initial assertion incorrectly used legacy `Get`, which
  deliberately hides owned rows; the assertion was corrected, not the owner guard.
- `gofmt` on both bridge sources; `go vet -mod=readonly -p=1 -overlay OVERLAY .`
  for BML and `./actmem` for Laputa: pass. OVERLAY adds the corresponding bridge
  as the target package's virtual `memory_model_test.go`; no source file is copied
  into either module. `git diff --check`: pass.
- `python3 scripts/memory_model.py --target bml --cases 20 --steps 100`: seeds
  17..36 pass, 204 operations each (4080 total). Includes changed-identity rejection,
  stale record CAS, retained original outcomes, Unicode text, foreign-owner search,
  tombstone visibility and acknowledged-write crash/restart recovery.
- BML module `go test -mod=readonly ./...`: pass. Native selected ownership/replay
  tests `go test -race -mod=readonly -run 'Test(Bound|.*Replay|.*Owner)' -count=3 ./...`:
  pass.
- Locked ACTMEM's existing `go test -mod=readonly ./actmem` and `go test -race
  -mod=readonly ./actmem`: pass. This is why an independent cancellation regression
  matters; those green results do not prove the missing contract.
- Recap generated differential traces, seeds 17..19, 60 generated steps: fail at
  step 129, before-replacement cancellation. Expected `context_canceled`, actual
  `recap_commit_unknown`; receipts generated. Saved receipt replay reproduces the
  same first mismatch on a fresh root.
- Curated seven-operation `recap-cancel.json`: fails at zero-based step 2 with
  the same mismatch. No source pin is moved and no failure is skipped.

## Diagnostic control

A temporary Go overlay adds the test bridge and replaces only
`actmem/recap_history.go` with a copy containing this guard before the existing
`s.recapUnknown = true` in the failed `recoverRecap` branch:

```go
if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
    return nil, err
}
```

This is the previously investigated PR #12 classification control, not a committed
fix or an acceptance of a new Laputa revision. The real sibling stays unchanged.
Build with `go test -race -mod=readonly -overlay OVERLAY -c -o BINARY ./actmem`,
then run native regressions and feed generated traces through the same Python
  `run_trace` / `compare` functions. Both cancellation boundaries must be checked.

Results: BML native overlay regression passes with `-race -test.count=3`;
two additional differential race traces pass (seeds 17, 18, 44 operations each).
Locked Recap native regressions fail before replacement 3/3 and pass after
replacement 3/3 under `-race`. The diagnostic control passes both native boundary
tests 3/3 under `-race`, ten differential race traces (seeds 17..26, 213 operations
each), and the curated seven-operation trace. No race report was emitted.

## Whole-product gate and exclusions

`just ci` was attempted and stopped in `ensure-laputa`, before product tests:
the local clone's origin is the configured Devin Git proxy, not the canonical
GitHub URL required by the repository bootstrap. The origin/guard was not changed
or bypassed. Focused checks above use the clean exact source pin and readonly
modules; they are not a green `just ci` claim. Existing unrelated integration
failures remain outside this tool-only change. No Windows, UI or power-loss result
is claimed. No external implementation was copied and no new dependency added.
