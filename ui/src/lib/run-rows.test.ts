import { describe, expect, it } from 'vitest';
import type { Message, RunLogEvent } from './api';
import { UNTRUSTED_RESULT_HEADER, buildTranscriptRows, foldRunEvents, recentRunIds } from './run-rows';

function event(seq: number, type: string, payload: Record<string, unknown>, createdAt = seq): RunLogEvent {
  return { run_id: 'run-1', seq, type, created_at: createdAt, payload_version: 2, payload };
}

function message(partial: Partial<Message> & { id: string }): Message {
  return { role: 'assistant', content: '', created_at: 1, ...partial };
}

const TOOL_TURN: RunLogEvent[] = [
  event(1, 'model.request', {}),
  event(2, 'model.reasoning_delta', { delta: '先看文件\n再决定' }),
  event(3, 'model.delta', { delta: '我查一下。' }),
  event(4, 'tool.requested', { tool_call_id: 'c1', tool_name: 'read_file', args: { path: 'a.go' } }),
  event(5, 'tool.started', { tool_call_id: 'c1', tool_name: 'read_file' }),
  event(6, 'tool.finished', { tool_call_id: 'c1', tool_name: 'read_file', result: `${UNTRUSTED_RESULT_HEADER}\nfile body` }),
  event(7, 'model.delta', { delta: '完成了。' }),
  event(8, 'model.completed', { byte_len: 4, content_sha256: 'x' }),
];

describe('foldRunEvents', () => {
  it('folds reasoning, assistant text and tool calls in event order', () => {
    const rows = foldRunEvents('run-1', TOOL_TURN);
    expect(rows.map((row) => row.kind)).toEqual(['reasoning', 'assistant', 'tool', 'assistant']);
    expect(rows[0]).toMatchObject({ kind: 'reasoning', text: '先看文件\n再决定', running: false });
    expect(rows[1]).toMatchObject({ kind: 'assistant', content: '我查一下。', streaming: false });
    expect(rows[3]).toMatchObject({ kind: 'assistant', content: '完成了。' });
  });

  it('carries tool identity, arguments, result and timing', () => {
    const rows = foldRunEvents('run-1', TOOL_TURN);
    const tool = rows[2];
    expect(tool?.kind).toBe('tool');
    if (tool?.kind !== 'tool') throw new Error('expected a tool row');
    expect(tool.call).toMatchObject({
      callId: 'c1',
      name: 'read_file',
      args: { path: 'a.go' },
      status: 'ok',
      startedAt: 5,
      endedAt: 6,
    });
    // 结果里的可信度信封在折叠时剥掉，展示层直接拿正文。
    expect(tool.call.result).toBe('file body');
  });

  it('restarts the reasoning lane after text, keeping block order', () => {
    const rows = foldRunEvents('run-1', [
      event(1, 'model.reasoning_delta', { delta: '第一段' }),
      event(2, 'model.delta', { delta: '正文' }),
      event(3, 'model.reasoning_delta', { delta: '第二段' }),
    ]);
    expect(rows.map((row) => row.kind)).toEqual(['reasoning', 'assistant', 'reasoning']);
  });

  it('marks the trailing block while the run is active', () => {
    const running = foldRunEvents('run-1', [event(1, 'model.reasoning_delta', { delta: '还在想' })], { active: true });
    expect(running[0]).toMatchObject({ kind: 'reasoning', running: true });
    const streaming = foldRunEvents('run-1', [event(1, 'model.delta', { delta: '写着' })], { active: true });
    expect(streaming[0]).toMatchObject({ kind: 'assistant', streaming: true });
  });

  it('derives failed, awaiting-approval and stopped tools', () => {
    const failed = foldRunEvents('run-1', [
      event(1, 'tool.requested', { tool_call_id: 'c1', tool_name: 'bash', args: { command: 'ls' } }),
      event(2, 'tool.finished', { tool_call_id: 'c1', tool_name: 'bash', result: '', error: 'exit status 1' }),
    ]);
    expect(failed[0]).toMatchObject({ kind: 'tool' });
    if (failed[0]?.kind !== 'tool') throw new Error('expected a tool row');
    expect(failed[0].call).toMatchObject({ status: 'error', error: 'exit status 1' });

    const waiting = foldRunEvents('run-1', [
      event(1, 'tool.requested', { tool_call_id: 'c1', tool_name: 'write_file', args: { path: 'a.go' } }),
      event(2, 'tool.approval_required', { tool_call_id: 'c1', tool_name: 'write_file', args: { path: 'a.go' }, approval_id: 'ap-1' }),
    ], { active: true });
    if (waiting[0]?.kind !== 'tool') throw new Error('expected a tool row');
    expect(waiting[0].call.status).toBe('awaiting-approval');

    // 运行结束却没有 tool.finished：按中断收尾，而不是永远“运行中”。
    const stopped = foldRunEvents('run-1', [
      event(1, 'tool.requested', { tool_call_id: 'c1', tool_name: 'bash', args: { command: 'sleep 30' } }),
      event(2, 'run.cancelled', {}),
    ]);
    if (stopped[0]?.kind !== 'tool') throw new Error('expected a tool row');
    expect(stopped[0].call.status).toBe('stopped');
  });

  it('keeps soft-converted failures failed on tool.finished.error', () => {
    // ND-3：可恢复失败把诊断放进了 result 供模型纠错，但行状态必须由
    // tool.finished.error 决定，outcome=recoverable 也不能翻回 ok。
    const rows = foldRunEvents('run-1', [
      event(1, 'tool.requested', { tool_call_id: 'c1', tool_name: 'read_file', args: { path: 'missing.go' } }),
      event(2, 'tool.finished', {
        tool_call_id: 'c1',
        tool_name: 'read_file',
        result: 'read "missing.go": file does not exist',
        error: 'read "missing.go": file does not exist',
        outcome: 'recoverable',
        reason: 'not_found',
        effects: 'none',
      }),
    ]);
    const tool = rows[0];
    if (tool?.kind !== 'tool') throw new Error('expected a tool row');
    expect(tool.call).toMatchObject({
      status: 'error',
      error: 'read "missing.go": file does not exist',
      result: 'read "missing.go": file does not exist',
    });
  });

  it('emits a compaction notice', () => {
    const rows = foldRunEvents('run-1', [event(1, 'context.compacted', { mode: 'auto', before_tokens: 900, after_tokens: 300 })]);
    expect(rows[0]).toMatchObject({ kind: 'notice', text: 'auto · 900 → 300 tokens' });
  });
});

describe('buildTranscriptRows', () => {
  const messages: Message[] = [
    message({ id: 'u1', role: 'user', content: '你好', run_id: 'run-1' }),
    message({ id: 'a1', role: 'assistant', content: '我查一下。', run_id: 'run-1' }),
    message({ id: 't1', role: 'tool', content: 'file body', run_id: 'run-1' }),
    message({ id: 'a2', role: 'assistant', content: '完成了。', run_id: 'run-1' }),
  ];

  it('renders an event-sourced run from events, reusing the projected messages', () => {
    const rows = buildTranscriptRows({ messages, runRows: { 'run-1': foldRunEvents('run-1', TOOL_TURN) }, liveRunId: null });
    expect(rows.map((row) => (row.kind === 'run' ? `run:${row.row.kind}` : row.kind))).toEqual([
      'user', 'run:reasoning', 'assistant', 'run:tool', 'assistant',
    ]);
    // 助手正文接回投影消息，操作栏（复制/回退/分叉）依旧可用。
    const first = rows[2];
    expect(first?.kind === 'assistant' && first.message.id).toBe('a1');
  });

  it('falls back to projected rows when a run has no events', () => {
    const rows = buildTranscriptRows({ messages, runRows: {}, liveRunId: null });
    expect(rows.map((row) => row.kind)).toEqual(['user', 'assistant', 'toolResult', 'assistant']);
  });

  it('appends a live run whose messages are not projected yet', () => {
    const rows = buildTranscriptRows({
      messages: [message({ id: 'u1', role: 'user', content: '你好', run_id: 'run-1' })],
      runRows: { 'run-1': foldRunEvents('run-1', TOOL_TURN) },
      liveRunId: 'run-1',
    });
    expect(rows.map((row) => (row.kind === 'run' ? `run:${row.row.kind}` : row.kind))).toEqual([
      'user', 'run:reasoning', 'assistant', 'run:tool', 'assistant',
    ]);
  });
});

describe('recentRunIds', () => {
  it('lists distinct run ids newest first within the limit', () => {
    const ids = recentRunIds([
      message({ id: '1', run_id: 'run-a' }),
      message({ id: '2', run_id: 'run-b' }),
      message({ id: '3', run_id: 'run-a' }),
      message({ id: '4', run_id: 'run-c' }),
    ], 2);
    expect(ids).toEqual(['run-c', 'run-a']);
  });
});
