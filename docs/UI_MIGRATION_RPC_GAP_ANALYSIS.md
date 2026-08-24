# UI 迁移 RPC 端点差距分析

本文档对比 Agent Diva GUI 所需的 Tauri commands 与 VIVY 现有的 JSON-RPC 端点，识别需要补充的端点。

## 1. 现有端点清单（VIVY）

基于 `internal/rpc/control.go` 的 `Handle` 方法：

### 会话管理
- ✅ `session/create` - 创建会话
- ✅ `session/list` - 列出会话
- ✅ `session/get` - 获取会话详情（含消息）
- ✅ `session/rename` - 重命名会话
- ✅ `session/delete` - 删除会话
- ✅ `session/messages` - 列出会话消息

### 运行控制
- ✅ `preflight/run` - 预检运行
- ✅ `turn/start` - 启动对话轮次
- ✅ `turn/interrupt` / `run/cancel` - 取消运行
- ✅ `run/get` - 获取运行状态
- ✅ `run/subscribe` - 订阅运事件流
- ✅ `run/unsubscribe` - 取消订阅
- ✅ `run/log` - 获取运行日志

### 审批与问答
- ✅ `approval/list` - 列出待审批项
- ✅ `approval/respond` - 响应审批（approve/deny）
- ✅ `question/list` - 列出待回答问题
- ✅ `question/respond` - 回答问题

### 审查中心
- ✅ `review/list` - 列出审查项
- ✅ `review/get` - 获取审查详情
- ✅ `review/respond` - 响应审查

### 子进程管理
- ✅ `child/start` - 启动子进程
- ✅ `child/get` - 获取子进程状态
- ✅ `child/list` - 列出子进程
- ✅ `child/wait` - 等待子进程完成
- ✅ `child/cancel` - 取消子进程

### 背景任务
- ✅ `background/recover` - 恢复后台任务
- ✅ `background/list` - 列出后台任务
- ✅ `background/attach` - 附加到后台任务

### Studio 相关
- ✅ `generations/list` - 列出生成物
- ✅ `generations/get` - 获取生成物详情
- ✅ `generations/create` - 创建生成物
- ✅ `generations/reject` - 拒绝生成物
- ✅ `evals/list` - 列出评估
- ✅ `evals/record` - 记录评估
- ✅ `evals/start` - 启动评估
- ✅ `promotions/list` - 列出晋升
- ✅ `promotions/promote` - 执行晋升
- ✅ `species/inspect` - 检查物种状态

### 设置
- ✅ `settings/get` - 获取设置
- ✅ `settings/update` - 更新设置

---

## 2. Agent Diva 所需但 VIVY 缺失的端点

### 2.1 计划管理（高优先级）

| Agent Diva Command | 用途 | VIVY 现状 | 建议方案 |
|-------------------|------|----------|---------|
| `get_active_plan` | 获取会话当前活动计划 | ❌ 缺失 | 新增 `plan/get_active` |
| `approve_active_plan_execution` | 批准计划并继续执行 | ❌ 缺失 | 新增 `plan/approve_execution` |
| `continue_approved_plan_execution` | 继续已批准计划的执行 | ❌ 缺失 | 复用 `turn/start` + plan context |
| `get_plan_reports` | 获取计划报告列表 | ❌ 缺失 | 新增 `plan/list_reports` |
| `return_active_plan_to_draft` | 将计划退回草稿 | ❌ 缺失 | 新增 `plan/return_to_draft` |

**实现建议：**
```go
// internal/rpc/control.go 添加：
case "plan/get_active":
    return h.getActivePlan(ctx, request)
case "plan/approve_execution":
    return h.approvePlanExecution(ctx, request)
case "plan/list_reports":
    return h.listPlanReports(ctx, request)
case "plan/return_to_draft":
    return h.returnPlanToDraft(ctx, request)
```

### 2.2 配置管理（中优先级）

| Agent Diva Command | 用途 | VIVY 现状 | 建议方案 |
|-------------------|------|----------|---------|
| `get_runtime_config` | 获取完整运行时配置 | ⚠️ 部分（仅 provider/model） | 扩展 `settings/get` |
| `update_runtime_config` | 更新运行时配置 | ⚠️ 部分（仅 provider/model） | 扩展 `settings/update` |
| `list_providers` | 列出所有 Provider | ❌ 缺失 | 新增 `providers/list` |
| `add_provider` | 添加自定义 Provider | ❌ 缺失 | 新增 `providers/add` |
| `delete_provider` | 删除 Provider | ❌ 缺失 | 新增 `providers/delete` |
| `test_provider_connection` | 测试 Provider 连接 | ❌ 缺失 | 新增 `providers/test` |

**实现建议：**
扩展现有 `settings/get` 和 `settings/update` 以包含更多配置字段，或拆分为专门的 `config/*` 端点。

### 2.3 Channel 管理（中优先级）

| Agent Diva Command | 用途 | VIVY 现状 | 建议方案 |
|-------------------|------|----------|---------|
| `list_channels` | 列出通道 | ✅ 已有（但未在 handler 中暴露） | 确认实现 |
| `create_channel` | 创建通道 | ✅ 已有（但未在 handler 中暴露） | 确认实现 |
| `update_channel` | 更新通道 | ❌ 缺失 | 新增 `channels/update` |
| `delete_channel` | 删除通道 | ❌ 缺失 | 新增 `channels/delete` |
| `test_channel_connection` | 测试通道连接 | ❌ 缺失 | 新增 `channels/test` |

**注意：** VIVY 的 `control.go` 中没有看到 channel 相关的 case，需要确认是否在别处实现。

### 2.4 Skills 管理（低优先级）

| Agent Diva Command | 用途 | VIVY 现状 | 建议方案 |
|-------------------|------|----------|---------|
| `list_skills` | 列出已安装 skills | ❌ 缺失 | 新增 `skills/list` |
| `install_skill` | 安装 skill | ❌ 缺失 | 新增 `skills/install` |
| `uninstall_skill` | 卸载 skill | ❌ 缺失 | 新增 `skills/uninstall` |
| `list_marketplace_skills` | 列出市场 skills | ❌ 缺失 | 新增 `skills/marketplace` |

### 2.5 记忆与 Persona（低优先级）

| Agent Diva Command | 用途 | VIVY 现状 | 建议方案 |
|-------------------|------|----------|---------|
| `list_memories` | 列出记忆条目 | ❌ 缺失 | 新增 `memories/list` |
| `get_memory` | 获取记忆详情 | ❌ 缺失 | 新增 `memories/get` |
| `delete_memory` | 删除记忆 | ❌ 缺失 | 新增 `memories/delete` |
| `search_memories` | 搜索记忆 | ❌ 缺失 | 新增 `memories/search` |
| `get_persona` | 获取 Persona | ❌ 缺失 | 新增 `persona/get` |
| `update_persona` | 更新 Persona | ❌ 缺失 | 新增 `persona/update` |
| `get_evolution_proposals` | 获取 Evolution 提案 | ❌ 缺失 | 新增 `evolution/list_proposals` |
| `apply_evolution_proposal` | 应用 Evolution 提案 | ❌ 缺失 | 新增 `evolution/apply` |

### 2.6 审计与诊断（低优先级）

| Agent Diva Command | 用途 | VIVY 现状 | 建议方案 |
|-------------------|------|----------|---------|
| `get_token_stats` | 获取 Token 统计 | ❌ 缺失 | 新增 `stats/tokens` |
| `get_audit_log` | 获取审计日志 | ❌ 缺失 | 新增 `audit/log` |
| `get_gui_log` | 获取 GUI 操作日志 | ❌ 缺失 | 新增 `audit/gui_log` |
| `get_raw_log` | 获取原始事件流 | ⚠️ 部分（`run/log`） | 扩展为跨 run 查询 |

### 2.7 其他辅助端点

| Agent Diva Command | 用途 | VIVY 现状 | 建议方案 |
|-------------------|------|----------|---------|
| `generate_session_title` | 自动生成会话标题 | ❌ 缺失 | 新增 `sessions/generate_title` |
| `pin_session` | Pin 会话 | ❌ 缺失 | 新增 `sessions/pin` |
| `unpin_session` | Unpin 会话 | ❌ 缺失 | 新增 `sessions/unpin` |
| `get_compaction_status` | 获取压缩状态 | ❌ 缺失 | 新增 `compaction/status` |
| `trigger_compaction` | 触发压缩 | ❌ 缺失 | 新增 `compaction/trigger` |

---

## 3. 实施优先级建议

### Phase 1A：核心功能必需（立即实施）
1. **计划管理端点**（5 个）
   - `plan/get_active`
   - `plan/approve_execution`
   - `plan/list_reports`
   - `plan/return_to_draft`
   - `continue_approved_plan_execution`（可复用 `turn/start`）

2. **会话标题生成**（1 个）
   - `sessions/generate_title`

**理由：** 这些是聊天界面和计划审批流程的核心依赖，没有它们无法完成基本的对话和计划管理功能。

### Phase 1B：配置管理（短期实施）
1. **Provider 管理**（4 个）
   - `providers/list`
   - `providers/add`
   - `providers/delete`
   - `providers/test`

2. **扩展设置端点**
   - 扩展 `settings/get` 以返回更多配置
   - 扩展 `settings/update` 以支持更多字段

**理由：** 用户需要能够配置 LLM Provider 和其他运行时参数。

### Phase 2：Channel 与 Skills（中期实施）
1. **Channel 管理**（5 个）
2. **Skills 管理**（4 个）

**理由：** 这些是多通道网关和扩展能力的核心，但对于单用户桌面场景不是立即必需的。

### Phase 3：记忆与审计（长期实施）
1. **记忆管理**（8 个）
2. **审计日志**（4 个）

**理由：** 高级功能，可以在核心功能稳定后逐步添加。

---

## 4. 数据模型差异

### 4.1 计划数据结构

**Agent Diva 的 PlanRuntimeState：**
```typescript
interface PlanRuntimeState {
  plan_id: string;
  revision: number | null;
  title: string;
  goal: string;
  phase: 'Draft' | 'AwaitingApproval' | 'Execute' | 'Verify' | 'Completed' | 'Failed' | 'Partial';
  status: string;
  strategy: string | null;
  summary: string | null;
  markdown: string | null;
  validation_issues?: ValidationIssue[];
  steps: PlanStep[];
  todos: TodoItem[];
  created_at: string;
  updated_at: string;
  execution_id?: string | null;
}
```

**VIVY 需要定义对应的 domain 类型和存储接口。**

### 4.2 Approval 数据结构差异

**Agent Diva 的 ApprovalView：**
```typescript
interface ApprovalView {
  request_id: string;
  version: number;
  status: 'pending' | 'approved' | 'denied' | 'cancelled';
  domain: 'tool' | 'plan';
  resource: {
    session_id: string;
    resource_id: string;
    type: string;
  };
  presentation?: {
    session_key: string;
  };
  expires_at: string;
  // ...
}
```

**VIVY 的 approvalResult：**
```go
type approvalResult struct {
    ID         string       `json:"id"`
    RunID      domain.RunID `json:"run_id"`
    ToolCallID string       `json:"tool_call_id"`
    Decision   string       `json:"decision"`
    ExpiresAt  int64        `json:"expires_at"`
}
```

**差异：** VIVY 缺少 `version`、`domain`、`resource`、`presentation` 等字段，需要扩展以支持计划审批。

---

## 5. 下一步行动

1. **确认 VIVY 的 Plan 域模型是否存在**
   - 检查 `internal/domain/` 是否有 Plan 相关类型
   - 如果没有，需要定义 `domain.Plan`、`domain.PlanRevision`、`domain.PlanReport` 等

2. **确认 VIVY 的 Approval Store 是否支持 Plan 审批**
   - 检查 `storage.ApprovalStore` 接口
   - 确认是否需要扩展以支持 `domain='plan'` 的审批

3. **设计计划管理的存储层**
   - 定义 `storage.PlanStore` 接口
   - 实现 SQLite backend

4. **逐个实施缺失的 RPC 端点**
   - 按优先级从高到低
   - 每个端点配套单元测试

5. **更新 UI 层的 `rpc.ts`**
   - 添加新的 RPC 调用封装
   - 确保类型安全

---

## 6. 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| Plan 域模型完全缺失 | 高 | 需要从头设计，参考 Agent Diva 的实现 |
| Approval Store 不支持 Plan | 中 | 扩展存储接口，保持向后兼容 |
| 配置管理复杂度高 | 中 | 分阶段实施，先支持 Provider，再扩展其他 |
| Channel/Skills 依赖外部服务 | 低 | 可以先实现 stub，后续对接真实服务 |

---

**文档版本：** v0.1  
**最后更新：** 2026-01-XX  
**维护者：** UI Migration Team
