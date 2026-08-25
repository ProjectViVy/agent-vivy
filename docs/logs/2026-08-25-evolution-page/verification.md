# Verification — evolution page

Date: 2026-08-25

## `just ci`（仓库根目录）

两轮通过（第二轮含 seed 副本修复后）：

- go fmt-check / vet / test / headless-compile：`ok`（cached 或通过）
- `pnpm typecheck`：通过
- `pnpm test`：**51 tests / 11 files 全部通过**（含新增 `demo-api.evolution.test.ts`
  6 个用例：播种、接受写头+历史、stale 判定、重复决策拒绝、CAS 冲突、硬删边界）
- `pnpm build`：`✓ built in 5.06s`（`routeTree.gen.ts` 由 Vite 插件重新生成，含
  `/_layout/evolution`）

首次运行曾失败：新增测试暴露 `readSkillRequests` 等播种函数返回模块级 MOCK 数组本体、
被治理操作原地污染（用例间状态泄漏）。修复为 `seedStore` 统一返回副本后全绿——该 bug
即测试发现的产品缺陷，非测试问题。

## 浏览器冒烟（split Vite，`http://127.0.0.1:3015/evolution`）

复用本机已在运行的拆分对（Vite 3015 + 控制面 8787）。经 in-app 浏览器逐项验证：

1. 侧边栏「进化」为真实链接（无「待实现」徽章），`/evolution` 渲染页面头 + DemoBanner。
2. 三 Tab 计数活体：Skill (0→1)、待审 (1→0)、AutoDream (3)。
3. 待审详情：标题/原因/状态徽章/基准哈希/提案全文/Evidence JSON 齐全。
4. **接受闭环**：点击「接受」→ 按钮 busy 禁用 → 请求转「已接受」、待审计数归零、
   Skill 计数 +1；Skill Tab 出现新权威 skill `vivy-doc-sync`，内容哈希更新、
   权威 Markdown 即提案全文；「已处理完毕」提示出现。
5. Skill 详情：编辑/停用/硬删/历史快照按钮齐全。
6. AutoDream：3 条运行（2 完成 1 失败，失败带原因）；run-demo-1 详情含触发方式/阶段/
   起止/尝试次数/输入摘要 + **5 条进度事件时间线** + 「查看待审请求」。
7. 跨 Tab 跳转：「查看待审请求」→ 切回待审 Tab 并自动选中对应提案详情。
8. 刷新页面后所有状态持久（localStorage 演示存储）。

过程中两个非缺陷说明：

- 旧浏览器标签页后期进入降级状态（点击/截图失败、事件面板误报空），换新标签页后
  事件面板正常；数据层另有单测佐证，判定为宿主标签页问题而非产品缺陷。
- 首次运行向导（WelcomeWizard，既有功能）在重载后弹出，与本改动无关，跳过即消失。
- 演示数据播种只在对应 localStorage 键缺失时发生；曾访问过旧版技能页的浏览器会缓存
  旧 `vivy.demo.skills`（无进化管理项），Skill Tab 显示为空——清除该键或全新环境即可
  看到种子数据（接受任意请求也会创建新的进化管理 skill）。

## 未验证项

- 「编辑保存/停用/硬删/新建请求」表单的浏览器端点击路径未逐一走查（对应 demo-api
  逻辑已被单测覆盖；UI 侧与已走查的接受路径共用同一 hook/busy/错误反馈模式）。
- Playwright e2e 仅覆盖首页冒烟（既有范围），未新增 evolution 用例。
