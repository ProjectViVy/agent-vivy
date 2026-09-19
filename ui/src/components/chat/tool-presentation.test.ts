import { describe, expect, it } from 'vitest';
import type { FoldedToolCall } from '@/lib/run-rows';
import {
  TOOL_BODY_MAX_LINES, headTailCap, splitLines, toolDiff, toolExitStatus, toolSummary, toolTitleKey, toolVariant,
} from './tool-presentation';

function call(partial: Partial<FoldedToolCall> = {}): FoldedToolCall {
  return {
    callId: 'c1',
    name: 'read_file',
    args: { path: 'ui/src/a.ts' },
    argsRaw: '{"path":"ui/src/a.ts"}',
    result: '',
    error: '',
    status: 'ok',
    startedAt: 1,
    endedAt: 2,
    ...partial,
  };
}

describe('tool variants', () => {
  it('maps wire tool names onto the DSH-style families', () => {
    expect(toolVariant('read_file')).toBe('read');
    expect(toolVariant('write_note')).toBe('write');
    expect(toolVariant('multiedit')).toBe('edit');
    expect(toolVariant('grep')).toBe('search');
    expect(toolVariant('commandline')).toBe('shell');
    expect(toolVariant('web_fetch')).toBe('web');
    expect(toolVariant('task_list')).toBe('todo');
    expect(toolVariant('ask_user')).toBe('ask');
    expect(toolVariant('echo_info')).toBe('generic');
    expect(toolTitleKey('read_file')).toBe('chat.toolTitleRead');
    expect(toolTitleKey('echo_info')).toBe('chat.toolTitleGeneric');
  });
});

describe('toolSummary', () => {
  it('prefers the semantic argument key', () => {
    expect(toolSummary(call())).toBe('ui/src/a.ts');
    expect(toolSummary(call({ name: 'bash', args: { command: 'ls -la', description: '列目录' } }))).toBe('列目录');
    expect(toolSummary(call({ name: 'grep', args: { pattern: 'ToolRow', path: 'ui/src' } }))).toBe('ToolRow');
  });

  it('keeps only the first line and prefixes the wire name for unknown tools', () => {
    expect(toolSummary(call({ name: 'bash', args: { command: 'echo 1\necho 2' } }))).toBe('echo 1');
    expect(toolSummary(call({ name: 'echo_info', args: { text: 'hi' } }))).toBe('echo_info · hi');
  });

  it('falls back to the call id when there are no arguments', () => {
    expect(toolSummary(call({ name: 'bash', args: null, argsRaw: '' }))).toBe('c1');
  });
});

describe('result parsing', () => {
  it('pulls the shell exit marker out of the visible body', () => {
    expect(toolExitStatus('line\n[exit code: 1]')).toEqual({ exitCode: '1', signal: '', body: 'line' });
    expect(toolExitStatus('out\n[killed by signal: SIGKILL]')).toEqual({ exitCode: '', signal: 'SIGKILL', body: 'out' });
    expect(toolExitStatus('plain')).toEqual({ exitCode: '', signal: '', body: 'plain' });
  });

  it('extracts the diff and its totals from a file mutation result', () => {
    const diff = ['--- a/a.ts', '+++ b/a.ts', '@@ -1,2 +1,2 @@', '-old', '+new', ' keep'].join('\n');
    const result = JSON.stringify({ path: 'a.ts', diff });
    expect(toolDiff(result)).toMatchObject({ path: 'a.ts', additions: 1, deletions: 1 });
    // 历史伪 diff（没有文件头）解析不出 ± 时，不宣称统计。
    expect(toolDiff(JSON.stringify({ path: 'a.ts', diff: '@@ -1,2 +1,2 @@\n-old\n+new' }))).toMatchObject({ additions: null, deletions: null });
    expect(toolDiff('not json')).toBeNull();
  });
});

describe('head/tail folding', () => {
  it('keeps both ends and reports the hidden count', () => {
    const lines = Array.from({ length: 20 }, (_, index) => `line ${index}`);
    const folded = headTailCap(lines, TOOL_BODY_MAX_LINES, false);
    expect(folded.head).toHaveLength(4);
    expect(folded.tail).toHaveLength(4);
    expect(folded.hidden).toBe(20 - TOOL_BODY_MAX_LINES);
    expect(headTailCap(lines, TOOL_BODY_MAX_LINES, true).hidden).toBe(0);
  });

  it('splits trailing newlines away', () => {
    expect(splitLines('a\nb\n')).toEqual(['a', 'b']);
    expect(splitLines('')).toEqual([]);
  });
});
