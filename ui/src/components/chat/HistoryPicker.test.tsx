// @vitest-environment happy-dom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';
import * as api from '@/lib/api';
import { HistoryPicker } from './HistoryPicker';

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

const sessionsPage = (sessions: api.HistorySession[], next_cursor = '') =>
  ({ sessions, next_cursor }) as Awaited<ReturnType<typeof api.historySessions>>;

const historyPage = (items: api.HistoryItem[], overrides: Partial<api.HistoryPage> = {}) =>
  ({ status: 'ok', items, next_cursor: '', truncated: false, redacted: false, warnings: [], ...overrides }) as api.HistoryPage;

const item = (kind: api.SourceRef['kind'], text: string, ref: Partial<api.SourceRef> = {}): api.HistoryItem => ({
  ref: { session_id: 'src-1', kind, message_id: `m-${text}`, created_at: 1, ...ref },
  author: 'user',
  text,
  redacted: false,
  truncated: false,
});

const preview = (status = 'ok'): api.ReferencePreview => ({
  selection: { source_session_id: 'src-1', refs: [{ session_id: 'src-1', kind: 'message', message_id: 'm-fix', created_at: 1 }] },
  items: [item('message', 'fix')],
  digest: 'd1',
  captured_at: 1700000000,
  byte_count: 3,
  source_status: status,
});

function byText(selector: string, text: string): HTMLElement {
  const match = [...document.querySelectorAll<HTMLElement>(selector)].find((node) => node.textContent?.includes(text));
  expect(match, `${selector}: ${text}`).toBeDefined();
  return match!;
}

describe('HistoryPicker', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    (globalThis as Record<string, unknown>).IS_REACT_ACT_ENVIRONMENT = true;
    hydrateLocale('en');
    vi.spyOn(window, 'matchMedia').mockImplementation((query: string) => ({
      matches: false, media: query, onchange: null,
      addEventListener: () => undefined, removeEventListener: () => undefined,
      addListener: () => undefined, removeListener: () => undefined,
      dispatchEvent: () => false,
    }) as MediaQueryList);
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
    vi.spyOn(api, 'historySessions').mockResolvedValue(sessionsPage([
      { id: 'src-1', title: 'Fix session', workspace_label: 'agent-vivy', updated_at: 1700000000 },
    ]));
    vi.spyOn(api, 'historySearch').mockResolvedValue(historyPage([item('message', 'fix')]));
    vi.spyOn(api, 'previewReference').mockResolvedValue(preview());
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    document.querySelectorAll('[data-radix-portal], [role="dialog"]').forEach((node) => node.remove());
    vi.restoreAllMocks();
  });

  async function open(onAttach = vi.fn()) {
    await act(async () => root.render(
      <HistoryPicker destinationSessionId="dst-1" open onOpenChange={vi.fn()} onAttach={onAttach} />,
    ));
    await act(async () => { await Promise.resolve(); });
    return onAttach;
  }

  it('lists session metadata only on open — no transcript reads', async () => {
    await open();
    expect(api.historySessions).toHaveBeenCalledTimes(1);
    expect(api.historySearch).not.toHaveBeenCalled();
    expect(byText('button', 'Fix session')).toBeDefined();
    expect(byText('button', 'Attach')).toHaveProperty('disabled', true);
  });

  it('searches records only on submit and attaches the previewed selection', async () => {
    const onAttach = await open();
    await act(async () => byText('button', 'Fix session').click());
    await act(async () => { await Promise.resolve(); });
    expect(api.historySearch).toHaveBeenCalledWith('dst-1', expect.objectContaining({ session_ids: ['src-1'], kinds: ['message'] }));

    await act(async () => document.querySelector<HTMLElement>('button[aria-label="Select user message"]')!.click()); // radix checkbox is a button
    expect(byText('button', 'Attach')).toHaveProperty('disabled', true); // preview first

    await act(async () => byText('button', 'Preview').click());
    await act(async () => { await Promise.resolve(); });
    expect(api.previewReference).toHaveBeenCalledWith('dst-1', expect.objectContaining({ source_session_id: 'src-1' }));

    const attach = byText('button', 'Attach') as HTMLButtonElement;
    expect(attach.disabled).toBe(false);
    await act(async () => attach.click());
    expect(onAttach).toHaveBeenCalledWith(
      preview(),
      { selection: preview().selection, expected_digest: 'd1' },
      false, // further-reading checkbox stays unchecked by default
    );
  });

  it('passes the explicit further-reading opt-in through onAttach', async () => {
    const onAttach = await open();
    await act(async () => byText('button', 'Fix session').click());
    await act(async () => { await Promise.resolve(); });
    await act(async () => document.querySelector<HTMLElement>('button[aria-label="Select user message"]')!.click());
    await act(async () => document.querySelector<HTMLElement>('button[aria-label="Allow further reading of the source session"]')!.click());
    await act(async () => byText('button', 'Preview').click());
    await act(async () => { await Promise.resolve(); });
    await act(async () => (byText('button', 'Attach') as HTMLButtonElement).click());
    expect(onAttach.mock.calls[0][2]).toBe(true);
  });

  it('keeps an over-limit selection selected but blocks attach with a narrowing hint', async () => {
    vi.spyOn(api, 'previewReference').mockResolvedValue(preview('too_large'));
    await open();
    await act(async () => byText('button', 'Fix session').click());
    await act(async () => { await Promise.resolve(); });
    const checkbox = document.querySelector<HTMLElement>('button[aria-label="Select user message"]')!;
    await act(async () => checkbox.click());
    await act(async () => byText('button', 'Preview').click());
    await act(async () => { await Promise.resolve(); });

    expect((byText('button', 'Attach') as HTMLButtonElement).disabled).toBe(true);
    expect(document.body.textContent).toContain('narrow it before attaching');
    expect(checkbox.dataset.state).toBe('checked'); // selection retained, never silently dropped
  });

  it('discards a record response that arrives after switching source sessions', async () => {
    vi.spyOn(api, 'historySessions').mockResolvedValue(sessionsPage([
      { id: 'src-1', title: 'Fix session', workspace_label: 'a', updated_at: 1 },
      { id: 'src-2', title: 'Other session', workspace_label: 'b', updated_at: 2 },
    ]));
    const stale = deferred<api.HistoryPage>();
    vi.spyOn(api, 'historySearch')
      .mockReturnValueOnce(stale.promise)
      .mockResolvedValue(historyPage([item('message', 'fresh', { session_id: 'src-2' })]));
    await open();
    await act(async () => byText('button', 'Fix session').click());
    await act(async () => byText('button', 'Other session').click());
    await act(async () => { await Promise.resolve(); });
    await act(async () => stale.resolve(historyPage([item('message', 'stale record')])));
    await act(async () => { await Promise.resolve(); });
    expect(document.body.textContent).toContain('fresh');
    expect(document.body.textContent).not.toContain('stale record');
  });

  it('closes without attaching when cancelled', async () => {
    const onOpenChange = vi.fn();
    await act(async () => root.render(
      <HistoryPicker destinationSessionId="dst-1" open onOpenChange={onOpenChange} onAttach={vi.fn()} />,
    ));
    await act(async () => { await Promise.resolve(); });
    await act(async () => byText('button', 'Cancel').click());
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(api.historySearch).not.toHaveBeenCalled();
  });

  it('renders the mobile sheet layout under 768px', async () => {
    Object.defineProperty(window, 'innerWidth', { writable: true, configurable: true, value: 500 });
    await open();
    // mobile shows the session list full-width first (Back appears after choosing)
    expect(document.querySelector('[role="dialog"]')).toBeTruthy();
    await act(async () => byText('button', 'Fix session').click());
    await act(async () => { await Promise.resolve(); });
    expect(document.querySelector('button[aria-label="Back"]')).toBeTruthy();
  });
});
