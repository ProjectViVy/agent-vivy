# Verification

全部命令在 worktree `../agent-vivy-chatbox-buttons`（分支 `feat/chatbox-buttons`）执行。

| 命令 | 结果 |
|---|---|
| `just ci`（fmt-check / vet / go test / headless-compile / ui-ci） | PASS（ui-ci: 21 文件 175 测试通过；vite build 成功，chunk 体积警告为既有现象） |
| `pnpm typecheck` / `pnpm test` / `pnpm build`（ui） | PASS |
| `just ui-e2e`（Playwright 自起 Go mock 后端） | 6 passed / 2 failed；两个失败在干净 main HEAD 上同样失败（见下） |

## e2e 失败归因（非本 lane 引入）

用 `git stash -u` + 重新 build 在干净 HEAD（2a77258）复跑：

- `runtime.spec.ts`：干净 HEAD 卡在早已失效的 `画图` 断言（按钮在 src 中不存在）；
  本 lane 已修掉该断言并使 spec 推进到设置页，剩余失败点是
  "密钥只由运行环境管理"（设置-模型 tab 断言过期，疑似相对 model-list-sync）。
- `welcome-wizard.spec.ts`：干净 HEAD 同样在"下一步 → 配置模型"步骤失败。

两处均记 `docs/TODO.md` §0.1 **E2E-STALE**。

## 真机 smoke（真实浏览器，mock provider 后端）

环境：worktree 内 `VIVY_ADDR=127.0.0.1:18787 go run ./cmd/vivy`（临时 mock config.yaml，
已删除）+ `VIVY_BACKEND_ADDR=http://127.0.0.1:18787 pnpm exec vite --port 3016`。
逐项通过：

1. 初始 DOM：`新建会话` 在聊天框顶栏；`语音`、`打开伙伴`（桌面伙伴）不存在；
   侧边栏与快照中无新建会话入口；附件/AutoDream/更多/思考模式按钮保留。
2. 点 `新建会话`：会话创建成功，历史抽屉列表 1 → 2 个会话，自动切到新会话，无失败提示。
3. 权限下拉：智能 → 谨慎，触发器标签与描述切换（`session/set_permission` 真实往返）。
4. 执行模式：选"询问模式"出现提示"询问模式暂未接入"且选中保持"智能体模式"；
   选"计划模式"触发器变更为"计划模式"。
5. 计划模式发送消息：预检返回 **"预检已阻止本次运行 / task_create is unavailable in plan
   mode"** —— mode 真实到达后端并被 plan 策略拦截（旧行为硬编码 normal 不会出现）。
6. 切回智能体模式重发：mock 回复 `mock reply to: smoke: agent mode message` 正常渲染
   （preflight → turn/start → 流式回复全链路）。
7. `审批中心` ShieldCheck：sheet 打开，标题"审批中心"、空列表（"暂无审批或问题记录"）。

## 已知未验证

- e2e 通过的 6 个用例覆盖会话/审批/设置/演示路径；设置-模型与欢迎向导两用例因
  E2E-STALE 既有失败未走通（与本 lane 无关）。
- 思考模式下拉（后端无能力）仅验证按钮保留，未验证行为。
