# Acceptance — WEB-2

## How a human can tell it works

With the sandbox in restricted mode (workspace-write, the default session mode), asking vivy
to "create config.json under src/deep/nested/" now completes normally: the file is written
and the diff appears in approval/Review Center. Before the fix, this kind of request always
failed with `resolve parent symlinks` — the model had to run bash mkdir first, making the
experience disjointed.

## Boundaries (things that must not happen)

- read-only mode still rejects every write (the regression test asserts ErrSandboxDenied).
- danger-full-access mode is unchanged.
- Symlink components are still rejected (safeWorkspacePath performs component-by-component
  Lstat), so directory creation cannot escape the workspace through a symlink.
- The proposal/approval flow (PrepareWriteFile → human approval → execution revalidation +
  pre-write hash) is unchanged.
