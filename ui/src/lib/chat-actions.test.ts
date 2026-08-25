import { describe, expect, it } from 'vitest';
import { regeneratePrompt } from './chat-actions';
import type { Message } from './api';

function msg(id: string, role: Message['role'], content: string): Message {
  return { id, role, content, created_at: 1 };
}

describe('regeneratePrompt', () => {
  it('返回目标助手消息之前最近一条用户消息的内容', () => {
    const messages = [
      msg('u1', 'user', '第一问'),
      msg('a1', 'assistant', '第一答'),
      msg('u2', 'user', '第二问'),
      msg('a2', 'assistant', '第二答'),
    ];
    expect(regeneratePrompt(messages, 'a1')).toBe('第一问');
    expect(regeneratePrompt(messages, 'a2')).toBe('第二问');
  });

  it('跳过中间的工具与系统消息，仍取最近的 user 消息', () => {
    const messages = [
      msg('u1', 'user', '唯一提问'),
      msg('t1', 'tool', '{"ok":true}'),
      msg('s1', 'system', 'preflight'),
      msg('a1', 'assistant', '回答'),
    ];
    expect(regeneratePrompt(messages, 'a1')).toBe('唯一提问');
  });

  it('助手消息之前没有 user 消息时返回 null', () => {
    const messages = [msg('s1', 'system', 'boot'), msg('a1', 'assistant', '问候')];
    expect(regeneratePrompt(messages, 'a1')).toBeNull();
  });

  it('目标是 user 或 tool 消息时返回 null（重试只作用于助手消息）', () => {
    const messages = [msg('u1', 'user', '提问'), msg('t1', 'tool', '结果'), msg('a1', 'assistant', '回答')];
    expect(regeneratePrompt(messages, 'u1')).toBeNull();
    expect(regeneratePrompt(messages, 't1')).toBeNull();
  });

  it('消息 id 不存在时返回 null', () => {
    const messages = [msg('u1', 'user', '提问'), msg('a1', 'assistant', '回答')];
    expect(regeneratePrompt(messages, 'missing')).toBeNull();
  });

  it('同一 id 出现多条时只匹配助手消息', () => {
    const messages = [msg('dup', 'user', '用户复用 id'), msg('dup', 'assistant', '助手复用 id')];
    expect(regeneratePrompt(messages, 'dup')).toBe('用户复用 id');
  });
});
