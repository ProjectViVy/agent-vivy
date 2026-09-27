// @vitest-environment happy-dom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';
import type { ReferencePreview } from '@/lib/api';
import { ContextReferenceChip } from './ContextReferenceChip';

function previewFixture(overrides: Partial<ReferencePreview> = {}): ReferencePreview {
  return {
    selection: { source_session_id: 'src-1', refs: [{ session_id: 'src-1', kind: 'message', message_id: 'm1', created_at: 1 }] },
    items: [
      { ref: { session_id: 'src-1', kind: 'message', message_id: 'm1', created_at: 1 }, author: 'user', text: 'fix the parser', redacted: false, truncated: false },
      { ref: { session_id: 'src-1', kind: 'tool_result', event_seq: 4, created_at: 2 }, author: 'tool', text: 'ok', redacted: false, truncated: false },
    ],
    digest: 'sha256:abc',
    captured_at: 1700000000,
    byte_count: 17,
    source_status: 'ok',
    ...overrides,
  };
}

describe('ContextReferenceChip', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    (globalThis as Record<string, unknown>).IS_REACT_ACT_ENVIRONMENT = true;
    hydrateLocale('en');
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    vi.restoreAllMocks();
  });

  it('renders source session, record count, kinds and capture time', async () => {
    await act(async () => root.render(<ContextReferenceChip preview={previewFixture()} />));
    const chip = container.querySelector('[data-testid="context-reference-chip"]')!;
    expect(chip.textContent).toContain('History: src-1');
    expect(chip.textContent).toContain('2');
    expect(chip.textContent).toContain('message/tool_result');
  });

  it('exposes view and remove callbacks with accessible labels', async () => {
    const onView = vi.fn();
    const onRemove = vi.fn();
    await act(async () => root.render(<ContextReferenceChip preview={previewFixture()} onView={onView} onRemove={onRemove} />));

    const view = container.querySelector<HTMLButtonElement>('button[aria-label="View reference"]')!;
    const remove = container.querySelector<HTMLButtonElement>('button[aria-label="Remove reference"]')!;
    await act(async () => view.click());
    expect(onView).toHaveBeenCalledTimes(1);
    await act(async () => remove.click());
    expect(onRemove).toHaveBeenCalledTimes(1);
  });
});
