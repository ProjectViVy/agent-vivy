# UI 迁移基础设施审查报告

## 执行摘要

已完成 VIVY Go backend RPC 端点的全面审查。结论如下：

### ✅ 已具备的能力
1. **完整的会话管理**（CRUD + 消息列表）
2. **运行控制与事件流**（SSE 订阅、取消、日志回放）
3. **审批与问答系统**（Tool 级别的 HITL）
4. **审查中心**（统一的 Approval/Question 视图）
5. **子进程管理**（启动/查询/等待/取消）
6. **Studio 生成物管理**（Generations/Evals/Promotions）
7. **基础设置管理**（Provider/Model/BaseURL）

### ⚠️ 需要补充的能力
1. **计划管理端点**（5 个新端点）- **高优先级**
2. **扩展配置管理**（Provider CRUD + 测试）- **中优先级**
3. **Channel 管理端点**（需确认是否已有实现）- **中优先级**
4. **Skills 管理端点** - **低优先级**
5. **记忆与 Persona 端点** - **低优先级**
6. **审计与诊断端点** - **低优先级**

### 📊 域模型现状
- ✅ **Todo** - 已存在 `domain.Todo` 和 `storage.TodoStore`
- ⚠️ **Plan** - 仅有 `PolicyProfilePlan` 枚举值，无完整域模型
- ❌ **PlanReport** - 完全缺失
- ⚠️ **Approval** - 存在但可能不支持 `domain='plan'`

---

## 详细分析

### 1. RPC 端点覆盖率

| 功能类别 | Agent Diva 需求 | VIVY 现有 | 覆盖率 | 缺口 |
|---------|----------------|----------|--------|------|
| 会话管理 | 6 | 6 | 100% | 0 |
| 运行控制 | 6 | 6 | 100% | 0 |
| 审批问答 | 4 | 4 | 100% | 0 |
| 审查中心 | 3 | 3 | 100% | 0 |
| **计划管理** | **5** | **0** | **0%** | **5** |
| 配置管理 | 6 | 2 | 33% | 4 |
| Channel 管理 | 5 | 0? | 0%? | 5? |
| Skills 管理 | 4 | 0 | 0% | 4 |
| 记忆管理 | 8 | 0 | 0% | 8 |
| 审计诊断 | 4 | 1 | 25% | 3 |
| 其他辅助 | 5 | 0 | 0% | 5 |
| **总计** | **56** | **22** | **39%** | **34** |

### 2. 关键缺口详解

#### 2.1 计划管理（Blocker）

**缺失原因：** VIVY 的 PRD v0.5 聚焦于核心 Agent Loop，计划功能是 Agent Diva 的 Pro 特性，尚未移植到 VIVY。

**影响：**
- 无法显示待审批的计划卡片
- 无法批准/拒绝计划
- 无法查看计划执行进度
- 无法将计划退回草稿

**解决方案选项：**

**选项 A：从 Agent Diva 移植完整的 Plan 域模型**
- 优点：功能完整，用户体验一致
- 缺点：工作量大（需设计 domain/storage/runtime 三层）
- 估计工时：2-3 周

**选项 B：简化版计划支持（仅审批决策）**
- 优点：快速实现核心路径
- 缺点：缺少计划详情展示、版本管理等高级功能
- 估计工时：3-5 天

**推荐：选项 B（MVP） → 后续迭代到选项 A**

#### 2.2 配置管理（重要但非 Blocker）

**现状：** VIVY 有 `settings/get` 和 `settings/update`，但仅支持 Provider/Model/BaseURL。

**需要扩展的字段：**
```typescript
interface RuntimeConfig {
  // 已有
  provider: string;
  default_model: string;
  base_url: string;
  
  // 需要添加
  api_key?: string;  // ⚠️ 注意：Secrets 不应持久化
  
  // Provider 列表
  providers: Record<string, ProviderConfig>;
  custom_providers: Record<string, ProviderConfig>;
  
  // 工具配置
  tools: ToolsConfig;
  
  // 预算与压缩
  budget: BudgetConfig;
  
  // 其他
  sandbox_policy: SandboxPolicy;
  compaction_threshold: number;
}
```

**注意：** VIVY 的设计原则是"Secrets 永不持久化"（D-010），因此 API Key 等敏感信息不能通过 settings API 管理。需要设计安全的密钥输入流程（内存中保存，重启后清除）。

#### 2.3 Channel 管理（需确认）

**疑问：** `control.go` 中没有看到 channel 相关的 case，但 README 提到 VIVY 是"personal gateway Agent"，理论上应该支持多通道。

**行动项：**
1. 检查 `internal/app/` 是否有 channel 管理器
2. 检查 `cmd/vivy/` 的入口点是否初始化了 channels
3. 如果确实缺失，需要设计 Channel Store 和 RPC 端点

---

## 实施路线图

### Phase 1：基础设施准备（当前阶段）

#### 1.1 RPC 端点补充（1-2 周）
- [ ] **计划管理 MVP**（3 个端点）
  - `plan/get_active` - 获取活动计划
  - `plan/approve` - 批准计划
  - `plan/reject` - 拒绝计划
  
- [ ] **会话辅助**（2 个端点）
  - `sessions/generate_title` - 自动生成标题
  - `sessions/pin` / `sessions/unpin` - Pin 操作

- [ ] **配置扩展**（1 周）
  - 扩展 `settings/get` 返回更多字段
  - 新增 `providers/list` 列出可用 Provider

**负责人：** Backend Team  
**依赖：** 无

#### 1.2 域模型定义（3-5 天）
- [ ] 定义 `domain.Plan` 类型
- [ ] 定义 `domain.PlanRevision` 类型
- [ ] 定义 `domain.PlanReport` 类型
- [ ] 扩展 `domain.Approval` 以支持 `domain='plan'`

**负责人：** Backend Team  
**依赖：** 无

#### 1.3 存储层实现（1 周）
- [ ] 定义 `storage.PlanStore` 接口
- [ ] 实现 SQLite backend（`storage/sqlite/plans.go`）
- [ ] 编写存储层单元测试

**负责人：** Backend Team  
**依赖：** 域模型定义完成

---

### Phase 2：国际化迁移（并行进行，1 周）

#### 2.1 翻译文件合并
- [ ] 提取 Agent Diva 的 `en.ts` 所有键
- [ ] 提取 Agent Diva 的 `zh.ts` 所有键
- [ ] 合并到 VIVY 的 `ui/src/app/i18n.ts`
- [ ] 解决键名冲突
- [ ] 补充缺失翻译

**负责人：** Frontend Team  
**依赖：** 无

#### 2.2 i18n 运行时增强
- [ ] 支持命名空间（避免键名冲突）
- [ ] 支持动态加载语言包
- [ ] 添加翻译完整性检查

**负责人：** Frontend Team  
**依赖：** 2.1 完成

---

### Phase 3：样式系统准备（并行进行，1 周）

#### 3.1 CSS Tokens 扩展
- [ ] 分析 Agent Diva 的 Tailwind 类使用
- [ ] 定义对应的 CSS 变量到 `tokens.css`
- [ ] 添加新组件的语义化类名

**负责人：** Frontend Team  
**依赖：** 无

#### 3.2 主题系统集成
- [ ] 合并 Agent Diva 的 `useTheme` 逻辑到 `preferences.ts`
- [ ] 确保深色/浅色模式正常切换
- [ ] 添加系统主题监听

**负责人：** Frontend Team  
**依赖：** 3.1 完成

---

### Phase 4：测试框架搭建（并行进行，3-5 天）

#### 4.1 单元测试框架
- [ ] 安装 Vitest
- [ ] 配置 TypeScript 支持
- [ ] 迁移 Agent Diva 的关键单元测试

**负责人：** QA Team  
**依赖：** 无

#### 4.2 E2E 测试框架
- [ ] 扩展现有 Playwright 配置
- [ ] 编写核心路径的 E2E 测试
  - 会话创建 → 发送消息 → 查看响应
  - 计划审批流程
  - 设置修改流程

**负责人：** QA Team  
**依赖：** Phase 1 的部分端点完成

---

## 风险与缓解

### 高风险
1. **Plan 域模型从零设计**
   - 风险：设计不当导致后续返工
   - 缓解：参考 Agent Diva 的实现，先做 MVP，再迭代

2. **Secrets 管理冲突**
   - 风险：Agent Diva 允许持久化 API Key，VIVY 禁止
   - 缓解：设计临时的内存密钥输入流程，明确告知用户限制

### 中风险
3. **Channel 管理缺失**
   - 风险：如果 VIVY 确实没有 Channel 支持，需要从头实现
   - 缓解：先确认现状，如果缺失则标记为 Phase 2 任务

4. **样式迁移工作量超预期**
   - 风险：Tailwind → CSS Modules 转换耗时
   - 缓解：采用混合策略，保留常用 Tailwind 类名

### 低风险
5. **国际化键名冲突**
   - 风险：两个项目的翻译键命名规范不同
   - 缓解：使用命名空间隔离（如 `chat.sendMessage` vs `settings.saveButton`）

---

## 下一步行动

1. **立即开始：** Phase 1.1 RPC 端点补充（计划管理 MVP）
2. **并行开展：** Phase 2 国际化迁移 + Phase 3 样式准备
3. **本周内完成：** 确认 Channel 管理现状
4. **下周评审：** Phase 1 完成情况，决定是否进入 Phase 2 的核心聊天系统迁移

---

**文档版本：** v0.1  
**创建日期：** 2026-01-XX  
**最后更新：** 2026-01-XX  
**维护者：** UI Migration Team  
**状态：** Draft - 待评审
