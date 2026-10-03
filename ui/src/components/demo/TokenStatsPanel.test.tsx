// @vitest-environment happy-dom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale } from '@/i18n';
import * as api from '@/lib/api';
import { resetStoreForTests, useVivyStore } from '@/lib/store';
import { TokenStatsPanel } from './TokenStatsPanel';

const coverage = (overrides: Partial<api.UsageCoverage> = {}): api.UsageCoverage => ({
  state: 'complete',
  observed_calls: 2,
  completed_with_usage: 2,
  reported_calls: 2,
  missing_usage_calls: 0,
  partial_usage_calls: 0,
  active_calls: 0,
  legacy_usage_records: 0,
  unknown_buckets: [],
  hidden_retries_observable: false,
  ...overrides,
});

const session = (overrides: Partial<api.TokenSessionUsage> = {}): api.TokenSessionUsage => ({
  id: 'ses_1',
  title: 'Chat',
  model: 'gpt-5',
  request_count: 2,
  total_input: 100,
  total_output: 50,
  total_tokens: 150,
  cost_usd: 0.01,
  cost_known: true,
  coverage: coverage(),
  ...overrides,
});

const snapshot = (overrides: Partial<api.TokenUsageSnapshot> = {}): api.TokenUsageSnapshot => ({
  period: '1d',
  scope: 'chat_runs',
  projection_version: 2,
  coverage: coverage(),
  total: {
    total_input: 100,
    total_output: 50,
    total_tokens: 150,
    total_reasoning: 10,
    total_cached: 5,
    request_count: 2,
    total_cost_usd: 0.01,
    cost_known: true,
  },
  models: [{ model: 'gpt-5', percentage: 100, total_tokens: 150, cost_usd: 0.01, cost_known: true, coverage: coverage() }],
  providers: [],
  timeline: [],
  sessions: [session()],
  ...overrides,
});

const flush = async () => {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
};

describe('TokenStatsPanel', () => {
  let host: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    hydrateLocale('en');
    resetStoreForTests();
    host = document.createElement('div');
    document.body.appendChild(host);
    root = createRoot(host);
  });

  afterEach(() => {
    act(() => root.unmount());
    host.remove();
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it('renders partial coverage counts and hides partial cost', async () => {
    vi.spyOn(api, 'getTokenUsage').mockResolvedValue(
      snapshot({
        coverage: coverage({
          state: 'partial',
          observed_calls: 5,
          reported_calls: 4,
          missing_usage_calls: 1,
          active_calls: 1,
          completed_with_usage: 2,
          partial_usage_calls: 1,
          legacy_usage_records: 1,
        }),
        total: { ...snapshot().total, cost_known: false, request_count: 5 },
      }),
    );
    act(() => root.render(<TokenStatsPanel />));
    await flush();
    expect(host.textContent).toContain('Coverage: partial');
    expect(host.textContent).toContain('observed 5 · reported 4 · missing 1 · active 1');
    expect(host.textContent).toContain('1 legacy record');
    expect(host.textContent).toContain('Usage reports');
    // Total cost card masked; fully-covered model/session rows keep their cost.
    const costCard = [...host.querySelectorAll('.rounded-lg')].find((c) => c.querySelector('p')?.textContent === 'Estimated cost');
    expect(costCard?.textContent).toBe('Estimated cost—');
  });

  it('shows unknown buckets as dashes in detail view', async () => {
    vi.spyOn(api, 'getTokenUsage').mockResolvedValue(
      snapshot({ coverage: coverage({ state: 'partial', unknown_buckets: ['reasoning'], missing_usage_calls: 1 }) }),
    );
    act(() => root.render(<TokenStatsPanel />));
    await flush();
    const detailButton = [...host.querySelectorAll('button')].find((b) => b.textContent?.includes('View detailed'));
    expect(detailButton).toBeTruthy();
    act(() => detailButton!.dispatchEvent(new MouseEvent('click', { bubbles: true })));
    await flush();
    expect(host.textContent).toContain('unknown buckets: reasoning');
    const metrics = [...host.querySelectorAll('.rounded-lg')];
    const reasoning = metrics.find((m) => m.textContent?.includes('Total reasoning tokens'));
    expect(reasoning?.textContent).toContain('—');
  });

  it('refreshes once per burst of authoritative run events', async () => {
    vi.useFakeTimers();
    const spy = vi.spyOn(api, 'getTokenUsage').mockResolvedValue(snapshot());
    act(() => root.render(<TokenStatsPanel />));
    await flush();
    const before = spy.mock.calls.length;
    act(() => {
      useVivyStore.setState({
        currentRun: { id: 'run_1' } as never,
        runEvents: [
          { run_id: 'run_1', seq: 1, type: 'model.delta', created_at: 1, payload_version: 1, payload: {} },
          { run_id: 'run_1', seq: 2, type: 'model.usage', created_at: 1, payload_version: 1, payload: {} },
        ],
      });
    });
    act(() => {
      useVivyStore.setState({
        runEvents: [
          ...useVivyStore.getState().runEvents,
          { run_id: 'run_1', seq: 3, type: 'model.call.finished', created_at: 1, payload_version: 1, payload: {} },
        ],
      });
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(500);
    });
    await flush();
    expect(spy.mock.calls.length).toBe(before + 1);
  });
});
