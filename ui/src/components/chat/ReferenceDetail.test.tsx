// @vitest-environment happy-dom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';
import * as api from '@/lib/api';
import { resetStoreForTests, useVivyStore } from '@/lib/store';
import { ReferenceDetail } from './ReferenceDetail';

const reference = (overrides: Partial<api.ContextReference> = {}): api.ContextReference => ({
  id: 'ref-1',
  destination_session_id: 'dest-1',
  destination_run_id: 'run-1',
  source_session_id: 'src-1',
  source_workspace: '/ws',
  captured_at: 1700000000,
  items: [
    { ref: { session_id: 'src-1', kind: 'message', message_id: 'm1', created_at: 1 }, author: 'user', text: 'fix the parser', redacted: false, truncated: false },
    { ref: { session_id: 'src-1', kind: 'tool_result', event_seq: 9, created_at: 2 }, author: 'tool', text: 'patched 3 files', redacted: false, truncated: false },
  ],
  digest: 'abc123def',
  origin: 'user_selection',
  ...overrides,
});

const view = (source_status: string, feed_status: string): api.ReferenceView => ({
  reference: reference(),
  source_status,
  feed_status,
});

const historyPage = (items: api.HistoryItem[], overrides: Partial<api.HistoryPage> = {}) =>
  ({ status: 'ok', items, next_cursor: '', truncated: false, redacted: false, warnings: [], ...overrides }) as api.HistoryPage;

function byText(selector: string, text: string): HTMLElement {
  const match = [...document.querySelectorAll<HTMLElement>(selector)].find((node) => node.textContent?.includes(text));
  expect(match, `${selector}: ${text}`).toBeDefined();
  return match!;
}

describe('ReferenceDetail', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    (globalThis as Record<string, unknown>).IS_REACT_ACT_ENVIRONMENT = true;
    hydrateLocale('en');
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
    resetStoreForTests();
    useVivyStore.setState(useVivyStore.getInitialState(), true);
    useVivyStore.setState({ activeSessionId: 'dest-1' });
    vi.spyOn(api, 'referenceGet').mockResolvedValue(view('ok', 'included'));
    vi.spyOn(api, 'historyRead').mockResolvedValue(historyPage([]));
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    vi.restoreAllMocks();
    document.body.innerHTML = '';
  });

  it('shows the saved excerpt and live source/feed status from reference/get', async () => {
    await act(async () => {
      root.render(<ReferenceDetail reference={reference()} />);
    });
    // 折叠行即暴露源会话标记；展开后拉取 reference/get 的活状态。
    byText('[data-reference-card="ref-1"]', 'src-1');
    const toggle = document.querySelector<HTMLElement>('[data-reference-card="ref-1"] button[aria-expanded]')!;
    await act(async () => { toggle.dispatchEvent(new MouseEvent('click', { bubbles: true })); });
    expect(api.referenceGet).toHaveBeenCalledWith('dest-1', 'ref-1');
    byText('[data-reference-card="ref-1"]', 'fix the parser');
    byText('[data-reference-card="ref-1"]', 'included');
    byText('[data-reference-card="ref-1"]', 'ok');
  });

  it('keeps the saved copy readable and disables source jump when the source is unavailable', async () => {
    vi.spyOn(api, 'referenceGet').mockResolvedValue(view('unavailable', 'elided_budget'));
    await act(async () => {
      root.render(<ReferenceDetail reference={reference()} />);
    });
    const toggle = document.querySelector<HTMLElement>('[data-reference-card="ref-1"] button[aria-expanded]')!;
    await act(async () => { toggle.dispatchEvent(new MouseEvent('click', { bubbles: true })); });
    byText('[data-reference-card="ref-1"]', 'fix the parser');
    byText('[data-reference-card="ref-1"]', 'unavailable');
    byText('[data-reference-card="ref-1"]', 'elided (budget)');
    const jump = document.querySelector<HTMLButtonElement>('[data-reference-card="ref-1"] [data-source-jump]')!;
    expect(jump.disabled).toBe(true);
    expect(api.historyRead).not.toHaveBeenCalled();
  });

  it('reads the source session through history/read on the source jump without switching sessions', async () => {
    vi.spyOn(api, 'historyRead').mockResolvedValue(historyPage([
      { ref: { session_id: 'src-1', kind: 'message', message_id: 'm1', created_at: 1 }, author: 'user', text: 'live source row', redacted: false, truncated: false },
    ]));
    await act(async () => {
      root.render(<ReferenceDetail reference={reference()} />);
    });
    const toggle = document.querySelector<HTMLElement>('[data-reference-card="ref-1"] button[aria-expanded]')!;
    await act(async () => { toggle.dispatchEvent(new MouseEvent('click', { bubbles: true })); });
    const jump = document.querySelector<HTMLButtonElement>('[data-reference-card="ref-1"] [data-source-jump]')!;
    expect(jump.disabled).toBe(false);
    await act(async () => { jump.dispatchEvent(new MouseEvent('click', { bubbles: true })); });
    expect(api.historyRead).toHaveBeenCalledWith('dest-1', {
      selection: {
        source_session_id: 'src-1',
        refs: [
          { session_id: 'src-1', kind: 'message', message_id: 'm1', created_at: 1 },
          { session_id: 'src-1', kind: 'tool_result', event_seq: 9, created_at: 2 },
        ],
      },
    });
    byText('[data-reference-card="ref-1"]', 'live source row');
    expect(useVivyStore.getState().activeSessionId).toBe('dest-1');
  });

  it('collapses on Escape and returns focus to the card toggle', async () => {
    await act(async () => {
      root.render(<ReferenceDetail reference={reference()} />);
    });
    const card = document.querySelector<HTMLElement>('[data-reference-card="ref-1"]')!;
    const toggle = card.querySelector<HTMLElement>('button[aria-expanded]')!;
    await act(async () => { toggle.dispatchEvent(new MouseEvent('click', { bubbles: true })); });
    expect(toggle.getAttribute('aria-expanded')).toBe('true');
    await act(async () => { card.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); });
    expect(toggle.getAttribute('aria-expanded')).toBe('false');
    expect(document.activeElement).toBe(toggle);
  });
});
