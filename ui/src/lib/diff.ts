// 统一 diff 解析（VC-1f）：服务端 boundedDiff 输出 go-udiff 的标准统一 diff，
// 但 32 KiB 截断可能砍断尾部 hunk，历史数据还可能是无行号、上下文不带空格
// 前缀的伪 diff —— 解析器因此刻意宽松：绝不依赖 hunk 头声明的行数，只按
// 行类型推进计数；无法确认是 diff 时返回 null，让调用方回退纯文本渲染。

export type DiffLineType = 'context' | 'add' | 'del' | 'meta';

export interface DiffLine {
  type: DiffLineType;
  /** 旧文件行号（hunk 头缺行号时缺省） */
  old?: number;
  /** 新文件行号（hunk 头缺行号时缺省） */
  new?: number;
  /** 不含 +/-/空格 前缀的行内容 */
  text: string;
}

export interface DiffHunk {
  header: string;
  lines: DiffLine[];
}

export interface ParsedDiff {
  hunks: DiffHunk[];
  additions: number;
  deletions: number;
  truncated: boolean;
}

const HUNK_HEADER_RE = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,\d+)? @@/;
const FILE_HEADER_RE = /^(?:diff |--- |Index: )/;
const TRUNCATED_MARKER = '[diff truncated]';

/** 文本是否是一份可渲染的统一 diff（文件头 + 至少一个 hunk）。 */
export function looksLikeDiff(text: string): boolean {
  return parseUnifiedDiff(text) !== null;
}

export function parseUnifiedDiff(text: string): ParsedDiff | null {
  const lines = text.split('\n');
  let i = 0;
  while (i < lines.length && lines[i].trim() === '') i += 1;
  if (i >= lines.length || !FILE_HEADER_RE.test(lines[i])) return null;

  const hunks: DiffHunk[] = [];
  let additions = 0;
  let deletions = 0;
  let truncated = false;
  let current: DiffHunk | null = null;
  let oldLine: number | null = null;
  let newLine: number | null = null;

  for (; i < lines.length; i += 1) {
    const raw = lines[i];
    if (raw.startsWith('@@')) {
      current = { header: raw, lines: [] };
      hunks.push(current);
      const match = HUNK_HEADER_RE.exec(raw);
      if (match) {
        oldLine = Number(match[1]);
        newLine = Number(match[3]);
      }
      continue;
    }
    // hunk 之外的行（+++ / --- 文件头等）不进入正文
    if (!current) continue;
    let line: DiffLine;
    if (raw.startsWith('+')) {
      line = { type: 'add', text: raw.slice(1), new: newLine ?? undefined };
      if (newLine !== null) newLine += 1;
      additions += 1;
    } else if (raw.startsWith('-')) {
      line = { type: 'del', text: raw.slice(1), old: oldLine ?? undefined };
      if (oldLine !== null) oldLine += 1;
      deletions += 1;
    } else if (raw.startsWith('\\')) {
      line = { type: 'meta', text: raw };
    } else {
      // 上下文行：标准格式带 ' ' 前缀；伪 diff 与被裁剪的行尾可能没有
      const body = raw.startsWith(' ') ? raw.slice(1) : raw;
      line = { type: 'context', text: body, old: oldLine ?? undefined, new: newLine ?? undefined };
      if (oldLine !== null) oldLine += 1;
      if (newLine !== null) newLine += 1;
    }
    if (line.text === TRUNCATED_MARKER) truncated = true;
    current.lines.push(line);
  }

  if (hunks.length === 0) return null;
  return { hunks, additions, deletions, truncated };
}

export interface DiffSplitRow {
  left: DiffLine | null;
  right: DiffLine | null;
}

/** 删除块与新增块按下标配对供分栏渲染；meta 行不成对，只在统一视图显示。 */
export function diffSplitRows(hunk: DiffHunk): DiffSplitRow[] {
  const rows: DiffSplitRow[] = [];
  let dels: DiffLine[] = [];
  let adds: DiffLine[] = [];
  const flush = () => {
    for (let i = 0; i < Math.max(dels.length, adds.length); i += 1) {
      rows.push({ left: dels[i] ?? null, right: adds[i] ?? null });
    }
    dels = [];
    adds = [];
  };
  for (const line of hunk.lines) {
    if (line.type === 'del') { dels.push(line); continue; }
    if (line.type === 'add') { adds.push(line); continue; }
    flush();
    if (line.type === 'context') rows.push({ left: line, right: line });
  }
  flush();
  return rows;
}

/**
 * 文件变更工具结果的 JSON（FileMutationResult：path/changed/bytes/sha256/diff）
 * 里提取 diff；非 JSON、JSON 但无 diff 字段时返回 null。
 */
export function parseToolResultDiff(content: string): { diff: string; path?: string } | null {
  if (!content.startsWith('{')) return null;
  try {
    const value: unknown = JSON.parse(content);
    if (typeof value !== 'object' || value === null) return null;
    const record = value as Record<string, unknown>;
    if (typeof record.diff !== 'string' || record.diff.trim() === '') return null;
    return {
      diff: record.diff,
      path: typeof record.path === 'string' && record.path !== '' ? record.path : undefined,
    };
  } catch {
    return null;
  }
}
