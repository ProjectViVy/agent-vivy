# Verification

## Red evidence

- Garden `go test ./agentapi -run '^TestIssue58' -count=1` failed with
  `bound owner is missing C1 Recap facade` and
  `closed Garden left ACTMEM owner readable: <nil>` before production changes.
- Vivy focused Issue58 tests failed with `cognitive bundle is missing optional
  Recap provider`; resolver failure was ignored by the previous factory.
- Recap unavailable/owner-busy/unknown-commit outcome tests failed with the old
  generic `failed` status before dispatch classification was added.

## Local implementation checks

- Garden Issue58 tests pass, covering clear/restore/current cursor, idempotent
  replay, conflict, operation conflict, invalid range, owner busy, cancellation,
  exact free text beyond old caps, worker/foreign/closed denial, legacy
  preservation, trusted root delivery, and owner release.
- Garden `go test ./agentapi ./internal/runtimecore/... -count=1` passes.
- Vivy `go test ./internal/modules/diva-cognitive ./internal/cognitivecontract
  -count=1` passes against the local source replacement. This is local adapter
  evidence, not final dependency-closure integration evidence.
- Laputa `go test ./...` passes.

- Full Garden `go test ./...` passes after building the actual embedded console
  assets with normal package policy intact. The initial run failed only for the
  absent console/dist prerequisite (root, console, internal/lifecycle/server).
- Garden focused race suites pass (agentapi 18.790s, runtimecore 16.021s).
- Vivy focused race suites pass (diva-cognitive 3.279s).
- Full Vivy `go test ./...` is still running in the existing app matrix at source
  handoff. Its final result is not claimed here; `/tmp/issue58-task2-vivy-test.log`
  retains output and the parent owns remaining exact-pin CI.
- Test commands carried GOSUMDB=off from Laputa's documented baseline spelling;
  carrying it into Vivy was unnecessary. Local cache/replacement tests do not
  establish published dependency closure. Normal default is sum.golang.org.
- Normal Garden `go mod verify` passes (`all modules verified`). Normal Vivy
  `go mod verify` exits 1 for 19 local agent-vivy/... v0.0.0 placeholder modules
  with `missing ziphash`; all names are retained in the task report. No workaround
  or verification-policy change was made.

The S1 Task 2 execution report records exact source/command boundaries. No dependency pin or lock is changed
here. Final dependency closure/bootstrap checks and `just ci` follow the parent's
reviewed upstream publication and coordinated pin update. Native Windows tests
are unavailable in this Linux environment; cross-compilation is not durability
acceptance. No new model/runtime implementation is added, so the design's Eino
capability check remains unchanged; all owner DTOs stay outside Eino imports.
