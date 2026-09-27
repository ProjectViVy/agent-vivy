// @vitest-environment happy-dom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';
import * as api from '@/lib/api';
import { resetStoreForTests, useVivyStore } from '@/lib/store';
import { DeliverableCard } from './DeliverableCard';

const deliverable = (overrides: Partial<api.Deliverable> = {}): api.Deliverable => ({
  id: 'itm_1',
  session_id: 'ses_1',
  run_id: 'run_1',
  workspace_id: 'ws_1',
  path: 'out/报告.txt',
  name: '报告.txt',
  description: 'final report',
  size: 12,
  sha256: 'a'.repeat(64),
  media_type: 'text/plain',
  captured_at: 1700000000,
  origin_tool_call_id: 'call_1',
  ...overrides,
});

const set = (overrides: Partial<api.DeliverySet> = {}): api.DeliverySet => ({
  id: 'dvs_1',
  session_id: 'ses_1',
  run_id: 'run_1',
  tool_call_id: 'call_1',
  created_at: 1700000000,
  title: '',
  items: [deliverable()],
  failures: [],
  status: 'ok',
  ...overrides,
});

function byText(selector: string, text: string): HTMLElement {
  const match = [...document.querySelectorAll<HTMLElement>(selector)].find((node) => node.textContent?.includes(text));
  expect(match, `${selector}: ${text}`).toBeDefined();
  return match!;
}

describe('DeliverableCard', () => {
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
    useVivyStore.setState({ activeSessionId: 'ses_1' });
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    vi.restoreAllMocks();
    document.body.innerHTML = '';
  });

  it('renders one group card per committed set with file count and unchecked state', async () => {
    const first = set({ id: 'dvs_1', items: [deliverable(), deliverable({ id: 'itm_2', name: 'b.bin', path: 'out/b.bin', media_type: 'application/octet-stream' })] });
    const second = set({ id: 'dvs_2', items: [deliverable({ id: 'itm_3', name: 'c.txt', path: 'out/c.txt' })] });
    await act(async () => {
      root.render(
        <>
          <DeliverableCard set={first} />
          <DeliverableCard set={second} />
        </>,
      );
    });
    expect(container.querySelectorAll('section[data-delivery-set]')).toHaveLength(2);
    expect(container.textContent).toContain('2 files delivered');
    // 无正文解析成第二通道：模型文本不进卡片。
    expect(container.textContent).not.toContain('Verification passed');
    byText('[data-delivery-item="itm_1"]', '报告.txt');
    byText('[data-delivery-item="itm_1"]', 'unverified');
    expect(container.querySelector('[data-delivery-item="itm_2"]')?.textContent).not.toContain('Preview');
  });

  it('shows failed entries with their reason and a partial badge', async () => {
    const failing = set({ status: 'partial', failures: [{ path: 'out/secret.txt', reason: 'path outside allowed root' }] });
    await act(async () => root.render(<DeliverableCard set={failing} />));
    byText('section[data-delivery-set]', 'partial');
    byText('section[data-delivery-set]', 'out/secret.txt');
    byText('section[data-delivery-set]', 'path outside allowed root');
  });

  it('verify probes deliverables/read and flips the item to verified', async () => {
    const read = vi.spyOn(api, 'deliverablesRead').mockResolvedValue({
      transfer_id: 'xfr_1', item_id: 'itm_1', digest: 'a'.repeat(64), offset: 0, data_base64: btoa('x'), eof: false, expires_at: 0,
    });
    const close = vi.spyOn(api, 'deliverablesClose').mockResolvedValue(undefined);
    await act(async () => root.render(<DeliverableCard set={set()} />));
    const verify = document.querySelector<HTMLElement>('button[aria-label="Verify 报告.txt"]')!;
    await act(async () => { verify.dispatchEvent(new MouseEvent('click', { bubbles: true })); });
    expect(read).toHaveBeenCalledWith('ses_1', expect.objectContaining({ item_id: 'itm_1', expected_digest: 'a'.repeat(64), offset: 0 }));
    expect(close).toHaveBeenCalledWith('ses_1', 'xfr_1');
    byText('[data-delivery-item="itm_1"]', 'verified');
  });

  it('changed probe renders the explanatory state and keeps a retry affordance', async () => {
    vi.spyOn(api, 'deliverablesRead').mockRejectedValue(new Error('deliverables/read failed: changed: digest mismatch'));
    vi.spyOn(api, 'deliverablesClose').mockResolvedValue(undefined);
    await act(async () => root.render(<DeliverableCard set={set()} />));
    const verify = document.querySelector<HTMLElement>('button[aria-label="Verify 报告.txt"]')!;
    await act(async () => { verify.dispatchEvent(new MouseEvent('click', { bubbles: true })); });
    byText('[data-delivery-item="itm_1"]', 'file changed since delivery');
    // 失败态仍可重试（未静默）。
    expect(document.querySelector('button[aria-label="Verify 报告.txt"]')).toBeDefined();
  });

  it('preview toggles a bounded text excerpt inside the card', async () => {
    vi.spyOn(api, 'deliverablesRead').mockResolvedValue({
      transfer_id: 'xfr_p', item_id: 'itm_1', digest: 'a'.repeat(64), offset: 0, data_base64: btoa('hello preview'), eof: false, expires_at: 0,
    });
    vi.spyOn(api, 'deliverablesClose').mockResolvedValue(undefined);
    await act(async () => root.render(<DeliverableCard set={set()} />));
    const preview = document.querySelector<HTMLElement>('button[aria-label="Preview 报告.txt"]')!;
    await act(async () => { preview.dispatchEvent(new MouseEvent('click', { bubbles: true })); });
    expect(document.querySelector('[data-delivery-preview="itm_1"]')?.textContent).toBe('hello preview');
  });
});
