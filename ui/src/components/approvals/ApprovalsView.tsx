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

const STATUS_LABELS: Record<ReviewStatus, string> = {
  pending: '待处理',
  approved: '已批准',
  denied: '已拒绝',
  answered: '已回答',
  cancelled: '已取消',
  expired: '已过期',
  stale: '已失效',
};

const VALUE_LABELS: Record<string, string> = {
  reversible: '可逆', irreversible: '不可逆', conditionally_reversible: '有条件可逆', unknown: '未知',
  workspace: '工作区', session: '会话', run: '运行', global: '全局',
  trusted: '可信', untrusted: '不可信', restricted: '受限',
};

function localizeValue(value: string): string { return VALUE_LABELS[value.toLowerCase()] ?? value; }

export function ApprovalsView({ panel = false }: { panel?: boolean }) {
  const reviews = useVivyStore((state) => state.reviews);
  const phase = useVivyStore((state) => state.reviewsPhase);
  const error = useVivyStore((state) => state.reviewsError);
  const busyId = useVivyStore((state) => state.reviewBusyId);
  const load = useVivyStore((state) => state.loadReviews);
  const respond = useVivyStore((state) => state.respondReview);
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
            <span className="truncate font-medium">{review.kind === 'approval' ? review.tool_name || review.action || '工具审批' : '用户问题'}</span>
            <Badge variant={review.status === 'pending' ? 'default' : 'secondary'}>{STATUS_LABELS[review.status]}</Badge>
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
          <span>{selected.kind === 'approval' ? '审批详情' : '问题详情'}</span>
          <Badge>{STATUS_LABELS[selected.status]}</Badge>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <dl className="grid grid-cols-[88px_minmax(0,1fr)] gap-2">
          <dt className="text-muted-foreground">运行</dt>
          <dd className="min-w-0 break-all"><code>{selected.run_id}</code></dd>
          {selected.action ? <><dt className="text-muted-foreground">动作</dt><dd className="min-w-0 break-words">{selected.action}</dd></> : null}
          {selected.target ? <><dt className="text-muted-foreground">目标</dt><dd className="min-w-0 break-words">{selected.target}</dd></> : null}
          {selected.effect ? <><dt className="text-muted-foreground">影响</dt><dd className="min-w-0 break-words">{localizeValue(selected.effect)}</dd></> : null}
          {selected.reversibility ? <><dt className="text-muted-foreground">可逆性</dt><dd className="min-w-0 break-words">{localizeValue(selected.reversibility)}</dd></> : null}
          {selected.scope ? <><dt className="text-muted-foreground">作用范围</dt><dd className="min-w-0 break-words">{localizeValue(selected.scope)}</dd></> : null}
          {selected.trust ? <><dt className="text-muted-foreground">信任状态</dt><dd className="min-w-0 break-words">{localizeValue(selected.trust)}</dd></> : null}
        </dl>
        {selected.prompt ? <div className="rounded-lg bg-muted p-3">{selected.prompt}</div> : null}
        {selected.preview ? <pre className="overflow-auto whitespace-pre-wrap break-words rounded-lg bg-muted p-3 text-xs">{selected.preview}</pre> : null}
        {selected.risk_findings?.length ? (
          <div>
            <div className="font-medium text-destructive">风险</div>
            <ul className="mt-1 list-disc pl-5 text-muted-foreground">{selected.risk_findings.map((risk) => <li key={risk}>{risk}</li>)}</ul>
          </div>
        ) : null}
        {selected.arguments ? (
          <details>
            <summary className="cursor-pointer text-muted-foreground">脱敏参数</summary>
            <pre className="mt-2 overflow-auto rounded bg-muted p-3 text-xs">{JSON.stringify(selected.arguments, null, 2)}</pre>
          </details>
        ) : null}
        {selected.status === 'pending' ? (
          <div className="space-y-2 border-t pt-4">
            <Textarea value={text} disabled={busy} onChange={(event) => setText(event.target.value)} placeholder={selected.kind === 'question' ? '输入回答' : '拒绝理由（拒绝时可选）'} />
            <div className="flex flex-wrap gap-2">
              {selected.kind === 'approval' ? (
                <>
                  <Button disabled={busy} onClick={() => void act('approve')}>{busyId === selected.id ? '处理中…' : '批准'}</Button>
                  <Button disabled={busy} variant="destructive" onClick={() => void act('deny')}>拒绝</Button>
                </>
              ) : (
                <>
                  <Button disabled={busy || !text.trim()} onClick={() => void act('answer')}>{busyId === selected.id ? '提交中…' : '提交回答'}</Button>
                  <Button disabled={busy} variant="outline" onClick={() => void act('cancel')}>取消问题</Button>
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
          <h1 className="flex items-center gap-2 text-2xl font-bold"><ShieldCheck className="h-6 w-6 shrink-0" />审批中心</h1>
          <p className="mt-1 text-sm text-muted-foreground">统一处理工具审批和运行中的问题。</p>
        </div>
      ) : null}
      <Button variant="outline" disabled={busy || phase === 'loading' || phase === 'refreshing'} onClick={() => void load()}>{phase === 'refreshing' ? '刷新中…' : '刷新'}</Button>
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
      return <Card><CardContent className="py-16 text-center text-muted-foreground">暂无审批或问题记录</CardContent></Card>;
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
        detail={detail ?? <Card><CardContent className="py-16 text-center text-muted-foreground">选择一条记录查看详情</CardContent></Card>}
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
