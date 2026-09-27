// 交付组卡片（SC-D4 §12）：渲染已提交 deliverables.presented 的不可变
// DeliverySet。可用性初始为 unchecked —— 用户点开验证/预览/下载时才走
// deliverables/read 探活；任何条目失败仍然展示（含 failures 列表），
// changed/missing 显示稳定的解释性状态而不是静默消失。
import { useState } from 'react';
import { Check, ChevronDown, ChevronRight, Download, Eye, FileWarning, Loader2, Package, RefreshCw } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { useTranslation } from '@/i18n';
import * as api from '@/lib/api';
import { isTextPreviewable } from '@/lib/deliverable-download';
import { useVivyStore } from '@/lib/store';
import { cn } from '@/lib/utils';

function formatBytes(size: number): string {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

const STATUS_KEYS: api.DeliveryItemStatus[] = ['unchecked', 'checking', 'available', 'downloading', 'downloaded', 'changed', 'missing', 'forbidden', 'unavailable'];
const SET_STATUS_KEYS = new Set(['ok', 'partial', 'failed']);

function DeliverableItemRow({ item, setId }: { item: api.Deliverable; setId: string }) {
  const { t } = useTranslation();
  const state = useVivyStore((s) => s.deliveryItemStates[item.id]);
  const checkDeliveryItem = useVivyStore((s) => s.checkDeliveryItem);
  const previewDeliveryItem = useVivyStore((s) => s.previewDeliveryItem);
  const downloadDeliveryItem = useVivyStore((s) => s.downloadDeliveryItem);
  const cancelDeliveryDownload = useVivyStore((s) => s.cancelDeliveryDownload);
  const [previewOpen, setPreviewOpen] = useState(false);
  const status: api.DeliveryItemStatus = state?.status ?? 'unchecked';
  const busy = status === 'checking' || status === 'downloading';
  const previewable = isTextPreviewable(item.media_type);
  const failureReason = ['changed', 'missing', 'forbidden', 'unavailable'].includes(status);

  return (
    <li className="rounded-md border border-border/60 bg-background/50 px-2 py-1.5" data-delivery-item={item.id} data-delivery-set={setId}>
      <div className="flex min-w-0 items-start gap-2">
        <span className="mt-0.5 shrink-0 text-muted-foreground">
          <Package className="h-3.5 w-3.5" aria-hidden />
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate text-xs font-medium" title={item.path}>
            {item.name}
          </p>
          <p className="truncate text-[11px] text-muted-foreground" title={item.path}>
            {item.path} · {formatBytes(item.size)}
            {item.media_type !== '' ? ` · ${item.media_type}` : ''}
          </p>
          {item.description !== '' ? <p className="mt-0.5 line-clamp-2 text-[11px] text-muted-foreground">{item.description}</p> : null}
          <p
            className={cn('mt-0.5 text-[11px]', failureReason ? 'text-amber-600' : 'text-muted-foreground')}
            role="status"
            aria-live="polite"
          >
            {t(`deliverableCard.state.${status}`)}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-1" role="group" aria-label={t('deliverableCard.actions', { name: item.name })}>
          {status === 'unchecked' || failureReason ? (
            <button
              type="button"
              className="inline-flex h-6 items-center gap-1 rounded border border-border px-1.5 text-[11px] text-muted-foreground hover:bg-accent disabled:opacity-50"
              aria-label={t('deliverableCard.verify', { name: item.name })}
              disabled={busy}
              onClick={() => void checkDeliveryItem(item)}
            >
              {status === 'checking' ? <Loader2 className="h-3 w-3 animate-spin" aria-hidden /> : <RefreshCw className="h-3 w-3" aria-hidden />}
              {t('deliverableCard.verifyShort')}
            </button>
          ) : null}
          {previewable && !failureReason ? (
            <button
              type="button"
              className="inline-flex h-6 items-center gap-1 rounded border border-border px-1.5 text-[11px] text-muted-foreground hover:bg-accent disabled:opacity-50"
              aria-label={t('deliverableCard.preview', { name: item.name })}
              aria-expanded={previewOpen}
              disabled={busy}
              onClick={() => {
                const next = !previewOpen;
                setPreviewOpen(next);
                if (next && state?.preview === undefined) void previewDeliveryItem(item);
              }}
            >
              {previewOpen ? <ChevronDown className="h-3 w-3" aria-hidden /> : <Eye className="h-3 w-3" aria-hidden />}
              {t('deliverableCard.previewShort')}
            </button>
          ) : null}
          {status === 'downloading' ? (
            <button
              type="button"
              className="inline-flex h-6 items-center gap-1 rounded border border-border px-1.5 text-[11px] text-muted-foreground hover:bg-accent"
              aria-label={t('deliverableCard.cancel', { name: item.name })}
              onClick={() => cancelDeliveryDownload(item.id)}
            >
              <Loader2 className="h-3 w-3 animate-spin" aria-hidden />
              {t('deliverableCard.cancelShort')}
            </button>
          ) : (
            <button
              type="button"
              className="inline-flex h-6 items-center gap-1 rounded border border-border px-1.5 text-[11px] text-muted-foreground hover:bg-accent disabled:opacity-50"
              aria-label={t('deliverableCard.download', { name: item.name })}
              disabled={busy}
              onClick={() => void downloadDeliveryItem(item)}
            >
              {status === 'downloaded' ? <Check className="h-3 w-3" aria-hidden /> : <Download className="h-3 w-3" aria-hidden />}
              {t('deliverableCard.downloadShort')}
            </button>
          )}
        </div>
      </div>
      {previewOpen && state?.preview !== undefined ? (
        <pre className="mt-1.5 max-h-40 overflow-auto rounded bg-muted/50 p-2 text-[11px] whitespace-pre-wrap" data-delivery-preview={item.id}>
          {state.preview}
        </pre>
      ) : previewOpen ? (
        <p className="mt-1.5 text-[11px] text-muted-foreground">{t('deliverableCard.previewLoading')}</p>
      ) : null}
    </li>
  );
}

export function DeliverableCard({ set }: { set: api.DeliverySet }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(true);
  const failed = set.failures.length;
  const statusKey = SET_STATUS_KEYS.has(set.status) ? set.status : 'partial';

  return (
    <section
      className="my-2 min-w-0 rounded-lg border border-border bg-card/60"
      data-delivery-set={set.id}
      id={`deliverable-${set.id}`}
      aria-label={t('deliverableCard.title')}
    >
      <button
        type="button"
        className="flex h-7 w-full min-w-0 items-center gap-1.5 rounded-md px-2 text-left"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        <span className="inline-flex h-4 w-4 shrink-0 items-center justify-center text-muted-foreground">
          {open ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
        </span>
        <span className="truncate text-[13px] leading-6 text-muted-foreground">
          {(set.title ?? '') !== '' ? `${set.title} · ` : ''}
          {t('deliverableCard.summary', { count: set.items.length })}
          {failed > 0 ? ` · ${t('deliverableCard.failures', { count: failed })}` : ''}
        </span>
        {statusKey !== 'ok' ? (
          <Badge variant="outline" className="ml-auto shrink-0 text-[10px]">
            {t(`deliverableCard.status.${statusKey}`)}
          </Badge>
        ) : null}
      </button>
      {open ? (
        <div className="px-2 pb-2">
          <ul className="space-y-1">
            {set.items.map((item) => (
              <DeliverableItemRow key={item.id} item={item} setId={set.id} />
            ))}
          </ul>
          {failed > 0 ? (
            <ul className="mt-1 space-y-1" aria-label={t('deliverableCard.failures', { count: failed })}>
              {set.failures.map((failure) => (
                <li key={failure.path} className="flex items-start gap-2 rounded-md border border-dashed border-border/60 px-2 py-1">
                  <FileWarning className="mt-0.5 h-3 w-3 shrink-0 text-amber-600" aria-hidden />
                  <div className="min-w-0 text-[11px] text-muted-foreground">
                    <span className="font-medium text-foreground/80">{failure.path}</span>
                    <span className="ml-1">{failure.reason}</span>
                  </div>
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
