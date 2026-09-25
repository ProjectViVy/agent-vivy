import { useState } from 'react';
import type { PlanAction, WorkGoal } from '@/lib/api';
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
  const sessions = useVivyStore((state) => state.sessions);
  const loadWork = useVivyStore((state) => state.loadWork);
  const createGoal = useVivyStore((state) => state.createGoal);
  const editGoal = useVivyStore((state) => state.editGoal);
  const pauseGoal = useVivyStore((state) => state.pauseGoal);
  const resumeGoal = useVivyStore((state) => state.resumeGoal);
  const clearGoal = useVivyStore((state) => state.clearGoal);
  const enterPlan = useVivyStore((state) => state.enterPlan);
  const leavePlan = useVivyStore((state) => state.leavePlan);
  const decidePlan = useVivyStore((state) => state.decidePlan);
  const [objective, setObjective] = useState('');
  const [maxRounds, setMaxRounds] = useState('3');
  const [creating, setCreating] = useState(false);
  const [editDraft, setEditDraft] = useState<{
    ref: Pick<WorkGoal, 'id' | 'revision'>; objective: string; maxRounds: string;
  } | null>(null);
  const [enteringPlan, setEnteringPlan] = useState(false);
  const goal = work?.goal;
  const plan = work?.plan;
  const goalIsActive = goal?.phase === 'active';
  const permission = sessions.find((session) => session.id === sessionId)?.permission_preset;
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
  const saveEdit = async () => {
    if (!editDraft) return;
    const rounds = Number(editDraft.maxRounds);
    if (!editDraft.objective.trim() || !Number.isInteger(rounds) || rounds <= 0 || rounds > 1000 || (goal && rounds < goal.rounds_started)) return;
    await runAction(async () => {
      await editGoal(editDraft.objective.trim(), rounds, editDraft.ref);
      setEditDraft(null);
    });
  };
  const handleEnterPlan = async () => {
    setEnteringPlan(true);
    try { await enterPlan(); } catch { /* workError remains visible */ }
    finally { setEnteringPlan(false); }
  };
  const handlePlanDecision = async (submissionId: string, action: PlanAction, feedback?: string, objective?: string, maxRounds?: number) => {
    await decidePlan(action, feedback, objective, maxRounds, submissionId);
  };
  if (phase === 'loading' && !work) return <div className="border-b px-4 py-2 text-xs text-muted-foreground">{t('workControl.title')}…</div>;
  if (phase === 'error' && !work) return <section aria-label={t('workControl.title')} className="flex items-center gap-2 border-b px-4 py-2 text-xs"><span role="alert" className="text-destructive">{t('workControl.error', { error: error ?? t('workControl.unavailable') })}</span><Button type="button" size="sm" variant="outline" onClick={() => void loadWork(sessionId)}>{t('workControl.refresh')}</Button></section>;
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
        {permission ? <span className="text-[11px] text-muted-foreground">{t('workControl.permission')}: {t('chatInput.permission' + permission[0].toUpperCase() + permission.slice(1))}</span> : null}
        {enteringPlan ? <span role="status" className="text-xs text-muted-foreground">{t('workControl.stoppingForPlan')}</span> : null}
        {work.current_run_id ? <span className="text-[11px] text-muted-foreground">{t('workControl.currentRun')}: {work.current_run_id}</span> : null}
        <div className="ml-auto flex flex-wrap gap-1">
          {goal && !editDraft ? <Button type="button" size="sm" variant="outline" disabled={busy || enteringPlan} onClick={() => setEditDraft({ ref: { id: goal.id, revision: goal.revision }, objective: goal.objective, maxRounds: String(goal.max_rounds) })}>{t('workControl.editGoal')}</Button> : null}
          {goal && goalIsActive && work.activation === 'armed' ? <Button type="button" size="sm" variant="outline" disabled={busy || enteringPlan} onClick={() => void runAction(pauseGoal)}>{t('workControl.pauseGoal')}</Button> : null}
          {goal && (goal.phase === 'paused' || (goalIsActive && work.activation === 'disarmed')) ? <Button type="button" size="sm" variant="outline" disabled={busy || enteringPlan || Boolean(work.plan.active) || goal.rounds_started >= goal.max_rounds} onClick={() => void runAction(resumeGoal)}>{t('workControl.resumeGoal')}</Button> : null}
          {goal ? <Button type="button" size="sm" variant="ghost" disabled={busy || enteringPlan} onClick={() => void runAction(clearGoal)}>{t('workControl.clear')}</Button> : null}
          {!goal && !work.plan.active ? <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => setCreating((value) => !value)}>{t('workControl.createGoal')}</Button> : null}
          {!work.plan.active ? <Button type="button" size="sm" variant="outline" disabled={busy || enteringPlan} onClick={() => void handleEnterPlan()}>{t('workControl.enterPlan')}</Button> : <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void runAction(leavePlan)}>{t('workControl.leavePlan')}</Button>}
        </div>
      </div>
      {creating ? (
        <div className="mx-auto mt-2 flex max-w-4xl flex-col gap-2 sm:flex-row">
          <Textarea value={objective} onChange={(event) => setObjective(event.target.value)} placeholder={t('workControl.objectivePlaceholder')} disabled={busy} className="min-h-9 flex-1" />
          <Input value={maxRounds} onChange={(event) => setMaxRounds(event.target.value)} aria-label={t('workControl.maxRounds')} type="number" min={1} max={1000} disabled={busy} className="w-24" />
          <Button type="button" disabled={busy || !objective.trim()} onClick={() => void create()}>{t('workControl.create')}</Button>
        </div>
      ) : null}
      {editDraft ? (
        <div className="mx-auto mt-2 flex max-w-4xl flex-col gap-2 sm:flex-row">
          <Textarea value={editDraft.objective} onChange={(event) => setEditDraft({ ...editDraft, objective: event.target.value })} aria-label={t('workControl.editObjective')} disabled={busy} className="min-h-9 flex-1" />
          <Input value={editDraft.maxRounds} onChange={(event) => setEditDraft({ ...editDraft, maxRounds: event.target.value })} aria-label={t('workControl.editRounds')} type="number" min={goal?.rounds_started || 1} max={1000} disabled={busy} className="w-24" />
          <Button type="button" disabled={busy || !editDraft.objective.trim() || !Number.isInteger(Number(editDraft.maxRounds)) || Number(editDraft.maxRounds) <= 0 || Number(editDraft.maxRounds) > 1000 || Boolean(goal && Number(editDraft.maxRounds) < goal.rounds_started)} onClick={() => void saveEdit()}>{t('workControl.saveGoal')}</Button>
          <Button type="button" variant="ghost" disabled={busy} onClick={() => setEditDraft(null)}>{t('workControl.cancelEdit')}</Button>
        </div>
      ) : null}
      {goal && (goal.reason || goal.evidence_run_id) ? (
        <div className="mx-auto mt-1 flex max-w-4xl flex-wrap gap-x-3 text-[11px] text-muted-foreground">
          {goal.reason ? <span>{t('workControl.reason')}: {goal.reason === 'round-limit' || goal.reason === 'round_limit' ? t('workControl.roundLimitReached') : goal.reason}</span> : null}
          {goal.evidence_run_id ? <span>{t('workControl.evidence')}: {goal.evidence_run_id}</span> : null}
        </div>
      ) : null}
      {plan?.active ? <PlanReview key={plan.submission_id ?? 'no-submission'} plan={plan} busy={busy} hasGoal={Boolean(goal)} onDecide={handlePlanDecision} /> : null}
      {error ? <div className="mx-auto mt-2 flex max-w-4xl items-center gap-2 text-xs text-destructive"><span role="alert">{t('workControl.error', { error })}</span><Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void loadWork(sessionId)}>{t('workControl.refresh')}</Button></div> : null}
    </section>
  );
}
