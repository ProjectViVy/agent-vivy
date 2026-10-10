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
vi.mock('@/lib/api', async (original) => ({ ...await original<typeof api>(), listSkills: vi.fn(async () => ({ skills: [] })), getRun: vi.fn(), startTurn: vi.fn() }));
vi.mock('@/lib/run-subscription', () => ({ subscribeRun: vi.fn(() => ({ close: vi.fn(), lastSeq: () => 0 })) }));

const initial = useVivyStore.getState();
const emptyWork: api.WorkState = { session_id: 's1', version: 1, activation: 'disarmed', plan: { active: false, review_status: 'none' } };
const skill: api.SkillSummary = { name: 'release-review', description: 'Review a release', origin: 'user', enabled: true, hash: 'h1', warnings: [] };

describe('composer slash commands', () => {
  let container: HTMLDivElement;
  let root: Root;
  let onSend: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
    hydrateLocale('en');
    useVivyStore.setState({ ...initial, activeSessionId: 's1', work: emptyWork, workPhase: 'ready',
      sessions: [{ id: 's1', title: 'Session', created_at: 1, permission_preset: 'smart' }] });
    vi.mocked(api.listSkills).mockResolvedValue({ skills: [skill, { ...skill, name: 'disabled', enabled: false }] });
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

  async function render(running = false, sessionId = 's1') {
    await act(async () => root.render(<ChatInput key={sessionId} onSend={onSend} running={running} />));
  }
  const input = () => container.querySelector('textarea')!;
  async function type(value: string) {
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(input(), value);
      input().dispatchEvent(new Event('input', { bubbles: true }));
    });
  }
  async function key(key: string, init: KeyboardEventInit = {}) {
    await act(async () => { input().dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init })); });
  }
  async function choose(label: string) {
    await act(async () => { [...container.querySelectorAll<HTMLElement>('[role="option"]')].find((item) => item.textContent?.includes(label))!.click(); });
  }
  const tag = () => container.querySelector('[data-command-tag]');

  it('shows the supported command menu on slash and selects a removable tag by keyboard', async () => {
    await render();
    expect(container.textContent).not.toContain('Agent mode');
    await type('/');
    expect(container.querySelector('[role="listbox"]')?.textContent).toContain('/plan');
    expect(container.querySelector('[role="listbox"]')?.textContent).toContain('/goal');
    expect(container.querySelector('[role="listbox"]')?.textContent).toContain('/skill');
    await key('ArrowDown');
    await key('Enter');
    expect(tag()?.textContent).toContain('/goal');
    expect(input().value).toBe('');
    expect(onSend).not.toHaveBeenCalled();
    await act(async () => { container.querySelector<HTMLButtonElement>('button[aria-label="Remove command"]')!.click(); });
    expect(tag()).toBeNull();
  });

  it('enters Plan through a command then sends its text through the ordinary turn path', async () => {
    const enterPlan = vi.fn(async () => {
      useVivyStore.setState({ work: { ...emptyWork, plan: { active: true, review_status: 'none' } } });
      return {};
    });
    useVivyStore.setState({ enterPlan: enterPlan as never });
    await render();
    await type('/plan Design the change');
    expect(tag()?.textContent).toContain('/plan');
    await key('Enter');
    expect(enterPlan).toHaveBeenCalledOnce();
    expect(onSend).toHaveBeenCalledWith('Design the change', 'normal', undefined, 'auto');
    expect(input().value).toBe('');
    expect(container.querySelector('[data-plan-mode]')).not.toBeNull();
  });

  it('allows an empty Plan command without a model turn', async () => {
    const enterPlan = vi.fn(async () => ({}));
    useVivyStore.setState({ enterPlan: enterPlan as never });
    await render();
    await type('/plan');
    await key('Enter');
    await key('Enter');
    expect(enterPlan).toHaveBeenCalledOnce();
    expect(onSend).not.toHaveBeenCalled();
  });

  it('creates a bounded Goal from the tag and preserves a rejected objective', async () => {
    const createGoal = vi.fn(async () => { throw new Error('work_conflict'); });
    useVivyStore.setState({ createGoal: createGoal as never });
    await render();
    await type('/goal Ship the release');
    await key('Enter');
    expect(createGoal).toHaveBeenCalledWith('Ship the release', 3);
    expect(onSend).not.toHaveBeenCalled();
    expect(input().value).toBe('Ship the release');
    expect(tag()?.textContent).toContain('/goal');
    expect(container.textContent).toContain('work_conflict');
  });

  it('hands off only the plan submission selected with the Goal command', async () => {
    const decidePlan = vi.fn(async () => ({}));
    useVivyStore.setState({ work: { ...emptyWork, plan: { active: true, review_status: 'pending', submission_id: 'p1', markdown: 'Exact plan' } }, decidePlan: decidePlan as never });
    await render();
    await type('/goal Ship it');
    await key('Enter');
    expect(decidePlan).toHaveBeenCalledWith('start_goal', '', 'Ship it', 3, 'p1');
    expect(onSend).not.toHaveBeenCalled();
  });

  it('does not approve a replacement Plan submission using an old tag', async () => {
    const decidePlan = vi.fn(async () => ({}));
    useVivyStore.setState({ work: { ...emptyWork, plan: { active: true, review_status: 'pending', submission_id: 'p1' } }, decidePlan: decidePlan as never });
    await render();
    await type('/goal Ship it');
    await act(async () => { useVivyStore.setState({ work: { ...emptyWork, plan: { active: true, review_status: 'pending', submission_id: 'p2' } } }); });
    await key('Enter');
    expect(decidePlan).not.toHaveBeenCalled();
    expect(input().value).toBe('Ship it');
    expect(container.textContent).toContain('submission changed');
  });

  it('lists only enabled skills and submits an explicit request through the existing skill middleware', async () => {
    await render();
    await type('/skill');
    await key('Enter');
    expect(container.querySelector('[role="listbox"]')?.textContent).toContain('release-review');
    expect(container.querySelector('[role="listbox"]')?.textContent).not.toContain('disabled');
    await choose('release-review');
    expect(tag()?.textContent).toContain('release-review');
    await type('Check this release');
    await key('Enter');
    expect(onSend).toHaveBeenCalledWith('Use the skill "release-review" for this request.\n\nCheck this release', 'normal', undefined, 'auto');
  });

  it('keeps the draft when a selected skill becomes disabled', async () => {
    await render();
    await type('/skill');
    await key('Enter');
    await choose('release-review');
    vi.mocked(api.listSkills).mockResolvedValue({ skills: [{ ...skill, enabled: false }] });
    await type('Review');
    await key('Enter');
    expect(onSend).not.toHaveBeenCalled();
    expect(input().value).toBe('Review');
    expect(tag()).not.toBeNull();
    expect(container.textContent).toContain('no longer available');
  });

  it.each(['/plan.md', '/goal/path', '/unknown', 'Use /plan here'])('keeps non-command text %s as ordinary text', async (text) => {
    await render();
    await type(text);
    await key('Enter');
    expect(tag()).toBeNull();
    expect(onSend).toHaveBeenCalledWith(text, 'normal', undefined, 'auto');
  });

  it('ignores composing Enter and uses Escape to dismiss discovery', async () => {
    await render(true);
    await type('/');
    await key('Enter', { isComposing: true });
    expect(tag()).toBeNull();
    expect(container.querySelector('[role="listbox"]')).not.toBeNull();
    await key('Escape');
    expect(container.querySelector('[role="listbox"]')).toBeNull();
    expect(input().value).toBe('/');
    expect(onSend).not.toHaveBeenCalled();
  });

  it('does not submit or carry a command into another session after an old mutation finishes', async () => {
    let resolve!: () => void;
    const pending = new Promise<void>((done) => { resolve = done; });
    useVivyStore.setState({ enterPlan: vi.fn(() => pending) as never });
    await render();
    await type('/plan Old session draft');
    await act(async () => { input().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); });
    await act(async () => { useVivyStore.setState({ activeSessionId: 's2', work: { ...emptyWork, session_id: 's2' } }); });
    await render(false, 's2');
    expect(tag()).toBeNull();
    expect(input().value).toBe('');
    await type('New session draft');
    await act(async () => { resolve(); await pending; });
    expect(onSend).not.toHaveBeenCalled();
    expect(input().value).toBe('New session draft');
  });

  it('uses the opening Goal reference to resume without creating a new Goal', async () => {
    const commitWork = vi.fn(async () => ({}));
    const createGoal = vi.fn();
    useVivyStore.setState({ work: { ...emptyWork, goal: { id: 'g1', revision: 7, objective: 'Ship it', phase: 'paused', max_rounds: 4, rounds_started: 2 } }, commitWork: commitWork as never, createGoal });
    await render();
    await type('/goal');
    await key('Tab');
    await key('Enter');
    expect(commitWork).toHaveBeenCalledWith('goal/resume', { goal_id: 'g1', goal_revision: 7 });
    expect(createGoal).not.toHaveBeenCalled();
    expect(onSend).not.toHaveBeenCalled();
  });

  it('keeps an empty session free of a Plan chip after a failed Plan command', async () => {
    useVivyStore.setState({ enterPlan: vi.fn(async () => { throw new Error('permission_denied'); }) as never });
    await render();
    await type('/plan Investigate');
    await key('Enter');
    expect(input().value).toBe('Investigate');
    expect(tag()?.textContent).toContain('/plan');
    expect(container.querySelector('[data-plan-mode]')).toBeNull();
    expect(container.textContent).toContain('permission_denied');
    expect(onSend).not.toHaveBeenCalled();
  });

  it('rejects invalid Goal round limits without discarding the objective', async () => {
    const createGoal = vi.fn();
    useVivyStore.setState({ createGoal });
    await render();
    await type('/goal Ship it');
    await act(async () => {
      const rounds = container.querySelector('input[aria-label="Round limit"]')!;
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(rounds, '0');
      rounds.dispatchEvent(new Event('input', { bubbles: true }));
    });
    await key('Enter');
    expect(createGoal).not.toHaveBeenCalled();
    expect(input().value).toBe('Ship it');
    expect(container.textContent).toContain('between 1 and 1000');
  });

  it('drops discovery results after the skill tag is removed', async () => {
    let resolve!: (value: { skills: api.SkillSummary[] }) => void;
    vi.mocked(api.listSkills).mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
    await render();
    await type('/skill');
    await key('Enter');
    await act(async () => { container.querySelector<HTMLButtonElement>('button[aria-label="Remove command"]')!.click(); });
    await act(async () => { resolve({ skills: [skill] }); });
    expect(tag()).toBeNull();
    expect(container.querySelector('[role="listbox"]')).toBeNull();
    expect(input().value).toBe('');
  });

  it('starts planning after the stopped Goal is reconciled even before its cancellation notification', async () => {
    const goal = { id: 'g1', revision: 1, objective: 'Ship it', phase: 'active' as const, max_rounds: 3, rounds_started: 1 };
    const stopped: api.Run = { id: 'goal-run', session_id: 's1', status: 'cancelled', created_at: 1 };
    useVivyStore.setState({ currentRun: { ...stopped, status: 'active' }, work: { ...emptyWork, goal, activation: 'armed', current_run_id: stopped.id },
      enterPlan: vi.fn(async () => { useVivyStore.setState({ work: { ...emptyWork, goal: { ...goal, phase: 'paused' }, plan: { active: true, review_status: 'none' } } }); return {}; }) as never });
    vi.mocked(api.getRun).mockResolvedValue(stopped);
    vi.mocked(api.startTurn).mockResolvedValue({ run_id: 'plan-run', status: 'active' });
    onSend = vi.fn((text: string, mode: api.RunMode) => initial.startRun('s1', { text, mode }));
    await render(true);
    await type('/plan Investigate');
    await key('Enter');
    expect(api.startTurn).toHaveBeenCalledWith('s1', { text: 'Investigate', mode: 'normal' });
    expect(useVivyStore.getState().kernelQueue?.pending ?? 0).toBe(0);
    expect(useVivyStore.getState().currentRun?.id).toBe('plan-run');
  });

  it('keeps Shift+Enter line breaks in slash-prefixed ordinary text', async () => {
    await render();
    await type('/plan');
    await key('Enter', { shiftKey: true });
    await type('/plan\n');
    expect(tag()).toBeNull();
    expect(input().value).toBe('/plan\n');
  });

  it.each(['completed', 'blocked'] as const)('requires clearing a %s Goal before creating another', async (phase) => {
    const createGoal = vi.fn();
    useVivyStore.setState({ work: { ...emptyWork, goal: { id: 'g1', revision: 4, objective: 'Previous Goal', phase, max_rounds: 3, rounds_started: 3 } }, createGoal });
    await render();
    await type('/goal New objective');
    await key('Enter');
    expect(createGoal).not.toHaveBeenCalled();
    expect(input().value).toBe('New objective');
    expect(container.textContent).toContain('Clear the existing Goal');
  });
});
