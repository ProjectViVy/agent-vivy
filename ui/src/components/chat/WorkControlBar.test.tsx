// @vitest-environment happy-dom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { hydrateLocale, resetLocaleForTests } from '@/i18n';
import type { WorkState } from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { WorkControlBar } from './WorkControlBar';

const baseWork: WorkState = {
  session_id: 's1', version: 4, activation: 'disarmed',
  plan: { active: false, review_status: 'none' },
  goal: { id: 'g1', revision: 2, objective: 'Ship the release', phase: 'active', max_rounds: 3, rounds_started: 1 },
};

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => { resolve = done; });
  return { promise, resolve };
}

describe('chat work controls', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    useVivyStore.setState({
      activeSessionId: 's1', sessions: [{ id: 's1', title: 'Session', created_at: 1, permission_preset: 'cautious' }],
      work: baseWork, workPhase: 'ready', workError: null, workBusy: false,
    });
    container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    useVivyStore.setState({ activeSessionId: null, sessions: [], work: null, workError: null, workBusy: false });
    resetLocaleForTests();
    vi.unstubAllGlobals();
  });

  async function render() {
    await act(async () => root.render(<WorkControlBar sessionId="s1" />));
  }

  function button(label: string): HTMLButtonElement | undefined {
    return [...container.querySelectorAll('button')].find((item) => item.textContent?.trim() === label);
  }

  async function enter(input: HTMLInputElement | HTMLTextAreaElement, value: string) {
    await act(async () => {
      const prototype = input instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
      Object.getOwnPropertyDescriptor(prototype, 'value')?.set?.call(input, value);
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
  }

  it('edits the displayed Goal with its opening reference and preserves spent rounds', async () => {
    const editGoal = vi.fn(async () => {
      useVivyStore.setState({ work: { ...baseWork, version: 5, goal: { ...baseWork.goal!, revision: 3, objective: 'Ship safely', max_rounds: 4 } } });
      return {};
    });
    useVivyStore.setState({ editGoal: editGoal as never });
    await render();
    expect(button('Save Goal')).toBeUndefined();
    await act(async () => { button('Edit Goal')?.click(); });
    const objective = container.querySelector<HTMLTextAreaElement>('textarea[aria-label="Goal objective"]')!;
    const rounds = container.querySelector<HTMLInputElement>('input[aria-label="Goal round limit"]')!;
    expect(objective.value).toBe('Ship the release');
    expect(rounds.value).toBe('3');
    await enter(objective, 'Ship safely');
    await enter(rounds, '4');
    await act(async () => { button('Save Goal')?.click(); });
    expect(editGoal).toHaveBeenCalledWith('Ship safely', 4, { id: 'g1', revision: 2 });
    expect(container.textContent).toContain('1 / 4 rounds');
    expect(button('Save Goal')).toBeUndefined();
  });

  it('keeps the edit draft and opening Goal reference after a conflict', async () => {
    const editGoal = vi.fn(async () => {
      useVivyStore.setState({ workError: 'stale_goal', work: { ...baseWork, goal: { ...baseWork.goal!, revision: 3, objective: 'Changed elsewhere' } } });
      throw new Error('stale_goal');
    });
    useVivyStore.setState({ editGoal: editGoal as never });
    await render();
    await act(async () => { button('Edit Goal')?.click(); });
    const objective = container.querySelector<HTMLTextAreaElement>('textarea[aria-label="Goal objective"]')!;
    await enter(objective, 'My revised objective');
    await act(async () => { button('Save Goal')?.click(); });
    expect(objective.value).toBe('My revised objective');
    expect(container.textContent).toContain('stale_goal');
    await act(async () => { button('Save Goal')?.click(); });
    expect(editGoal).toHaveBeenNthCalledWith(2, 'My revised objective', 3, { id: 'g1', revision: 2 });
  });

  it('offers Resume for an active but disarmed Goal and shows its independent permission', async () => {
    await render();
    expect(button('Resume Goal')).toBeDefined();
    expect(button('Pause Goal')).toBeUndefined();
    expect(container.textContent).toContain('Permission: Cautious');
    expect(container.textContent).toContain('Disarmed');
  });

  it('offers Pause when armed and keeps stopping for Plan visible until the Enter Plan request settles', async () => {
    const pending = deferred();
    const enterPlan = vi.fn(() => pending.promise);
    useVivyStore.setState({ work: { ...baseWork, activation: 'armed', current_run_id: 'r1' }, enterPlan: enterPlan as never });
    await render();
    expect(button('Pause Goal')).toBeDefined();
    expect(button('Enter Plan')).toBeDefined();
    await act(async () => { button('Enter Plan')?.click(); });
    expect(enterPlan).toHaveBeenCalledTimes(1);
    expect(container.textContent).toContain('Stopping for Plan');
    expect(button('Enter Plan')?.disabled).toBe(true);
    await act(async () => { pending.resolve(); await pending.promise; });
    expect(container.textContent).not.toContain('Stopping for Plan');
  });

  it('shows the blocked round limit and keeps the admitted count visible', async () => {
    useVivyStore.setState({ work: { ...baseWork, goal: { ...baseWork.goal!, phase: 'blocked', rounds_started: 3, reason: 'round-limit' } } });
    await render();
    expect(container.textContent).toContain('Blocked');
    expect(container.textContent).toContain('3 / 3 rounds');
    expect(container.textContent).toContain('Round limit reached');
    expect(button('Resume Goal')).toBeUndefined();
  });

  it('keeps the submitted plan and its review controls visible after a stale decision', async () => {
    const decidePlan = vi.fn(async () => { useVivyStore.setState({ workError: 'review_stale' }); throw new Error('review_stale'); });
    useVivyStore.setState({
      work: { ...baseWork, goal: undefined, plan: { active: true, review_status: 'pending', submission_id: 'submission-7', markdown: 'Exact submitted plan' } },
      decidePlan: decidePlan as never,
    });
    await render();
    expect(container.textContent).toContain('Exact submitted plan');
    await act(async () => { button('Execute plan once')?.click(); });
    expect(decidePlan).toHaveBeenCalledWith('execute_once', undefined, undefined, undefined, 'submission-7');
    expect(container.textContent).toContain('review_stale');
    expect(button('Execute plan once')).toBeDefined();
    expect(button('Start Goal')).toBeDefined();
  });

  it('does not offer Start Goal over the paused Goal retained during planning', async () => {
    useVivyStore.setState({ work: {
      ...baseWork,
      goal: { ...baseWork.goal!, phase: 'paused' },
      plan: { active: true, review_status: 'pending', submission_id: 'submission-8', markdown: 'Next steps' },
    } });
    await render();
    expect(button('Execute plan once')).toBeDefined();
    expect(button('Start Goal')).toBeUndefined();
    expect(container.textContent).toContain('Paused');
  });

  it('keeps Goal active when every Todo is complete', async () => {
    useVivyStore.setState({ todos: [{
      id: 't1', session_id: 's1', subject: 'Task', description: '', status: 'completed',
      blocks: [], blocked_by: [], position: 0, created_at: 1, updated_at: 2,
    }] });
    await render();
    expect(container.textContent).toContain('Active');
    expect(container.textContent).toContain('1 / 3 rounds');
    expect(container.textContent).not.toContain('Goal: Ship the release · Completed');
  });

  it('offers a retry when the authoritative WorkView cannot load', async () => {
    const loadWork = vi.fn(async () => undefined);
    useVivyStore.setState({ work: null, workPhase: 'error', workError: 'connection lost', loadWork: loadWork as never });
    await render();
    expect(container.textContent).toContain('connection lost');
    await act(async () => { button('Refresh work state')?.click(); });
    expect(loadWork).toHaveBeenCalledWith('s1');
  });

  it('drops revision draft controls when the backend presents a different submission', async () => {
    useVivyStore.setState({ work: { ...baseWork, goal: undefined,
      plan: { active: true, review_status: 'pending', submission_id: 'submission-7', markdown: 'First plan' },
    } });
    await render();
    await act(async () => { button('Request revision')?.click(); });
    expect(button('Send feedback')).toBeDefined();

    await act(async () => { useVivyStore.setState({ work: { ...baseWork, goal: undefined,
      plan: { active: true, review_status: 'pending', submission_id: 'submission-8', markdown: 'Second plan' },
    } }); });
    expect(container.textContent).toContain('Second plan');
    expect(button('Request revision')).toBeDefined();
    expect(button('Send feedback')).toBeUndefined();
  });
});
