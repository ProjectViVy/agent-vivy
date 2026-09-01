import { useState } from 'react';
import type { ReviewItem, ReviewStatus } from '@/lib/api';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Textarea } from '@/components/ui/textarea';
import { DiffView } from '@/components/ui/DiffView';
import { looksLikeDiff } from '@/lib/diff';
import { dateTimeLocale, useTranslation } from '@/i18n';

type Translate = ReturnType<typeof useTranslation>['t'];
export type ReviewResponse = { action: 'approve' | 'deny' | 'answer' | 'cancel'; reason?: string; answer?: string };

export function statusLabel(t: Translate, status: ReviewStatus): string {
  return t(`approvals.status.${status}`);
}

/** 审批详情里的枚举值（可逆性/范围/信任）显示为本地化词条，未收录时保留原值。 */
function localizeValue(t: Translate, value: string): string {
  const normalized = value.toLowerCase();
  const keyMap: Record<string, string> = {
    reversible: 'approvals.values.reversible',
    irreversible: 'approvals.values.irreversible',
    conditionally_reversible: 'approvals.values.conditionally_reversible',
    unknown: 'approvals.values.unknown',
    workspace: 'approvals.values.workspace',
    session: 'approvals.values.session',
    run: 'approvals.values.runScope',
    global: 'approvals.values.global',
    trusted: 'approvals.values.trusted',
    untrusted: 'approvals.values.untrusted',
    restricted: 'approvals.values.restricted',
  };
  const key = keyMap[normalized];
  return key ? t(key) : value;
}

/** Review Center 队列与 Run Inspector 共用的同一张审批卡（含内联决策控件）。 */
export function ReviewCard({ review, busy, onRespond }: { review: ReviewItem; busy: boolean; onRespond: (response: ReviewResponse) => Promise<void> | void }) {
  const { t } = useTranslation();
  const [text, setText] = useState('');
  const act = async (action: 'approve' | 'deny' | 'answer' | 'cancel') => {
    await onRespond(action === 'answer' ? { action, answer: text } : action === 'deny' ? { action, reason: text } : { action });
    setText('');
  };
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center justify-between gap-2">
          <span>{review.kind === 'approval' ? t('approvals.approvalDetail') : t('approvals.questionDetail')}</span>
          <Badge>{statusLabel(t, review.status)}</Badge>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <dl className="grid grid-cols-[88px_minmax(0,1fr)] gap-2">
          <dt className="text-muted-foreground">{t('approvals.run')}</dt>
          <dd className="min-w-0 break-all"><code>{review.run_id}</code></dd>
          {review.action ? <><dt className="text-muted-foreground">{t('approvals.action')}</dt><dd className="min-w-0 break-words">{review.action}</dd></> : null}
          {review.target ? <><dt className="text-muted-foreground">{t('approvals.target')}</dt><dd className="min-w-0 break-words">{review.target}</dd></> : null}
          {review.effect ? <><dt className="text-muted-foreground">{t('approvals.effect')}</dt><dd className="min-w-0 break-words">{localizeValue(t, review.effect)}</dd></> : null}
          {review.reversibility ? <><dt className="text-muted-foreground">{t('approvals.reversibility')}</dt><dd className="min-w-0 break-words">{localizeValue(t, review.reversibility)}</dd></> : null}
          {review.scope ? <><dt className="text-muted-foreground">{t('approvals.scope')}</dt><dd className="min-w-0 break-words">{localizeValue(t, review.scope)}</dd></> : null}
          {review.trust ? <><dt className="text-muted-foreground">{t('approvals.trust')}</dt><dd className="min-w-0 break-words">{localizeValue(t, review.trust)}</dd></> : null}
          {review.actor ? <><dt className="text-muted-foreground">{t('approvals.actor')}</dt><dd className="min-w-0 break-words">{review.actor}</dd></> : null}
          <dt className="text-muted-foreground">{t('approvals.createdAt')}</dt><dd className="min-w-0 break-words">{new Date(review.created_at).toLocaleString(dateTimeLocale())}</dd>
          <dt className="text-muted-foreground">{t('approvals.expiresAt')}</dt><dd className="min-w-0 break-words">{new Date(review.expires_at).toLocaleString(dateTimeLocale())}</dd>
          {review.decided_at ? <><dt className="text-muted-foreground">{t('approvals.decidedAt')}</dt><dd className="min-w-0 break-words">{new Date(review.decided_at).toLocaleString(dateTimeLocale())}</dd></> : null}
          {review.precondition_hash ? <><dt className="text-muted-foreground">{t('approvals.precondition')}</dt><dd className="min-w-0 break-all"><code>{review.precondition_hash}</code></dd></> : null}
          {review.stale_reason ? <><dt className="text-muted-foreground">{t('approvals.staleReason')}</dt><dd className="min-w-0 break-words">{review.stale_reason}</dd></> : null}
          {review.decision_reason ? <><dt className="text-muted-foreground">{t('approvals.decisionReason')}</dt><dd className="min-w-0 break-words">{review.decision_reason}</dd></> : null}
          {review.error ? <><dt className="text-muted-foreground">{t('approvals.errorLabel')}</dt><dd className="min-w-0 break-words text-destructive">{review.error}</dd></> : null}
        </dl>
        {review.prompt ? <div className="rounded-lg bg-muted p-3">{review.prompt}</div> : null}
        {review.preview ? (looksLikeDiff(review.preview)
          ? <DiffView diff={review.preview} />
          : <pre className="overflow-auto whitespace-pre-wrap break-words rounded-lg bg-muted p-3 text-xs">{review.preview}</pre>) : null}
        {review.risk_findings?.length ? (
          <div>
            <div className="font-medium text-destructive">{t('approvals.risk')}</div>
            <ul className="mt-1 list-disc pl-5 text-muted-foreground">{review.risk_findings.map((risk) => <li key={risk}>{risk}</li>)}</ul>
          </div>
        ) : null}
        {review.arguments ? (
          <details>
            <summary className="cursor-pointer text-muted-foreground">{t('approvals.redactedArgs')}</summary>
            <pre className="mt-2 overflow-auto rounded bg-muted p-3 text-xs">{JSON.stringify(review.arguments, null, 2)}</pre>
          </details>
        ) : null}
        {review.status === 'pending' ? (
          <div className="space-y-2 border-t pt-4">
            <Textarea value={text} disabled={busy} onChange={(event) => setText(event.target.value)} placeholder={review.kind === 'question' ? t('approvals.answerPlaceholder') : t('approvals.denyReasonPlaceholder')} />
            <div className="flex flex-wrap gap-2">
              {review.kind === 'approval' ? (
                <>
                  <Button disabled={busy} onClick={() => void act('approve')}>{busy ? t('approvals.processing') : t('approvals.approve')}</Button>
                  <Button disabled={busy} variant="destructive" onClick={() => void act('deny')}>{t('approvals.deny')}</Button>
                </>
              ) : (
                <>
                  <Button disabled={busy || !text.trim()} onClick={() => void act('answer')}>{busy ? t('approvals.submitting') : t('approvals.submitAnswer')}</Button>
                  <Button disabled={busy} variant="outline" onClick={() => void act('cancel')}>{t('approvals.cancelQuestion')}</Button>
                </>
              )}
            </div>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
