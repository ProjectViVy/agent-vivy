# Summary — 聊天框功能栏伪操作清理与闭环（UI-COMPOSER / UI-CHAT-TOOLBAR）

## What changed

聊天输入框 `ChatInput.tsx` 顶栏完成对齐项目架构正本要求（“无权威能力位的控件继续隐藏，不得以 demo 假控件替代”），彻底清理历史残留的假控件与点不通纸伤：

1. **AutoDream 按钮彻底移除**：
   - 移除顶栏常驻的 `GitBranch` 图标按钮及其点击提示 `chatInput.autodreamUnavailable`。
   - 该能力所属内核轨道 MEM-1 处于 DEFERRED 状态，未来待有能力提案时再以真实功能/能力开关形式呈现，不再常驻无用假控件。

2. **执行模式下拉菜单收敛**：
   - 从 `MODES` 及 `ExecMode` 类型中彻底移除点击仅弹窗报不通的 `ask`（询问模式）。
   - 下拉菜单仅保留并展示系统已真实端到端接通的两种执行模式：
     - **智能体模式** (`agent`，映射到内核 `RunModeNormal`)
     - **计划模式** (`plan`，映射到内核 `RunModePlan`)
   - 简化菜单选中事件处理，不再包含无内核语义的不可达分支。

3. **i18n 词条清理**：
   - 同步清理 `zh.ts` 与 `en.ts` 中仅服务于上述伪操作的 dead keys（`askMode`, `askModeDesc`, `askUnavailable`, `autodreamTrigger`, `autodreamUnavailable`），保持双语字典严格对称与精简。

4. **自动化回归与规格保障**：
   - 更新 `ui/e2e/thinking-gate.spec.ts` 浏览器端到端规格，断言 AutoDream 按钮不再出现、执行模式下拉仅包含智能体与计划模式、询问模式不再出现。
   - 补充 `internal/codeface/launch_test.go` 中 `project` 路径规范化（`filepath.EvalSymlinks`），消除 Windows 短路径与规范路径比较的误报。

5. **归档与结案**：
   - 状态持久化方面，此前复核已拍板遵从 Crush 语义（“思考与执行模式为逐回合选择，不做会话级持久化”）。
   - 在 `docs/TODO.md` §0.1 与 §10 中正式将 `UI-COMPOSER` 与 `UI-CHAT-TOOLBAR` 翻 DONE 结案归档。

## Scope / What was explicitly not done

- **内核 Ask / 只读问答语义**：本轮纯前端做减法，直接清退假控件，不在内核凭空添加未经提案的第三种 RunMode。
- **AutoDream 后端能力**：MEM-1 保持 DEFERRED，未来有完整记忆系统能力提案时再按正规生命周期接入。
- **思考/执行模式会话级持久化**：维持逐回合选择（Crush 语义）。
