/**
 * 轨迹面板演示数据（本地静态示例）。
 * 仅为「中控台 → 轨迹」页面提供假数据，不代表 Vivy 服务端状态，
 * 不写入 localStorage、不经过 RPC；所有时间戳为固定基线，保证确定性。
 *
 * 数据结构与字段语义参考 DeepSeek Harness `packages/client/ui-trajectory`
 * 的 `trajectory-record.ts` / `layout.ts` 子集。
 */

/** 轨迹记录类型的闭合集合（与 DSH TrajectoryCellKind 一致）。 */
export type TrajectoryCellKind =
  | 'system'
  | 'user'
  | 'context'
  | 'compacted'
  | 'message'
  | 'tool'
  | 'subtool';

/** 类型徽标文案（与 DSH KIND_LABEL 一致，不做本地化）。 */
export const TRAJECTORY_KIND_LABEL: Record<TrajectoryCellKind, string> = {
  system: 'SYSTEM',
  user: 'USER',
  context: 'CONTEXT',
  compacted: 'COMPACTED',
  message: 'ASSISTANT',
  tool: 'TOOL',
  subtool: 'SUBTOOL',
};

/** Token 用量明细。 */
export interface TrajectoryTokens {
  input?: number;
  output?: number;
  think?: number;
  cacheRead?: number;
  cacheWrite?: number;
}

/** 一条轨迹记录（账本行 + 时间轴条带 + 详情面板的共同数据源）。 */
export interface TrajectoryRecord {
  /** 全局唯一记录索引（1 起）。 */
  index: number;
  /** 稳定身份，供折叠与选中状态引用。 */
  id: string;
  /** 所属回合：null 表示会话起始区段。 */
  turn: number | null;
  /** 步骤分组名，如 `Step 1`；会话区段为 `Session`。 */
  group: string;
  kind: TrajectoryCellKind;
  /** 账本行主文案。 */
  text: string;
  /** 自身耗时（秒），未知为 null。 */
  timeSeconds: number | null;
  /** 操作实际开始的 Unix 毫秒（固定基线推算），未知为 null。 */
  startedAt: number | null;
  tokens?: TrajectoryTokens;
  /** 工具/助理行的内联结果摘要；失败时结合 isError 渲染为红色。 */
  result?: string;
  isError?: boolean;
  /** 助理行：首 Token 耗时（毫秒），用于时间轴 TTFT 分段。 */
  ttftMs?: number;
  /** 详情面板：请求入参 / 完整输入。 */
  inputDetail?: string;
  /** 详情面板：完整输出 / 工具结果 / 系统提示正文。 */
  outputDetail?: string;
  /** 详情面板：完整思考内容。 */
  thinkingDetail?: string;
  callId?: string;
  provider?: string;
  model?: string;
  /** 该行是否开启一个新的模型回合。 */
  opensTurn?: boolean;
}

/** 请求级信息（详情面板 Summary/Usage/Timing 的来源）。 */
export interface TrajectoryRequest {
  number: number;
  turn: number | null;
  group: string;
  purpose?: 'compaction';
  status: 'complete' | 'error';
  startedAt: number;
  completedAt: number;
  provider: string;
  model: string;
  requestConfig?: { provider: string; model: string };
  usage: Required<TrajectoryTokens>;
  error?: string;
  retry?: number;
  maxRetries?: number;
  retryDelayMs?: number;
}

/** 时间轴投影模式：等宽序列 / 按真实时长（压缩空闲）。 */
export type TrajectoryTimelineMode = 'sequence' | 'duration';

/** 时间轴选区（闭区间，作用于当前投影的域）。 */
export interface TrajectoryTimeRange {
  start: number;
  end: number;
}

/** 固定演示基线：2026-08-24 10:00:00 UTC（与中控台快照同一天）。 */
const BASE = Date.UTC(2026, 7, 24, 10, 0, 0);

/** 记账行 constructor：字段顺序即时间顺序，仅一处维护索引/基线。 */
interface DemoRow {
  turn: number | null;
  group: string;
  kind: TrajectoryCellKind;
  text: string;
  timeSeconds?: number | null;
  /** 相对 BASE 的毫秒偏移。 */
  offsetMs?: number;
  tokens?: TrajectoryTokens;
  result?: string;
  isError?: boolean;
  ttftMs?: number;
  inputDetail?: string;
  outputDetail?: string;
  thinkingDetail?: string;
  callId?: string;
  provider?: string;
  model?: string;
}

/** 行间推进的时序状态（固定推导，确保 duration 模式条带不重叠）。 */
interface RowClock {
  at: number;
}

function row(clock: RowClock, entry: DemoRow): TrajectoryRecord[] {
  const startedAt = entry.offsetMs === undefined ? null : BASE + clock.at + entry.offsetMs;
  const endedAt = entry.offsetMs === undefined
    ? null
    : BASE + clock.at + entry.offsetMs + (entry.timeSeconds ?? 0) * 1000;
  const record: TrajectoryRecord = {
    index: 0,
    id: '',
    turn: entry.turn,
    group: entry.group,
    kind: entry.kind,
    text: entry.text,
    timeSeconds: entry.timeSeconds ?? null,
    startedAt,
    tokens: entry.tokens,
    result: entry.result,
    isError: entry.isError,
    ttftMs: entry.ttftMs,
    inputDetail: entry.inputDetail,
    outputDetail: entry.outputDetail,
    thinkingDetail: entry.thinkingDetail,
    callId: entry.callId,
    provider: entry.provider,
    model: entry.model,
    opensTurn: entry.kind === 'user',
  };
  if (startedAt !== null) clock.at = endedAt! + 40; // 40ms 微小间隙，供空闲压缩演示
  return [record];
}

function cells(clock: RowClock, entries: DemoRow[]): TrajectoryRecord[] {
  return entries.flatMap((entry) => row(clock, entry));
}

const RAW_ROWS: DemoRow[] = [
  // ── 会话起始区段（turn null） ──
  {
    turn: null,
    group: 'Session',
    kind: 'system',
    text: '系统提示装载 · Vivy 化身与工具目录就绪',
    tokens: { input: 3214 },
    inputDetail: '「你是 Vivy，运行在本机的 AI 代理…」（截断示例）',
    outputDetail: '系统提示全文（演示样例，共 3214 tokens）：\n你是 Vivy……\n可用工具：bash、read、write、grep、web_search、notebook_report。',
  },
  {
    turn: null,
    group: 'Session',
    kind: 'context',
    text: '注入工作区上下文 · docs/ 目录清单（24 项，48 KB）',
    tokens: { input: 1820 },
    inputDetail: 'workspace://docs 下的 24 个文件路径与大小清单（演示样例）。',
  },
  // ── Turn #1 ──
  {
    turn: 1,
    group: 'User',
    kind: 'user',
    text: '帮我修复 README 中过时的启动命令，并验证文档里的示例脚本仍然可用。',
    inputDetail: '用户消息全文：帮我修复 README 中过时的启动命令……',
  },
  {
    turn: 1,
    group: 'Step 1',
    kind: 'message',
    text: '我先读取 README 与相关脚本，确认哪些命令已过时。',
    offsetMs: 0,
    timeSeconds: 2.76,
    ttftMs: 812,
    tokens: { input: 4523, output: 412, think: 156, cacheRead: 1890 },
    inputDetail: '请求入参（截断示例）：system + user 消息拼接……',
    outputDetail: '输出全文：好的，我先读取 README 与相关脚本，确认哪些命令已过时。',
    thinkingDetail: '用户要求修复过时命令；先做只读侦察：读取 README、解析命令引用、核对 package.json scripts。',
    provider: 'deepseek',
    model: 'deepseek-chat',
  },
  {
    turn: 1,
    group: 'Step 1',
    kind: 'tool',
    text: 'read · README.md',
    offsetMs: 0,
    timeSeconds: 0.31,
    callId: 'call-read-1',
    inputDetail: '{"path": "README.md", "range": "full"}',
    outputDetail: '21 段内容读取完成；命中 2 处 `pnpm run` 旧命令引用。',
    result: '21 段读取 · 命中 2 处旧命令',
  },
  {
    turn: 1,
    group: 'Step 2',
    kind: 'message',
    text: '已找到两处过时命令：`pnpm run dev:legacy` 与 `node scripts/serve.js`。',
    offsetMs: 0,
    timeSeconds: 1.98,
    ttftMs: 645,
    tokens: { input: 4980, output: 328, think: 96, cacheRead: 2210 },
    inputDetail: '请求入参（截断示例）……',
    outputDetail: '输出全文：已找到两处过时命令：`pnpm run dev:legacy` 与 `node scripts/serve.js`。',
    thinkingDetail: '两处引用分别对应 dev-server 与静态服务脚本；旧命令缺失 --host 与 --port 参数。',
    provider: 'deepseek',
    model: 'deepseek-chat',
  },
  {
    turn: 1,
    group: 'Step 2',
    kind: 'tool',
    text: 'write · README.md',
    offsetMs: 0,
    timeSeconds: 0.24,
    callId: 'call-write-1',
    inputDetail: '{"path": "README.md", "patch": "+1 −2 lines"}',
    outputDetail: 'README.md 已更新（+1 −2）。',
    result: 'README.md 已更新（+1 −2）',
  },
  {
    turn: 1,
    group: 'Step 2',
    kind: 'tool',
    text: 'bash · pnpm test:docs',
    offsetMs: 0,
    timeSeconds: 1.42,
    callId: 'call-bash-1',
    inputDetail: '{"command": "pnpm test:docs", "cwd": "workspace"}',
    outputDetail: 'exit 0 · 12 tests passed, 1.8s',
    result: 'exit 0 · 12 tests passed',
  },
  {
    turn: 1,
    group: 'Step 3',
    kind: 'message',
    text: '启动命令已更新为 `pnpm dev`，文档示例脚本全部通过验证。',
    offsetMs: 0,
    timeSeconds: 2.4,
    ttftMs: 734,
    tokens: { input: 5341, output: 289, think: 0, cacheRead: 2560 },
    inputDetail: '请求入参（截断示例）……',
    outputDetail: '输出全文：启动命令已更新为 `pnpm dev`，文档示例脚本全部通过验证。',
    thinkingDetail: '经验证脚本全部通过，可向用户交付结论。',
    provider: 'deepseek',
    model: 'deepseek-chat',
  },
  // ── Turn #2 ──
  {
    turn: 2,
    group: 'User',
    kind: 'user',
    text: '顺便把 CI 配置里的 Node 版本对齐到运行时要求。',
    inputDetail: '用户消息全文：顺便把 CI 配置里的 Node 版本对齐到运行时要求。',
  },
  {
    turn: 2,
    group: 'Step 1',
    kind: 'message',
    text: '我会检查 .github/workflows 与 package.json 的 engines 字段。',
    offsetMs: 0,
    timeSeconds: 2.1,
    ttftMs: 903,
    tokens: { input: 5760, output: 341, think: 122, cacheRead: 2890 },
    inputDetail: '请求入参（截断示例）……',
    outputDetail: '输出全文：我会检查 .github/workflows 与 package.json 的 engines 字段。',
    thinkingDetail: '目标是将 ci.yml 的 node-version 与 engines.node 对齐；先读取两处配置。',
    provider: 'deepseek',
    model: 'deepseek-chat',
  },
  {
    turn: 2,
    group: 'Step 1',
    kind: 'tool',
    text: 'read · .github/workflows/ci.yml',
    offsetMs: 0,
    timeSeconds: 0.26,
    callId: 'call-read-2',
    inputDetail: '{"path": ".github/workflows/ci.yml", "range": "full"}',
    outputDetail: 'workflow 读取完成 · node-version: 18',
    result: 'node-version: 18',
  },
  {
    turn: 2,
    group: 'Step 1',
    kind: 'tool',
    text: 'read · package.json',
    offsetMs: 0,
    timeSeconds: 0.2,
    callId: 'call-read-3',
    inputDetail: '{"path": "package.json", "range": "engines"}',
    outputDetail: 'engines.node: >=20.0.0',
    result: 'engines.node: >=20.0.0',
  },
  {
    turn: 2,
    group: 'Step 1',
    kind: 'subtool',
    text: 'grep · node-version',
    offsetMs: 0,
    timeSeconds: 0.18,
    callId: 'call-grep-1',
    inputDetail: '{"pattern": "node-version", "path": ".github/**"}',
    outputDetail: '2 处引用：ci.yml:18、deploy.yml:9',
    result: '2 处引用',
  },
  {
    turn: 2,
    group: 'Step 2',
    kind: 'tool',
    text: 'write · .github/workflows/ci.yml',
    offsetMs: 0,
    timeSeconds: 0.3,
    callId: 'call-write-2',
    inputDetail: '{"path": ".github/workflows/ci.yml", "patch": "+1 −1"}',
    outputDetail: 'ci.yml 已更新（+1 −1）。',
    result: '已更新（+1 −1）',
  },
  {
    turn: 2,
    group: 'Step 2',
    kind: 'message',
    text: 'CI 的 Node 版本已对齐到 20，并补了一句注释说明原因。',
    offsetMs: 0,
    timeSeconds: 1.86,
    ttftMs: 598,
    tokens: { input: 6120, output: 264, think: 0, cacheRead: 3120 },
    inputDetail: '请求入参（截断示例）……',
    outputDetail: '输出全文：CI 的 Node 版本已对齐到 20，并补了一句注释说明原因。',
    thinkingDetail: '两处配置文件均已对齐，补充注释说明 engines 来源。',
    provider: 'deepseek',
    model: 'deepseek-chat',
  },
  // ── Turn #3 ──
  {
    turn: 3,
    group: 'User',
    kind: 'user',
    text: '运行一次端到端验证，把结果整理到发布说明。',
    inputDetail: '用户消息全文：运行一次端到端验证，把结果整理到发布说明。',
  },
  {
    turn: 3,
    group: 'Step 1',
    kind: 'message',
    text: '我来跑完整验证并生成发布说明。',
    offsetMs: 0,
    timeSeconds: 3.4,
    ttftMs: 1128,
    tokens: { input: 6630, output: 476, think: 210, cacheRead: 3340 },
    inputDetail: '请求入参（截断示例）……',
    outputDetail: '输出全文：我来跑完整验证并生成发布说明。',
    thinkingDetail: '端到端验证需要构建产物；先执行构建再看结果。',
    provider: 'deepseek',
    model: 'deepseek-chat',
  },
  {
    turn: 3,
    group: 'Step 1',
    kind: 'tool',
    text: 'bash · pnpm build',
    offsetMs: 0,
    timeSeconds: 12.4,
    callId: 'call-bash-2',
    isError: true,
    inputDetail: '{"command": "pnpm build", "cwd": "workspace"}',
    outputDetail: 'exit 1 · dist/ 缺少字体资源（woff2 ×2）',
    result: 'exit 1 · 产物检查失败',
  },
  {
    turn: 3,
    group: 'Step 2',
    kind: 'message',
    text: '构建失败：`dist/` 缺少字体资源。我重试一次并记录根因。',
    offsetMs: 0,
    timeSeconds: 3.9,
    ttftMs: 1520,
    tokens: { input: 6891, output: 512, think: 268, cacheRead: 3450 },
    inputDetail: '请求入参（截断示例）……',
    outputDetail: '输出全文：构建失败：`dist/` 缺少字体资源。我重试一次并记录根因。',
    thinkingDetail: '失败根因是字体资源未随构建复制；重试一次并补校验脚本。',
    provider: 'deepseek',
    model: 'deepseek-chat',
  },
  {
    turn: 3,
    group: 'Step 2',
    kind: 'tool',
    text: 'bash · pnpm build -- --list-fonts',
    offsetMs: 0,
    timeSeconds: 8.6,
    callId: 'call-bash-3',
    inputDetail: '{"command": "pnpm build -- --list-fonts", "cwd": "workspace"}',
    outputDetail: 'exit 0 · 字体资源清单缺 2 项（Inter-Regular.woff2、JetBrainsMono-Bold.woff2）',
    result: 'exit 0 · 缺 2 项字体资源',
  },
  {
    turn: 3,
    group: 'Step 2',
    kind: 'tool',
    text: 'write · scripts/check-fonts.mjs',
    offsetMs: 0,
    timeSeconds: 0.28,
    callId: 'call-write-3',
    inputDetail: '{"path": "scripts/check-fonts.mjs", "content": "(演示样例)"}',
    outputDetail: '校验脚本已落盘。',
    result: '校验脚本已落盘',
  },
  {
    turn: 3,
    group: 'Step 3',
    kind: 'message',
    text: '端到端验证通过，已生成发布说明并附构建失败根因条目。',
    offsetMs: 0,
    timeSeconds: 2.2,
    ttftMs: 876,
    tokens: { input: 7312, output: 433, think: 96, cacheRead: 3720 },
    inputDetail: '请求入参（截断示例）……',
    outputDetail: '输出全文：端到端验证通过，已生成发布说明并附构建失败根因条目。',
    thinkingDetail: '验证通过；将失败根因写入发布说明「已知问题」。',
    provider: 'deepseek',
    model: 'deepseek-chat',
  },
  {
    turn: 3,
    group: 'Compaction',
    kind: 'compacted',
    text: '较早回合的对话与工具调用已压缩进上下文摘要。',
    tokens: { input: 2410, cacheWrite: 2410 },
    inputDetail: '压缩摘要（演示样例）：…',
  },
];

/** 组装全部演示记录：赋 index/id，并按帧推进时钟。 */
function buildRecords(): TrajectoryRecord[] {
  const clock: RowClock = { at: 0 };
  const records = cells(clock, RAW_ROWS);
  return records.map((record, index) => ({ ...record, index: index + 1, id: `rec-${index + 1}` }));
}

/** 中控台 → 轨迹面板使用的全部演示记录（确定性常量，勿在模块顶层随机化）。 */
export const DEMO_TRAJECTORY_RECORDS: TrajectoryRecord[] = buildRecords();

/** 请求级演示数据：与各 Step 的 ASSISTANT 行一一对应，另含一次压缩。 */
export const DEMO_TRAJECTORY_REQUESTS: TrajectoryRequest[] = [
  {
    number: 1,
    turn: 1,
    group: 'Step 1',
    status: 'complete',
    startedAt: BASE,
    completedAt: BASE + 2760,
    provider: 'deepseek',
    model: 'deepseek-chat',
    requestConfig: { provider: 'deepseek', model: 'deepseek-chat' },
    usage: { input: 4523, output: 412, think: 156, cacheRead: 1890, cacheWrite: 0 },
  },
  {
    number: 2,
    turn: 1,
    group: 'Step 2',
    status: 'complete',
    startedAt: BASE + 3150,
    completedAt: BASE + 5130,
    provider: 'deepseek',
    model: 'deepseek-chat',
    requestConfig: { provider: 'deepseek', model: 'deepseek-chat' },
    usage: { input: 4980, output: 328, think: 96, cacheRead: 2210, cacheWrite: 0 },
  },
  {
    number: 3,
    turn: 1,
    group: 'Step 3',
    status: 'complete',
    startedAt: BASE + 6910,
    completedAt: BASE + 9310,
    provider: 'deepseek',
    model: 'deepseek-chat',
    requestConfig: { provider: 'deepseek', model: 'deepseek-chat' },
    usage: { input: 5341, output: 289, think: 0, cacheRead: 2560, cacheWrite: 764 },
  },
  {
    number: 4,
    turn: 2,
    group: 'Step 1',
    status: 'complete',
    startedAt: BASE + 9350,
    completedAt: BASE + 11450,
    provider: 'deepseek',
    model: 'deepseek-chat',
    requestConfig: { provider: 'deepseek', model: 'deepseek-chat' },
    usage: { input: 5760, output: 341, think: 122, cacheRead: 2890, cacheWrite: 0 },
  },
  {
    number: 5,
    turn: 2,
    group: 'Step 2',
    status: 'complete',
    startedAt: BASE + 12590,
    completedAt: BASE + 14450,
    provider: 'deepseek',
    model: 'deepseek-chat',
    requestConfig: { provider: 'deepseek', model: 'deepseek-chat' },
    usage: { input: 6120, output: 264, think: 0, cacheRead: 3120, cacheWrite: 0 },
  },
  {
    number: 6,
    turn: 3,
    group: 'Step 1',
    status: 'complete',
    startedAt: BASE + 14490,
    completedAt: BASE + 17890,
    provider: 'deepseek',
    model: 'deepseek-chat',
    requestConfig: { provider: 'deepseek', model: 'deepseek-chat' },
    usage: { input: 6630, output: 476, think: 210, cacheRead: 3340, cacheWrite: 0 },
  },
  {
    number: 7,
    turn: 3,
    group: 'Step 2',
    status: 'complete',
    startedAt: BASE + 30370,
    completedAt: BASE + 34270,
    provider: 'deepseek',
    model: 'deepseek-chat',
    requestConfig: { provider: 'deepseek', model: 'deepseek-chat' },
    retry: 1,
    maxRetries: 2,
    retryDelayMs: 800,
    usage: { input: 6891, output: 512, think: 268, cacheRead: 3450, cacheWrite: 0 },
  },
  {
    number: 8,
    turn: 3,
    group: 'Step 3',
    status: 'complete',
    startedAt: BASE + 43270,
    completedAt: BASE + 45470,
    provider: 'deepseek',
    model: 'deepseek-chat',
    requestConfig: { provider: 'deepseek', model: 'deepseek-chat' },
    usage: { input: 7312, output: 433, think: 96, cacheRead: 3720, cacheWrite: 0 },
  },
  {
    number: 9,
    turn: null,
    group: 'Compaction',
    purpose: 'compaction',
    status: 'complete',
    startedAt: BASE + 45510,
    completedAt: BASE + 46010,
    provider: 'deepseek',
    model: 'deepseek-chat',
    requestConfig: { provider: 'deepseek', model: 'deepseek-chat' },
    usage: { input: 2410, output: 0, think: 0, cacheRead: 0, cacheWrite: 2410 },
  },
];

/** 返回某请求覆盖记录的请求级信息；找不到返回 undefined。 */
export function requestByNumber(number: number): TrajectoryRequest | undefined {
  return DEMO_TRAJECTORY_REQUESTS.find((request) => request.number === number);
}

/** 返回请求号对应的起始记录（用于账本请求标记的渲染）。 */
export function recordForRequest(number: number): TrajectoryRecord | undefined {
  const request = requestByNumber(number);
  if (request === undefined) return undefined;
  return DEMO_TRAJECTORY_RECORDS.find(
    (record) => record.turn === request.turn && record.group === request.group && record.kind === 'message',
  );
}