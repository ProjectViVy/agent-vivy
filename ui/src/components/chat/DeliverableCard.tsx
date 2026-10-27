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

const SET_STATUS_KEYS = new Set(['ok', 'partial', 'failed']);
const PREVIEW_LINES = 12;
const ACTION_CLASS =
  'inline-flex h-7 items-center gap-1 rounded border border-line px-1.5 text-2xs text-fg-muted hover:bg-hover disabled:opacity-50';

function formatBytes(size: number): string {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function DeliverableItemRow({ item, setId }: { item: api.Deliverable; setId: string }) {
  const { t } = useTranslation();
  const state = useVivyStore((s) => s.deliveryItemStates[item.id]);
  const checkDeliveryItem = useVivyStore((s) => s.checkDeliveryItem);
  const previewDeliveryItem = useVivyStore((s) => s.previewDeliveryItem);
  const downloadDeliveryItem = useVivyStore((s) => s.downloadDeliveryItem);
  const cancelDeliveryDownload = useVivyStore((s) => s.cancelDeliveryDownload);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [previewExpanded, setPreviewExpanded] = useState(false);
  const status: api.DeliveryItemStatus = state?.status ?? 'unchecked';
  const busy = status === 'checking' || status === 'downloading';
  const previewable = isTextPreviewable(item.media_type);
  const failureReason = ['changed', 'missing', 'forbidden', 'unavailable'].includes(status);
  const previewLines = state?.preview?.split('\n') ?? [];
  const previewHidden = !previewExpanded && previewLines.length > PREVIEW_LINES;
  const shownPreview = previewHidden ? previewLines.slice(0, PREVIEW_LINES).join('\n') : state?.preview;

  return (
    <li className="rounded-md border border-line/60 bg-canvas/50 px-2 py-1.5" data-delivery-item={item.id} data-delivery-set={setId}>
      <div className="flex min-w-0 items-start gap-2">
        <span className="mt-0.5 shrink-0 text-fg-muted">
          <Package className="h-3.5 w-3.5" aria-hidden />
        </span>
        <div className="min-w-0 flex-1">
          <p className="break-words text-ui font-medium" title={item.name}>
            {item.name}
          </p>
          <p className="break-all text-2xs text-fg-muted" title={item.path}>
            {item.path} · {formatBytes(item.size)}
            {item.media_type !== '' ? ` · ${item.media_type}` : ''}
          </p>
          {item.description !== '' ? <p className="mt-0.5 text-meta text-fg-muted">{item.description}</p> : null}
          <p
            className={cn('mt-0.5 text-2xs', failureReason ? 'text-warn' : 'text-fg-muted')}
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
              className={ACTION_CLASS}
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
              className={ACTION_CLASS}
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
              className={ACTION_CLASS}
              aria-label={t('deliverableCard.cancel', { name: item.name })}
              onClick={() => cancelDeliveryDownload(item.id)}
            >
              <Loader2 className="h-3 w-3 animate-spin" aria-hidden />
              {t('deliverableCard.cancelShort')}
            </button>
          ) : (
            <button
              type="button"
              className={ACTION_CLASS}
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
      {previewOpen && shownPreview !== undefined ? (
        <div className="mt-1.5 rounded bg-sunken/60 p-2">
          <pre className="overflow-x-auto whitespace-pre-wrap break-words text-2xs leading-ui" data-delivery-preview={item.id}>
            {shownPreview}
          </pre>
          {previewHidden ? (
            <button
              type="button"
              className="mt-1 text-2xs text-fg-muted underline-offset-2 hover:underline"
              onClick={() => setPreviewExpanded(true)}
            >
              {t('deliverableCard.showMore')}
            </button>
          ) : previewExpanded ? (
            <button
              type="button"
              className="mt-1 text-2xs text-fg-muted underline-offset-2 hover:underline"
              onClick={() => setPreviewExpanded(false)}
            >
              {t('deliverableCard.showLess')}
            </button>
          ) : null}
        </div>
      ) : previewOpen ? (
        <p className="mt-1.5 text-2xs text-fg-muted">{t('deliverableCard.previewLoading')}</p>
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
      className="my-2 min-w-0 rounded-lg border border-line bg-raised/60"
      data-delivery-set={set.id}
      id={`deliverable-${set.id}`}
      aria-label={t('deliverableCard.title')}
    >
      <button
        type="button"
        className="flex min-h-7 w-full min-w-0 items-center gap-1.5 rounded-md px-2 py-1 text-left"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        <span className="inline-flex h-4 w-4 shrink-0 items-center justify-center text-fg-muted">
          {open ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
        </span>
        <span className="line-clamp-2 min-w-0 flex-1 break-words text-ui text-fg-muted">
          {(set.title ?? '') !== '' ? `${set.title} · ` : ''}
          {t('deliverableCard.summary', { count: set.items.length })}
          {failed > 0 ? ` · ${t('deliverableCard.failures', { count: failed })}` : ''}
        </span>
        {statusKey !== 'ok' ? (
          <Badge variant="outline" className="ml-auto shrink-0 text-2xs">
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
                <li key={failure.path} className="flex items-start gap-2 rounded-md border border-dashed border-line/60 px-2 py-1">
                  <FileWarning className="mt-0.5 h-3 w-3 shrink-0 text-warn" aria-hidden />
                  <div className="min-w-0 break-words text-2xs text-fg-muted">
                    <span className="font-medium text-fg/80">{failure.path}</span>
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
