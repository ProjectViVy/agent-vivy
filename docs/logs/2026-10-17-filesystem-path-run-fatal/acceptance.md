# Acceptance — filesystem path run-fatal fix

Date: 2026-10-17

## How a human can tell it worked

1. Start a session with the read-only sandbox (the configuration in which the
   reported run died) and ask the model to look at the workspace, for example
   `查看一下当前工作区里有什么东西`.
2. The model may call `list_dir` with `"."`, `"./"`, `".//"` or an empty path;
   all of them list the run workspace root. No tool row shows
   `filesystem: invalid path`.
3. If the model then calls `list_dir` with a rooted path (`/`, `/etc`,
   `C:\Windows`), a traversal (`../x`) or a path whose component is a file, the
   chat shows one unsuccessful tool row instead of a failed turn:
   `tool.finished.outcome = recoverable`, `reason = invalid_arguments`, and the
   result text contains `tool argument "path": ...`. The run stays alive and the
   model's next turn can correct the call.
4. What must NOT happen: the run ending with
   `engine event error: [NodeRunError] failed to stream tool call ...: filesystem: invalid path`
   and the session marked failed.

## Product-level checks

- `list_dir`, `read_file`, `search_files`, `glob` and `grep` share the same
  resolve seam, so the same acceptance holds for all of them.
- A rooted path is now refused on Windows exactly as it already was on Linux;
  it is no longer silently treated as workspace-relative.
- Deliberate denials were not loosened: `.env`/`keys.txt`/`secrets` targets and
  symlink components are still refused, and the sandbox's read-only/confined
  rules are unchanged.
- No user configuration is required: existing sessions and settings behave the
  same, minus the crash.
