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

describe('composer single primary button (queue/steer)', () => {
  let container: HTMLDivElement;
  let root: Root;
  let onSend: ReturnType<typeof vi.fn>;
  let onSteer: ReturnType<typeof vi.fn>;
  let onFollowUp: ReturnType<typeof vi.fn>;
  let onDequeue: ReturnType<typeof vi.fn>;
  let onCancel: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    useVivyStore.setState({ ...initial, activeSessionId: 's1', work: emptyWork, workPhase: 'ready',
      sessions: [{ id: 's1', title: 'Session', created_at: 1, permission_preset: 'smart' }] });
    onSend = vi.fn(async () => undefined);
    onSteer = vi.fn(async () => undefined);
    onFollowUp = vi.fn(async () => undefined);
    onDequeue = vi.fn(async () => 'restored text');
    onCancel = vi.fn(async () => undefined);
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

  async function render(running = true, busyEnter: 'queue' | 'steer' = 'queue') {
    await act(async () => root.render(
      <ChatInput onSend={onSend} onSteer={onSteer} onFollowUp={onFollowUp} onDequeue={onDequeue} onCancel={onCancel} running={running} busyEnter={busyEnter} />,
    ));
  }
  const input = () => container.querySelector('textarea')!;
  const button = (label: string) => [...container.querySelectorAll<HTMLButtonElement>('button')].filter((b) => b.getAttribute('aria-label') === label);
  const primary = () => [...container.querySelectorAll<HTMLButtonElement>('button')].filter((b) => ['Send', 'Queue', 'Steer', 'Cancel run'].includes(b.getAttribute('aria-label') ?? ''));
  async function type(value: string) {
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(input(), value);
      input().dispatchEvent(new Event('input', { bubbles: true }));
    });
  }
  async function key(key: string, init: KeyboardEventInit = {}) {
    await act(async () => { input().dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init })); });
  }

  it('queues on busy Enter by default', async () => {
    await render(true);
    await type('nudge it');
    await key('Enter');
    expect(onFollowUp).toHaveBeenCalledWith('nudge it', 'normal', undefined, 'auto');
    expect(onSteer).not.toHaveBeenCalled();
    expect(onSend).not.toHaveBeenCalled();
  });

  it('steers on busy Enter when the preference is steer', async () => {
    await render(true, 'steer');
    await type('nudge it');
    await key('Enter');
    expect(onSteer).toHaveBeenCalledWith('nudge it', 'normal', undefined, 'auto');
    expect(onFollowUp).not.toHaveBeenCalled();
    expect(onSend).not.toHaveBeenCalled();
  });

  it('inverts the preference on Cmd/Ctrl+Enter', async () => {
    await render(true, 'queue');
    await type('nudge it');
    await key('Enter', { metaKey: true });
    expect(onSteer).toHaveBeenCalledWith('nudge it', 'normal', undefined, 'auto');
    expect(onFollowUp).not.toHaveBeenCalled();
    expect(onSend).not.toHaveBeenCalled();

    vi.clearAllMocks();
    await render(true, 'steer');
    await type('nudge it');
    await key('Enter', { ctrlKey: true });
    expect(onFollowUp).toHaveBeenCalledWith('nudge it', 'normal', undefined, 'auto');
    expect(onSteer).not.toHaveBeenCalled();
    expect(onSend).not.toHaveBeenCalled();
  });

  it('no longer submits on Shift+Alt+Enter', async () => {
    await render(true);
    await type('after this');
    await key('Enter', { shiftKey: true, altKey: true });
    expect(onFollowUp).not.toHaveBeenCalled();
    expect(onSteer).not.toHaveBeenCalled();
    expect(onSend).not.toHaveBeenCalled();
  });

  it('labels the single primary button Queue or Steer and drops the old extra buttons', async () => {
    await render(true, 'queue');
    await type('draft');
    expect(primary().map((b) => b.getAttribute('aria-label'))).toEqual(['Queue']);
    expect(button('Send now')).toHaveLength(0);
    expect(button('Queue follow-up')).toHaveLength(0);

    await render(true, 'steer');
    await type('draft');
    expect(primary().map((b) => b.getAttribute('aria-label'))).toEqual(['Steer']);
  });

  it('clicking the primary button uses the preference', async () => {
    await render(true, 'queue');
    await type('swap task');
    await act(async () => { primary()[0].click(); });
    expect(onFollowUp).toHaveBeenCalledWith('swap task', 'normal', undefined, 'auto');
    expect(onSteer).not.toHaveBeenCalled();
  });

  it('shows Stop only while running with an empty draft', async () => {
    await render(true);
    expect(primary().map((b) => b.getAttribute('aria-label'))).toEqual(['Cancel run']);
    await act(async () => { primary()[0].click(); });
    expect(onCancel).toHaveBeenCalledOnce();

    await type('draft');
    expect(primary().map((b) => b.getAttribute('aria-label'))).toEqual(['Queue']);
  });

  it('cancels on Escape only after the queue is empty', async () => {
    const clearQueue = vi.fn(async () => undefined);
    useVivyStore.setState({
      kernelQueue: { steering: [], follow_up: [{ id: 'q2', session_id: 's1', track: 'follow_up', text: 'follow text', created_at: 2 }], steer_mode: 'one-at-a-time', follow_up_mode: 'all', pending: 1 },
      clearQueue: clearQueue as never,
    });
    await render(true);
    await key('Escape');
    expect(clearQueue).toHaveBeenCalledOnce();
    expect(onCancel).not.toHaveBeenCalled();

    await act(async () => { useVivyStore.setState({ kernelQueue: null }); });
    await key('Escape');
    expect(onCancel).toHaveBeenCalledOnce();
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
      queuedMessages: [{ id: 'l1', text: 'local attach' }],
      removeKernelQueued: removeKernelQueued as never,
    });
    await render(true);
    expect(container.textContent).toContain('3 queued');
    expect(container.textContent).toContain('steer text');
    expect(container.textContent).toContain('follow text');
    expect(container.textContent).toContain('local attach');
    await act(async () => {
      [...container.querySelectorAll<HTMLButtonElement>('button')].find((b) => b.getAttribute('aria-label') === 'Remove: follow text')!.click();
    });
    expect(removeKernelQueued).toHaveBeenCalledWith('q2');
  });

  it('flushes the queue from the pill without touching Clear queue', async () => {
    const sendQueuedNow = vi.fn(async () => undefined);
    useVivyStore.setState({
      kernelQueue: { steering: [], follow_up: [{ id: 'q2', session_id: 's1', track: 'follow_up', text: 'follow text', created_at: 2 }], steer_mode: 'one-at-a-time', follow_up_mode: 'all', pending: 1 },
      sendQueuedNow: sendQueuedNow as never,
    });
    await render(true);
    const flush = button('Send queued now');
    expect(flush).toHaveLength(1);
    expect(button('Clear queue')).toHaveLength(1);
    await act(async () => { flush[0].click(); });
    expect(sendQueuedNow).toHaveBeenCalledOnce();
  });

  it('keeps idle Enter on the normal send path and labels the button Send', async () => {
    await render(false);
    expect(primary().map((b) => b.getAttribute('aria-label'))).toEqual(['Send']);
    await type('fresh turn');
    await key('Enter');
    expect(onSend).toHaveBeenCalledWith('fresh turn', 'normal', undefined, 'auto');
    expect(onSteer).not.toHaveBeenCalled();
    expect(onFollowUp).not.toHaveBeenCalled();
  });
});
