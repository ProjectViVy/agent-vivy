# Acceptance — VC-1c grep/glob

## How a human can tell it worked

1. Start the split pair (`just dev`) and open `http://127.0.0.1:3015`.
2. In any session, ask the model something like
   “找出工作区里所有包含 `JobRegistry` 的 Go 代码行” or
   “列出 internal/runtime 下所有 `*_test.go` 文件”.
3. The model should call the `grep` (and/or `glob`) tool — visible in the
   tool-call transcript as a readonly call that executes immediately without
   an approval card, even when the session approval policy is `ask`.
4. Results are workspace-relative paths with line numbers and the matched
   line (grep), or newest-first file lists with sizes (glob). `.gitignore`d
   paths do not appear in grep results on hosts with ripgrep installed.

## What to look for

- `grep`/`glob` appear in the default tool surface without config edits
  (they are in the default `tools.enabled` list).
- No approval interrupt fires for either tool under any approval policy —
  they are readonly by construction.
- On a host without ripgrep, grep still works (pure-Go fallback) but no
  longer honors `.gitignore` — only hidden/ignored-directory pruning. The
  tool description states this honestly so the model can adapt.

## Rollback

Single commit on `feat/vc1a-bash-tool`; revert the commit to remove both
tools, the backend implementation, and the config default additions.
doublestar stays an inert dependency after revert.
