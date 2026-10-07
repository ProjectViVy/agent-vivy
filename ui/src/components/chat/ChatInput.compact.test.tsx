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
vi.mock('@/lib/api', async (original) => ({
  ...await original<typeof api>(),
  listSkills: vi.fn(async () => ({ skills: [] })),
  compactSession: vi.fn(),
  getSessionContext: vi.fn(async () => null),
}));
vi.mock('@/lib/run-subscription', () => ({ subscribeRun: vi.fn(() => ({ close: vi.fn(), lastSeq: () => 0 })) }));

const initial = useVivyStore.getState();
const contextWithCompaction: api.SessionContext = {
  session_id: 's1', total_messages: 4, feed_messages: 4, feed_bytes: 2048,
  feed_tokens: 512, limit_bytes: 1_000_000, model_limit_tokens: 8192,
  thinking_supported: false, compaction_enabled: true, trigger_tokens: 6000,
  would_compact: false, has_compaction_summary: false, last_compaction: null,
} as api.SessionContext;

describe('chat-level compact control (VCP-D3)', () => {
  let container: HTMLDivElement;
  let root: Root;
  let onSend: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    useVivyStore.setState({ ...initial, activeSessionId: 's1', workPhase: 'ready',
      sessions: [{ id: 's1', title: 'Session', created_at: 1, permission_preset: 'smart' }] });
    onSend = vi.fn(async () => undefined);
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

  async function render(running = false, context: api.SessionContext | null = contextWithCompaction) {
    await act(async () => root.render(
      <ChatInput onSend={onSend} running={running} context={context} />,
    ));
  }
  const compactButton = () =>
    Array.from(container.querySelectorAll('button')).find((b) => b.getAttribute('aria-label') === 'Compact context');
  async function click(el: Element) {
    await act(async () => { el.dispatchEvent(new MouseEvent('click', { bubbles: true })); });
  }

  it('hides the control when compaction is not enabled', async () => {
    await render(false, { ...contextWithCompaction, compaction_enabled: false });
    expect(compactButton()).toBeUndefined();
  });

  it('compacts with instructions from the popover', async () => {
    vi.mocked(api.compactSession).mockResolvedValue({ before_tokens: 1000, after_tokens: 200, folded_messages: 3, skipped: false });
    await render(false);
    const trigger = compactButton()!;
    expect(trigger).toBeDefined();
    await click(trigger);
    const instructions = document.body.querySelector<HTMLInputElement>('input[placeholder="Instructions (optional)"]')!;
    expect(instructions).toBeTruthy();
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(instructions, 'keep auth decisions');
      instructions.dispatchEvent(new Event('input', { bubbles: true }));
    });
    const compactNow = Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent === 'Compact now')!;
    await click(compactNow);
    expect(api.compactSession).toHaveBeenCalledWith('s1', 'keep auth decisions');
  });

  it('omits instructions when the field is blank', async () => {
    vi.mocked(api.compactSession).mockResolvedValue({ before_tokens: 1000, after_tokens: 200, folded_messages: 3, skipped: false });
    await render(false);
    await click(compactButton()!);
    const compactNow = Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent === 'Compact now')!;
    await click(compactNow);
    expect(api.compactSession).toHaveBeenCalledWith('s1', undefined);
  });

  it('stays clickable while a run is active and surfaces the busy error', async () => {
    vi.mocked(api.compactSession).mockRejectedValue(new Error('session is busy'));
    await render(true);
    const trigger = compactButton()!;
    expect(trigger.hasAttribute('disabled')).toBe(false);
    await click(trigger);
    const compactNow = Array.from(document.body.querySelectorAll('button')).find((b) => b.textContent === 'Compact now')!;
    await click(compactNow);
    expect(api.compactSession).toHaveBeenCalledWith('s1', undefined);
    expect(container.textContent).toContain('session is busy');
  });
});
