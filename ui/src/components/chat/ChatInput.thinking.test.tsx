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
  getSessionContext: vi.fn(async () => null),
}));
vi.mock('@/lib/run-subscription', () => ({ subscribeRun: vi.fn(() => ({ close: vi.fn(), lastSeq: () => 0 })) }));

const initial = useVivyStore.getState();

function sessionContext(overrides: Partial<api.SessionContext> = {}): api.SessionContext {
  return {
    session_id: 's1', total_messages: 2, feed_messages: 2, feed_bytes: 512, feed_tokens: 128,
    limit_bytes: 1_000_000, model_limit_tokens: 128000, thinking_supported: false,
    thinking_levels: [], thinking_default: '',
    compaction_enabled: false, trigger_tokens: 0, would_compact: false,
    has_compaction_summary: false, last_compaction: null,
    ...overrides,
  } as api.SessionContext;
}

describe('composer thinking control', () => {
  let container: HTMLDivElement;
  let root: Root;
  let onSend: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    useVivyStore.setState({
      ...initial, activeSessionId: 's1', workPhase: 'ready',
      sessions: [{ id: 's1', title: 'Session', created_at: 1, permission_preset: 'smart' }],
    });
    onSend = vi.fn(async () => undefined);
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    document.querySelectorAll('[role="menu"]').forEach((node) => node.remove());
    useVivyStore.setState(initial);
    resetLocaleForTests();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  async function render(context: api.SessionContext) {
    await act(async () => root.render(<ChatInput onSend={onSend} context={context} />));
  }

  function trigger(): HTMLButtonElement {
    const match = [...container.querySelectorAll<HTMLButtonElement>('button')]
      .find((node) => node.getAttribute('aria-label') === 'Thinking mode');
    expect(match, 'thinking trigger').toBeDefined();
    return match!;
  }

  function menuItems(): HTMLButtonElement[] {
    return [...document.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')];
  }

  // Radix 的 DropdownMenuTrigger 在 pointerdown 上开菜单（happy-dom 里没有真实
  // 指针事件，用同形 MouseEvent 触发已足够）。
  async function openMenu() {
    await act(async () => {
      trigger().dispatchEvent(new MouseEvent('pointerdown', { bubbles: true, button: 0 }));
    });
  }

  async function sendMessage(text = 'hello') {
    const textarea = container.querySelector('textarea')!;
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(textarea, text);
      textarea.dispatchEvent(new Event('input', { bubbles: true }));
    });
    const send = [...container.querySelectorAll<HTMLButtonElement>('button')]
      .find((node) => node.getAttribute('aria-label') === 'Send');
    await act(async () => send!.click());
  }

  it('keeps a greyed trigger when the model declares no thinking support', async () => {
    await render(sessionContext());

    const button = trigger();
    expect(button.hasAttribute('disabled')).toBe(true);
    expect(button.className).toContain('text-muted-foreground');
    expect(button.getAttribute('title')).toBe('The active model does not declare extended-thinking support');
    // 不支持时不挂下拉触发器：真实点击也不会弹菜单。
    expect(button.hasAttribute('aria-haspopup')).toBe(false);

    await act(async () => button.click());
    expect(document.querySelector('[role="menu"]')).toBeNull();
  });

  it('offers the declared levels and omits the ones the model does not accept', async () => {
    await render(sessionContext({ thinking_supported: true, thinking_levels: ['low', 'high'], thinking_default: 'high' }));

    const button = trigger();
    expect(button.hasAttribute('disabled')).toBe(false);
    expect(button.getAttribute('aria-haspopup')).toBe('menu');
    await openMenu();

    const labels = menuItems().map((node) => node.textContent?.trim());
    expect(labels).toEqual(['Follow default', 'Auto', 'low', 'high', 'On', 'Off']);
  });

  it('falls back to the seven-level surface when a thinking model declares none', async () => {
    await render(sessionContext({ thinking_supported: true, thinking_levels: [] }));

    await openMenu();
    const labels = menuItems().map((node) => node.textContent?.trim());
    expect(labels).toEqual(['Follow default', 'Auto', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max', 'On', 'Off']);
  });

  it('sends the picked level as the per-turn override and tints the trigger by strength', async () => {
    await render(sessionContext({ thinking_supported: true, thinking_levels: ['low', 'high'] }));

    await openMenu();
    const high = menuItems().find((node) => node.textContent?.trim() === 'high')!;
    await act(async () => high.click());

    expect(trigger().className).toContain('text-primary');
    await sendMessage();
    expect(onSend).toHaveBeenCalledTimes(1);
    expect(onSend.mock.calls[0][3]).toBe('high');
  });

  it('sends no thinking field when the turn rides the saved default', async () => {
    await render(sessionContext({ thinking_supported: true, thinking_levels: ['low', 'high'] }));

    await sendMessage();
    expect(onSend).toHaveBeenCalledTimes(1);
    expect(onSend.mock.calls[0][3]).toBeUndefined();
  });

  it('never sends a level the active model does not declare', async () => {
    await render(sessionContext());

    await sendMessage();
    expect(onSend).toHaveBeenCalledTimes(1);
    expect(onSend.mock.calls[0][3]).toBeUndefined();
  });
  it('drops a picked level once the model no longer declares it, label and payload alike', async () => {
    await render(sessionContext({ thinking_supported: true, thinking_levels: ['low', 'high', 'max'] }));
    await openMenu();
    await act(async () => menuItems().find((node) => node.textContent?.trim() === 'max')!.click());
    expect(trigger().getAttribute('title')).toContain('max');

    await render(sessionContext({ thinking_supported: true, thinking_levels: ['low', 'high'] }));
    expect(trigger().getAttribute('title')).toContain('Follow default');
    await sendMessage();
    expect(onSend).toHaveBeenCalledTimes(1);
    expect(onSend.mock.calls[0][3]).toBeUndefined();
  });
});
