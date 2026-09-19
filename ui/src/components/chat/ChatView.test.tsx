// @vitest-environment happy-dom
// 实时运行的转写只有一个渲染者（VC-TOOL-UI 回归）：ChatView 曾同时渲染事件折叠
// 出的转写行与旧的流式兜底气泡，于是思考进行中出现两个思考入口、正文流式时同一段
// 文字出现两只气泡。这里用假 store 驱动真实的 ChatView，断言页面上的每种事实
// （思考入口、助手正文、真实消息 id）各出现一次；历史消息的投影兜底保持不变。
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale, resetLocaleForTests } from '@/i18n';
import type { Message, Run, RunLogEvent } from '@/lib/api';
import { ChatView } from './ChatView';

/** 每次渲染时叠加到真实初始状态上的字段；只覆盖测试需要的那几个。 */
const view = vi.hoisted(() => ({ state: {} as Record<string, unknown> }));

vi.mock('@/lib/store', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/store')>();
  const snapshot = () => ({ ...original.useVivyStore.getState(), ...view.state });
  const useVivyStore = Object.assign(
    (selector: (value: unknown) => unknown) => selector(snapshot()),
    { getState: snapshot, setState: () => undefined, subscribe: () => () => undefined },
  );
  return { ...original, useVivyStore };
});
// 输入区与待办条不参与转写渲染，避免把无关的 RPC 副作用带进断言。
vi.mock('./ChatInput', () => ({ ChatInput: () => null }));
vi.mock('./TodoProgressStrip', () => ({ TodoProgressStrip: () => null }));

const RUN: Run = { id: 'r1', session_id: 's1', status: 'active', created_at: 10 };
const USER: Message = { id: 'm1', run_id: 'r1', role: 'user', content: '问题', created_at: 9 };

function event(seq: number, type: string, payload: Record<string, unknown>): RunLogEvent {
  return { run_id: 'r1', seq, type, created_at: 10 + seq, payload_version: 1, payload };
}

function occurrences(haystack: string, needle: string): number {
  return haystack.split(needle).length - 1;
}

describe('ChatView transcript', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    resetLocaleForTests();
    hydrateLocale('zh');
    view.state = {};
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    resetLocaleForTests();
    vi.unstubAllGlobals();
  });

  async function render(): Promise<void> {
    await act(async () => root.render(<ChatView sessionId="s1" />));
  }

  it('renders a streaming reasoning segment through the reasoning row alone', async () => {
    view.state = {
      activeSessionId: 's1', messages: [USER], messagesPhase: 'ready', currentRun: RUN,
      runEvents: [event(1, 'model.reasoning_delta', { delta: '第一行\n最新一行' })],
      streamingReasoning: '第一行\n最新一行', streamingText: '',
    };
    await render();

    expect(container.querySelectorAll('[data-reasoning-row]')).toHaveLength(1);
    expect(container.querySelector('[data-reasoning-row]')?.getAttribute('data-state')).toBe('running');
    expect(container.textContent).toContain('最新一行');
    // 旧的兜底气泡会让同一段思考再出现一个 <details> 思考入口。
    expect(container.querySelectorAll('details')).toHaveLength(0);
    expect(container.textContent).not.toContain('思考过程（进行中）');
  });

  it('prints a streaming answer once', async () => {
    view.state = {
      activeSessionId: 's1', messages: [USER], messagesPhase: 'ready', currentRun: RUN,
      runEvents: [event(1, 'model.delta', { delta: '正在输出的答案' })],
      streamingText: '正在输出的答案', streamingReasoning: '',
    };
    await render();

    expect(occurrences(container.textContent ?? '', '正在输出的答案')).toBe(1);
  });

  it('keeps the settled run on the same single path and reuses the projected message', async () => {
    const answer: Message = { id: 'm2', run_id: 'r1', role: 'assistant', content: '答案', created_at: 12 };
    view.state = {
      activeSessionId: 's1', messages: [USER, answer], messagesPhase: 'ready',
      currentRun: { ...RUN, status: 'completed' },
      runEvents: [
        event(1, 'model.reasoning_delta', { delta: '第一行\n第二行' }),
        event(2, 'model.delta', { delta: '答案' }),
        event(3, 'model.completed', { content: '答案' }),
        event(4, 'run.completed', {}),
      ],
    };
    await render();

    expect(container.querySelectorAll('[data-reasoning-row]')).toHaveLength(1);
    expect(container.querySelectorAll('[data-reasoning-row]')[0]?.getAttribute('data-state')).toBe('ok');
    expect(occurrences(container.textContent ?? '', '答案')).toBe(1);
    // 事件行接回真实的投影消息，复制 / 重生成 / 回退 / 分叉的操作栏依赖该 id。
    expect(container.querySelector('[data-message-id="m2"]')).not.toBeNull();
  });

  it('falls back to the projection when a run has no loaded events', async () => {
    const answer: Message = { id: 'm2', run_id: 'r9', role: 'assistant', content: '历史答案', created_at: 12 };
    view.state = { activeSessionId: 's1', messages: [USER, answer], messagesPhase: 'ready', currentRun: null, runEvents: [], runLogs: {} };
    await render();

    expect(container.querySelectorAll('[data-reasoning-row]')).toHaveLength(0);
    expect(container.querySelector('[data-message-id="m2"]')).not.toBeNull();
    expect(container.textContent).toContain('历史答案');
  });
});
