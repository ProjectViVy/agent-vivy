# Verification — SBX-DEADCOND

日期：2026-09-01 ｜ worktree `agent-vivy-vc0`（分支 `feat/vc1a-bash-tool`）

```
go test ./internal/runtime/ -run TestIsDangerousCommand -race -count=1 -v
    → TestIsDangerousCommandRootDeletion PASS（23 个危险组合：
      rm -rf /、-fr、-Rf、-r -f、--recursive --force、/*、.、..、~、C:\、C:\*、
      --no-preserve-root、引号包裹根、尾随空白根、RM 大写、del /f /s /q *、
      del /s C:\、del /s *、rd /s /q .、rmdir /s c:/、format、diskpart；
      同表逐条经 ConfineCommandWithMode(danger) 断言 ErrSandboxDenied）
    → TestIsDangerousCommandAllowsWorkbenchDeletes PASS（11 个放行组合：
      rm -rf build、node_modules/pkg、-f file.txt、rm file、-r dist、
      --recursive tmp、-rf c:/temp/x、del /f /q、del /s build、rd /s /q dist、
      go test ./...）
go vet ./internal/runtime/                    → ok
go test ./internal/runtime/ -race -count=1    → ok（88.7s，全包无回归）
gofmt -l internal/runtime/                    → 空（首轮 test 文件格式化一次）
just ci                                       → CI_EXIT=0
```

## 无浏览器面

本修复为内核命令校验逻辑，无 UI/user-visible 面；产品路径验证 = `just ci`
（含 lint + 全量测试）。execute/commandline 工具在 danger 模式下的实际拦截
行为由 `ConfineCommandWithMode` 级断言覆盖（上表逐条）。
