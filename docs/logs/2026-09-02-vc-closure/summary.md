# VIVY-CODE 轨道收口（VC-0 / VC-1 / VC-2 → DONE）

## What changed

- **落岸**：`feat/vc1a-bash-tool`（71 提交）合并进 main（merge 后 fast-forward 至 `3c25562`）。冲突面 15 文件（docs/TODO.md、internal/rpc/control.go(+test)、internal/runtime/preflight.go、internal/tools/security.go、ui chat 三件、DashboardDemoView、i18n en/zh、lib api/demo-api(+test)/types），按三条原则解净：main 的 demo/死面删除照单全收；lane 的功能新增（attachments、Masks、文件面板、queue）照单保留；TODO 与 i18n 取并集。`feat/rb1-rollback-research` 因被 vc0 分支完全包含，`git merge` 判 Already up to date，`git log main..feat/rb1-rollback-research` 为空，rb1 闭账。两个 worktree 移除。
- **收口走查（拍板：脚本回放）**：新增 `internal/runtime/vc1_walkthrough_test.go` 的 `TestVC1Walkthrough`——ScriptedModel 回放，真实 workspace（`NewWorkspaceManager(t.TempDir())` + Eino 文件/命令后端 + file_versions recorder），脚本链 `write_file`（造 calc.sh）→ `read_file`（读码）→ `grep`（找 FIAL 行）→ `multiedit`（FIAL→PASS 修复）→ `bash`（`grep PASS calc.sh` 跑验证）→ 终文。断言：零 `tool.approval_required`（write/multiedit 经 EngineConfig.AutoApproveTools，bash 经 ApprovalPolicyAuto 安全分类）、5 个 `tool.finished` 有序且全零错误、multiedit payload 含 `"diff"`（FileMutationResult.Diff）、`file_versions` 表该文件版本链 ≥2 版（首版空 baseline=创建前快照，末版含 `echo PASS`）、run 单 `run.completed` 无 failed。轨道级验收「读码→grep→multiedit→bash 跑测试→看 diff」就此成证。
- **走查连带修复的两个真实缺陷**：
  1. `internal/tools/multiedit.go`：`edits` 参数缺 `Type: "array"`，引擎参数校验按 legacy string 契约把数组拒为 "must be a string"——工具单测直调 InvokableRun 掩盖了引擎路径。补上 Type 后引擎路径 multiedit 可用。
  2. `internal/storage/sqlite/fileversions.go` + `internal/storage/postgres/fileversions.go`：新建文件的 pre-mutation content 为 nil，直接落 NULL 触发 NOT NULL 约束失败、回滚**整笔**版本记录（含应记录的 new content）。`insertFileVersion` 统一 nil→`[]byte{}`。
- **TODO**：§0.1 VC-0 / VC-1 / VC-2 三行翻 DONE 2026-09-02（收口注记入行），§10 增收口行。

## Explicitly not done

- VC-3 文件版本 history 的恢复侧（RB-L2-DEFER）与 VC-4 生态项不在本片。
- channel 全家按拍板继续留待办；WEB-1、UI-TITLE 为后续独立小片。
- 走查里 `sh calc.sh` 换成 `grep PASS calc.sh`：bash 分类器在 ApprovalPolicyAuto 下只快进 InvocationSafe 命令，`sh` 属 ask 级——这是既有治理语义，未改。

## Notes

- 走查 runID 随机，故以 `write_file` 起链预置文件而非测试外种子；file_versions 断言用 modernc sqlite 直查（driver 已在 go.mod，blank import）。
- 冲突合并阶段 `internal/tools/security.go` 结果 = main 删 ScanPrompt（孤儿扫描器）+ lane 的 RedactSensitive 委托 logging.Redact，两者叠加均生效。
