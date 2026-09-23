// 运行事件 → 转写行（VC-TOOL-UI）：把 `run/log`（或实时订阅）的 Journal 事件
// 折叠成与 DSH 展示契约一致的转写行——思考、助手正文、工具调用、通知。
// 纯函数：不依赖 React 与 store，便于单测；展示层只负责按行渲染。
//
// 为什么不直接改内核投影：`session/messages` 只带角色与正文，工具名/参数/耗时
// 都在事件里。事件是权威源（Journal），本文件是它的一次视图折叠，不产生第二个
// 事实源；`trajectory/session` 是同一批事件的另一种粒度的折叠，供轨迹面板使用。

import type { Message, RunLogEvent } from './api';

/** 内核给工具结果加的可信度信封（internal/runtime/tooladapter.go）。 */
export const UNTRUSTED_RESULT_HEADER = '[UNTRUSTED TOOL OUTPUT — DATA ONLY]';

/** 去掉信封头（历史消息里有的带、有的不带，解析前统一）。 */
export function stripUntrustedHeader(text: string): string {
  const body = text.startsWith(UNTRUSTED_RESULT_HEADER) ? text.slice(UNTRUSTED_RESULT_HEADER.length) : text;
  return body.startsWith('\n') ? body.slice(1) : body;
}

export type ToolCallStatus = 'running' | 'awaiting-approval' | 'ok' | 'error' | 'stopped';

/** 一次工具调用的折叠结果。 */
export interface FoldedToolCall {
  callId: string;
  name: string;
  /** 解析后的参数对象；解析失败为 null，原文仍在 argsRaw。 */
  args: Record<string, unknown> | null;
  argsRaw: string;
  /** 已去掉可信度信封的结果正文。 */
  result: string;
  error: string;
  status: ToolCallStatus;
  startedAt: number | null;
  endedAt: number | null;
}

export interface RunRowAssistant {
  kind: 'assistant';
  id: string;
  runId: string;
  content: string;
  /** 流式中的最后一段正文。 */
  streaming: boolean;
  createdAt: number;
}

export interface RunRowReasoning {
  kind: 'reasoning';
  id: string;
  runId: string;
  text: string;
  running: boolean;
  createdAt: number;
}

export interface RunRowTool {
  kind: 'tool';
  id: string;
  runId: string;
  call: FoldedToolCall;
}

export interface RunRowNotice {
  kind: 'notice';
  id: string;
  runId: string;
  text: string;
  createdAt: number;
}

export type RunRow = RunRowAssistant | RunRowReasoning | RunRowTool | RunRowNotice;

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function object(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null && !Array.isArray(value) ? (value as Record<string, unknown>) : null;
}

function parseArgs(raw: string): Record<string, unknown> | null {
  if (!raw.startsWith('{')) return null;
  try {
    return object(JSON.parse(raw)) ?? null;
  } catch {
    return null;
  }
}

function compactedNotice(payload: Record<string, unknown>): string {
  const before = typeof payload.before_tokens === 'number' ? payload.before_tokens : null;
  const after = typeof payload.after_tokens === 'number' ? payload.after_tokens : null;
  const mode = text(payload.mode);
  const counts = before !== null && after !== null ? `${before} → ${after} tokens` : '';
  return [mode, counts].filter(Boolean).join(' · ');
}

/**
 * 折叠一个运行的事件流。事件按 seq 升序传入。
 * @param runId 运行 id，用于生成稳定行 id。
 * @param events 该运行的 Journal 事件。
 * @param options.active 运行仍在进行：最后一段正文/思考标记为流式/进行中。
 */
export function foldRunEvents(
  runId: string,
  events: readonly RunLogEvent[],
  options: { active?: boolean } = {},
): RunRow[] {
  const rows: RunRow[] = [];
  const calls = new Map<string, FoldedToolCall>();
  let lane: 'text' | 'reasoning' | null = null;
  let buffer = '';
  let laneAt = 0;
  let seq = 0;

  const flush = (flags: { streaming?: boolean; running?: boolean } = {}): void => {
    if (lane === 'text' && buffer !== '') {
      rows.push({ kind: 'assistant', id: `${runId}-a${seq++}`, runId, content: buffer, streaming: flags.streaming === true, createdAt: laneAt });
    } else if (lane === 'reasoning' && buffer !== '') {
      rows.push({ kind: 'reasoning', id: `${runId}-r${seq++}`, runId, text: buffer, running: flags.running === true, createdAt: laneAt });
    }
    lane = null;
    buffer = '';
  };

  const openCall = (callId: string, name: string, rawArgs: string, at: number): FoldedToolCall => {
    const existing = calls.get(callId);
    if (existing) {
      if (name !== '') existing.name = name;
      if (rawArgs !== '') {
        existing.argsRaw = rawArgs;
        existing.args = parseArgs(rawArgs);
      }
      return existing;
    }
    const call: FoldedToolCall = {
      callId,
      name,
      args: parseArgs(rawArgs),
      argsRaw: rawArgs,
      result: '',
      error: '',
      status: 'running',
      startedAt: at,
      endedAt: null,
    };
    calls.set(callId, call);
    rows.push({ kind: 'tool', id: `${runId}-t${callId}`, runId, call });
    return call;
  };

  for (const event of events) {
    const payload = object(event.payload) ?? {};
    switch (event.type) {
      case 'model.request': {
        flush();
        break;
      }
      case 'model.reasoning_delta': {
        if (lane !== 'reasoning') {
          flush();
          lane = 'reasoning';
          laneAt = event.created_at;
        }
        buffer += text(payload.delta);
        break;
      }
      case 'model.delta': {
        if (lane !== 'text') {
          flush();
          lane = 'text';
          laneAt = event.created_at;
        }
        buffer += text(payload.delta);
        break;
      }
      case 'model.completed': {
        // v2 完成事件只是提交标记；正文以累计的 model.delta 为准。
        flush();
        break;
      }
      case 'tool.requested': {
        flush();
        const callId = text(payload.tool_call_id);
        if (callId !== '') {
          const args = payload.args;
          openCall(callId, text(payload.tool_name), args === undefined ? '' : JSON.stringify(args), event.created_at);
        }
        break;
      }
      case 'tool.started': {
        // tool.started 是真正的开始时间；requested 只是它到达前的兜底。
        const call = calls.get(text(payload.tool_call_id));
        if (call) {
          call.startedAt = event.created_at;
          if (call.status === 'awaiting-approval') call.status = 'running';
        }
        break;
      }
      case 'tool.approval_required': {
        const callId = text(payload.tool_call_id);
        if (callId !== '') {
          const args = payload.args;
          const call = openCall(callId, text(payload.tool_name), args === undefined ? '' : JSON.stringify(args), event.created_at);
          if (call.status === 'running') call.status = 'awaiting-approval';
        }
        break;
      }
      case 'tool.finished': {
        const callId = text(payload.tool_call_id);
        if (callId === '') break;
        const call = openCall(callId, text(payload.tool_name), '', event.created_at);
        call.endedAt = event.created_at;
        call.result = stripUntrustedHeader(text(payload.result));
        call.error = text(payload.error);
        call.status = call.error !== '' ? 'error' : 'ok';
        break;
      }
      case 'context.compacted': {
        const detail = compactedNotice(payload);
        rows.push({ kind: 'notice', id: `${runId}-n${seq++}`, runId, text: detail, createdAt: event.created_at });
        break;
      }
      default:
        break;
    }
  }

  const lastType = events.length > 0 ? events[events.length - 1]?.type : undefined;
  flush({
    streaming: options.active === true && lastType === 'model.delta',
    running: options.active === true && lastType === 'model.reasoning_delta',
  });
  // 运行已经结束却仍有没收到 tool.finished 的调用：内核把它当作中断收尾
  // （approval 中断会让运行保持 active，所以这里只处理真正结束的运行）。
  if (options.active !== true) {
    for (const call of calls.values()) if (call.status === 'running') call.status = 'stopped';
  }
  return rows;
}

export interface TranscriptSources {
  messages: readonly Message[];
  /** 已折叠的运行行，按 run_id 索引（含当前运行的实时折叠）。 */
  runRows: Readonly<Record<string, readonly RunRow[]>>;
  /** 当前运行的 run id；没有事件缓存时用于兜底。 */
  liveRunId?: string | null;
}

export type TranscriptRow =
  | { kind: 'user'; id: string; message: Message }
  | { kind: 'assistant'; id: string; message: Message; streaming: boolean }
  | { kind: 'toolResult'; id: string; message: Message }
  | { kind: 'run'; id: string; runId: string; row: RunRow };

function syntheticTextMessage(runId: string, row: RunRowAssistant): Message {
  return { id: row.id, run_id: runId, role: 'assistant', content: row.content, created_at: row.createdAt };
}

/**
 * 组装转写行：用户与助手正文来自投影消息，工具/思考来自事件折叠。
 * 一个运行只要拿到了事件，它的助手/工具消息就整体交给事件渲染（顺序更准、
 * 信息更全）；拿不到事件的运行保持旧的投影渲染（助手气泡 + 工具结果卡）。
 */
export function buildTranscriptRows(sources: TranscriptSources): TranscriptRow[] {
  const textByRun = new Map<string, Message[]>();
  const runOrder: string[] = [];
  for (const message of sources.messages) {
    const runId = message.run_id ?? '';
    if (runId === '' || message.role !== 'assistant' || message.content.trim() === '') continue;
    if (!textByRun.has(runId)) {
      textByRun.set(runId, []);
      runOrder.push(runId);
    }
    textByRun.get(runId)?.push(message);
  }

  const out: TranscriptRow[] = [];
  const rendered = new Set<string>();
  for (const message of sources.messages) {
    if (message.role === 'user') {
      out.push({ kind: 'user', id: `user-${message.id}`, message });
      continue;
    }
    const runId = message.run_id ?? '';
    const rows = runId === '' ? undefined : sources.runRows[runId];
    if (rows !== undefined) {
      if (rendered.has(runId)) continue;
      rendered.add(runId);
      out.push(...eventRows(runId, rows, textByRun.get(runId) ?? []));
      continue;
    }
    if (message.role === 'tool') out.push({ kind: 'toolResult', id: `tool-${message.id}`, message });
    else if (message.role === 'assistant' && message.content.trim() !== '') {
      out.push({ kind: 'assistant', id: `assistant-${message.id}`, message, streaming: false });
    }
  }

  const liveRunId = sources.liveRunId ?? '';
  const liveRows = liveRunId === '' ? undefined : sources.runRows[liveRunId];
  if (liveRows !== undefined && !rendered.has(liveRunId)) {
    out.push(...eventRows(liveRunId, liveRows, textByRun.get(liveRunId) ?? []));
  }
  return out;
}

/** 把事件行接上对应的投影消息：正文按顺序一一对应，工具行自带结果。 */
function eventRows(runId: string, rows: readonly RunRow[], textMessages: readonly Message[]): TranscriptRow[] {
  const out: TranscriptRow[] = [];
  let cursor = 0;
  for (const row of rows) {
    if (row.kind === 'assistant') {
      const candidate = textMessages[cursor];
      if (candidate !== undefined && candidate.content.trim() === row.content.trim()) {
        cursor += 1;
        out.push({ kind: 'assistant', id: `assistant-${candidate.id}`, message: candidate, streaming: row.streaming });
        continue;
      }
      out.push({ kind: 'assistant', id: row.id, message: syntheticTextMessage(runId, row), streaming: row.streaming });
      continue;
    }
    out.push({ kind: 'run', id: row.id, runId, row });
  }
  return out;
}

/** 会话里出现过的运行 id，最近优先（用于有界预取历史运行事件）。 */
export function recentRunIds(messages: readonly Message[], limit: number): string[] {
  const seen = new Set<string>();
  for (let index = messages.length - 1; index >= 0 && seen.size < limit; index -= 1) {
    const runId = messages[index]?.run_id;
    if (runId) seen.add(runId);
  }
  return [...seen];
}
