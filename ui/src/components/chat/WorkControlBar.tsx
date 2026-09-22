import { useState } from 'react';
import type { PlanAction } from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { useTranslation } from '@/i18n';
import { PlanReview } from './PlanReview';

export function WorkControlBar({ sessionId }: { sessionId: string }) {
  const { t } = useTranslation();
  const work = useVivyStore((state) => state.work);
  const phase = useVivyStore((state) => state.workPhase);
  const error = useVivyStore((state) => state.workError);
  const busy = useVivyStore((state) => state.workBusy);
  const createGoal = useVivyStore((state) => state.createGoal);
  const pauseGoal = useVivyStore((state) => state.pauseGoal);
  const resumeGoal = useVivyStore((state) => state.resumeGoal);
  const clearGoal = useVivyStore((state) => state.clearGoal);
  const enterPlan = useVivyStore((state) => state.enterPlan);
  const leavePlan = useVivyStore((state) => state.leavePlan);
  const decidePlan = useVivyStore((state) => state.decidePlan);
  const [objective, setObjective] = useState('');
  const [maxRounds, setMaxRounds] = useState('3');
  const [creating, setCreating] = useState(false);
  const goal = work?.goal;
  const plan = work?.plan;
  const goalIsActive = goal?.phase === 'active';
  const runAction = async (action: () => Promise<unknown>) => {
    try { await action(); } catch { /* workError is authoritative in the store */ }
  };
  const create = async () => {
    const rounds = Number(maxRounds);
    if (!objective.trim() || !Number.isInteger(rounds) || rounds <= 0) return;
    await runAction(async () => {
      await createGoal(objective.trim(), rounds);
      setObjective('');
      setCreating(false);
    });
  };
  const handlePlanDecision = async (action: PlanAction, feedback?: string) => {
    await runAction(() => decidePlan(action, feedback));
  };
  if (phase === 'loading' && !work) return <div className="border-b px-4 py-2 text-xs text-muted-foreground">{t('workControl.title')}…</div>;
  if (!work) return null;
  return (
    <section aria-label={t('workControl.title')} className="border-b bg-card/60 px-4 py-2">
      <div className="mx-auto flex max-w-4xl flex-wrap items-center gap-2">
        <span className="text-xs font-semibold">{t('workControl.title')}</span>
        {goal ? (
          <span className="max-w-full truncate text-xs text-muted-foreground" title={goal.objective}>
            {t('workControl.goal')}: {goal.objective} · {t('workControl.' + goal.phase)} · {t('workControl.rounds', { started: goal.rounds_started, max: goal.max_rounds })}
          </span>
        ) : <span className="text-xs text-muted-foreground">{t('workControl.noGoal')}</span>}
        <span className="rounded-full border px-2 py-0.5 text-[11px] text-muted-foreground">{t('workControl.' + work.activation)}</span>
        <div className="ml-auto flex flex-wrap gap-1">
          {goal && goalIsActive ? <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void runAction(pauseGoal)}>{t('workControl.pause')}</Button> : null}
          {goal && goal.phase === 'paused' ? <Button type="button" size="sm" variant="outline" disabled={busy || Boolean(work.plan.active)} onClick={() => void runAction(resumeGoal)}>{t('workControl.resume')}</Button> : null}
          {goal ? <Button type="button" size="sm" variant="ghost" disabled={busy} onClick={() => void runAction(clearGoal)}>{t('workControl.clear')}</Button> : null}
          {!goal && !work.plan.active ? <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => setCreating((value) => !value)}>{t('workControl.createGoal')}</Button> : null}
          {!work.plan.active ? <Button type="button" size="sm" variant="outline" disabled={busy || goalIsActive} onClick={() => void runAction(enterPlan)}>{t('workControl.enterPlan')}</Button> : <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void runAction(leavePlan)}>{t('workControl.leavePlan')}</Button>}
        </div>
      </div>
      {creating ? (
        <div className="mx-auto mt-2 flex max-w-4xl flex-col gap-2 sm:flex-row">
          <Textarea value={objective} onChange={(event) => setObjective(event.target.value)} placeholder={t('workControl.objectivePlaceholder')} disabled={busy} className="min-h-9 flex-1" />
          <Input value={maxRounds} onChange={(event) => setMaxRounds(event.target.value)} aria-label={t('workControl.maxRounds')} type="number" min={1} max={1000} disabled={busy} className="w-24" />
          <Button type="button" disabled={busy || !objective.trim()} onClick={() => void create()}>{t('workControl.create')}</Button>
        </div>
      ) : null}
      {plan?.active ? <PlanReview plan={plan} busy={busy} onDecide={handlePlanDecision} /> : null}
      {error ? <p className="mx-auto mt-2 max-w-4xl text-xs text-destructive">{t('workControl.error', { error })}</p> : null}
      <span className="sr-only">{sessionId}</span>
    </section>
  );
}
