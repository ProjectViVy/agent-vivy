// @vitest-environment happy-dom
import { act, StrictMode } from 'react';
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

describe('complete queued editor restoration', () => {
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

  it('restores captured image and thinking preferences before edited resend', async () => {
    const turn = {
      id: 'q1', session_id: 's1', track: 'follow_up', text: 'captured turn', created_at: 1,
      mode: 'normal', thinking: 'high', face: 'code', policy_profile: 'full_auto',
      collaboration_mode: 'plan', collaboration_version: 1,
      attachments: [{ name: 'capture.png', mime_type: 'image/png', data: 'aW1hZ2U=' }],
      file_contexts: [{ path: 'source.go', name: 'source.go', size: 8, content: 'c25hcHNob3Q=' }],
      continuity: { request_id: 'original-request', history_scope: { workspace: true } },
    };
    const preset = { text: turn.text, seq: 1, turn };
    await act(async () => root.render(
      <ChatInput onSend={onSend} onDequeue={onDequeue} draftPreset={preset} running={false} />,
    ));
    expect(input().value).toBe('captured turn');
    expect(container.querySelector('img')?.getAttribute('src')).toBe('data:image/png;base64,aW1hZ2U=');
    await type('edited turn');
    await key('Enter');
    expect(onSend.mock.calls[0]?.slice(0, 4)).toEqual(['edited turn', 'normal', turn.attachments, 'high']);
    expect(onSend.mock.calls[0]?.[4]).toEqual(expect.objectContaining({
      face: 'code', policy_profile: 'full_auto', collaboration_mode: 'plan', collaboration_version: 1,
      file_contexts: turn.file_contexts, continuity: turn.continuity,
    }));
  });

  it('recalls a complete turn with Alt+Up and retains it after a failed steer', async () => {
    const turn: api.QueuedTurn = { id: 'q1', session_id: 's1', track: 'follow_up', text: 'restore', created_at: 1,
      mode: 'plan', thinking: 'high', face: 'code', attachments: [{ name: 'x.png', mime_type: 'image/png', data: 'aW1hZ2U=' }],
      continuity: { request_id: 'stable', history_scope: { workspace: true } } };
    onDequeue.mockResolvedValue(turn);
    onSteer.mockRejectedValueOnce(new Error('temporary'));
    await render();
    await key('ArrowUp', { altKey: true });
    await type('edited');
    expect(container.querySelector<HTMLSelectElement>('[aria-label="Execution mode"]')?.value).toBe('plan');
    await key('Enter');
    expect(input().value).toBe('edited');
    expect(useVivyStore.getState().draftRequestId).toBe('stable');
    await key('Enter');
    expect(onSteer).toHaveBeenCalledTimes(2);
    expect(onSteer.mock.calls[1]).toEqual(['edited', 'plan', turn.attachments, 'high', turn]);
    expect(input().value).toBe('');
  });

  it('preserves an existing user draft without consuming a queued turn', async () => {
    await render();
    await type('my draft');
    await key('ArrowUp', { altKey: true });
    expect(onDequeue).not.toHaveBeenCalled();
    expect(input().value).toBe('my draft');
  });

  it('retains a recalled DTO separately when a user draft appears in flight', async () => {
    const turn: api.QueuedTurn = { id: 'q1', session_id: 's1', track: 'follow_up', text: 'returned', created_at: 1, thinking: 'high' };
    let resolve!: (turn: api.QueuedTurn) => void;
    onDequeue.mockImplementation(() => new Promise<api.QueuedTurn>((done) => { resolve = done; }));
    await render();
    await key('ArrowUp', { altKey: true });
    await type('my newer draft');
    await act(async () => resolve(turn));
    expect(input().value).toBe('my newer draft');
    expect(useVivyStore.getState().queueRecoveryTurns.s1).toEqual([turn]);
    expect(container.textContent).toContain('1 returned turn(s): recall');
  });

  it('shows captured context and removes it explicitly before resend', async () => {
    const turn: api.QueuedTurn = { id:'q1', session_id:'s1', track:'follow_up', text:'restore', created_at:1,
      file_contexts:[{path:'a.go',name:'a.go',size:8,content:'c25hcHNob3Q='}],context_paths:['a.go'] };
    onDequeue.mockResolvedValue(turn);
    await render(false);
    await key('ArrowUp',{altKey:true});
    const remove = container.querySelector<HTMLButtonElement>('[aria-label="Remove captured context a.go"]');
    expect(remove).not.toBeNull();
    await act(async () => remove!.click());
    await key('Enter');
    expect(onSend.mock.calls[0][4]).toMatchObject({file_contexts:[],context_paths:undefined});
  });

  it('keeps restored attachments when React replays mount effects', async () => {
    const turn: api.QueuedTurn={id:'q1',session_id:'s1',track:'follow_up',text:'restore',created_at:1,attachments:[{name:'x.png',mime_type:'image/png',data:'aW1hZ2U='}]};
    await act(async () => root.render(<StrictMode><ChatInput onSend={onSend} draftPreset={{text:turn.text,seq:1,turn}} /></StrictMode>));
    expect(container.querySelector('img')?.getAttribute('src')).toBe('data:image/png;base64,aW1hZ2U=');
  });

});
