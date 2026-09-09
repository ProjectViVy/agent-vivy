# Verification — WEB-2

Date: 2026-09-01 | worktree `agent-vivy-vc0` (branch `feat/vc1a-bash-tool`)

```
go test ./internal/runtime/ -run TestEinoFilesystemBackendWriteFileConfinedFreshNestedDir -race -count=1 -v
    → PASS (regression case: writing a/b/c/new.txt (CreateParents) under the
      workspace-write sandbox succeeds and reads back byte-for-byte identically;
      the same request under the read-only sandbox → ErrSandboxDenied)
gofmt -l internal/runtime/filesystem_backend.go internal/runtime/filesystem_backend_test.go
    → empty
go test ./internal/runtime/ ./internal/tools/ -race -count=1
    → ok 96.2s / ok 1.9s (the write path underpins many tool/approval cases; the full
      package has no regressions)
just ci → CI_EXIT=0
```

## Before-the-fix comparison

The same regression case reported before the fix:
`sandbox: runtime: resolve parent symlinks: ...` (The system cannot find the
path specified.) — completely new nested directories were always falsely rejected in
restricted mode; the test name pins down this scenario.

## No browser surface

This is a kernel file-tool path with no UI/user-visible surface; product-path verification =
`just ci`.
