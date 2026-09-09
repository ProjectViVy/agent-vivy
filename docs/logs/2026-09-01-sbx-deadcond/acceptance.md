# Acceptance — SBX-DEADCOND

## How a human can tell it works

With `sandbox.mode: danger-full-access` (full-trust mode), the following classes
of commands are still rejected and return "command pattern is too dangerous even
in full-access mode", even when the allowlist is open:

- `rm -rf /`, `rm -fr /*`, `rm -r /`, `rm --recursive --force /` (any flag
  combination that recursively deletes the system root)
- `rm -rf .` / `..` (recursively deletes the current/parent directory as a whole)
- `rm -rf ~`, `rm -rf C:\`, `rm -rf C:\*` (home directory, drive root)
- Any `rm` with `--no-preserve-root`
- Windows: `del /f /s /q *`, `del /s C:\`, `rd /s /q .`

At the same time, normal uses that must not be caught remain available:
`rm -rf build`, `rm -rf node_modules`, `rm -f file.txt`, `del /q file.txt`,
`rm -rf c:/temp/x` (a specific subdirectory), and so on.

## Boundaries

- This defense covers only the danger mode of the execute/commandline path; the
  bash tool path is covered by its existing deny table (unchanged).
- confined (workspace-write/read-only) modes are unaffected; they do not rely on
  this defense in the first place.

## Before-fix comparison

The original implementation's single-argument cross-condition was always false,
so `rm -rf /` could pass danger-mode validation and execute directly. This slice
pins the behavior with `TestIsDangerousCommandRootDeletion`.
