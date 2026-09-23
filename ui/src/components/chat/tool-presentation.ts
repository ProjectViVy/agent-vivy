// 工具行的展示派生（对照 DSH `ui-tool` 的 tool-call-model）：
// 线名 → 变体（标题/图标）、参数摘要、路径、退出码/信号、结果的行数折叠。
// 纯函数 + 常量表；组件只负责渲染，不在这里做 IO。

import {
  BookMarked, FilePlus2, FileText, Globe, ListChecks, MessageCircleQuestion, Package, PenLine, Search, Terminal, Wrench, type LucideIcon,
} from 'lucide-react';
import { parseToolResultDiff, parseUnifiedDiff } from '@/lib/diff';
import type { FoldedToolCall } from '@/lib/run-rows';

/** 工具体裁（对照 DSH 的 tool variant 集合，按 Vivy 的线名归类）。 */
export type ToolVariant = 'read' | 'write' | 'edit' | 'search' | 'shell' | 'web' | 'todo' | 'ask' | 'history' | 'deliver' | 'generic';

const TOOL_VARIANTS: Record<string, ToolVariant> = {
  read_file: 'read',
  read_note: 'read',
  list_dir: 'read',
  list_notes: 'read',
  skills_list: 'read',
  skill_view: 'read',
  write_file: 'write',
  write_note: 'write',
  patch: 'edit',
  multiedit: 'edit',
  skill_manage: 'edit',
  search_files: 'search',
  grep: 'search',
  glob: 'search',
  bash: 'shell',
  commandline: 'shell',
  execute: 'shell',
  job_output: 'shell',
  job_kill: 'shell',
  network_search: 'web',
  web_fetch: 'web',
  http_request: 'web',
  download: 'web',
  task_create: 'todo',
  task_get: 'todo',
  task_update: 'todo',
  task_list: 'todo',
  ask_user: 'ask',
  history_search: 'history',
  history_read: 'history',
  history_trace: 'history',
  reference_preview: 'history',
  present_files: 'deliver',
};

const VARIANT_TITLE: Record<ToolVariant, string> = {
  read: 'chat.toolTitleRead',
  write: 'chat.toolTitleWrite',
  edit: 'chat.toolTitleEdit',
  search: 'chat.toolTitleSearch',
  shell: 'chat.toolTitleShell',
  web: 'chat.toolTitleWeb',
  todo: 'chat.toolTitleTodo',
  ask: 'chat.toolTitleAsk',
  history: 'chat.toolTitleHistory',
  deliver: 'chat.toolTitleDeliver',
  generic: 'chat.toolTitleGeneric',
};

const VARIANT_ICON: Record<ToolVariant, LucideIcon> = {
  read: FileText,
  write: FilePlus2,
  edit: PenLine,
  search: Search,
  shell: Terminal,
  web: Globe,
  todo: ListChecks,
  ask: MessageCircleQuestion,
  history: BookMarked,
  deliver: Package,
  generic: Wrench,
};

/** 摘要优先取的参数键（对照 DSH 的每变体键序）。 */
const SUMMARY_KEYS: Record<ToolVariant, readonly string[]> = {
  read: ['path', 'file_path', 'pattern', 'query', 'id', 'name'],
  write: ['path', 'file_path', 'name', 'id'],
  edit: ['path', 'file_path', 'name', 'id'],
  search: ['pattern', 'query', 'path', 'glob'],
  shell: ['description', 'command', 'cmd', 'job_id', 'run_id', 'id'],
  web: ['query', 'q', 'url', 'urls'],
  todo: ['subject', 'title', 'action', 'task_id', 'id'],
  ask: ['question', 'prompt'],
  history: ['query', 'pattern', 'reference_id', 'session_id'],
  deliver: ['title', 'files'],
  generic: [],
};

/** 工具行正文的行数上限：与 DSH 一致，聊天里比独立面板更紧。 */
export const TOOL_BODY_MAX_LINES = 8;

export function toolVariant(name: string): ToolVariant {
  return TOOL_VARIANTS[name] ?? 'generic';
}

export function toolTitleKey(name: string): string {
  return VARIANT_TITLE[toolVariant(name)];
}

export function toolIcon(name: string): LucideIcon {
  return VARIANT_ICON[toolVariant(name)];
}

function firstString(record: Record<string, unknown>, keys: readonly string[]): string {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === 'string' && value.trim() !== '') return value;
  }
  return '';
}

function firstAnyString(record: Record<string, unknown>): string {
  for (const value of Object.values(record)) {
    if (typeof value === 'string' && value.trim() !== '') return value;
  }
  return '';
}

/** 单行摘要：优先语义键，其次任意字符串参数，再次参数原文/调用 id。 */
export function toolSummary(call: FoldedToolCall): string {
  const variant = toolVariant(call.name);
  const args = call.args;
  let base = '';
  if (args !== null) base = firstString(args, SUMMARY_KEYS[variant]) || firstAnyString(args);
  if (base === '') base = call.argsRaw === '' ? call.callId : call.argsRaw;
  const line = base.split('\n')[0]?.trim() ?? '';
  if (variant !== 'generic' || call.name === '') return line;
  return line === '' ? call.name : `${call.name} · ${line}`;
}

/** 文件类变体暴露的路径（用于展示与后续“打开文件”）。 */
export function toolPath(call: FoldedToolCall): string {
  const variant = toolVariant(call.name);
  if (variant !== 'read' && variant !== 'write' && variant !== 'edit') return '';
  const args = call.args;
  if (args === null) return '';
  return firstString(args, ['path', 'file_path']);
}

/** 结果最后一行里的退出标记（governed shell 约定）。 */
export function toolExitStatus(result: string): { exitCode: string; signal: string; body: string } {
  const visible = result.replace(/\s+$/, '');
  const exit = /\[exit code: (\d+)\]$/.exec(visible);
  if (exit) return { exitCode: exit[1] ?? '', signal: '', body: visible.slice(0, exit.index).replace(/\s+$/, '') };
  const signal = /\[killed by signal: ([^\]\n]+)\]$/.exec(visible);
  if (signal) return { exitCode: '', signal: signal[1] ?? '', body: visible.slice(0, signal.index).replace(/\s+$/, '') };
  return { exitCode: '', signal: '', body: visible };
}

/** 文件变更结果里的统一 diff 与 ± 统计（结果外层信封由调用方先剥掉）。
    统计为 null 表示这段 diff 无法按统一格式解析（历史伪 diff），此时不宣称 ±。 */
export function toolDiff(result: string): { diff: string; path?: string; additions: number | null; deletions: number | null } | null {
  const parsed = parseToolResultDiff(result);
  if (parsed === null) return null;
  const stats = parseUnifiedDiff(parsed.diff);
  return { diff: parsed.diff, path: parsed.path, additions: stats?.additions ?? null, deletions: stats?.deletions ?? null };
}

export interface HeadTail {
  head: string[];
  tail: string[];
  hidden: number;
}

/** 头尾折叠（对照 DSH headTailCap）：hidden = total - max，head = ceil(max/2)。 */
export function headTailCap(lines: readonly string[], maxLines: number, expanded: boolean): HeadTail {
  if (expanded || lines.length <= maxLines) return { head: [...lines], tail: [], hidden: 0 };
  const head = Math.ceil(maxLines / 2);
  return { head: lines.slice(0, head), tail: lines.slice(lines.length - (maxLines - head)), hidden: lines.length - maxLines };
}

/** 正文按行折叠；单行超长文本仍作为一行处理。 */
export function splitLines(text: string): string[] {
  return text === '' ? [] : text.replace(/\s+$/, '').split('\n');
}
