// 已提交历史引用卡（SC-D4 §10）：展示 destination 持有的 sanitized 快照，
// 活状态（source_status / feed_status）由 reference/get 在读时计算，与保存
// 内容分开展示。源跳转复用 history/read，不切换当前会话。
import { useRef, useState } from 'react';
import { BookMarked, ChevronDown, ChevronRight, Loader2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { useTranslation } from '@/i18n';
import * as api from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { cn } from '@/lib/utils';

const KIND_KEYS: Record<string, string> = {
  message: 'historyPicker.kind.message',
  tool_call: 'historyPicker.kind.tool',
  tool_result: 'historyPicker.kind.tool',
  file: 'historyPicker.kind.file',
  diff: 'historyPicker.kind.file',
};

const STATUS_KEYS = ['ok', 'partial', 'forbidden', 'not_found', 'unavailable', 'cancelled', 'conflict', 'included', 'hidden', 'detached', 'elided_budget', 'invalid_argument'] as const;
const JUMPABLE = new Set(['ok']);

function kindsLabel(items: readonly api.HistoryItem[], t: (key: string, params?: Record<string, string | number>) => string): string {
  const kinds = [...new Set(items.map((item) => item.ref.kind))];
  return kinds.map((kind) => t(KIND_KEYS[kind] ?? 'chat.toolTitleGeneric')).join(' + ');
}

export function ReferenceDetail({ reference }: { reference: api.ContextReference }) {
  const { t } = useTranslation();
  const view = useVivyStore((state) => state.referenceViews[reference.id]);
  const loadReferenceView = useVivyStore((state) => state.loadReferenceView);
  const [expanded, setExpanded] = useState(false);
  const [sourceItems, setSourceItems] = useState<api.HistoryItem[] | null>(null);
  const [sourcePhase, setSourcePhase] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle');
  const toggleRef = useRef<HTMLButtonElement>(null);
  const open = expanded;
  const status = (value: string) => (STATUS_KEYS.includes(value as (typeof STATUS_KEYS)[number]) ? t(`referenceDetail.status.${value}`) : value);
  const captured = reference.captured_at > 0 ? new Date(reference.captured_at * 1000).toLocaleTimeString() : '';

  const loadSource = async () => {
    const sessionId = useVivyStore.getState().activeSessionId;
    if (!sessionId || sourcePhase === 'loading') return;
    setSourcePhase('loading');
    try {
      const page = await api.historyRead(sessionId, {
        selection: { source_session_id: reference.source_session_id, refs: reference.items.map((item) => item.ref) },
      });
      setSourceItems(page.items);
      setSourcePhase('ready');
    } catch {
      setSourcePhase('error');
    }
  };

  return (
    <div
      className="my-2 min-w-0 rounded-lg border border-line bg-raised/60"
      data-reference-card={reference.id}
      onKeyDown={(event) => {
        if (event.key === 'Escape' && open) {
          setExpanded(false);
          toggleRef.current?.focus();
        }
      }}
    >
      <button
        ref={toggleRef}
        type="button"
        className="flex h-6 w-full min-w-0 items-center gap-1.5 rounded-md px-1 text-left"
        aria-expanded={open}
        onClick={() => {
          const next = !open;
          setExpanded(next);
          if (next && view === undefined) void loadReferenceView(reference.id);
        }}
      >
        <span className="inline-flex h-4 w-4 shrink-0 items-center justify-center text-fg-muted">
          {open ? <ChevronDown className="h-3.5 w-3.5" /> : <BookMarked className="h-3.5 w-3.5" />}
        </span>
        <span className="shrink-0 text-ui leading-6 text-fg-muted">{t('referenceDetail.title')}</span>
        <span aria-hidden className="mx-1 h-0.5 w-0.5 shrink-0 rounded-full bg-muted-foreground/60" />
        <span className="min-w-0 flex-1 truncate text-ui leading-6 text-fg-muted/80">
          {reference.source_session_id} · {reference.items.length} {t('referenceDetail.records')} · {kindsLabel(reference.items, t)}
        </span>
        <Badge variant="outline" className="ml-1 shrink-0 text-2xs">
          {t(reference.origin === 'model_tool' ? 'referenceDetail.originModel' : 'referenceDetail.originUser')}
        </Badge>
      </button>

      {open ? (
        <div className="space-y-2 border-t border-line px-3 py-2">
          <div className="flex flex-wrap items-center gap-2 text-2xs text-fg-muted">
            <span>{t('referenceDetail.sourceStatus')} <Badge variant="outline">{status(view?.source_status ?? 'unknown')}</Badge></span>
            <span>{t('referenceDetail.feedStatus')} <Badge variant="outline">{status(view?.feed_status ?? 'unknown')}</Badge></span>
            <span className="ml-auto flex items-center gap-1">
              {sourcePhase === 'loading' ? <Loader2 className="h-3 w-3 animate-spin" /> : null}
              <button
                type="button"
                data-source-jump=""
                className="rounded px-1.5 py-0.5 text-2xs text-fg-muted underline-offset-2 enabled:hover:underline disabled:opacity-50"
                disabled={view?.source_status === undefined || !JUMPABLE.has(view.source_status)}
                onClick={() => void loadSource()}
              >
                {t('referenceDetail.viewSource')}
              </button>
            </span>
          </div>

          <div>
            <div className="mb-0.5 text-2xs font-medium tracking-wide text-fg-muted">{t('referenceDetail.excerpt')}</div>
            <div className="space-y-1">
              {reference.items.map((item, index) => (
                <div key={`${item.ref.message_id ?? item.ref.event_seq ?? index}`} className="rounded-md bg-sunken p-2">
                  <div className="text-2xs text-fg-muted">{item.author} · {t(KIND_KEYS[item.ref.kind] ?? 'chat.toolTitleGeneric')}</div>
                  <pre className={cn('overflow-x-auto whitespace-pre-wrap break-words text-meta leading-5', item.redacted && 'italic text-fg-muted')}>
                    {item.redacted ? t('referenceDetail.redacted') : item.text}
                  </pre>
                </div>
              ))}
            </div>
          </div>

          {sourcePhase === 'ready' && sourceItems !== null ? (
            <div>
              <div className="mb-0.5 text-2xs font-medium tracking-wide text-fg-muted">{t('referenceDetail.sourceLive')}</div>
              <div className="space-y-1">
                {sourceItems.map((item, index) => (
                  <div key={`live-${index}`} className="rounded-md bg-sunken p-2">
                    <div className="text-2xs text-fg-muted">{item.author}</div>
                    <pre className="overflow-x-auto whitespace-pre-wrap break-words text-meta leading-5">{item.text}</pre>
                  </div>
                ))}
              </div>
            </div>
          ) : null}
          {sourcePhase === 'error' ? <p className="text-2xs text-danger">{t('referenceDetail.sourceFailed')}</p> : null}

          <details className="text-2xs text-fg-muted">
            <summary className="cursor-pointer select-none">{t('referenceDetail.details')}</summary>
            <dl className="mt-1 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 font-mono">
              <dt>{t('referenceDetail.captured')}</dt><dd>{captured}</dd>
              <dt>{t('referenceDetail.sourceSession')}</dt><dd className="break-all">{reference.source_session_id}</dd>
              <dt>{t('referenceDetail.destination')}</dt><dd className="break-all">{reference.destination_session_id} / {reference.destination_run_id}</dd>
            </dl>
          </details>
        </div>
      ) : null}
    </div>
  );
}
