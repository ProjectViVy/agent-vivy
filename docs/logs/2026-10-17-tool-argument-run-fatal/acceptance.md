# Acceptance — 2026-10-17 tool argument run-fatal cut

Product view: a model mistake must cost one tool call, not the run.

## What a human should now observe

1. **No approval prompt for an impossible review.** With the smart preset
   (workspace write + ask), ask the assistant to patch a file with text that is
   not in it. Before: the run ended `failed` while the review was being built.
   Now: the tool row reports a failure ("tool argument … was not found in the
   file; re-read it and copy the exact text"), no approval card appears, and the
   assistant keeps working in the same run.
2. **The same for a wrong path.** Patching a path that does not exist now
   reports "does not exist; create it with write_file first" instead of ending
   the run.
3. **No run-fatal full stop on argument shape.** `read_file` with a reversed
   range, a `search_files` call with an empty query, an out-of-cap
   `write_file`, a bad `glob`/`grep` pattern, or a tool whose arguments do not
   decode all produce a failed tool row and the run continues.
4. **A boundary denial reads as a refusal.** A call that touches a protected
   path or a symlink is refused with a policy-denied record: the run continues
   and the refusal is auditable, instead of the run dying on it.
5. **Real problems still stop the run.** A genuine tool-implementation error
   (unwired backend, journal failure), a stale-read conflict, a `proposal stale`
   conflict, or an unknown/hook-blocked/not-selected tool still fails the run —
   the change does not convert infrastructure or lifecycle failures into
   recoverable noise.

## How to check it by hand

- Split dev pair on the root tree: `just run` (or `VIVY_ADDR=127.0.0.1:879x go
  run ./cmd/vivy`) plus `cd ui; VIVY_BACKEND_ADDR=http://127.0.0.1:879x pnpm dev`,
  then open `http://127.0.0.1:3015`. With no provider key, use the frozen env
  provider against a local mock (see the `testing-vivy-ui` skill); the mock can
  emit a `patch` call with an `old_string` that is absent from the target file.
- Expected: the run reaches `completed`; the transcript shows one failed tool row
  with the argument diagnostic; no approval card appears (smart preset); the
  run's Journal has no `tool_approval_required` event for that call.
- Regression check for the guard: make the mock call an unselected tool name or
  break the filesystem backend wiring and confirm the run still ends `failed`
  with an internal-error outcome, not a silent refusal.

## Evidence

Verified live on 2026-10-17 through the split pair (disposable backend + frozen
env provider + local mock, Vite on `:3017`) with the same prompt sent twice —
once pre-fix, once post-fix: the pre-fix run showed
"The run could not be paused for approval. Please try again." and the run row
ended `failed`; the post-fix run showed the tool row
`Failed · Edit · tool argument "path": does not exist; create it with write_file first`,
no approval prompt, and the run ended `completed`. Journal detail in
[`verification.md`](verification.md).

- `internal/runtime/tool_failure_test.go`:
  `TestPatchArgumentMistakeRecoversWithoutApprovalUnderAsk`,
  `TestPatchArgumentMistakeRecoversWhenAutoApproved`,
  `TestProtectedPathReadIsRefusedInsideRun`,
  `TestToolFailureAllowlistRejectsInfrastructureErrors`.
- Pre-fix replays in [`verification.md`](verification.md) show the same tests
  failing with the reported run-fatal signature when only the fix is reverted.

## Not applicable

No UI, RPC, schema, storage, or product-contract change, so there is no visual
or contract-level acceptance surface beyond the transcript row described above.
