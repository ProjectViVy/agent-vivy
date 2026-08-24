# Agent Diva GUI → VIVY UI 全量化迁移计划 - 执行总结

## 执行摘要

本文档总结了 Phase 1（基础设施准备）的完成情况和后续阶段的规划。

### ✅ Phase 1 完成情况（2026-01-XX）

| 任务 | 状态 | 产出物 |
|------|------|--------|
| RPC 端点审查 | ✅ 完成 | `docs/UI_MIGRATION_RPC_GAP_ANALYSIS.md` |
| 国际化迁移计划 | ✅ 完成 | `docs/UI_MIGRATION_I18N_PLAN.md` |
| 样式系统扩展计划 | ✅ 完成 | `docs/UI_MIGRATION_STYLES_PLAN.md` |
| 基础设施审查报告 | ✅ 完成 | `docs/UI_MIGRATION_INFRASTRUCTURE_REVIEW.md` |

**本阶段结论：**
- VIVY 现有 RPC 端点覆盖率 **39%**（22/56），核心缺口在**计划管理**（0%）
- 国际化需新增约 **300 个翻译键**（排除 PET 相关后）
- 样式系统需扩展 **10+ 组 CSS 变量**和 **10+ 个组件样式文件**
- 预计 Phase 1 实际实施需 **2-3 周**

---

## 关键发现

### 1. RPC 端点差距分析

**高优先级缺口（Blocker）：**
- ❌ `plan/get_active` - 获取活动计划
- ❌ `plan/approve` - 批准计划
- ❌ `plan/reject` - 拒绝计划
- ❌ `plan/list_reports` - 获取计划报告列表
- ❌ `sessions/generate_title` - 自动生成会话标题

**影响：** 没有这些端点，无法实现计划审批流程，这是 Agent Diva 的核心差异化功能。

**建议方案：** 
- **MVP 路径**：先实现简化版计划支持（仅审批决策，无版本管理）
- **完整路径**：从 Agent Diva 移植完整的 Plan 域模型（2-3 周工作量）

### 2. 国际化策略

**决策：** 采用**混合命名空间**方案
- 保留 VIVY 现有的扁平键名（如 `newSession`）
- 新功能使用前缀分组（如 `chatPlaceholder`, `settingsTitle`）
- 避免破坏性变更，同时保持可扩展性

**工作量估算：** 3.5-4.5 天
- 扩展 i18n.ts 结构：1 天
- 更新 translate() 函数：0.5 天
- 编写完整性检查脚本：0.5 天
- 人工审核翻译质量：1-2 天
- 添加翻译键文档：0.5 天

### 3. 样式系统对齐

**设计原则：** 
- **保留 VIVY 的中性工业风**（符合 DeepSeek Harness IDE 风格）
- **复用 Agent Diva 的布局结构和交互模式**
- **调整颜色以匹配 VIVY 调色板**（不使用粉色渐变）

**工作量估算：** 5.5-7.5 天
- 扩展 tokens.css：1 天
- 创建组件样式文件：2-3 天
- 导入新样式文件：0.5 天
- 移除 Tailwind 依赖：1 天
- 视觉回归测试：1-2 天

---

## 后续阶段规划

### Phase 2：核心聊天系统迁移（2-3 周）

**目标：** 实现完整的对话界面，包括消息渲染、流式输出、工具卡片等。

**关键任务：**
1. **增强 `conversation/view.ts`**
   - 实现消息列表渲染器
   - 集成 Markdown 渲染（markdown-it + highlight.js）
   - 添加工具卡片组件（ToolCallCard）
   - 实现 Thinking block 折叠/展开

2. **实现 Composer 输入区**
   - 文本输入 + 提交
   - Plan mode toggle
   - Draft 持久化

3. **集成 SSE 事件流**
   - 复用现有 `sse.ts`
   - 解析 text.delta/tool.start/tool.finish 事件
   - 管理流式占位符（isStreaming 状态）

**依赖：** 
- RPC 端点 `turn/start`, `run/subscribe` 已存在 ✅
- 需要补充 `plan/get_active`（如果支持计划模式）⚠️

**验收标准：**
- [ ] 用户可以发送消息并查看流式响应
- [ ] 工具调用卡片正常显示（名称/参数/结果）
- [ ] Thinking block 可以折叠/展开
- [ ] Markdown 代码块正确高亮

---

### Phase 3：会话与审批中心（1-2 周）

**目标：** 实现会话侧边栏和审批中心，完成 HITL（Human-in-the-Loop）流程。

**关键任务：**
1. **增强 `sessions/view.ts`**
   - 会话列表渲染（标题/摘要/时间戳）
   - 搜索过滤
   - Pin/Unpin 操作
   - 重命名对话框

2. **实现 `reviews/view.ts`（审批中心）**
   - 审批列表分页加载
   - 审批详情展示（工具名称/参数/风险上下文）
   - 决策按钮（Allow/Deny/Cancel）
   - AskUserQuestion 轮询定时器

**依赖：**
- RPC 端点 `session/list`, `session/rename`, `approval/list`, `question/list` 已存在 ✅
- 需要补充 `sessions/generate_title`, `sessions/pin` ⚠️

**验收标准：**
- [ ] 用户可以创建/重命名/删除会话
- [ ] 审批中心显示待处理审批项
- [ ] 用户可以批准/拒绝审批
- [ ] AskUserQuestion 正常轮询并显示

---

### Phase 4：设置面板（2-3 周）

**目标：** 实现完整的设置界面，包括 Provider/Channel/Skills/MCP 管理。

**关键任务：**
1. **大幅扩展 `settings/view.ts`**
   - Provider 管理（内置 + 自定义）
   - Channel 管理（Telegram/Discord/QQ 等）
   - Skills 市场浏览与安装
   - MCP 服务器管理
   - 审计日志查看
   - 主题/语言切换

2. **实现表单控件**
   - Input/Select/Checkbox
   - Wizard 分步表单
   - 验证和错误提示

**依赖：**
- RPC 端点 `settings/get`, `settings/update` 已存在 ✅
- 需要补充 `providers/list`, `channels/list`, `skills/list` 等 ⚠️

**验收标准：**
- [ ] 用户可以添加/编辑/删除 Provider
- [ ] 用户可以配置 Channel
- [ ] 用户可以浏览和安装 Skills
- [ ] 主题和语言切换正常工作

---

### Phase 5：记忆与高级功能（2-3 周）

**目标：** 实现记忆管理、Persona 工作区和 Evolution 提案审查。

**关键任务：**
1. **创建 `memory/view.ts`**
   - BML 记忆浏览（短期/长期/核心）
   - FTS5 全文搜索
   - 记忆条目编辑/删除

2. **实现 Persona Markdown 编辑器**
   - 集成 CodeMirror
   - Frozen Core 锁定
   - 版本历史

3. **实现 Evolution 提案审查**
   - AutoDream 生成的变更建议
   - Governed apply（审查后应用）
   - Rollback 支持

**依赖：**
- 需要新增 `memories/list`, `persona/get`, `evolution/list_proposals` 等端点 ❌

**验收标准：**
- [ ] 用户可以浏览和搜索记忆
- [ ] 用户可以编辑 Persona Markdown
- [ ] 用户可以审查和应用 Evolution 提案

---

### Phase 6：控制台与诊断（1 周）

**目标：** 实现 Gateway 状态监控、日志查看器和 Token 统计。

**关键任务：**
1. **创建 `console/view.ts`**
   - Gateway 健康检查
   - 通道连接状态
   - Cron 任务列表

2. **实现日志查看器**
   - 实时日志流（SSE）
   - 过滤/搜索
   - 级别切换（info/debug/error）

3. **添加 Token 统计面板**
   - 会话级 token 消耗
   - 预算阈值警告

**依赖：**
- 需要新增 `stats/tokens`, `audit/log` 等端点 ❌

**验收标准：**
- [ ] 用户可以查看 Gateway 状态
- [ ] 用户可以查看实时日志
- [ ] 用户可以查看 Token 统计

---

### Phase 7：Onboarding 与收尾（1 周）

**目标：** 实现欢迎向导，完善错误处理和边界情况。

**关键任务：**
1. **创建 `onboarding/view.ts`**
   - DeepSeek API Key 输入
   - Bocha search key 配置
   - 快速导航（chat/providers/network/console）

2. **完善错误处理**
   - 网络超时重试
   - RPC 错误提示
   - 边界情况处理

3. **性能优化**
   - 虚拟滚动（长消息列表）
   - 懒加载（设置面板按需渲染）

**验收标准：**
- [ ] 首次启动显示欢迎向导
- [ ] 所有错误都有友好的提示
- [ ] 长列表滚动流畅（60fps）

---

### Phase 8：测试与发布（1-2 周）

**目标：** 完成全面测试，发布候选版本。

**关键任务：**
1. **单元测试**
   - 迁移 Agent Diva 的 Vitest 测试
   - 覆盖关键工具函数（Markdown 渲染、审批 idempotency 等）

2. **E2E 测试**
   - 扩展现有 Playwright 配置
   - 编写核心路径测试：
     - 会话创建 → 发送消息 → 查看响应
     - 计划审批流程
     - 设置修改流程

3. **视觉回归测试**
   - 截图对比关键页面
   - 确保 Light/Dark 模式正常

4. **性能基准测试**
   - 首屏加载时间 ≤ 2s
   - Lighthouse 评分 ≥ 90
   - 构建产物大小 ≤ 500KB（gzipped）

**验收标准：**
- [ ] 所有核心路径的 E2E 测试通过
- [ ] 无 console error/warning
- [ ] Lighthouse 性能评分 ≥ 90
- [ ] 构建产物大小 ≤ 500KB（gzipped）
- [ ] 用户验收测试通过

---

## 总体时间表

| 阶段 | 工期 | 开始日期 | 结束日期 |
|------|------|---------|---------|
| Phase 1: 基础设施准备 | 2-3 周 | 2026-01-XX | 2026-02-XX |
| Phase 2: 核心聊天系统 | 2-3 周 | 2026-02-XX | 2026-03-XX |
| Phase 3: 会话与审批 | 1-2 周 | 2026-03-XX | 2026-03-XX |
| Phase 4: 设置面板 | 2-3 周 | 2026-03-XX | 2026-04-XX |
| Phase 5: 记忆与高级功能 | 2-3 周 | 2026-04-XX | 2026-05-XX |
| Phase 6: 控制台与诊断 | 1 周 | 2026-05-XX | 2026-05-XX |
| Phase 7: Onboarding 与收尾 | 1 周 | 2026-05-XX | 2026-05-XX |
| Phase 8: 测试与发布 | 1-2 周 | 2026-05-XX | 2026-06-XX |
| **总计** | **12-18 周** | | |

**注意：** 以上时间为串行估算，实际执行时部分任务可并行开展（如国际化迁移可与样式扩展同时进行）。

---

## 资源需求

### 人员配置
- **Frontend Developer**：2 人（全职）
- **Backend Developer**：1 人（兼职，负责 RPC 端点补充）
- **QA Engineer**：1 人（兼职，Phase 8 全职）
- **Product Manager**：1 人（兼职，负责翻译审核和视觉验收）

### 技术依赖
- **Node.js**：>= 18（Vite 要求）
- **TypeScript**：>= 5.0
- **Playwright**：>= 1.40
- **Go**：>= 1.26（backend 开发）

---

## 风险总览

### 高风险
1. **Plan 域模型从零设计**
   - 影响：可能导致 Phase 2 延期
   - 缓解：采用 MVP 路径，先实现简化版

2. **RPC 端点补充工作量超预期**
   - 影响：阻塞前端开发
   - 缓解：Backend 提前介入，Phase 1 期间完成高优先级端点

### 中风险
3. **样式迁移工作量大**
   - 影响：UI 视觉效果不一致
   - 缓解：严格遵循 VIVY 设计令牌，不做创造性发挥

4. **国际化键名冲突**
   - 影响：翻译混乱
   - 缓解：使用命名空间前缀，运行完整性检查脚本

### 低风险
5. **Tailwind 依赖残留**
   - 影响：构建产物体积增大
   - 缓解：grep 扫描确认无 Tailwind 类名

---

## 下一步行动

### 立即开始（本周）
1. **Backend Team：** 实现高优先级 RPC 端点
   - `plan/get_active`
   - `plan/approve`
   - `plan/reject`
   - `sessions/generate_title`

2. **Frontend Team：** 开始 Phase 1 实际实施
   - 扩展 `i18n.ts`（按 `UI_MIGRATION_I18N_PLAN.md`）
   - 扩展 `tokens.css`（按 `UI_MIGRATION_STYLES_PLAN.md`）

3. **PM：** 组织评审会议
   - 审查本计划文档
   - 确认优先级和时间表
   - 分配资源

### 下周目标
- 完成 Phase 1 的基础设施实施
- 开始 Phase 2 的核心聊天系统开发
- Backend 完成高优先级 RPC 端点

---

## 附录：参考文档

1. **RPC 端点差距分析**：`docs/UI_MIGRATION_RPC_GAP_ANALYSIS.md`
2. **国际化迁移计划**：`docs/UI_MIGRATION_I18N_PLAN.md`
3. **样式系统扩展计划**：`docs/UI_MIGRATION_STYLES_PLAN.md`
4. **基础设施审查报告**：`docs/UI_MIGRATION_INFRASTRUCTURE_REVIEW.md`
5. **原始迁移计划**：（见 plan mode 退出时的完整计划）

---

**文档版本：** v0.1  
**创建日期：** 2026-01-XX  
**最后更新：** 2026-01-XX  
**维护者：** UI Migration Team  
**状态：** ✅ Phase 1 规划完成，待实施
