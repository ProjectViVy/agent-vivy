# VIVY-CODE track closeout (VC-0 / VC-1 / VC-2 → DONE)

## What changed

- **Landing**: `feat/vc1a-bash-tool` (71 commits) merged into main (fast-forwarded to
  `3c25562` after merge). The 15-file conflict surface (docs/TODO.md,
  internal/rpc/control.go(+test), internal/runtime/preflight.go,
  internal/tools/security.go, three ui chat files, DashboardDemoView, i18n en/zh, lib
  api/demo-api(+test)/types) was resolved by three principles: accept all main's demo/dead-
  surface deletions; retain all lane functionality additions (attachments, Masks, file
  panel, queue); take the union of TODO and i18n. `feat/rb1-rollback-research` was fully
  contained in the vc0 branch, so `git merge` reported Already up to date,
  `git log main..feat/rb1-rollback-research` was empty, and rb1 was closed out. The two
  worktrees were removed.
- **Closeout walkthrough (decision: scripted replay)**: added
  `internal/runtime/vc1_walkthrough_test.go`'s `TestVC1Walkthrough` — ScriptedModel replay,
  real workspace (`NewWorkspaceManager(t.TempDir())` + Eino file/command backends +
  file_versions recorder), with the script chain `write_file` (create calc.sh) → `read_file`
  (read code) → `grep` (find the FIAL line) → `multiedit` (fix FIAL→PASS) → `bash` (run
  `grep PASS calc.sh` verification) → final text. Assertions: zero `tool.approval_required`
  (write/multiedit via EngineConfig.AutoApproveTools, bash via ApprovalPolicyAuto safe
  classification), five ordered `tool.finished` events with zero errors, multiedit payload
  contains `"diff"` (FileMutationResult.Diff), the file's `file_versions` chain has ≥2
  versions (first empty baseline = pre-creation snapshot, final contains `echo PASS`), and
  the run has one `run.completed` with no failed. The track-level acceptance "read code →
  grep → multiedit → run tests with bash → inspect diff" is now evidenced.
- **Two real defects fixed alongside the walkthrough**:
  1. `internal/tools/multiedit.go`: the `edits` parameter lacked `Type: "array"`; engine
     parameter validation applied the legacy string contract and rejected the array with
     "must be a string", while direct InvokableRun unit tests masked the engine path. Adding
     the Type made the engine path's multiedit usable.
  2. `internal/storage/sqlite/fileversions.go` + `internal/storage/postgres/fileversions.go`:
     pre-mutation content for a new file was nil; writing NULL violated the NOT NULL
     constraint and rolled back the **entire** version record (including the new content).
     `insertFileVersion` now normalizes nil → `[]byte{}`.
- **TODO**: moved the three §0.1 VC-0 / VC-1 / VC-2 lines to DONE 2026-09-02 (closeout note
  added in-line), and added the closeout line in §10.

## Explicitly not done

- The recovery side of VC-3 file-version history (RB-L2-DEFER) and the VC-4 ecosystem item
  are outside this slice.
- The entire channel family remains on the TODO list per the decision; WEB-1 and UI-TITLE are
  later independent slices.
- In the walkthrough, `sh calc.sh` was replaced with `grep PASS calc.sh`: under
  ApprovalPolicyAuto, the bash classifier fast-forwards only InvocationSafe commands, while
  `sh` is ask-level — existing governance semantics, unchanged.

## Notes

- The walkthrough runID is random, so the chain starts by provisioning the file with
  `write_file` rather than seeding it outside the test; the file_versions assertion queries
  modernc sqlite directly (the driver is already in go.mod, blank import).
- The conflict-merge result in `internal/tools/security.go` = main deleting ScanPrompt (an
  orphan scanner) + lane's RedactSensitive delegation to logging.Redact; both effects are
  active.
