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
  listReviews: vi.fn(async () => ({ reviews: [] })),
  respondReview: vi.fn(async () => ({ review_id: 'review-1', status: 'approved' })),
  getSessionContext: vi.fn(async () => null),
}));
vi.mock('@/lib/run-subscription', () => ({ subscribeRun: vi.fn(() => ({ close: vi.fn(), lastSeq: () => 0 })) }));

const initial = useVivyStore.getState();
const review = (overrides: Partial<api.ReviewItem> = {}): api.ReviewItem => ({
  id: 'review-1', kind: 'approval', status: 'pending', session_id: 's1', run_id: 'run-1',
  tool_name: 'bash', created_at: 10, expires_at: 1000, preview: 'rm -rf build', ...overrides,
});

describe('composer approval quick card lifecycle', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    useVivyStore.setState({ ...initial, activeSessionId: 's1', workPhase: 'ready',
      sessions: [{ id: 's1', title: 'Session', created_at: 1, permission_preset: 'smart' }],
      reviews: [], reviewsPhase: 'empty' });
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

  async function render() {
    await act(async () => root.render(<ChatInput onSend={vi.fn(async () => undefined)} />));
  }

  const card = () => container.querySelector('#approval-quick-card');

  async function click(label: string) {
    const button = [...container.querySelectorAll('button')].find((node) => node.textContent === label);
    expect(button).toBeDefined();
    await act(async () => { button!.dispatchEvent(new MouseEvent('click', { bubbles: true })); });
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  }

  async function clickClose() {
    const button = container.querySelector('button[aria-label="Close"]');
    expect(button).not.toBeNull();
    await act(async () => { button!.dispatchEvent(new MouseEvent('click', { bubbles: true })); });
  }

  it('opens automatically for a pending approval', async () => {
    useVivyStore.setState({ reviews: [review()] });
    await render();
    expect(card()).not.toBeNull();
    expect(card()!.textContent).toContain('rm -rf build');
  });

  it('opens when a pending approval arrives after mount', async () => {
    await render();
    expect(card()).toBeNull();
    await act(async () => { useVivyStore.setState({ reviews: [review()] }); });
    expect(card()).not.toBeNull();
  });

  it('closes after the approval succeeds', async () => {
    useVivyStore.setState({ reviews: [review()] });
    await render();
    await click('Approve');
    expect(api.respondReview).toHaveBeenCalledWith('review-1', { action: 'approve' });
    expect(card()).toBeNull();
  });

  it('stays open and shows the error when the decision fails', async () => {
    vi.mocked(api.respondReview).mockRejectedValue(new Error('review is stale'));
    useVivyStore.setState({ reviews: [review()] });
    await render();
    await click('Approve');
    expect(card()).not.toBeNull();
    expect(card()!.textContent).toContain('review is stale');
  });

  it('does not force the card back after the user closes it', async () => {
    useVivyStore.setState({ reviews: [review()] });
    await render();
    await clickClose();
    expect(card()).toBeNull();
    await act(async () => { useVivyStore.setState({ reviews: [review({ id: 'review-2', created_at: 20 })] }); });
    expect(card()).toBeNull();
  });
});
