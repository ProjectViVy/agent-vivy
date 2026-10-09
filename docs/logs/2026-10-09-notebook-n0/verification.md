# N0 — verification

Date: 2026-10-09. Fixture repo: `~/repos/agent-vivy` @ `notebook` (baseline `e40c98cb`).

## RED (before removal)

`go test ./internal/runtime -run TestServiceDoesNotInjectNotebook -count=1`
→ FAIL: "ordinary turn listed notes 1 times" — the seeded marker note
`bluebird-notebook-marker-7f3a` was injected into the run preamble before
the change. This was a behavioral failure, not a missing dependency.

## GREEN (after removal)

- `go test ./internal/runtime ./internal/tools -count=1` →
  `ok internal/runtime` (~65s), `ok internal/tools` (0.458s).
  `TestServiceDoesNotInjectNotebook` covers an ordinary turn, a continuation
  turn in the same service, and a turn in a reopened service; all captured
  model request contents are asserted free of the marker. Explicit note tools
  (`tools.EchoInfoName` resolved) still execute.
- `go test ./internal/app -count=1` → `ok` (10.8s) after removing the
  `Notes: backend` wiring.
- `go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1` → `ok` (54.0s) after re-pinning
  the internal source digest.
- Symbol audit: `rg -n 'notesDigest|formatNotesDigest' internal` →
  no production matches (0 hits outside comments/tests).

## Full gate

`GIT_CONFIG_GLOBAL=/dev/null just ci` → exit 0 (complete run: fmt-check,
backend-ci, frontend build+tests, plugin-ci, conformance). Run log at
`/tmp/justci_n0.log`. Note: `GIT_CONFIG_GLOBAL=/dev/null` is required because
`ensure-laputa` performs an origin check that fails under the git-manager
proxy; this matches established repo practice.
