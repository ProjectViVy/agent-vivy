# SBX-DEADCOND: Fix the dead condition for recursive force deletion in isDangerousCommand

Date: 2026-09-01 | Branch: `feat/vc1a-bash-tool` | worktree: `agent-vivy-vc0`

## What changed

`isDangerousCommand` in `internal/runtime/sandbox_manager.go` is an independent
defense for the execute/commandline path in danger-full-access mode (the bash
tool has a separate deny table). The original implementation required the
**same argv entry** to serve as both a flag and a target:

```go
if (arg == "-rf" || arg == "-fr" || arg == "/f" || arg == "/s") &&
    (arg == "/" || arg == "*" || arg == ".") { return true }
```

One argument cannot be both `-rf` and `/`, so the condition was always false and
the `rm -rf /` check was effectively useless.

Fix = collect in two passes and then cross-check (the remedy named by the TODO
row):

- `rm`: collect recursive signals (`-r`/`-rf`/`-fr`/`-Rf` short-flag clusters,
  `--recursive`) and root-like targets in one pass, then block when both are
  present; block `--no-preserve-root` directly (its sole purpose is to enable
  root deletion).
- `del`/`rd`/`rmdir` (Windows): `/s` is the recursive signal, cross-checked with
  root-like targets.
- Added `isRootLikeDeleteTarget`, which recognizes a filesystem root (including
  quoted roots and roots with trailing whitespace),
  root wildcards (`/*`, `C:\*`), the whole current/parent directory
  (`.`, `..`), and home (`~`, `~/*`),
  and drive roots (`C:\`, `C:/`, `c:`). It consistently strips quotes,
  normalizes backslashes, and applies TrimRight before classification;
  Specific paths below a drive root (`c:/temp/x`) and ordinary workspace targets
  (`build`, `node_modules/pkg`) are always allowed. This defense only applies in
  danger mode; confined mode already uses the allowlist, so normal cleanup
  commands are not tightened by this fix.

## Deliberately not done

- No full shell parsing (pipes/variables/nested quotes): the scope is an
  "obviously dangerous pattern" heuristic layer, separate from the bash deny
  table; parser-level protection belongs to the bash tool path.
- Do not separately allow or add an allowlist mechanism for `rm -rf *` (a cwd
  wildcard): retain the original target set, with blocking still limited to
  danger mode.

## Acceptance criteria

- Before the fix: `rm -rf /` passed through `ConfineCommandWithMode(danger)`
  (the defense was breached).
- After the fix: all 23 dangerous combinations, including `rm -rf /`,
  `rm -r /`, `del /s *`, and `rd /s /q .`, are rejected, while all 11 normal
  deletion/build-cleanup combinations are allowed (see verification.md).
