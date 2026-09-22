import { useState } from 'react';
import type { PlanAction, WorkPlan } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { useTranslation } from '@/i18n';

export function PlanReview({ plan, busy, onDecide }: {
  plan: WorkPlan;
  busy: boolean;
  onDecide: (action: PlanAction, feedback?: string) => Promise<void>;
}) {
  const { t } = useTranslation();
  const [feedback, setFeedback] = useState('');
  const [revising, setRevising] = useState(false);
  if (!plan.submission_id) return null;
  const pending = plan.review_status === 'pending';
  const decide = async (action: PlanAction) => {
    try {
      await onDecide(action, action === 'revise' ? feedback : undefined);
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
          <div className="flex flex-wrap gap-2">
            <Button type="button" size="sm" variant="outline" disabled={busy || (revising && !feedback.trim())} onClick={() => revising ? void decide('revise') : setRevising(true)}>
              {t(revising ? 'workControl.reviseSubmit' : 'workControl.revise')}
            </Button>
            <Button type="button" size="sm" variant="outline" disabled title={t('workControl.handoffUnavailable')} onClick={() => void decide('execute_once')}>
              {t('workControl.executeOnce')}
            </Button>
            <Button type="button" size="sm" disabled title={t('workControl.handoffUnavailable')} onClick={() => void decide('start_goal')}>
              {t('workControl.startGoal')}
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">{t('workControl.handoffUnavailable')}</p>
        </div>
      ) : null}
    </section>
  );
}
