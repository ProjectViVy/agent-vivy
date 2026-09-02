# acceptance — FACE-TUI-1 F2

## 人如何确认

1. **出厂行为不变**：不用 vivy-sdk 重新打包，直接跑日常 `vivy.exe run "…"` —— 行为与 F1 之前完全一致（内核 headless 循环）。默认注册器是 nil，face 分支只在 `--face` 打包的 generation 里激活。
2. **五步打出"带嘴"的 generation**：
   - `vivy-sdk verify faces/headless` → ok
   - `vivy-sdk pack --face headless` → 新 EXE + generation.json，其中 `recipe.face` = `headless`、`face.kind` = `headless`、grants 只含 tty/argv/rpc.client
   - 在该 EXE 旁运行 `vivy.exe run --continue "x"`（空日志）→ stderr 出现 `headless: no sessions to continue…`（器官在说话，不是内核）
3. **§14④ 可看见**：对着 packed EXE 触发一个需要审批的工具（或看单元测试 `TestApprovalBlockCancelsAndReturnsCancelled`）→ stderr 打印 "requires human approval and the headless face cannot ask for it"，run 被取消（exit 2），进程不挂起、不静默放行。
4. **SDK 守门**：把 faces/headless 的 manifest 改坏（如加 tools、listen: true、 Grants 加 fs.read）→ `vivy-sdk verify` 报错，拒绝打包。

## 边界（明确不验收）

- faces/tui 交互 TUI、faces/web 迁移、`face.listen: true` —— 不在本切片。
- face grants 的运行时逐 grant 仲裁（本批由 verifier 把关）。
