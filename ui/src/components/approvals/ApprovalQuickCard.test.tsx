// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale, resetLocaleForTests } from '@/i18n';
import type { ReviewItem } from '@/lib/api';
import { ApprovalQuickCard, clampOffset, pickPendingReview } from './ApprovalQuickCard';

const review = (overrides: Partial<ReviewItem> = {}): ReviewItem => ({
  id: 'review-1', kind: 'approval', status: 'pending', session_id: 's1', run_id: 'run-1',
  tool_name: 'bash', created_at: 10, expires_at: 1000, preview: 'rm -rf build', ...overrides,
});

describe('approval quick card selection', () => {
  it('prefers the active session oldest pending review', () => {
    const picked = pickPendingReview([
      review({ id: 'other', session_id: 's2', created_at: 1 }),
      review({ id: 'newer', session_id: 's1', created_at: 20 }),
      review({ id: 'oldest', session_id: 's1', created_at: 5 }),
      review({ id: 'decided', session_id: 's1', status: 'approved', created_at: 2 }),
    ], 's1');
    expect(picked?.id).toBe('oldest');
  });

  it('falls back to the oldest pending review of any session', () => {
    expect(pickPendingReview([review({ id: 'a', session_id: 's2', created_at: 7 }), review({ id: 'b', session_id: 's3', created_at: 3 })], 's1')?.id).toBe('b');
    expect(pickPendingReview([review({ id: 'a', session_id: 's2', created_at: 7 })], null)?.id).toBe('a');
    expect(pickPendingReview([review({ status: 'denied' })], 's1')).toBeNull();
  });
});

describe('approval quick card placement', () => {
  it('keeps a dragged card inside the viewport', () => {
    const natural = { left: 700, right: 1000, top: 300, bottom: 500 };
    const viewport = { width: 1024, height: 768 };
    expect(clampOffset(natural, viewport, { x: 400, y: 400 })).toEqual({ x: 16, y: 260 });
    expect(clampOffset(natural, viewport, { x: -2000, y: -2000 })).toEqual({ x: -692, y: -292 });
    expect(clampOffset(natural, viewport, { x: 0, y: 0 })).toEqual({ x: 0, y: 0 });
  });
});

describe('ApprovalQuickCard', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    resetLocaleForTests();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  function button(label: string): HTMLButtonElement | undefined {
    return [...container.querySelectorAll('button')].find((node) => node.textContent === label);
  }
  async function click(element: Element | undefined) {
    expect(element).toBeDefined();
    await act(async () => { element!.dispatchEvent(new MouseEvent('click', { bubbles: true })); });
  }
  async function render(props: Partial<Parameters<typeof ApprovalQuickCard>[0]> = {}) {
    await act(async () => root.render(
      <ApprovalQuickCard
        review={review()}
        pendingCount={1}
        busy={false}
        error={null}
        offset={{ x: 0, y: 0 }}
        onOffsetChange={() => undefined}
        onRespond={() => undefined}
        onOpenCenter={() => undefined}
        onClose={() => undefined}
        {...props}
      />,
    ));
  }

  it('shows the pending command and decides it inline', async () => {
    const onRespond = vi.fn(async () => undefined);
    await render({ onRespond });
    expect(container.textContent).toContain('Command');
    expect(container.textContent).toContain('rm -rf build');
    await click(button('Approve'));
    expect(onRespond).toHaveBeenCalledWith('approve');
    await click(button('Deny'));
    expect(onRespond).toHaveBeenCalledWith('deny');
  });

  it('disables both decisions while the response is in flight', async () => {
    await render({ busy: true });
    expect(button('Processing…')?.hasAttribute('disabled')).toBe(true);
    expect(button('Deny')?.hasAttribute('disabled')).toBe(true);
  });

  it('stays usable when a decision fails and reports the store error', async () => {
    const onRespond = vi.fn(async () => { throw new Error('stale review'); });
    await render({ onRespond, error: 'stale review' });
    await click(button('Approve'));
    expect(onRespond).toHaveBeenCalledWith('approve');
    expect(button('Approve')?.hasAttribute('disabled')).toBe(false);
    expect(container.textContent).toContain('stale review');
  });

  it('routes questions to the approval center instead of blind decisions', async () => {
    const onOpenCenter = vi.fn();
    await render({ review: review({ kind: 'question', prompt: 'Which file?' }), onOpenCenter });
    expect(container.textContent).toContain('Which file?');
    expect(container.textContent).toContain('Answer this question in the approval center.');
    expect(button('Approve')).toBeUndefined();
    expect(button('Deny')).toBeUndefined();
    await click(button('Approval center'));
    expect(onOpenCenter).toHaveBeenCalledTimes(1);
  });

  it('counts the remaining pending items and closes on request', async () => {
    const onClose = vi.fn();
    await render({ pendingCount: 3, onClose });
    expect(container.textContent).toContain('2 more pending');
    await click(container.querySelector('button[aria-label="Close"]') ?? undefined);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('shows the empty state when nothing is pending', async () => {
    await render({ review: null, pendingCount: 0 });
    expect(container.textContent).toContain('Nothing is waiting for your decision');
    expect(button('Approve')).toBeUndefined();
    expect(button('Approval center')).toBeDefined();
  });

  it('names the session when the pending review belongs to another one', async () => {
    await render({ review: review({ session_id: 's2', session_title: 'Deploy script' }), activeSessionId: 's1' });
    expect(container.textContent).toContain('From another session: Deploy script');
    await render({ review: review({ session_id: 's2' }), activeSessionId: 's1' });
    expect(container.textContent).toContain('From another session: s2');
    await render({ review: review({ session_id: 's1', session_title: 'Mine' }), activeSessionId: 's1' });
    expect(container.textContent).not.toContain('From another session');
  });
});
