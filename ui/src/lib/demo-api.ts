/**
 * 本地演示数据层。此文件不代表 Vivy 服务端状态。
 * 仅供明确标记为“演示 / 本地模拟”的页面使用。
 */

import type {
  Message,
  Session,
  SessionHistory,
  AgentResponse,
  PlanRuntimeState,
  PlanApprovalResult,
  PlanStreamEvent,
  ApprovalView,
  ApprovalListPage,
  ApprovalEventView,
  SkillDto,
  SkillDocument,
  SkillRequest,
  CreateSkillRequestPayload,
  FileAttachmentDto,
  RuntimeConfig,
  ToolsConfigShape,
  GatewayProcessStatus,
  TokenStatsSnapshot,
  PersonaProfile,
  ChecklistItem,
  NotebookReport,
  SessionSearchResponse,
  PersonaKind,
  PersonaDocument,
  PersonaChangeRequest,
  PersonaHistoryEntry,
  PersonaHistoryRevision,
  ReportPeriod,
  CronJobDto,
  PlanSidebarData,
  DemoDashboardSnapshot,
  DemoTokenPeriod,
  DemoTokenUsageSnapshot,
  DemoMemoryItem,
  DemoMcpServer,
  DemoComposerState,
} from './types';

// ==================== 工具函数 ====================

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

const generateId = () => Math.random().toString(36).substring(2, 15);

const now = () => new Date().toISOString();

// localStorage 键名
const STORAGE_KEYS = {
  SESSIONS: 'vivy.demo.sessions',
  MESSAGES: 'vivy.demo.messages',
  CONFIG: 'vivy.demo.config',
  TOOLS_CONFIG: 'vivy.demo.tools-config',
  SKILLS: 'vivy.demo.skills',
  SKILL_REQUESTS: 'vivy.demo.skill-requests',
  APPROVALS: 'vivy.demo.approvals',
  PERSONA: 'vivy.demo.persona',
  DASHBOARD: 'vivy.demo.dashboard',
  MEMORY: 'vivy.demo.memory',
  MCP: 'vivy.demo.mcp',
  COMPOSER: 'vivy.demo.composer',
};

// ==================== 模拟数据生成器 ====================

const MOCK_SESSIONS: Session[] = [
  {
    id: 'session-1',
    title: '欢迎使用 Vivy 演示',
    created_at: '2024-01-15T10:00:00Z',
    updated_at: '2024-01-15T10:30:00Z',
    message_count: 5,
    last_message_preview: '你好！我是你的 AI 助手...',
  },
  {
    id: 'session-2',
    title: '项目规划讨论',
    created_at: '2024-01-14T14:00:00Z',
    updated_at: '2024-01-14T15:30:00Z',
    message_count: 12,
    last_message_preview: '我们可以先制定一个详细的计划...',
  },
  {
    id: 'session-3',
    title: '代码审查',
    created_at: '2024-01-13T09:00:00Z',
    updated_at: '2024-01-13T09:45:00Z',
    message_count: 8,
    last_message_preview: '这段代码有几个可以优化的地方...',
  },
];

const MOCK_MESSAGES: Record<string, Message[]> = {
  'session-1': [
    {
      id: 'msg-1',
      role: 'user',
      content: '你好',
      timestamp: Date.now() - 300000,
    },
    {
      id: 'msg-2',
      role: 'agent',
      content: '你好！这是 Vivy 的本地演示数据。',
      timestamp: Date.now() - 299000,
    },
    {
      id: 'msg-3',
      role: 'user',
      content: '帮我创建一个 React 项目',
      timestamp: Date.now() - 200000,
    },
    {
      id: 'msg-4',
      role: 'agent',
      content: '好的，我来帮你创建一个 React 项目。我会使用 Vite 作为构建工具，并配置好 TypeScript 支持。\n\n首先，让我检查一下当前的环境...',
      isThinking: false,
      timestamp: Date.now() - 199000,
    },
    {
      id: 'msg-5',
      role: 'tool',
      content: '',
      toolName: 'create_project',
      toolArgs: '{"name": "my-react-app", "template": "react-ts"}',
      toolResult: 'Project created successfully',
      toolStatus: 'success',
      timestamp: Date.now() - 150000,
    },
  ],
};

const MOCK_SKILLS: SkillDto[] = [
  {
    slug: 'react-design',
    name: 'React Design',
    description: 'React 19 + Vite + TanStack Router + shadcn/ui 脚手架的工程规范',
    source: 'builtin',
    enabled: true,
    always: false,
    available: true,
    active: true,
    content_hash: 'abc123',
    updated_at: '2024-01-15T10:00:00Z',
    can_hard_delete: false,
    path: '/skills/react-design',
    can_delete: false,
  },
  {
    slug: 'meoo-cloud',
    name: '本地演示 Provider',
    description: '提供云服务后端能力（数据库、认证、文件存储、Edge Functions）',
    source: 'builtin',
    enabled: true,
    always: false,
    available: true,
    active: true,
    content_hash: 'def456',
    updated_at: '2024-01-15T10:00:00Z',
    can_hard_delete: false,
    path: '/skills/meoo-cloud',
    can_delete: false,
  },
  {
    slug: 'web-design-guidelines',
    name: 'Web Design Guidelines',
    description: '根据 Web 界面设计规范审查 UI 代码',
    source: 'builtin',
    enabled: true,
    always: false,
    available: true,
    active: false,
    content_hash: 'ghi789',
    updated_at: '2024-01-15T10:00:00Z',
    can_hard_delete: true,
    path: '/skills/web-design-guidelines',
    can_delete: true,
  },
];

const MOCK_APPROVALS: ApprovalView[] = [
  {
    request_id: 'approval-1',
    version: 1,
    domain: 'command',
    capability: 'file_write',
    resource: {
      workspace_id: 'ws-1',
      session_id: 'session-1',
      kind: 'file',
      resource_id: '/home/project/src/App.tsx',
      boundary: null,
    },
    risk: 'write',
    status: 'pending',
    created_at: '2024-01-15T10:00:00Z',
    expires_at: '2024-01-15T11:00:00Z',
    subject: { kind: 'command', id: 'cmd-1' },
    evidence: [],
    receipt: null,
    reason_code: null,
    actions: ['allow', 'deny'],
    presentation: {
      title: '允许写入文件',
      description: 'Agent 请求修改 src/App.tsx 文件',
    },
  },
];

const MOCK_PLAN: PlanRuntimeState = {
  plan_id: 'plan-1',
  revision: 1,
  title: '创建 React 示例项目',
  goal: '创建一个功能完整的 React 前端示例项目，包含聊天界面和设置页面',
  phase: 'planning',
  status: 'draft',
  strategy: '分阶段实施，先建立基础架构，再实现核心功能',
  summary: '本地演示计划，不代表 Vivy 服务端状态',
  markdown: `# 创建 React 示例项目

## 目标
创建一个功能完整的 React 前端示例项目

## 范围
- 聊天界面
- 会话管理
- 设置页面
- 技能管理

## 计划步骤
1. 建立类型系统
2. 实现 Mock API 层
3. 创建自定义 Hooks
4. 转换核心组件

## 风险与假设
- 假设：Vue 到 React 的转换不会丢失核心功能
- 风险：部分复杂组件可能需要简化

## 验证方法
- 构建通过
- 基本交互可用
`,
  validation_issues: [],
  steps: [
    {
      id: 'step-1',
      ordinal: 1,
      title: '建立类型系统',
      rationale: '为项目提供完整的类型定义',
      expected_output: 'types.ts 文件',
      status: 'completed',
    },
    {
      id: 'step-2',
      ordinal: 2,
      title: '实现 Mock API 层',
      rationale: '模拟后端接口',
      expected_output: '演示数据文件',
      status: 'in_progress',
    },
  ],
  todos: [
    {
      id: 'todo-1',
      plan_step_id: 'step-1',
      title: '提取核心类型定义',
      detail: '从原项目提取 Message、Session、Plan 等类型',
      status: 'completed',
      priority: 'high',
      evidence_ref: null,
      block_reason: null,
      updated_at: '2024-01-15T10:00:00Z',
    },
    {
      id: 'todo-2',
      plan_step_id: 'step-2',
      title: '创建模拟数据',
      detail: '生成合理的模拟数据用于演示',
      status: 'in_progress',
      priority: 'high',
      evidence_ref: null,
      block_reason: null,
      updated_at: '2024-01-15T10:00:00Z',
    },
  ],
  created_at: '2024-01-15T09:00:00Z',
  updated_at: '2024-01-15T10:00:00Z',
  execution_id: null,
  initialization_status: 'Ready',
  initialization_error: null,
};

const MOCK_CONFIG: RuntimeConfig = {
  provider: 'deepseek',
  model: 'deepseek-chat',
  api_base: 'https://api.deepseek.com/v1',
  api_key: '',
  temperature: 0.7,
  max_tokens: 4096,
};

const MOCK_TOOLS_CONFIG: ToolsConfigShape = {
  sandbox_enabled: true,
  command_rules: [
    { command: 'read', enabled: true, approval_required: false },
    { command: 'write', enabled: true, approval_required: true },
    { command: 'bash', enabled: true, approval_required: true },
  ],
};

const MOCK_PERSONA: PersonaProfile = {
  id: 'persona-default',
  name: '默认助手',
  avatar_url: null,
  system_prompt: '你是一个专业的 AI 助手，能够帮助用户完成各种任务。',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-15T10:00:00Z',
};

// ==================== API 函数实现 ====================

/**
 * 发送消息并获取 AI 回复
 */
export async function sendMessage(sessionId: string, content: string): Promise<AgentResponse> {
  await delay(500 + Math.random() * 1000);

  // 简单的模拟回复逻辑
  const responses: Record<string, string> = {
    '你好': '你好！这是 Vivy 的本地演示回复。',
    '帮我创建一个项目': '好的，我来帮你创建一个项目。请告诉我你想要创建什么类型的项目？',
    '默认': '我理解你的需求。让我来处理这个问题...\n\n我已经完成了相关操作。如果你还有其他问题，请随时告诉我。',
  };

  let replyContent = responses['默认'];
  for (const [key, value] of Object.entries(responses)) {
    if (key !== '默认' && content.includes(key)) {
      replyContent = value;
      break;
    }
  }

  return {
    content: replyContent,
    reasoning: '基于用户输入的内容，我生成了相应的回复。',
  };
}

/**
 * 获取会话列表
 */
export async function getSessions(): Promise<Session[]> {
  await delay(200);
  const cached = localStorage.getItem(STORAGE_KEYS.SESSIONS);
  if (cached) {
    return JSON.parse(cached);
  }
  localStorage.setItem(STORAGE_KEYS.SESSIONS, JSON.stringify(MOCK_SESSIONS));
  return MOCK_SESSIONS;
}

/**
 * 创建新会话
 */
export async function createSession(title?: string): Promise<Session> {
  await delay(200);
  const sessions = await getSessions();
  const newSession: Session = {
    id: `session-${generateId()}`,
    title: title || '新会话',
    created_at: now(),
    updated_at: now(),
    message_count: 0,
  };
  const updated = [newSession, ...sessions];
  localStorage.setItem(STORAGE_KEYS.SESSIONS, JSON.stringify(updated));
  return newSession;
}

/**
 * 删除会话
 */
export async function deleteSession(sessionId: string): Promise<void> {
  await delay(200);
  const sessions = await getSessions();
  const updated = sessions.filter((s) => s.id !== sessionId);
  localStorage.setItem(STORAGE_KEYS.SESSIONS, JSON.stringify(updated));
  // 同时删除该会话的消息
  localStorage.removeItem(`${STORAGE_KEYS.MESSAGES}-${sessionId}`);
}

/**
 * 重置会话（清空消息）
 */
export async function resetSession(sessionId: string): Promise<void> {
  await delay(200);
  localStorage.removeItem(`${STORAGE_KEYS.MESSAGES}-${sessionId}`);
  // 更新会话的更新时间
  const sessions = await getSessions();
  const updated = sessions.map((s) =>
    s.id === sessionId ? { ...s, updated_at: now(), message_count: 0 } : s
  );
  localStorage.setItem(STORAGE_KEYS.SESSIONS, JSON.stringify(updated));
}

/**
 * 获取会话历史消息
 */
export async function getSessionMessages(sessionId: string): Promise<Message[]> {
  await delay(200);
  const cached = localStorage.getItem(`${STORAGE_KEYS.MESSAGES}-${sessionId}`);
  if (cached) {
    return JSON.parse(cached);
  }
  const messages = MOCK_MESSAGES[sessionId] || [];
  localStorage.setItem(`${STORAGE_KEYS.MESSAGES}-${sessionId}`, JSON.stringify(messages));
  return messages;
}

/**
 * 保存消息到会话
 */
export async function saveMessage(sessionId: string, message: Message): Promise<void> {
  const messages = await getSessionMessages(sessionId);
  messages.push(message);
  localStorage.setItem(`${STORAGE_KEYS.MESSAGES}-${sessionId}`, JSON.stringify(messages));

  // 更新会话的最后预览
  const sessions = await getSessions();
  const updated = sessions.map((s) =>
    s.id === sessionId
      ? {
          ...s,
          updated_at: now(),
          message_count: messages.length,
          last_message_preview: message.content.substring(0, 50),
        }
      : s
  );
  localStorage.setItem(STORAGE_KEYS.SESSIONS, JSON.stringify(updated));
}

/**
 * 获取活跃计划
 */
export async function getActivePlan(): Promise<PlanRuntimeState | null> {
  await delay(300);
  return MOCK_PLAN;
}

/**
 * 批准计划执行
 */
export async function approveActivePlanExecution(): Promise<PlanApprovalResult> {
  await delay(500);
  return {
    plan: { ...MOCK_PLAN, status: 'approved' },
    receipt: {
      plan_id: MOCK_PLAN.plan_id,
      revision: MOCK_PLAN.revision || 1,
      approved_at: now(),
      todo_policy: 'Optional',
      todos_materialized: true,
    },
  };
}

/**
 * 将活跃计划返回草稿状态
 */
export async function returnActivePlanToDraft(): Promise<PlanRuntimeState> {
  await delay(300);
  return { ...MOCK_PLAN, status: 'draft' };
}

/**
 * 获取命令审批列表
 */
export async function getCommandApprovals(cursor?: string | null): Promise<ApprovalListPage> {
  await delay(300);
  return {
    approvals: MOCK_APPROVALS,
    next_cursor: null,
  };
}

/**
 * 处理命令审批
 */
export async function resolveCommandApproval(
  requestId: string,
  decision: 'allow' | 'deny',
  grant?: 'once' | 'session' | 'rule'
): Promise<ApprovalView> {
  await delay(300);
  const approval = MOCK_APPROVALS.find((a) => a.request_id === requestId);
  if (!approval) {
    throw new Error('Approval not found');
  }
  return {
    ...approval,
    status: decision === 'allow' ? 'allowed' : 'denied',
  };
}

/**
 * 获取技能列表
 */
export async function listSkills(): Promise<SkillDto[]> {
  await delay(200);
  const cached = localStorage.getItem(STORAGE_KEYS.SKILLS);
  if (cached) {
    return JSON.parse(cached);
  }
  localStorage.setItem(STORAGE_KEYS.SKILLS, JSON.stringify(MOCK_SKILLS));
  return MOCK_SKILLS;
}

/**
 * 获取技能文档
 */
export async function getSkillDocument(slug: string): Promise<SkillDocument | null> {
  await delay(200);
  const skill = MOCK_SKILLS.find((s) => s.slug === slug);
  if (!skill) return null;

  return {
    slug: skill.slug,
    description: skill.description,
    source: skill.source,
    enabled: skill.enabled,
    always: skill.always,
    available: skill.available,
    content_hash: skill.content_hash,
    updated_at: skill.updated_at,
    can_hard_delete: skill.can_hard_delete,
    evolution_managed: skill.evolution_managed,
    markdown: `# ${skill.name}\n\n${skill.description}\n\n## 使用说明\n\n这是一个模拟的技能文档内容。在实际项目中，这里会包含详细的技能说明和使用指南。`,
  };
}

/**
 * 创建技能请求
 */
export async function createSkillRequest(payload: CreateSkillRequestPayload): Promise<SkillRequest> {
  await delay(300);
  const requests = JSON.parse(localStorage.getItem(STORAGE_KEYS.SKILL_REQUESTS) || '[]');
  const newRequest: SkillRequest = {
    id: `req-${generateId()}`,
    slug: payload.slug,
    title: payload.title,
    proposed_markdown: payload.proposed_markdown,
    evidence: payload.evidence || [],
    attestation: payload.attestation,
    base_hash: payload.base_hash,
    source: 'user_request',
    reason: payload.reason,
    status: 'pending',
    created_at: now(),
    updated_at: now(),
  };
  requests.push(newRequest);
  localStorage.setItem(STORAGE_KEYS.SKILL_REQUESTS, JSON.stringify(requests));
  return newRequest;
}

/**
 * 获取技能请求列表
 */
export async function getSkillRequests(): Promise<SkillRequest[]> {
  await delay(200);
  return JSON.parse(localStorage.getItem(STORAGE_KEYS.SKILL_REQUESTS) || '[]');
}

/**
 * 获取文件附件列表
 */
export async function getFileAttachments(): Promise<FileAttachmentDto[]> {
  await delay(200);
  return [
    {
      file_id: 'file-1',
      filename: 'example.pdf',
      size: 1024000,
      mime_type: 'application/pdf',
      channel: 'chat',
      message_id: 'msg-1',
      uploaded_by: 'user-1',
      stored_at: '2024-01-15T10:00:00Z',
      ref_count: 1,
    },
  ];
}

/**
 * 获取运行时配置
 */
export async function getConfig(): Promise<RuntimeConfig> {
  await delay(200);
  const cached = localStorage.getItem(STORAGE_KEYS.CONFIG);
  if (cached) {
    return JSON.parse(cached);
  }
  localStorage.setItem(STORAGE_KEYS.CONFIG, JSON.stringify(MOCK_CONFIG));
  return MOCK_CONFIG;
}

/**
 * 更新运行时配置
 */
export async function updateConfig(config: Partial<RuntimeConfig>): Promise<RuntimeConfig> {
  await delay(200);
  const current = await getConfig();
  const updated = { ...current, ...config };
  localStorage.setItem(STORAGE_KEYS.CONFIG, JSON.stringify(updated));
  return updated;
}

/**
 * 获取工具配置
 */
export async function getToolsConfig(): Promise<ToolsConfigShape> {
  await delay(200);
  const cached = localStorage.getItem(STORAGE_KEYS.TOOLS_CONFIG);
  if (cached) {
    return JSON.parse(cached);
  }
  localStorage.setItem(STORAGE_KEYS.TOOLS_CONFIG, JSON.stringify(MOCK_TOOLS_CONFIG));
  return MOCK_TOOLS_CONFIG;
}

/**
 * 更新工具配置
 */
export async function updateToolsConfig(config: Partial<ToolsConfigShape>): Promise<ToolsConfigShape> {
  await delay(200);
  const current = await getToolsConfig();
  const updated = { ...current, ...config };
  localStorage.setItem(STORAGE_KEYS.TOOLS_CONFIG, JSON.stringify(updated));
  return updated;
}

/**
 * 获取网关进程状态
 */
export async function getGatewayStatus(): Promise<GatewayProcessStatus> {
  await delay(200);
  return {
    running: false,
    pid: null,
    executable_path: null,
    details: '网关未启动（示例模式）',
  };
}

/**
 * 获取 Token 统计
 */
export async function getTokenStats(sessionId: string): Promise<TokenStatsSnapshot | null> {
  await delay(200);
  return {
    session_id: sessionId,
    total_tokens: 12345,
    prompt_tokens: 8000,
    completion_tokens: 4345,
    cached_tokens: 2000,
    updated_at: now(),
  };
}

/**
 * 获取 Persona 配置
 */
export async function getPersonaProfile(): Promise<PersonaProfile> {
  await delay(200);
  const cached = localStorage.getItem(STORAGE_KEYS.PERSONA);
  if (cached) {
    return JSON.parse(cached);
  }
  localStorage.setItem(STORAGE_KEYS.PERSONA, JSON.stringify(MOCK_PERSONA));
  return MOCK_PERSONA;
}

/**
 * 更新 Persona 配置
 */
export async function updatePersonaProfile(profile: Partial<PersonaProfile>): Promise<PersonaProfile> {
  await delay(200);
  const current = await getPersonaProfile();
  const updated = { ...current, ...profile, updated_at: now() };
  localStorage.setItem(STORAGE_KEYS.PERSONA, JSON.stringify(updated));
  return updated;
}

/**
 * 获取配置状态（用于检查是否已初始化）
 */
export async function getConfigStatus(): Promise<{ configured: boolean; missing_fields: string[] }> {
  await delay(200);
  const config = await getConfig();
  const missingFields: string[] = [];
  if (!config.api_key) missingFields.push('api_key');
  if (!config.model) missingFields.push('model');
  return {
    configured: missingFields.length === 0,
    missing_fields: missingFields,
  };
}

/**
 * 获取运行时配置（别名）
 */
export async function getRuntimeConfig(): Promise<RuntimeConfig> {
  return getConfig();
}

// ==================== Notebook 相关 Mock ====================

const MOCK_NOTEBOOK_REPORTS: NotebookReport[] = [
  {
    id: 'report-1',
    period: 'daily',
    date: '2024-01-15',
    title: '每日工作摘要 - 2024/01/15',
    summary: '今天完成了项目初始化和核心功能开发',
    content: `# 每日工作摘要

## 完成的工作
- 建立了类型系统
- 实现了 Mock API 层
- 创建了自定义 Hooks

## 明日计划
- 转换核心组件
- 设计路由结构

## 遇到的问题
暂无
`,
    generatedAt: '2024-01-15T23:00:00Z',
    generatedBy: 'vivy-demo',
    generationMode: 'llm_curated',
  },
  {
    id: 'report-2',
    period: 'weekly',
    date: '2024-01-14',
    title: '周度总结 - 第2周',
    summary: '本周主要聚焦于前端架构搭建',
    content: `# 周度总结

## 本周成果
- 完成技术选型
- 搭建项目脚手架
- 实现基础功能模块

## 下周目标
- 完善用户界面
- 添加更多交互功能
`,
    generatedAt: '2024-01-14T23:00:00Z',
    generatedBy: 'vivy-demo',
    generationMode: 'deterministic_fallback',
  },
];

const STORAGE_KEYS_NOTEBOOK = 'vivy.demo.notebook-reports';

/**
 * 获取笔记本报告列表
 */
export async function getNotebookReports(period?: ReportPeriod): Promise<NotebookReport[]> {
  await delay(200);
  const cached = localStorage.getItem(STORAGE_KEYS_NOTEBOOK);
  if (cached) {
    const reports: NotebookReport[] = JSON.parse(cached);
    return period ? reports.filter((r) => r.period === period) : reports;
  }
  localStorage.setItem(STORAGE_KEYS_NOTEBOOK, JSON.stringify(MOCK_NOTEBOOK_REPORTS));
  return period ? MOCK_NOTEBOOK_REPORTS.filter((r) => r.period === period) : MOCK_NOTEBOOK_REPORTS;
}

/**
 * 搜索会话内容
 */
export async function searchSessions(query: string): Promise<SessionSearchResponse> {
  await delay(300);
  const sessions = await getSessions();
  const hits: SessionSearchResponse['hits'] = [];

  for (const session of sessions) {
    const messages = await getSessionMessages(session.id);
    for (let i = 0; i < messages.length; i++) {
      const msg = messages[i];
      if (msg.content.toLowerCase().includes(query.toLowerCase())) {
        hits.push({
          session_id: session.id,
          timestamp: new Date(msg.timestamp || Date.now()).toISOString(),
          snippet: msg.content.substring(0, 200),
          source_uri: `/chat/${session.id}`,
          hash: `hash-${session.id}-${i}`,
          source: 'session',
          snippet_truncated: msg.content.length > 200,
          message_index: i,
        });
      }
    }
  }

  return {
    hits,
    diagnostics: [],
    scanned_files: sessions.length,
    skipped_files: 0,
    total_response_bytes: JSON.stringify(hits).length,
    file_limit_reached: false,
    result_limit_reached: false,
    byte_limit_reached: false,
  };
}

// ==================== Persona Memory 相关 Mock ====================

const PERSONA_KINDS: PersonaKind[] = ['identity', 'relationship', 'redline', 'user', 'world', 'dream', 'dark'];

const MOCK_PERSONA_DOCS: Record<PersonaKind, PersonaDocument> = {
  identity: {
    kind: 'identity',
    content: '# IDENTITY.MD\n\n这是 Vivy Persona 的本地演示文档。\n\n## 核心特质\n- 专业且友好\n- 善于分析和解决问题\n- 注重细节和准确性',
    revision: 1,
    updated_at: '2024-01-15T10:00:00Z',
  },
  relationship: {
    kind: 'relationship',
    content: '# RELATIONSHIP.MD\n\n## 与用户的关系\n- 助手与协作者\n- 尊重用户的决策权\n- 主动提供建议但不越界',
    revision: 1,
    updated_at: '2024-01-15T10:00:00Z',
  },
  redline: {
    kind: 'redline',
    content: '# REDLINE.MD\n\n## 行为红线\n- 不执行危险操作\n- 不泄露敏感信息\n- 不生成有害内容',
    revision: 1,
    updated_at: '2024-01-15T10:00:00Z',
  },
  user: {
    kind: 'user',
    content: '# USER.MD\n\n## 用户偏好\n- 喜欢简洁明了的回答\n- 偏好代码示例\n- 重视实用性',
    revision: 1,
    updated_at: '2024-01-15T10:00:00Z',
  },
  world: {
    kind: 'world',
    content: '# WORLD.MD\n\n## 世界观\n- 技术驱动进步\n- 开放协作\n- 持续学习',
    revision: 1,
    updated_at: '2024-01-15T10:00:00Z',
  },
  dream: {
    kind: 'dream',
    content: '# DREAM.MD\n\n## 愿景\n成为最懂你的 AI 伙伴',
    revision: 1,
    updated_at: '2024-01-15T10:00:00Z',
  },
  dark: {
    kind: 'dark',
    content: '# DARK.MD\n\n## 注意事项\n此文档包含敏感设定，请谨慎编辑',
    revision: 1,
    updated_at: '2024-01-15T10:00:00Z',
  },
};

const STORAGE_KEYS_PERSONA_DOCS = 'vivy.demo.persona-docs';
const STORAGE_KEYS_PERSONA_REQUESTS = 'vivy.demo.persona-requests';
const STORAGE_KEYS_PERSONA_HISTORY = 'vivy.demo.persona-history';

/**
 * 获取 Persona 文档
 */
export async function getPersonaDocument(kind: PersonaKind): Promise<PersonaDocument> {
  await delay(200);
  const cached = localStorage.getItem(STORAGE_KEYS_PERSONA_DOCS);
  const docs: Record<PersonaKind, PersonaDocument> = cached ? JSON.parse(cached) : MOCK_PERSONA_DOCS;
  if (!cached) {
    localStorage.setItem(STORAGE_KEYS_PERSONA_DOCS, JSON.stringify(MOCK_PERSONA_DOCS));
  }
  return docs[kind];
}

/**
 * 保存 Persona 文档
 */
export async function savePersonaDocument(
  kind: PersonaKind,
  content: string,
  revision: number
): Promise<{ document: PersonaDocument; changed: boolean }> {
  await delay(300);
  const cached = localStorage.getItem(STORAGE_KEYS_PERSONA_DOCS);
  const docs: Record<PersonaKind, PersonaDocument> = cached ? JSON.parse(cached) : { ...MOCK_PERSONA_DOCS };

  const current = docs[kind];
  const changed = current.content !== content;

  const updated: PersonaDocument = {
    ...current,
    content,
    revision: current.revision + 1,
    updated_at: now(),
  };
  docs[kind] = updated;
  localStorage.setItem(STORAGE_KEYS_PERSONA_DOCS, JSON.stringify(docs));

  // 记录历史
  const historyCached = localStorage.getItem(STORAGE_KEYS_PERSONA_HISTORY);
  const history: Record<PersonaKind, PersonaHistoryEntry[]> = historyCached ? JSON.parse(historyCached) : {};
  if (!history[kind]) history[kind] = [];
  history[kind].unshift({
    revision: updated.revision,
    updated_at: updated.updated_at,
    content_hash: `hash-${updated.revision}`,
  });
  localStorage.setItem(STORAGE_KEYS_PERSONA_HISTORY, JSON.stringify(history));

  return { document: updated, changed };
}

/**
 * 获取 Persona 变更请求列表
 */
export async function listPersonaRequests(kind: PersonaKind): Promise<PersonaChangeRequest[]> {
  await delay(200);
  const cached = localStorage.getItem(STORAGE_KEYS_PERSONA_REQUESTS);
  const allRequests: PersonaChangeRequest[] = cached ? JSON.parse(cached) : [];
  return allRequests.filter((r) => r.kind === kind);
}

/**
 * 接受 Persona 变更请求
 */
export async function acceptPersonaRequest(requestId: string): Promise<void> {
  await delay(300);
  const cached = localStorage.getItem(STORAGE_KEYS_PERSONA_REQUESTS);
  const requests: PersonaChangeRequest[] = cached ? JSON.parse(cached) : [];
  const updated = requests.map((r) =>
    r.id === requestId ? { ...r, state: 'accepted' as const } : r
  );
  localStorage.setItem(STORAGE_KEYS_PERSONA_REQUESTS, JSON.stringify(updated));
}

/**
 * 拒绝 Persona 变更请求
 */
export async function rejectPersonaRequest(requestId: string): Promise<void> {
  await delay(300);
  const cached = localStorage.getItem(STORAGE_KEYS_PERSONA_REQUESTS);
  const requests: PersonaChangeRequest[] = cached ? JSON.parse(cached) : [];
  const updated = requests.map((r) =>
    r.id === requestId ? { ...r, state: 'rejected' as const } : r
  );
  localStorage.setItem(STORAGE_KEYS_PERSONA_REQUESTS, JSON.stringify(updated));
}

/**
 * 获取 Persona 历史记录
 */
export async function listPersonaHistory(kind: PersonaKind): Promise<PersonaHistoryEntry[]> {
  await delay(200);
  const cached = localStorage.getItem(STORAGE_KEYS_PERSONA_HISTORY);
  const history: Record<PersonaKind, PersonaHistoryEntry[]> = cached ? JSON.parse(cached) : {};
  return history[kind] || [];
}

/**
 * 获取 Persona 历史版本内容
 */
export async function getPersonaHistoryRevision(
  kind: PersonaKind,
  revision: number
): Promise<PersonaHistoryRevision> {
  await delay(200);
  return {
    revision,
    content: `# 历史版本 ${revision}\n\n这是 ${kind} 的第 ${revision} 个版本的内容快照。`,
    updated_at: '2024-01-15T10:00:00Z',
  };
}

// ==================== Cron Tasks 相关 Mock ====================

const MOCK_CRON_JOBS: CronJobDto[] = [
  {
    id: 'cron-1',
    name: '每日报告生成',
    enabled: true,
    schedule: { kind: 'cron', expr: '0 9 * * *', tz: 'Asia/Shanghai' },
    payload: { kind: 'notebook_report', message: '生成每日工作摘要', deliver: true, channel: 'chat' },
    state: {
      nextRunAtMs: Date.now() + 86400000,
      lastRunAtMs: Date.now() - 86400000,
      lastStatus: 'completed',
      lastError: null,
    },
    createdAtMs: Date.now() - 7 * 86400000,
    updatedAtMs: Date.now() - 86400000,
    deleteAfterRun: false,
    isRunning: false,
    activeRun: null,
    computedStatus: 'scheduled',
  },
  {
    id: 'cron-2',
    name: '会话清理',
    enabled: false,
    schedule: { kind: 'every', everyMs: 604800000 },
    payload: { kind: 'cleanup', message: '清理过期会话', deliver: false },
    state: {
      nextRunAtMs: null,
      lastRunAtMs: Date.now() - 604800000,
      lastStatus: 'completed',
      lastError: null,
    },
    createdAtMs: Date.now() - 30 * 86400000,
    updatedAtMs: Date.now() - 604800000,
    deleteAfterRun: false,
    isRunning: false,
    activeRun: null,
    computedStatus: 'paused',
  },
];

const STORAGE_KEYS_CRON = 'vivy.demo.cron-jobs';

/**
 * 获取定时任务列表
 */
export async function getCronJobs(): Promise<CronJobDto[]> {
  await delay(200);
  const cached = localStorage.getItem(STORAGE_KEYS_CRON);
  if (cached) {
    return JSON.parse(cached);
  }
  localStorage.setItem(STORAGE_KEYS_CRON, JSON.stringify(MOCK_CRON_JOBS));
  return MOCK_CRON_JOBS;
}

/**
 * 创建定时任务
 */
export async function createCronJob(job: Omit<CronJobDto, 'id' | 'createdAtMs' | 'updatedAtMs' | 'state' | 'isRunning' | 'activeRun' | 'computedStatus'>): Promise<CronJobDto> {
  await delay(300);
  const jobs = await getCronJobs();
  const newJob: CronJobDto = {
    ...job,
    id: `cron-${generateId()}`,
    createdAtMs: Date.now(),
    updatedAtMs: Date.now(),
    state: {
      nextRunAtMs: null,
      lastRunAtMs: null,
      lastStatus: null,
      lastError: null,
    },
    isRunning: false,
    activeRun: null,
    computedStatus: job.enabled ? 'scheduled' : 'paused',
  };
  const updated = [...jobs, newJob];
  localStorage.setItem(STORAGE_KEYS_CRON, JSON.stringify(updated));
  return newJob;
}

/**
 * 更新定时任务
 */
export async function updateCronJob(id: string, updates: Partial<CronJobDto>): Promise<CronJobDto> {
  await delay(300);
  const jobs = await getCronJobs();
  const index = jobs.findIndex((j) => j.id === id);
  if (index === -1) throw new Error('Job not found');
  const updated = { ...jobs[index], ...updates, updatedAtMs: Date.now() };
  jobs[index] = updated;
  localStorage.setItem(STORAGE_KEYS_CRON, JSON.stringify(jobs));
  return updated;
}

/**
 * 删除定时任务
 */
export async function deleteCronJob(id: string): Promise<void> {
  await delay(300);
  const jobs = await getCronJobs();
  const updated = jobs.filter((j) => j.id !== id);
  localStorage.setItem(STORAGE_KEYS_CRON, JSON.stringify(updated));
}

/**
 * 手动触发定时任务
 */
export async function triggerCronJob(id: string): Promise<void> {
  await delay(300);
  const jobs = await getCronJobs();
  const job = jobs.find((j) => j.id === id);
  if (!job) throw new Error('Job not found');
  job.isRunning = true;
  job.state.lastRunAtMs = Date.now();
  job.state.lastStatus = 'running';
  localStorage.setItem(STORAGE_KEYS_CRON, JSON.stringify(jobs));

  // 模拟执行完成
  setTimeout(async () => {
    const currentJobs = await getCronJobs();
    const currentJob = currentJobs.find((j) => j.id === id);
    if (currentJob) {
      currentJob.isRunning = false;
      currentJob.state.lastStatus = 'completed';
      localStorage.setItem(STORAGE_KEYS_CRON, JSON.stringify(currentJobs));
    }
  }, 2000);
}

// ==================== Plan Sidebar 相关 Mock ====================

/**
 * 获取计划侧边栏数据
 */
export async function getPlanSidebarData(): Promise<PlanSidebarData> {
  await delay(200);
  const plan = await getActivePlan();
  return {
    plan,
    todos: plan?.todos || [],
    validation_issues: plan?.validation_issues || [],
  };
}

// ==================== Notebook 生成报告 Mock ====================

/**
 * 生成指定周期的报告（日报/周报/月报）
 * 基于当前会话内容生成一份模拟报告，并持久化到 localStorage
 */
export async function generateNotebookReport(period: ReportPeriod): Promise<NotebookReport> {
  await delay(600);

  const sessions = await getSessions();
  const today = new Date();
  const dateStr = today.toISOString().slice(0, 10);

  const periodLabel = period === 'daily' ? '日报' : period === 'weekly' ? '周报' : '月报';
  const title = `${periodLabel} - ${dateStr}`;

  // 汇总会话消息作为报告素材
  let totalMessages = 0;
  const topics: string[] = [];
  for (const session of sessions) {
    const messages = await getSessionMessages(session.id);
    totalMessages += messages.length;
    if (session.title) topics.push(session.title);
  }

  const content = `# ${title}

## 概览
- 会话数量：${sessions.length}
- 消息总数：${totalMessages}
- 涉及主题：${topics.length > 0 ? topics.join('、') : '暂无'}

## 主要进展
- 完成了与 AI 助手的多轮对话
- 围绕 ${topics[0] || '日常交流'} 展开了讨论

## 待办事项
- 继续推进当前计划
- 关注后续任务进展

## 备注
本报告由 Vivy 本地演示生成，不使用真实会话数据。
`;

  const report: NotebookReport = {
    id: generateId(),
    period,
    date: dateStr,
    title,
    summary: `共 ${sessions.length} 个会话、${totalMessages} 条消息的自动归纳`,
    content,
    generatedAt: now(),
    generatedBy: 'vivy-demo',
    generationMode: 'llm_curated',
  };

  // 持久化
  const cached = localStorage.getItem(STORAGE_KEYS_NOTEBOOK);
  const reports: NotebookReport[] = cached ? JSON.parse(cached) : [];
  reports.unshift(report);
  localStorage.setItem(STORAGE_KEYS_NOTEBOOK, JSON.stringify(reports));

  return report;
}

// ==================== Restored local demo surfaces ====================

const DEFAULT_DASHBOARD: DemoDashboardSnapshot = {
  sessionCount: 12,
  activeRuns: 2,
  pendingReviews: 1,
  tokenUsage: 42860,
  recentActivity: [
    { id: 'activity-1', title: '日报已生成', detail: '汇总了 3 个演示会话', occurredAt: '今天 09:30' },
    { id: 'activity-2', title: '技能变更待处理', detail: 'react-design 请求更新', occurredAt: '昨天 18:10' },
    { id: 'activity-3', title: '定时任务完成', detail: '会话清理运行成功', occurredAt: '昨天 12:00' },
  ],
};

const DEFAULT_MEMORIES: DemoMemoryItem[] = [
  { id: 'memory-1', title: '回答偏好', category: 'preference', content: '偏好简洁、可执行并带验证结果的回答。', updatedAt: '2024-01-15T10:00:00Z' },
  { id: 'memory-2', title: 'Vivy UI 重构', category: 'project', content: '当前界面采用 React、TanStack Router 与蓝色主题。', updatedAt: '2024-01-14T16:30:00Z' },
  { id: 'memory-3', title: '演示数据边界', category: 'decision', content: '没有后端 API 的页面只使用 vivy.demo.* 本地数据。', updatedAt: '2024-01-13T09:20:00Z' },
];

const DEFAULT_MCP_SERVERS: DemoMcpServer[] = [
  { id: 'mcp-files', name: 'Workspace Files', transport: 'stdio', command: 'npx -y @modelcontextprotocol/server-filesystem .', enabled: true, toolCount: 8 },
  { id: 'mcp-browser', name: 'Browser Tools', transport: 'http', url: 'http://127.0.0.1:9123/mcp', enabled: false, toolCount: 5 },
];

const DEFAULT_COMPOSER: DemoComposerState = { mode: 'agent', secure: true, recording: false };

function readDemoValue<T>(key: string, fallback: T): T {
  const cached = localStorage.getItem(key);
  if (cached) {
    try { return JSON.parse(cached) as T; }
    catch { localStorage.removeItem(key); }
  }
  localStorage.setItem(key, JSON.stringify(fallback));
  return fallback;
}

function writeDemoValue<T>(key: string, value: T): T {
  localStorage.setItem(key, JSON.stringify(value));
  return value;
}

export async function getDemoDashboard(): Promise<DemoDashboardSnapshot> {
  await delay(120);
  return readDemoValue(STORAGE_KEYS.DASHBOARD, DEFAULT_DASHBOARD);
}

const TOKEN_PERIOD_SCALE: Record<DemoTokenPeriod, number> = {
  '1d': 1,
  '3d': 2.4,
  '1w': 4.8,
  '1m': 16,
  '6m': 72,
  '1y': 130,
};

const BASE_TOKEN_SESSIONS = [
  { id: 'session-1', title: '欢迎使用 Vivy 演示', model: 'deepseek-chat', request_count: 18, total_input: 9200, total_output: 4100, total_cost: 0.082 },
  { id: 'session-2', title: '中控台设计讨论', model: 'claude-sonnet-4', request_count: 11, total_input: 6400, total_output: 2800, total_cost: 0.146 },
  { id: 'session-3', title: '插件打包排障', model: 'gpt-4.1-mini', request_count: 9, total_input: 5100, total_output: 1900, total_cost: 0.037 },
  { id: 'session-4', title: '人格文档整理', model: 'deepseek-chat', request_count: 7, total_input: 3600, total_output: 1500, total_cost: 0.028 },
  { id: 'session-5', title: '定时任务验收', model: 'gpt-4.1-mini', request_count: 4, total_input: 1800, total_output: 700, total_cost: 0.012 },
] as const;

const BASE_TOKEN_ENDPOINTS = [
  { key: '对话补全', total_tokens: 24100, total_cost: 0.198, request_count: 32 },
  { key: '工具调用', total_tokens: 9800, total_cost: 0.072, request_count: 12 },
  { key: '上下文压缩', total_tokens: 4200, total_cost: 0.035, request_count: 5 },
] as const;

const TOKEN_TIMELINE_LABELS: Record<DemoTokenPeriod, string[]> = {
  '1d': ['00:00', '02:00', '04:00', '06:00', '08:00', '10:00', '12:00', '14:00', '16:00', '18:00', '20:00', '22:00'],
  '3d': ['8/23', '8/24', '8/25'],
  '1w': ['8/19', '8/20', '8/21', '8/22', '8/23', '8/24', '8/25'],
  '1m': ['第1周', '第2周', '第3周', '第4周'],
  '6m': ['3月', '4月', '5月', '6月', '7月', '8月'],
  '1y': ['1月', '2月', '3月', '4月', '5月', '6月', '7月', '8月', '9月', '10月', '11月', '12月'],
};

function scaleCount(value: number, factor: number): number {
  return Math.round(value * factor);
}

function scaleCost(value: number, factor: number): number {
  return Math.round(value * factor * 10000) / 10000;
}

function buildDemoTokenUsage(period: DemoTokenPeriod): DemoTokenUsageSnapshot {
  const factor = TOKEN_PERIOD_SCALE[period];
  const sessions = BASE_TOKEN_SESSIONS.map((session) => {
    const total_input = scaleCount(session.total_input, factor);
    const total_output = scaleCount(session.total_output, factor);
    return {
      id: session.id,
      title: session.title,
      model: session.model,
      request_count: scaleCount(session.request_count, factor),
      total_input,
      total_output,
      total_tokens: total_input + total_output,
      total_cost: scaleCost(session.total_cost, factor),
    };
  });
  const total_input = sessions.reduce((sum, session) => sum + session.total_input, 0);
  const total_output = sessions.reduce((sum, session) => sum + session.total_output, 0);
  const total_tokens = total_input + total_output;
  const total_cost = sessions.reduce((sum, session) => sum + session.total_cost, 0);
  const request_count = sessions.reduce((sum, session) => sum + session.request_count, 0);
  const modelTotals = new Map<string, number>();
  for (const session of sessions) {
    modelTotals.set(session.model, (modelTotals.get(session.model) ?? 0) + session.total_tokens);
  }
  const models = [...modelTotals.entries()]
    .map(([model, tokens]) => ({
      model,
      total_tokens: tokens,
      percentage: total_tokens === 0 ? 0 : Math.round((tokens / total_tokens) * 1000) / 10,
    }))
    .sort((left, right) => right.total_tokens - left.total_tokens);
  const labels = TOKEN_TIMELINE_LABELS[period];
  const weights = labels.map((_, index) => 0.35 + ((index * 7) % 10) / 12);
  const weightSum = weights.reduce((sum, weight) => sum + weight, 0);
  const timeline = labels.map((label, index) => {
    const share = weights[index] / weightSum;
    const pointTokens = scaleCount(total_tokens * share, 1);
    const inputShare = 0.62 + ((index % 5) - 2) * 0.03;
    const total_input_point = scaleCount(pointTokens * inputShare, 1);
    const total_output_point = Math.max(0, pointTokens - total_input_point);
    return {
      time_bucket: `${period}-${index}`,
      label,
      total_input: total_input_point,
      total_output: total_output_point,
      total_tokens: total_input_point + total_output_point,
    };
  });
  return {
    period,
    total: {
      total_input,
      total_output,
      total_tokens,
      total_cache_creation: scaleCount(2400, factor),
      total_cache_read: scaleCount(8600, factor),
      request_count,
      total_cost: Math.round(total_cost * 10000) / 10000,
    },
    models,
    endpoints: BASE_TOKEN_ENDPOINTS.map((endpoint) => {
      const share = endpoint.total_tokens / BASE_TOKEN_ENDPOINTS.reduce((sum, item) => sum + item.total_tokens, 0);
      return {
        key: endpoint.key,
        total_tokens: scaleCount(total_tokens * share, 1),
        total_cost: scaleCost(total_cost * share, 1),
        request_count: scaleCount(request_count * share, 1),
      };
    }),
    timeline,
    sessions,
  };
}

export function formatTokenCount(count: number): string {
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(2)}M`;
  if (count >= 1_000) return `${(count / 1_000).toFixed(1)}K`;
  return String(count);
}

export function formatTokenCost(cost: number): string {
  if (cost < 0.01) return `$${cost.toFixed(4)}`;
  return `$${cost.toFixed(2)}`;
}

export async function getDemoTokenUsage(period: DemoTokenPeriod = '1d'): Promise<DemoTokenUsageSnapshot> {
  await delay(120);
  return buildDemoTokenUsage(period);
}

export async function getDemoMemories(): Promise<DemoMemoryItem[]> {
  await delay(120);
  return readDemoValue(STORAGE_KEYS.MEMORY, DEFAULT_MEMORIES);
}

export async function getDemoMcpServers(): Promise<DemoMcpServer[]> {
  await delay(120);
  return readDemoValue(STORAGE_KEYS.MCP, DEFAULT_MCP_SERVERS);
}

export interface DemoMcpServerInput {
  name: string;
  transport: DemoMcpServer['transport'];
  command?: string;
  url?: string;
}

function normalizeMcpInput(input: DemoMcpServerInput) {
  const name = input.name.trim();
  if (!name) throw new Error('请输入 MCP 服务名称');
  if (input.transport === 'http') {
    const url = (input.url ?? '').trim();
    if (!url) throw new Error('请输入 HTTP 服务地址');
    let parsed: URL;
    try {
      parsed = new URL(url);
    } catch {
      throw new Error('HTTP 服务地址不是有效的 URL');
    }
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      throw new Error('HTTP 服务地址必须以 http:// 或 https:// 开头');
    }
    return { name, transport: 'http' as const, url, command: undefined };
  }
  const command = (input.command ?? '').trim();
  if (!command) throw new Error('请输入 STDIO 启动命令');
  return { name, transport: 'stdio' as const, command, url: undefined };
}

function assertMcpNameAvailable(servers: DemoMcpServer[], name: string, exceptId?: string) {
  if (servers.some((server) => server.id !== exceptId && server.name.toLowerCase() === name.toLowerCase())) {
    throw new Error('已存在同名 MCP 服务');
  }
}

export async function toggleDemoMcpServer(id: string): Promise<DemoMcpServer[]> {
  await delay(160);
  const servers = readDemoValue(STORAGE_KEYS.MCP, DEFAULT_MCP_SERVERS).map((server) => server.id === id ? { ...server, enabled: !server.enabled } : server);
  return writeDemoValue(STORAGE_KEYS.MCP, servers);
}

export async function addDemoMcpServer(input: DemoMcpServerInput): Promise<DemoMcpServer[]> {
  await delay(180);
  const normalized = normalizeMcpInput(input);
  const servers = readDemoValue(STORAGE_KEYS.MCP, DEFAULT_MCP_SERVERS);
  assertMcpNameAvailable(servers, normalized.name);
  return writeDemoValue(STORAGE_KEYS.MCP, [
    ...servers,
    {
      id: `mcp-${generateId()}`,
      ...normalized,
      enabled: true,
      toolCount: 0,
    },
  ]);
}

export async function updateDemoMcpServer(id: string, input: DemoMcpServerInput): Promise<DemoMcpServer[]> {
  await delay(180);
  const normalized = normalizeMcpInput(input);
  const servers = readDemoValue(STORAGE_KEYS.MCP, DEFAULT_MCP_SERVERS);
  if (!servers.some((server) => server.id === id)) throw new Error('MCP 服务不存在');
  assertMcpNameAvailable(servers, normalized.name, id);
  return writeDemoValue(STORAGE_KEYS.MCP, servers.map((server) => server.id === id ? { ...server, ...normalized } : server));
}

export async function removeDemoMcpServer(id: string): Promise<DemoMcpServer[]> {
  await delay(160);
  const servers = readDemoValue(STORAGE_KEYS.MCP, DEFAULT_MCP_SERVERS);
  return writeDemoValue(STORAGE_KEYS.MCP, servers.filter((server) => server.id !== id));
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export async function importDemoMcpConfig(config: unknown): Promise<DemoMcpServer[]> {
  await delay(180);
  const entries: Array<Pick<DemoMcpServer, 'name' | 'transport' | 'command' | 'url' | 'enabled' | 'toolCount'>> = [];

  const appendEntry = (nameHint: string, value: unknown) => {
    const record = isRecord(value) ? value : {};
    const name = (typeof record.name === 'string' ? record.name : nameHint).trim();
    if (!name) return;

    const transportValue = record.transport ?? (typeof record.url === 'string' ? 'http' : 'stdio');
    const transport = transportValue === 'http' ? 'http' : 'stdio';
    const enabled = typeof record.enabled === 'boolean' ? record.enabled : true;
    const toolCount = typeof record.toolCount === 'number' && Number.isFinite(record.toolCount)
      ? Math.max(0, Math.round(record.toolCount))
      : 0;
    const command = typeof record.command === 'string' && record.command.trim() ? record.command.trim() : undefined;
    const url = typeof record.url === 'string' && record.url.trim() ? record.url.trim() : undefined;
    entries.push({
      name,
      transport,
      command: transport === 'stdio' ? command : undefined,
      url: transport === 'http' ? url : undefined,
      enabled,
      toolCount,
    });
  };

  if (Array.isArray(config)) {
    config.forEach((value) => appendEntry(isRecord(value) && typeof value.name === 'string' ? value.name : '', value));
  } else if (isRecord(config)) {
    const tools = isRecord(config.tools) ? config.tools : null;
    const serverMap = isRecord(config.mcpServers)
      ? config.mcpServers
      : tools && isRecord(tools.mcpServers)
        ? tools.mcpServers
        : null;
    if (serverMap) {
      Object.entries(serverMap).forEach(([name, value]) => appendEntry(name, value));
    } else if ('name' in config || 'transport' in config || 'url' in config || 'command' in config) {
      appendEntry('', config);
    } else {
      Object.entries(config).forEach(([name, value]) => appendEntry(name, value));
    }
  }

  if (!entries.length) throw new Error('JSON 中没有可导入的 MCP 服务');

  const servers = readDemoValue(STORAGE_KEYS.MCP, DEFAULT_MCP_SERVERS);
  const merged = [...servers];
  entries.forEach((entry) => {
    const index = merged.findIndex((server) => server.name.toLowerCase() === entry.name.toLowerCase());
    const next = {
      id: index >= 0 ? merged[index].id : `mcp-${generateId()}`,
      ...entry,
    };
    if (index >= 0) merged[index] = next;
    else merged.push(next);
  });

  return writeDemoValue(STORAGE_KEYS.MCP, merged);
}

export async function exportDemoMcpConfig(): Promise<{ mcpServers: Record<string, Record<string, unknown>> }> {
  await delay(80);
  const servers = readDemoValue(STORAGE_KEYS.MCP, DEFAULT_MCP_SERVERS);
  return {
    mcpServers: Object.fromEntries(
      servers.map((server) => [server.name, {
        transport: server.transport,
        ...(server.transport === 'stdio' && server.command ? { command: server.command } : {}),
        ...(server.transport === 'http' && server.url ? { url: server.url } : {}),
        enabled: server.enabled,
        toolCount: server.toolCount,
      }]),
    ),
  };
}

export async function getDemoComposerState(): Promise<DemoComposerState> {
  const cached = localStorage.getItem(STORAGE_KEYS.COMPOSER);
  if (!cached) return DEFAULT_COMPOSER;
  try { return JSON.parse(cached) as DemoComposerState; }
  catch { localStorage.removeItem(STORAGE_KEYS.COMPOSER); return DEFAULT_COMPOSER; }
}

export async function updateDemoComposerState(update: Partial<DemoComposerState>): Promise<DemoComposerState> {
  const current = readDemoValue(STORAGE_KEYS.COMPOSER, DEFAULT_COMPOSER);
  return writeDemoValue(STORAGE_KEYS.COMPOSER, { ...current, ...update });
}
