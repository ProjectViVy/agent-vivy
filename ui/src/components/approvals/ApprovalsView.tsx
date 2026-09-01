import { useEffect, useState } from 'react';
import { ShieldCheck } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { MasterDetail } from '@/components/layout/MasterDetail';
import { ReviewCard, statusLabel } from '@/components/approvals/ReviewCard';
import { useVivyStore } from '@/lib/store';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/i18n';

export function ApprovalsView({ panel = false }: { panel?: boolean }) {
  const reviews = useVivyStore((state) => state.reviews);
  const phase = useVivyStore((state) => state.reviewsPhase);
  const error = useVivyStore((state) => state.reviewsError);
  const busyIds = useVivyStore((state) => state.reviewBusyIds);
  const load = useVivyStore((state) => state.loadReviews);
  const respond = useVivyStore((state) => state.respondReview);
  const { t } = useTranslation();
  const [selectedId, setSelectedId] = useState<string | null>(null);

  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    if (selectedId && reviews.some((item) => item.id === selectedId)) return;
    setSelectedId(panel ? reviews[0]?.id ?? null : null);
  }, [reviews, selectedId, panel]);

  const selected = reviews.find((item) => item.id === selectedId) ?? null;
  const selectedBusy = selected !== null && busyIds.includes(selected.id);

  const list = (
    <div className="space-y-2">
      {reviews.map((review) => (
        <button
          type="button"
          key={review.id}
          disabled={busyIds.includes(review.id)}
          onClick={() => setSelectedId(review.id)}
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
    <ReviewCard key={selected.id} review={selected} busy={selectedBusy} onRespond={(response) => respond(selected.id, response)} />
  ) : null;

  const header = (
    <div className={cn('flex items-center justify-between gap-3', panel ? 'mb-4 justify-end' : 'mb-6')}>
      {!panel ? (
        <div className="min-w-0">
          <h1 className="flex items-center gap-2 text-2xl font-bold"><ShieldCheck className="h-6 w-6 shrink-0" />{t('approvals.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t('approvals.subtitle')}</p>
        </div>
      ) : null}
      <Button variant="outline" disabled={phase === 'loading' || phase === 'refreshing'} onClick={() => void load()}>{phase === 'refreshing' ? t('approvals.refreshing') : t('common.refresh')}</Button>
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
