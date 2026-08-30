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
  SkillWriteOutcome,
  SkillRequest,
  SkillHistoryEntry,
  SkillHistoryDocument,
  CreateSkillRequestPayload,
  AutoDreamRunRecord,
  AutoDreamRunEvent,
  FileAttachmentDto,
  RuntimeConfig,
  ToolsConfigShape,
  GatewayProcessStatus,
  TokenStatsSnapshot,
  ChecklistItem,
  NotebookReport,
  SessionSearchResponse,
  PersonaKind,
  PersonaDocument,
  PersonaChangeRequest,
  PersonaHistoryEntry,
  PersonaHistoryRevision,
  ReportPeriod,
  PlanSidebarData,
  DemoDashboardSnapshot,
  DemoTokenPeriod,
  DemoTokenUsageSnapshot,
  DemoMemoryItem,
  DemoComposerState,
  DemoGenParams,
} from './types';
import { t } from '@/i18n';

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
  SKILL_DOCS: 'vivy.demo.skill-docs',
  AUTODREAM: 'vivy.demo.autodream',
  APPROVALS: 'vivy.demo.approvals',
  DASHBOARD: 'vivy.demo.dashboard',
  MEMORY: 'vivy.demo.memory',
  COMPOSER: 'vivy.demo.composer',
  /** 按模型键名（provider/baseUrl/model）独立保存的演示生成参数 */
  GEN_PARAMS: 'vivy.demo.gen-params',
};

// ==================== 模拟数据生成器 ====================

const MOCK_SESSIONS: Session[] = [
  {
    id: 'session-1',
    title: t('demo.sessions.welcomeTitle'),
    created_at: '2024-01-15T10:00:00Z',
    updated_at: '2024-01-15T10:30:00Z',
    message_count: 5,
    last_message_preview: t('demo.sessions.welcomePreview'),
  },
  {
    id: 'session-2',
    title: t('demo.sessions.planningTitle'),
    created_at: '2024-01-14T14:00:00Z',
    updated_at: '2024-01-14T15:30:00Z',
    message_count: 12,
    last_message_preview: t('demo.sessions.planningPreview'),
  },
  {
    id: 'session-3',
    title: t('demo.sessions.reviewTitle'),
    created_at: '2024-01-13T09:00:00Z',
    updated_at: '2024-01-13T09:45:00Z',
    message_count: 8,
    last_message_preview: t('demo.sessions.reviewPreview'),
  },
];

const MOCK_MESSAGES: Record<string, Message[]> = {
  'session-1': [
    {
      id: 'msg-1',
      role: 'user',
      content: t('demo.sessions.msgHello'),
      timestamp: Date.now() - 300000,
    },
    {
      id: 'msg-2',
      role: 'agent',
      content: t('demo.sessions.msgWelcomeReply'),
      timestamp: Date.now() - 299000,
    },
    {
      id: 'msg-3',
      role: 'user',
      content: t('demo.sessions.msgProjectRequest'),
      timestamp: Date.now() - 200000,
    },
    {
      id: 'msg-4',
      role: 'agent',
      content: t('demo.sessions.msgProjectReply'),
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
    description: t('demo.skills.reactDescription'),
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
    name: t('demo.skills.meooName'),
    description: t('demo.skills.meooDescription'),
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
    description: t('demo.skills.webDescription'),
    source: 'builtin',
    enabled: true,
    always: false,
    available: true,
    active: false,
    content_hash: 'ghi789',
    updated_at: '2024-01-15T10:00:00Z',
    can_hard_delete: false,
    path: '/skills/web-design-guidelines',
    can_delete: false,
  },
  {
    slug: 'vivy-code-review',
    name: t('demo.evolution.skillReviewName'),
    description: t('demo.evolution.skillReviewDescription'),
    source: 'home',
    enabled: true,
    always: false,
    available: true,
    active: true,
    content_hash: 'evo-hash-review-2',
    updated_at: '2026-08-24T09:30:00Z',
    can_hard_delete: true,
    evolution_managed: true,
    path: '/skills/home/vivy-code-review',
    can_delete: true,
  },
  {
    slug: 'vivy-doc-sync',
    name: t('demo.evolution.skillSyncName'),
    description: t('demo.evolution.skillSyncDescription'),
    source: 'home',
    enabled: true,
    always: false,
    available: true,
    active: true,
    content_hash: 'evo-hash-sync-1',
    updated_at: '2026-08-23T16:00:00Z',
    can_hard_delete: true,
    evolution_managed: true,
    path: '/skills/home/vivy-doc-sync',
    can_delete: true,
  },
];

// ==================== 进化治理（Skill 权威 / 待审请求 / AutoDream） ====================

/** 历史快照项：SkillHistoryDocument 全文 + SkillHistoryEntry 时间信息（演示层私有） */
export interface DemoSkillHistoryItem extends SkillHistoryDocument, SkillHistoryEntry {}

/** 每个 Skill 的权威文档与历史快照（演进写入时保留旧版本） */
export interface DemoSkillDocRecord {
  markdown: string;
  history: DemoSkillHistoryItem[];
}

const MOCK_SKILL_DOCS: Record<string, DemoSkillDocRecord> = {
  'vivy-code-review': {
    markdown: t('demo.evolution.skillReviewMarkdown'),
    history: [
      {
        slug: 'vivy-code-review',
        revision: 1,
        content_hash: 'evo-hash-review-1',
        updated_at: '2026-08-22T11:00:00Z',
        markdown: t('demo.evolution.skillReviewMarkdownV1'),
      },
    ],
  },
  'vivy-doc-sync': {
    markdown: t('demo.evolution.skillSyncMarkdown'),
    history: [],
  },
};

const MOCK_SKILL_REQUESTS: SkillRequest[] = [
  {
    id: 'req-demo-pending',
    slug: 'vivy-doc-sync',
    title: t('demo.evolution.pendingRequestTitle'),
    proposed_markdown: t('demo.evolution.pendingRequestMarkdown'),
    evidence: [{ autodream_run_id: 'run-demo-1', tool: 'autodream', artifact: null }],
    attestation: null,
    base_hash: 'evo-hash-sync-1',
    source: 'autodream',
    reason: t('demo.evolution.pendingRequestReason'),
    status: 'pending',
    created_at: '2026-08-24T10:06:00Z',
    updated_at: '2026-08-24T10:06:00Z',
  },
  {
    id: 'req-demo-accepted',
    slug: 'vivy-code-review',
    title: t('demo.evolution.acceptedRequestTitle'),
    proposed_markdown: t('demo.evolution.skillReviewMarkdown'),
    evidence: [],
    attestation: t('demo.evolution.acceptedRequestAttestation'),
    base_hash: 'evo-hash-review-1',
    source: 'user_request',
    reason: t('demo.evolution.acceptedRequestReason'),
    status: 'accepted',
    created_at: '2026-08-21T14:00:00Z',
    updated_at: '2026-08-22T09:00:00Z',
  },
  {
    id: 'req-demo-stale',
    slug: 'vivy-code-review',
    title: t('demo.evolution.staleRequestTitle'),
    proposed_markdown: t('demo.evolution.skillReviewMarkdownV1'),
    evidence: [{ autodream_run_id: 'run-demo-2', tool: 'autodream', artifact: null }],
    attestation: null,
    base_hash: 'evo-hash-review-0',
    source: 'autodream',
    reason: t('demo.evolution.staleRequestReason'),
    status: 'stale',
    created_at: '2026-08-21T09:00:00Z',
    updated_at: '2026-08-22T09:00:00Z',
  },
];

interface DemoAutoDreamStore {
  runs: AutoDreamRunRecord[];
  events: Record<string, AutoDreamRunEvent[]>;
}

function mockRunEvent(runId: string, kind: string, messageKey: string, createdAt: string): AutoDreamRunEvent {
  return { id: `${runId}-ev-${kind}`, run_id: runId, kind, message: t(messageKey), created_at: createdAt };
}

const MOCK_AUTODREAM: DemoAutoDreamStore = {
  runs: [
    {
      id: 'run-demo-1',
      started_at: '2026-08-24T10:00:00Z',
      completed_at: '2026-08-24T10:06:00Z',
      state: 'completed',
      trigger: 'manual',
      summary: t('demo.evolution.run1Summary'),
      input_summary: {
        total_items: 24,
        included_sources: [
          { source: 'sessions', included_items: 18, total_bytes: 48210, truncated: false },
          { source: 'actmem', included_items: 6, total_bytes: 12980, truncated: false },
        ],
        total_bytes: 61190,
        truncated: false,
      },
      proposal_ids: ['req-demo-pending'],
      orchestration: {
        schema_version: 1,
        phase: 'completed',
        attempt: 1,
        deadline_at: '2026-08-24T10:20:00Z',
        updated_at: '2026-08-24T10:06:00Z',
      },
      failure_code: null,
      error: null,
    },
    {
      id: 'run-demo-2',
      started_at: '2026-08-21T09:00:00Z',
      completed_at: '2026-08-21T09:04:00Z',
      state: 'completed',
      trigger: 'session_threshold',
      summary: t('demo.evolution.run2Summary'),
      input_summary: {
        total_items: 9,
        included_sources: [{ source: 'sessions', included_items: 9, total_bytes: 17340, truncated: false }],
        total_bytes: 17340,
        truncated: false,
      },
      proposal_ids: [],
      orchestration: {
        schema_version: 1,
        phase: 'completed',
        attempt: 1,
        deadline_at: '2026-08-21T09:15:00Z',
        updated_at: '2026-08-21T09:04:00Z',
      },
      failure_code: null,
      error: null,
    },
    {
      id: 'run-demo-3',
      started_at: '2026-08-18T15:00:00Z',
      completed_at: '2026-08-18T15:02:00Z',
      state: 'failed',
      trigger: 'manual',
      summary: t('demo.evolution.run3Summary'),
      input_summary: null,
      proposal_ids: [],
      orchestration: {
        schema_version: 1,
        phase: 'failed',
        attempt: 1,
        deadline_at: '2026-08-18T15:15:00Z',
        updated_at: '2026-08-18T15:02:00Z',
      },
      failure_code: 'worker_timeout',
      error: t('demo.evolution.run3Error'),
    },
  ],
  events: {
    'run-demo-1': [
      mockRunEvent('run-demo-1', 'run_started', 'demo.evolution.eventStarted', '2026-08-24T10:00:00Z'),
      mockRunEvent('run-demo-1', 'inputs_gathered', 'demo.evolution.eventGathered', '2026-08-24T10:01:00Z'),
      mockRunEvent('run-demo-1', 'reflecting_started', 'demo.evolution.eventReflecting', '2026-08-24T10:02:00Z'),
      mockRunEvent('run-demo-1', 'candidate_validated', 'demo.evolution.eventValidated', '2026-08-24T10:05:00Z'),
      mockRunEvent('run-demo-1', 'proposal_published', 'demo.evolution.eventPublished', '2026-08-24T10:06:00Z'),
    ],
    'run-demo-2': [
      mockRunEvent('run-demo-2', 'run_started', 'demo.evolution.eventStarted', '2026-08-21T09:00:00Z'),
      mockRunEvent('run-demo-2', 'inputs_gathered', 'demo.evolution.eventGathered', '2026-08-21T09:01:00Z'),
      mockRunEvent('run-demo-2', 'run_completed', 'demo.evolution.eventCompleted', '2026-08-21T09:04:00Z'),
    ],
    'run-demo-3': [
      mockRunEvent('run-demo-3', 'run_started', 'demo.evolution.eventStarted', '2026-08-18T15:00:00Z'),
      mockRunEvent('run-demo-3', 'run_failed', 'demo.evolution.eventFailed', '2026-08-18T15:02:00Z'),
    ],
  },
};

const demoHash = () => `evo-hash-${generateId()}`;

/** 播种时必须返回副本：治理操作会原地修改返回的记录，不能污染模块级种子 */
function seedStore<T>(key: string, seed: T): T {
  const copy = JSON.parse(JSON.stringify(seed)) as T;
  localStorage.setItem(key, JSON.stringify(copy));
  return copy;
}

function readSkillStore(): SkillDto[] {
  const cached = localStorage.getItem(STORAGE_KEYS.SKILLS);
  if (cached) {
    return JSON.parse(cached) as SkillDto[];
  }
  return seedStore(STORAGE_KEYS.SKILLS, MOCK_SKILLS);
}

function writeSkillStore(skills: SkillDto[]) {
  localStorage.setItem(STORAGE_KEYS.SKILLS, JSON.stringify(skills));
}

function readSkillDocStore(): Record<string, DemoSkillDocRecord> {
  const cached = localStorage.getItem(STORAGE_KEYS.SKILL_DOCS);
  if (cached) {
    return JSON.parse(cached) as Record<string, DemoSkillDocRecord>;
  }
  const seeded: Record<string, DemoSkillDocRecord> = {};
  for (const skill of MOCK_SKILLS) {
    seeded[skill.slug] =
      MOCK_SKILL_DOCS[skill.slug] ?? {
        markdown: t('demo.skills.docTemplate', { name: skill.name, description: skill.description }),
        history: [],
      };
  }
  return seedStore(STORAGE_KEYS.SKILL_DOCS, seeded);
}

function writeSkillDocStore(docs: Record<string, DemoSkillDocRecord>) {
  localStorage.setItem(STORAGE_KEYS.SKILL_DOCS, JSON.stringify(docs));
}

function readSkillRequests(): SkillRequest[] {
  const cached = localStorage.getItem(STORAGE_KEYS.SKILL_REQUESTS);
  if (cached) {
    return JSON.parse(cached) as SkillRequest[];
  }
  return seedStore(STORAGE_KEYS.SKILL_REQUESTS, MOCK_SKILL_REQUESTS);
}

function writeSkillRequests(requests: SkillRequest[]) {
  localStorage.setItem(STORAGE_KEYS.SKILL_REQUESTS, JSON.stringify(requests));
}

function readAutoDreamStore(): DemoAutoDreamStore {
  const cached = localStorage.getItem(STORAGE_KEYS.AUTODREAM);
  if (cached) {
    return JSON.parse(cached) as DemoAutoDreamStore;
  }
  return seedStore(STORAGE_KEYS.AUTODREAM, MOCK_AUTODREAM);
}

/** 将提案写入 Skill 权威头并保留历史快照；返回新的 content_hash */
function applyProposalToSkill(slug: string, markdown: string): string {
  const skills = readSkillStore();
  const docs = readSkillDocStore();
  const skill = skills.find((s) => s.slug === slug);
  const record = docs[slug] ?? { markdown: '', history: [] };
  const newHash = demoHash();
  if (skill) {
    record.history.push({
      slug,
      revision: record.history.length + 1,
      content_hash: skill.content_hash,
      updated_at: now(),
      markdown: record.markdown,
    });
    skill.content_hash = newHash;
    skill.updated_at = now();
    skill.evolution_managed = true;
    skill.enabled = true;
  } else {
    skills.push({
      slug,
      name: slug,
      description: '',
      source: 'home',
      enabled: true,
      always: false,
      available: true,
      active: true,
      content_hash: newHash,
      updated_at: now(),
      can_hard_delete: true,
      evolution_managed: true,
      path: `/skills/home/${slug}`,
      can_delete: true,
    });
  }
  record.markdown = markdown;
  docs[slug] = record;
  writeSkillStore(skills);
  writeSkillDocStore(docs);
  return newHash;
}

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
      title: t('demo.approval.title'),
      description: t('demo.approval.description'),
    },
  },
];

const MOCK_PLAN: PlanRuntimeState = {
  plan_id: 'plan-1',
  revision: 1,
  title: t('demo.plan.title'),
  goal: t('demo.plan.goal'),
  phase: 'planning',
  status: 'draft',
  strategy: t('demo.plan.strategy'),
  summary: t('demo.plan.summary'),
  markdown: t('demo.plan.markdown'),
  validation_issues: [],
  steps: [
    {
      id: 'step-1',
      ordinal: 1,
      title: t('demo.plan.step1Title'),
      rationale: t('demo.plan.step1Rationale'),
      expected_output: t('demo.plan.step1Output'),
      status: 'completed',
    },
    {
      id: 'step-2',
      ordinal: 2,
      title: t('demo.plan.step2Title'),
      rationale: t('demo.plan.step2Rationale'),
      expected_output: t('demo.plan.step2Output'),
      status: 'in_progress',
    },
  ],
  todos: [
    {
      id: 'todo-1',
      plan_step_id: 'step-1',
      title: t('demo.plan.todo1Title'),
      detail: t('demo.plan.todo1Detail'),
      status: 'completed',
      priority: 'high',
      evidence_ref: null,
      block_reason: null,
      updated_at: '2024-01-15T10:00:00Z',
    },
    {
      id: 'todo-2',
      plan_step_id: 'step-2',
      title: t('demo.plan.todo2Title'),
      detail: t('demo.plan.todo2Detail'),
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

// ==================== 按模型独立的演示生成参数 ====================

/** 新模型的演示生成参数默认值（与 MOCK_CONFIG 一致）。 */
const DEFAULT_DEMO_GEN_PARAMS: DemoGenParams = { temperature: 0.7, max_tokens: 4096 };

/** 逐条校验并归一化存储中的模型参数；非法条目回落到默认值。 */
function normalizeDemoGenParams(value: unknown): DemoGenParams {
  if (typeof value !== 'object' || value === null) return { ...DEFAULT_DEMO_GEN_PARAMS };
  const entry = value as Record<string, unknown>;
  const temperature = typeof entry.temperature === 'number' ? Math.min(2, Math.max(0, entry.temperature)) : DEFAULT_DEMO_GEN_PARAMS.temperature;
  const max_tokens = typeof entry.max_tokens === 'number' ? Math.max(1, Math.round(entry.max_tokens)) : DEFAULT_DEMO_GEN_PARAMS.max_tokens;
  return { temperature, max_tokens };
}

function readDemoGenParamsStore(): Record<string, DemoGenParams> {
  try {
    const raw = localStorage.getItem(STORAGE_KEYS.GEN_PARAMS);
    if (!raw) return {};
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) return {};
    return Object.fromEntries(
      Object.entries(parsed as Record<string, unknown>).map(([key, value]) => [key, normalizeDemoGenParams(value)]),
    );
  } catch {
    return {};
  }
}

/**
 * 读取某模型的演示生成参数；未保存过或数据损坏时返回默认值，不写 localStorage
 * （编辑并保存参数才会落 vivy.demo.gen-params）。
 */
export function getDemoGenParams(modelKey: string): DemoGenParams {
  return readDemoGenParamsStore()[modelKey] ?? { ...DEFAULT_DEMO_GEN_PARAMS };
}

/**
 * 保存某模型的演示生成参数（vivy.demo.gen-params，键为模型运行三元组
 * `provider/baseUrl/model`，各模型独立存储互不覆盖；绝不传给真实 Provider）。
 * 返回归一化后的已保存值。
 */
export function saveDemoGenParams(modelKey: string, params: DemoGenParams): DemoGenParams {
  const normalized = normalizeDemoGenParams(params);
  const store = readDemoGenParamsStore();
  store[modelKey] = normalized;
  try {
    localStorage.setItem(STORAGE_KEYS.GEN_PARAMS, JSON.stringify(store));
  } catch {
    // Demo persistence is best effort.
  }
  return { ...normalized };
}

// ==================== API 函数实现 ====================

/**
 * 发送消息并获取 AI 回复
 */
export async function sendMessage(sessionId: string, content: string): Promise<AgentResponse> {
  await delay(500 + Math.random() * 1000);

  // 简单的模拟回复逻辑
  const responses: Record<string, string> = {
    '你好': t('demo.sessions.replyHello'),
    '帮我创建一个项目': t('demo.sessions.replyProject'),
    '默认': t('demo.sessions.replyDefault'),
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
    reasoning: t('demo.sessions.reasoning'),
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
    title: title || t('errors.newSessionDefault'),
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
  return readSkillStore();
}

/**
 * 获取技能文档（读取本地权威存储，与进化治理写入保持一致）
 */
export async function getSkillDocument(slug: string): Promise<SkillDocument | null> {
  await delay(200);
  const skill = readSkillStore().find((s) => s.slug === slug);
  if (!skill) return null;
  const record = readSkillDocStore()[slug];
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
    markdown: record?.markdown ?? '',
  };
}

/**
 * 编辑保存 Skill 文档（base_hash 冲突时拒绝，保留旧内容为历史快照）
 */
export async function updateSkillDocument(slug: string, markdown: string, baseHash: string): Promise<SkillWriteOutcome> {
  await delay(300);
  const skills = readSkillStore();
  const skill = skills.find((s) => s.slug === slug);
  if (!skill) throw new Error(t('evolution.errors.skillNotFound'));
  if (skill.content_hash !== baseHash) throw new Error(t('evolution.errors.hashConflict'));
  const docs = readSkillDocStore();
  const record = docs[slug] ?? { markdown: '', history: [] };
  const changed = record.markdown !== markdown;
  if (changed) {
    record.history.push({
      slug,
      revision: record.history.length + 1,
      content_hash: skill.content_hash,
      updated_at: now(),
      markdown: record.markdown,
    });
    record.markdown = markdown;
    docs[slug] = record;
    skill.content_hash = demoHash();
    skill.updated_at = now();
    writeSkillStore(skills);
    writeSkillDocStore(docs);
  }
  const document = await getSkillDocument(slug);
  if (!document) throw new Error(t('evolution.errors.skillNotFound'));
  return { document, changed };
}

/**
 * 启用 / 停用 Skill
 */
export async function setSkillEnabled(slug: string, enabled: boolean): Promise<SkillDto> {
  await delay(200);
  const skills = readSkillStore();
  const skill = skills.find((s) => s.slug === slug);
  if (!skill) throw new Error(t('evolution.errors.skillNotFound'));
  skill.enabled = enabled;
  skill.updated_at = now();
  writeSkillStore(skills);
  return skill;
}

/**
 * 硬删除 home Skill（仅 can_hard_delete，删除权威头与历史）
 */
export async function deleteSkill(slug: string): Promise<void> {
  await delay(200);
  const skills = readSkillStore();
  const skill = skills.find((s) => s.slug === slug);
  if (!skill) throw new Error(t('evolution.errors.skillNotFound'));
  if (!skill.can_hard_delete) throw new Error(t('evolution.errors.cannotDelete'));
  writeSkillStore(skills.filter((s) => s.slug !== slug));
  const docs = readSkillDocStore();
  delete docs[slug];
  writeSkillDocStore(docs);
}

/**
 * 获取 Skill 历史快照列表
 */
export async function getSkillHistory(slug: string): Promise<SkillHistoryEntry[]> {
  await delay(150);
  const record = readSkillDocStore()[slug];
  if (!record) return [];
  return record.history.map(({ revision, content_hash, updated_at }) => ({ revision, content_hash, updated_at }));
}

/**
 * 获取指定历史快照全文
 */
export async function getSkillHistoryDocument(slug: string, revision: number): Promise<SkillHistoryDocument | null> {
  await delay(150);
  const record = readSkillDocStore()[slug];
  return record?.history.find((entry) => entry.revision === revision) ?? null;
}

/**
 * 创建技能请求
 */
export async function createSkillRequest(payload: CreateSkillRequestPayload): Promise<SkillRequest> {
  await delay(300);
  const requests = readSkillRequests();
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
  writeSkillRequests(requests);
  return newRequest;
}

/**
 * 获取技能请求列表
 */
export async function getSkillRequests(): Promise<SkillRequest[]> {
  await delay(200);
  return readSkillRequests();
}

/**
 * 接受技能请求：base_hash 与当前权威头不一致时置为 stale 并拒绝
 */
export async function acceptSkillRequest(id: string): Promise<SkillRequest> {
  await delay(300);
  const requests = readSkillRequests();
  const request = requests.find((r) => r.id === id);
  if (!request) throw new Error(t('evolution.errors.requestNotFound'));
  if (request.status !== 'pending') throw new Error(t('evolution.errors.requestNotPending'));
  const skill = readSkillStore().find((s) => s.slug === request.slug);
  if (skill && skill.content_hash !== request.base_hash) {
    request.status = 'stale';
    request.updated_at = now();
    writeSkillRequests(requests);
    throw new Error(t('evolution.errors.requestStale'));
  }
  applyProposalToSkill(request.slug, request.proposed_markdown);
  request.status = 'accepted';
  request.updated_at = now();
  writeSkillRequests(requests);
  return request;
}

/**
 * 拒绝技能请求（仅 pending 可拒绝）
 */
export async function rejectSkillRequest(id: string): Promise<SkillRequest> {
  await delay(300);
  const requests = readSkillRequests();
  const request = requests.find((r) => r.id === id);
  if (!request) throw new Error(t('evolution.errors.requestNotFound'));
  if (request.status !== 'pending') throw new Error(t('evolution.errors.requestNotPending'));
  request.status = 'rejected';
  request.updated_at = now();
  writeSkillRequests(requests);
  return request;
}

/**
 * 获取 AutoDream 运行列表
 */
export async function listAutoDreamRuns(): Promise<AutoDreamRunRecord[]> {
  await delay(200);
  return readAutoDreamStore().runs;
}

/**
 * 获取 AutoDream 运行的进度事件
 */
export async function getAutoDreamRunEvents(runId: string): Promise<AutoDreamRunEvent[]> {
  await delay(150);
  return readAutoDreamStore().events[runId] ?? [];
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
    details: t('demo.gateway.notRunning'),
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
    title: t('demo.notebook.dailyTitle'),
    summary: t('demo.notebook.dailySummary'),
    content: t('demo.notebook.dailyContent'),
    generatedAt: '2024-01-15T23:00:00Z',
    generatedBy: 'vivy-demo',
    generationMode: 'llm_curated',
  },
  {
    id: 'report-2',
    period: 'weekly',
    date: '2024-01-14',
    title: t('demo.notebook.weeklyTitle'),
    summary: t('demo.notebook.weeklySummary'),
    content: t('demo.notebook.weeklyContent'),
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
    content: t('demo.personaDocs.identity'),
    revision: 1,
    updated_at: '2024-01-15T10:00:00Z',
  },
  relationship: {
    kind: 'relationship',
    content: t('demo.personaDocs.relationship'),
    revision: 1,
    updated_at: '2024-01-15T10:00:00Z',
  },
  redline: {
    kind: 'redline',
    content: t('demo.personaDocs.redline'),
    revision: 1,
    updated_at: '2024-01-15T10:00:00Z',
  },
  user: {
    kind: 'user',
    content: t('demo.personaDocs.user'),
    revision: 1,
    updated_at: '2024-01-15T10:00:00Z',
  },
  world: {
    kind: 'world',
    content: t('demo.personaDocs.world'),
    revision: 1,
    updated_at: '2024-01-15T10:00:00Z',
  },
  dream: {
    kind: 'dream',
    content: t('demo.personaDocs.dream'),
    revision: 1,
    updated_at: '2024-01-15T10:00:00Z',
  },
  dark: {
    kind: 'dark',
    content: t('demo.personaDocs.dark'),
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
    content: t('demo.personaDocs.historySnapshot', { revision, kind }),
    updated_at: '2024-01-15T10:00:00Z',
  };
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

  const periodLabel = t(`demo.notebook.periodLabel.${period}`);
  const title = `${periodLabel} - ${dateStr}`;

  // 汇总会话消息作为报告素材
  let totalMessages = 0;
  const topics: string[] = [];
  for (const session of sessions) {
    const messages = await getSessionMessages(session.id);
    totalMessages += messages.length;
    if (session.title) topics.push(session.title);
  }

  const content = t('demo.notebook.reportContent', {
    title,
    sessions: sessions.length,
    messages: totalMessages,
    topics: topics.length > 0 ? topics.join('、') : t('demo.notebook.noTopics'),
    firstTopic: topics[0] || t('demo.notebook.defaultTopic'),
  });

  const report: NotebookReport = {
    id: generateId(),
    period,
    date: dateStr,
    title,
    summary: t('demo.notebook.generatedSummary', { sessions: sessions.length, messages: totalMessages }),
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
    { id: 'activity-1', title: t('demo.dashboard.activityReportTitle'), detail: t('demo.dashboard.activityReportDetail'), occurredAt: t('demo.dashboard.activityReportAt') },
    { id: 'activity-2', title: t('demo.dashboard.activitySkillTitle'), detail: t('demo.dashboard.activitySkillDetail'), occurredAt: t('demo.dashboard.activitySkillAt') },
    { id: 'activity-3', title: t('demo.dashboard.activityCronTitle'), detail: t('demo.dashboard.activityCronDetail'), occurredAt: t('demo.dashboard.activityCronAt') },
  ],
};

const DEFAULT_MEMORIES: DemoMemoryItem[] = [
  { id: 'memory-1', title: t('demo.memories.preferenceTitle'), category: 'preference', content: t('demo.memories.preferenceContent'), updatedAt: '2024-01-15T10:00:00Z' },
  { id: 'memory-2', title: t('demo.memories.projectTitle'), category: 'project', content: t('demo.memories.projectContent'), updatedAt: '2024-01-14T16:30:00Z' },
  { id: 'memory-3', title: t('demo.memories.decisionTitle'), category: 'decision', content: t('demo.memories.decisionContent'), updatedAt: '2024-01-13T09:20:00Z' },
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
  { id: 'session-1', title: t('demo.sessions.tokenOverview'), model: 'deepseek-chat', request_count: 18, total_input: 9200, total_output: 4100, total_cost: 0.082 },
  { id: 'session-2', title: t('demo.sessions.tokenDesign'), model: 'claude-sonnet-4', request_count: 11, total_input: 6400, total_output: 2800, total_cost: 0.146 },
  { id: 'session-3', title: t('demo.sessions.tokenPlugin'), model: 'gpt-4.1-mini', request_count: 9, total_input: 5100, total_output: 1900, total_cost: 0.037 },
  { id: 'session-4', title: t('demo.sessions.tokenPersona'), model: 'deepseek-chat', request_count: 7, total_input: 3600, total_output: 1500, total_cost: 0.028 },
  { id: 'session-5', title: t('demo.sessions.tokenCron'), model: 'gpt-4.1-mini', request_count: 4, total_input: 1800, total_output: 700, total_cost: 0.012 },
];

const BASE_TOKEN_ENDPOINTS = [
  { key: t('demo.tokens.endpoints.chatCompletion'), total_tokens: 24100, total_cost: 0.198, request_count: 32 },
  { key: t('demo.tokens.endpoints.toolCalls'), total_tokens: 9800, total_cost: 0.072, request_count: 12 },
  { key: t('demo.tokens.endpoints.compaction'), total_tokens: 4200, total_cost: 0.035, request_count: 5 },
];

const TOKEN_TIMELINE_LABELS: Record<DemoTokenPeriod, string[]> = {
  '1d': ['00:00', '02:00', '04:00', '06:00', '08:00', '10:00', '12:00', '14:00', '16:00', '18:00', '20:00', '22:00'],
  '3d': ['8/23', '8/24', '8/25'],
  '1w': ['8/19', '8/20', '8/21', '8/22', '8/23', '8/24', '8/25'],
  '1m': Array.from({ length: 4 }, (_, index) => t('demo.tokens.timeline.weekN', { count: index + 1 })),
  '6m': Array.from({ length: 6 }, (_, index) => t(`demo.tokens.timeline.months.${2 + index}`)),
  '1y': Array.from({ length: 12 }, (_, index) => t(`demo.tokens.timeline.months.${index}`)),
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
