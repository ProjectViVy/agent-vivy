// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale, resetLocaleForTests } from '@/i18n';
import * as api from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { ChatInput } from './ChatInput';

vi.mock('./WorkspaceSelector', () => ({ WorkspaceSelector: () => null }));
vi.mock('./HistoryPicker', () => ({ HistoryPicker: () => null }));
vi.mock('@/lib/api', async (original) => ({ ...await original<typeof api>(), listSkills: vi.fn(async () => ({ skills: [] })) }));
vi.mock('@/lib/run-subscription', () => ({ subscribeRun: vi.fn(() => ({ close: vi.fn(), lastSeq: () => 0 })) }));

const initial = useVivyStore.getState();
const emptyWork: api.WorkState = { session_id: 's1', version: 1, activation: 'disarmed', plan: { active: false, review_status: 'none' } };

describe('composer queue tracks (VCP-B3, pi parity)', () => {
  let container: HTMLDivElement;
  let root: Root;
  let onSend: ReturnType<typeof vi.fn>;
  let onSteer: ReturnType<typeof vi.fn>;
  let onFollowUp: ReturnType<typeof vi.fn>;
  let onDequeue: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    useVivyStore.setState({ ...initial, activeSessionId: 's1', work: emptyWork, workPhase: 'ready',
      sessions: [{ id: 's1', title: 'Session', created_at: 1, permission_preset: 'smart' }] });
    onSend = vi.fn(async () => undefined);
    onSteer = vi.fn(async () => undefined);
    onFollowUp = vi.fn(async () => undefined);
    onDequeue = vi.fn(async () => 'restored text');
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    useVivyStore.setState(initial);
    resetLocaleForTests();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  async function render(running = true) {
    await act(async () => root.render(
      <ChatInput onSend={onSend} onSteer={onSteer} onFollowUp={onFollowUp} onDequeue={onDequeue} running={running} />,
    ));
  }
  const input = () => container.querySelector('textarea')!;
  async function type(value: string) {
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(input(), value);
      input().dispatchEvent(new Event('input', { bubbles: true }));
    });
  }
  async function key(key: string, init: KeyboardEventInit = {}) {
    await act(async () => { input().dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init })); });
  }

  it('routes busy Enter to the steer track', async () => {
    await render(true);
    await type('nudge it');
    await key('Enter');
    expect(onSteer).toHaveBeenCalledWith('nudge it', 'normal', undefined, 'auto');
    expect(onFollowUp).not.toHaveBeenCalled();
    expect(onSend).not.toHaveBeenCalled();
  });

  it('routes Shift+Alt+Enter to the follow-up track', async () => {
    await render(true);
    await type('after this');
    await key('Enter', { shiftKey: true, altKey: true });
    expect(onFollowUp).toHaveBeenCalledWith('after this', 'normal', undefined, 'auto');
    expect(onSteer).not.toHaveBeenCalled();
  });

  it('restores dequeued text into the editor on Alt+Up', async () => {
    await render(true);
    await key('ArrowUp', { altKey: true });
    expect(onDequeue).toHaveBeenCalledOnce();
    expect(input().value).toBe('restored text');
  });

  it('renders both kernel lanes plus the local FIFO with per-item remove', async () => {
    const removeKernelQueued = vi.fn(async () => undefined);
    useVivyStore.setState({
      kernelQueue: { steering: [{ id: 'q1', session_id: 's1', track: 'steer', text: 'steer text', created_at: 1 }], follow_up: [{ id: 'q2', session_id: 's1', track: 'follow_up', text: 'follow text', created_at: 2 }], steer_mode: 'one-at-a-time', follow_up_mode: 'all', pending: 2 },
      removeKernelQueued: removeKernelQueued as never,
    });
    await render(true);
    expect(container.textContent).toContain('2 queued');
    expect(container.textContent).toContain('steer text');
    expect(container.textContent).toContain('follow text');
    await act(async () => {
      [...container.querySelectorAll<HTMLButtonElement>('button')].find((b) => b.getAttribute('aria-label') === 'Remove: follow text')!.click();
    });
    expect(removeKernelQueued).toHaveBeenCalledWith('q2');
  });

  it('keeps idle Enter on the normal send path', async () => {
    await render(false);
    await type('fresh turn');
    await key('Enter');
    expect(onSend).toHaveBeenCalledWith('fresh turn', 'normal', undefined, 'auto');
    expect(onSteer).not.toHaveBeenCalled();
    expect(onFollowUp).not.toHaveBeenCalled();
  });
});
