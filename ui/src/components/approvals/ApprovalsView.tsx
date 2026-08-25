import { useEffect, useState } from 'react';
import { ShieldCheck } from 'lucide-react';
import type { ReviewStatus } from '@/lib/api';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Textarea } from '@/components/ui/textarea';
import { MasterDetail } from '@/components/layout/MasterDetail';
import { useVivyStore } from '@/lib/store';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/i18n';

type Translate = ReturnType<typeof useTranslation>['t'];

function statusLabel(t: Translate, status: ReviewStatus): string {
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

export function ApprovalsView({ panel = false }: { panel?: boolean }) {
  const reviews = useVivyStore((state) => state.reviews);
  const phase = useVivyStore((state) => state.reviewsPhase);
  const error = useVivyStore((state) => state.reviewsError);
  const busyId = useVivyStore((state) => state.reviewBusyId);
  const load = useVivyStore((state) => state.loadReviews);
  const respond = useVivyStore((state) => state.respondReview);
  const { t } = useTranslation();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [text, setText] = useState('');

  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    if (selectedId && reviews.some((item) => item.id === selectedId)) return;
    setSelectedId(panel ? reviews[0]?.id ?? null : null);
  }, [reviews, selectedId, panel]);

  const selected = reviews.find((item) => item.id === selectedId) ?? null;
  const busy = busyId !== null;
  const act = async (action: 'approve' | 'deny' | 'answer' | 'cancel') => {
    if (!selected) return;
    await respond(selected.id, action === 'answer' ? { action, answer: text } : action === 'deny' ? { action, reason: text } : { action });
    setText('');
  };

  const list = (
    <div className="space-y-2">
      {reviews.map((review) => (
        <button
          type="button"
          key={review.id}
          disabled={busy}
          onClick={() => { setSelectedId(review.id); setText(''); }}
          className={cn(
            'w-full cursor-pointer rounded-xl border p-3 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-60',
            selectedId === review.id ? 'border-primary bg-primary/5' : 'bg-card hover:bg-muted/50',
          )}
        >
          <div className="flex items-center justify-between gap-2">
            <span className="truncate font-medium">{review.kind === 'approval' ? review.tool_name || review.action || t('approvals.toolApproval') : t('approvals.userQuestion')}</span>
            <Badge variant={review.status === 'pending' ? 'default' : 'secondary'}>{statusLabel(t, review.status)}</Badge>
          </div>
          <p className="mt-1 truncate text-xs text-muted-foreground">{review.session_title || review.session_id}</p>
        </button>
      ))}
    </div>
  );

  const detail = selected ? (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center justify-between gap-2">
          <span>{selected.kind === 'approval' ? t('approvals.approvalDetail') : t('approvals.questionDetail')}</span>
          <Badge>{statusLabel(t, selected.status)}</Badge>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <dl className="grid grid-cols-[88px_minmax(0,1fr)] gap-2">
          <dt className="text-muted-foreground">{t('approvals.run')}</dt>
          <dd className="min-w-0 break-all"><code>{selected.run_id}</code></dd>
          {selected.action ? <><dt className="text-muted-foreground">{t('approvals.action')}</dt><dd className="min-w-0 break-words">{selected.action}</dd></> : null}
          {selected.target ? <><dt className="text-muted-foreground">{t('approvals.target')}</dt><dd className="min-w-0 break-words">{selected.target}</dd></> : null}
          {selected.effect ? <><dt className="text-muted-foreground">{t('approvals.effect')}</dt><dd className="min-w-0 break-words">{localizeValue(t, selected.effect)}</dd></> : null}
          {selected.reversibility ? <><dt className="text-muted-foreground">{t('approvals.reversibility')}</dt><dd className="min-w-0 break-words">{localizeValue(t, selected.reversibility)}</dd></> : null}
          {selected.scope ? <><dt className="text-muted-foreground">{t('approvals.scope')}</dt><dd className="min-w-0 break-words">{localizeValue(t, selected.scope)}</dd></> : null}
          {selected.trust ? <><dt className="text-muted-foreground">{t('approvals.trust')}</dt><dd className="min-w-0 break-words">{localizeValue(t, selected.trust)}</dd></> : null}
        </dl>
        {selected.prompt ? <div className="rounded-lg bg-muted p-3">{selected.prompt}</div> : null}
        {selected.preview ? <pre className="overflow-auto whitespace-pre-wrap break-words rounded-lg bg-muted p-3 text-xs">{selected.preview}</pre> : null}
        {selected.risk_findings?.length ? (
          <div>
            <div className="font-medium text-destructive">{t('approvals.risk')}</div>
            <ul className="mt-1 list-disc pl-5 text-muted-foreground">{selected.risk_findings.map((risk) => <li key={risk}>{risk}</li>)}</ul>
          </div>
        ) : null}
        {selected.arguments ? (
          <details>
            <summary className="cursor-pointer text-muted-foreground">{t('approvals.redactedArgs')}</summary>
            <pre className="mt-2 overflow-auto rounded bg-muted p-3 text-xs">{JSON.stringify(selected.arguments, null, 2)}</pre>
          </details>
        ) : null}
        {selected.status === 'pending' ? (
          <div className="space-y-2 border-t pt-4">
            <Textarea value={text} disabled={busy} onChange={(event) => setText(event.target.value)} placeholder={selected.kind === 'question' ? t('approvals.answerPlaceholder') : t('approvals.denyReasonPlaceholder')} />
            <div className="flex flex-wrap gap-2">
              {selected.kind === 'approval' ? (
                <>
                  <Button disabled={busy} onClick={() => void act('approve')}>{busyId === selected.id ? t('approvals.processing') : t('approvals.approve')}</Button>
                  <Button disabled={busy} variant="destructive" onClick={() => void act('deny')}>{t('approvals.deny')}</Button>
                </>
              ) : (
                <>
                  <Button disabled={busy || !text.trim()} onClick={() => void act('answer')}>{busyId === selected.id ? t('approvals.submitting') : t('approvals.submitAnswer')}</Button>
                  <Button disabled={busy} variant="outline" onClick={() => void act('cancel')}>{t('approvals.cancelQuestion')}</Button>
                </>
              )}
            </div>
          </div>
        ) : null}
      </CardContent>
    </Card>
  ) : null;

  const header = (
    <div className={cn('flex items-center justify-between gap-3', panel ? 'mb-4 justify-end' : 'mb-6')}>
      {!panel ? (
        <div className="min-w-0">
          <h1 className="flex items-center gap-2 text-2xl font-bold"><ShieldCheck className="h-6 w-6 shrink-0" />{t('approvals.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t('approvals.subtitle')}</p>
        </div>
      ) : null}
      <Button variant="outline" disabled={busy || phase === 'loading' || phase === 'refreshing'} onClick={() => void load()}>{phase === 'refreshing' ? t('approvals.refreshing') : t('common.refresh')}</Button>
    </div>
  );

  const body = (() => {
    if (phase === 'loading') {
      return (
        <div className={cn('grid gap-4', !panel && 'md:grid-cols-[320px_1fr]')}>
          <div className="h-48 animate-pulse rounded-xl bg-muted" />
          {!panel ? <div className="hidden h-72 animate-pulse rounded-xl bg-muted md:block" /> : null}
        </div>
      );
    }
    if (reviews.length === 0) {
      return <Card><CardContent className="py-16 text-center text-muted-foreground">{t('approvals.empty')}</CardContent></Card>;
    }
    if (panel) {
      return <div className="space-y-4">{list}{detail}</div>;
    }
    return (
      <MasterDetail
        selected={selectedId !== null}
        onBack={() => setSelectedId(null)}
        columnsClassName="md:grid-cols-[320px_minmax(0,1fr)] md:gap-4"
        master={list}
        detail={detail ?? <Card><CardContent className="py-16 text-center text-muted-foreground">{t('approvals.selectHint')}</CardContent></Card>}
      />
    );
  })();

  if (panel) {
    return (
      <div className="flex h-full min-h-0 flex-col p-4">
        {header}
        {error ? <p className="mb-4 rounded bg-destructive/10 p-3 text-sm text-destructive">{error}</p> : null}
        <div className="min-h-0 flex-1 overflow-auto">{body}</div>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col p-4 sm:p-6">
      <div className="mx-auto flex min-h-0 w-full max-w-6xl flex-1 flex-col">
        {header}
        {error ? <p className="mb-4 rounded bg-destructive/10 p-3 text-sm text-destructive">{error}</p> : null}
        <div className="min-h-0 flex-1">{body}</div>
      </div>
    </div>
  );
}
