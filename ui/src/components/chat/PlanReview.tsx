import { useState } from 'react';
import type { PlanAction, WorkPlan } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { useTranslation } from '@/i18n';

export function PlanReview({ plan, busy, hasGoal, onDecide }: {
  plan: WorkPlan;
  busy: boolean;
  hasGoal: boolean;
  onDecide: (submissionId: string, action: PlanAction, feedback?: string, objective?: string, maxRounds?: number) => Promise<void>;
}) {
  const { t } = useTranslation();
  const [feedback, setFeedback] = useState('');
  const [goalObjective, setGoalObjective] = useState('');
  const [goalRounds, setGoalRounds] = useState('3');
  const [revising, setRevising] = useState(false);
  if (!plan.submission_id) return null;
  const pending = plan.review_status === 'pending';
  const rounds = Number(goalRounds);
  const validGoal = goalObjective.trim().length > 0 && Number.isInteger(rounds) && rounds > 0 && rounds <= 1000;
  const decide = async (action: PlanAction) => {
    try {
      await onDecide(plan.submission_id!, action, action === 'revise' ? feedback : undefined, action === 'start_goal' ? goalObjective.trim() : undefined, action === 'start_goal' ? rounds : undefined);
      if (action === 'revise') {
        setFeedback('');
        setRevising(false);
      }
    } catch {
      // The store keeps the authoritative error visible in WorkControlBar.
    }
  };
  const reviewLabel = plan.review_status === 'pending'
    ? t('workControl.reviewPending')
    : plan.review_status === 'accepted'
      ? t('workControl.reviewAccepted')
      : t('workControl.reviewRejected');
  return (
    <section className="mt-3 rounded-xl border bg-background p-3">
      <div className="mb-2 flex items-center justify-between gap-2">
        <h3 className="text-sm font-semibold">{t('workControl.planMarkdown')}</h3>
        <span className="text-xs text-muted-foreground">{reviewLabel}</span>
      </div>
      <pre className="max-h-72 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/50 p-3 text-xs leading-5">{plan.markdown}</pre>
      {plan.feedback ? <p className="mt-2 text-xs text-muted-foreground">{plan.feedback}</p> : null}
      {pending ? (
        <div className="mt-3 space-y-2">
          {revising ? <Textarea value={feedback} onChange={(event) => setFeedback(event.target.value)} placeholder={t('workControl.revisePlaceholder')} disabled={busy} /> : null}
          {!hasGoal ? <div className="grid gap-2 sm:grid-cols-[1fr_auto]">
            <Textarea value={goalObjective} onChange={(event) => setGoalObjective(event.target.value)} placeholder={t('workControl.goalObjectivePlaceholder')} disabled={busy} className="min-h-9" />
            <Input value={goalRounds} onChange={(event) => setGoalRounds(event.target.value)} aria-label={t('workControl.startGoalRounds')} type="number" min={1} max={1000} disabled={busy} className="w-24" />
          </div> : null}
          <div className="flex flex-wrap gap-2">
            <Button type="button" size="sm" variant="outline" disabled={busy || (revising && !feedback.trim())} onClick={() => revising ? void decide('revise') : setRevising(true)}>
              {t(revising ? 'workControl.reviseSubmit' : 'workControl.revise')}
            </Button>
            <Button type="button" size="sm" variant={hasGoal ? 'default' : 'outline'} disabled={busy} onClick={() => void decide('execute_once')}>
              {t('workControl.executeOnce')}
            </Button>
            {!hasGoal ? <Button type="button" size="sm" disabled={busy || !validGoal} onClick={() => void decide('start_goal')}>
              {t('workControl.startGoal')}
            </Button> : null}
          </div>
        </div>
      ) : null}
    </section>
  );
}
