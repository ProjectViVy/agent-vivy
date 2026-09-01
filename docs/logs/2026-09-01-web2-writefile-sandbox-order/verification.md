# Verification — WEB-2

日期：2026-09-01 ｜ worktree `agent-vivy-vc0`（分支 `feat/vc1a-bash-tool`）

```
go test ./internal/runtime/ -run TestEinoFilesystemBackendWriteFileConfinedFreshNestedDir -race -count=1 -v
    → PASS（回归用例：workspace-write 沙箱下写 a/b/c/new.txt（CreateParents）
      成功落盘且读回逐字节一致；read-only 沙箱同请求 → ErrSandboxDenied）
gofmt -l internal/runtime/filesystem_backend.go internal/runtime/filesystem_backend_test.go
    → 空
go test ./internal/runtime/ ./internal/tools/ -race -count=1
    → ok 96.2s / ok 1.9s（写路径是众多工具/审批用例的底座，全包无回归）
just ci → CI_EXIT=0
```

## 修复前对照

相同回归用例在修复前报
`sandbox: runtime: resolve parent symlinks: ...`（The system cannot find the
path specified.）——全新嵌套目录在受限模式一律误拒；测试名即钉死该场景。

## 无浏览器面

内核文件工具路径，无 UI/user-visible 面；产品路径验证 = `just ci`。
