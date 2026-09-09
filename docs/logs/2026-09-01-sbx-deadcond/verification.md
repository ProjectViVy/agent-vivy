# Verification — SBX-DEADCOND

Date: 2026-09-01 | worktree `agent-vivy-vc0` (branch `feat/vc1a-bash-tool`)

```
go test ./internal/runtime/ -run TestIsDangerousCommand -race -count=1 -v
    → TestIsDangerousCommandRootDeletion PASS (23 dangerous combinations:
      rm -rf /, -fr, -Rf, -r -f, --recursive --force, /*, ., .., ~, C:\, C:\*,
      --no-preserve-root, quoted root, root with trailing whitespace, uppercase
      RM, del /f /s /q *, del /s C:\, del /s *, rd /s /q ., rmdir /s c:/,
      format, diskpart; each row was checked through
      ConfineCommandWithMode(danger) for ErrSandboxDenied)
    → TestIsDangerousCommandAllowsWorkbenchDeletes PASS (11 allowed combinations:
      rm -rf build, node_modules/pkg, -f file.txt, rm file, -r dist,
      --recursive tmp, -rf c:/temp/x, del /f /q, del /s build, rd /s /q dist,
      go test ./...)
go vet ./internal/runtime/                    → ok
go test ./internal/runtime/ -race -count=1    → ok (88.7s, no regression across the full package)
gofmt -l internal/runtime/                    → empty (test file formatted once on the first run)
just ci                                       → CI_EXIT=0
```

## No browser surface

This fix is kernel command-validation logic with no UI/user-visible surface; the
product-path verification is `just ci` (including lint and the full test suite).
The actual blocking behavior of execute/commandline tools in danger mode is
covered by assertions at the `ConfineCommandWithMode` level (each row in the
table above).
