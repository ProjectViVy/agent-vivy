# UI 迁移计划文档索引

本目录包含 Agent Diva GUI → VIVY UI 全量化迁移的详细规划和执行总结。

## 📚 文档列表

### 1. [执行总结](./UI_MIGRATION_EXECUTION_SUMMARY.md) ⭐ **从这里开始**
- **内容：** Phase 1 完成情况、后续阶段规划、总体时间表
- **适合人群：** 项目负责人、PM、所有团队成员
- **阅读时间：** 10 分钟

### 2. [RPC 端点差距分析](./UI_MIGRATION_RPC_GAP_ANALYSIS.md)
- **内容：** Agent Diva 所需端点与 VIVY 现有端点的对比，识别需要补充的 RPC 方法
- **适合人群：** Backend Developer
- **关键发现：** VIVY 现有覆盖率 39%，核心缺口在计划管理（0%）
- **阅读时间：** 15 分钟

### 3. [国际化迁移计划](./UI_MIGRATION_I18N_PLAN.md)
- **内容：** 如何将 Agent Diva 的 400+ 翻译键迁移到 VIVY 的 i18n 系统
- **适合人群：** Frontend Developer、翻译审核人员
- **工作量估算：** 3.5-4.5 天
- **阅读时间：** 10 分钟

### 4. [样式系统扩展计划](./UI_MIGRATION_STYLES_PLAN.md)
- **内容：** 如何将 Agent Diva 的 TailwindCSS + 自定义变量适配到 VIVY 的设计令牌系统
- **适合人群：** Frontend Developer、UI Designer
- **工作量估算：** 5.5-7.5 天
- **阅读时间：** 15 分钟

### 5. [基础设施审查报告](./UI_MIGRATION_INFRASTRUCTURE_REVIEW.md)
- **内容：** VIVY Go backend 的全面审查结果，包括现有能力、缺口分析、实施路线图
- **适合人群：** Tech Lead、架构师
- **阅读时间：** 20 分钟

---

## 🚀 快速开始

### 对于 Backend Developer
1. 阅读 [RPC 端点差距分析](./UI_MIGRATION_RPC_GAP_ANALYSIS.md)
2. 优先实现高优先级端点（Phase 1A）：
   - `plan/get_active`
   - `plan/approve`
   - `plan/reject`
   - `sessions/generate_title`

### 对于 Frontend Developer
1. 阅读 [执行总结](./UI_MIGRATION_EXECUTION_SUMMARY.md) 了解整体规划
2. 开始 Phase 1 实施：
   - 按 [国际化迁移计划](./UI_MIGRATION_I18N_PLAN.md) 扩展 `i18n.ts`
   - 按 [样式系统扩展计划](./UI_MIGRATION_STYLES_PLAN.md) 扩展 `tokens.css`

### 对于 QA Engineer
1. 阅读 [执行总结](./UI_MIGRATION_EXECUTION_SUMMARY.md) 的 Phase 8 部分
2. 准备 Playwright 测试环境
3. 设计核心路径的 E2E 测试用例

### 对于 Product Manager
1. 阅读 [执行总结](./UI_MIGRATION_EXECUTION_SUMMARY.md)
2. 组织评审会议，确认优先级和时间表
3. 安排翻译审核人员

---

## 📊 项目状态

| 阶段 | 状态 | 完成度 |
|------|------|--------|
| Phase 1: 基础设施准备 | 🟡 规划完成，待实施 | 0% |
| Phase 2: 核心聊天系统 | ⬜ 未开始 | 0% |
| Phase 3: 会话与审批 | ⬜ 未开始 | 0% |
| Phase 4: 设置面板 | ⬜ 未开始 | 0% |
| Phase 5: 记忆与高级功能 | ⬜ 未开始 | 0% |
| Phase 6: 控制台与诊断 | ⬜ 未开始 | 0% |
| Phase 7: Onboarding 与收尾 | ⬜ 未开始 | 0% |
| Phase 8: 测试与发布 | ⬜ 未开始 | 0% |

**总体进度：** 0%（规划阶段完成）

---

## 🎯 关键里程碑

- **2026-02-XX：** Phase 1 完成（基础设施实施）
- **2026-03-XX：** Phase 2 完成（核心聊天系统可用）
- **2026-04-XX：** Phase 3-4 完成（会话、审批、设置）
- **2026-05-XX：** Phase 5-7 完成（高级功能、Onboarding）
- **2026-06-XX：** Phase 8 完成（测试通过，发布候选版本）

---

## 💬 沟通渠道

- **周会：** 每周一上午 10:00
- **即时通讯：** Teams #ui-migration 频道
- **代码审查：** GitHub PR 标签 `ui-migration`
- **问题追踪：** GitHub Issues 标签 `ui-migration`

---

## 📝 更新日志

- **2026-01-XX：** 初始版本，完成 Phase 1 规划
  - 创建 5 份详细文档
  - 识别 RPC 端点缺口
  - 制定国际化和样式迁移策略
  - 估算总体工作量（12-18 周）

---

**最后更新：** 2026-01-XX  
**维护者：** UI Migration Team  
**联系：** team@vivy.example.com
