import { Clock, X } from 'lucide-react';
import type { QueuedTurn } from '@/lib/api';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/i18n';

export type KernelQueueItem = { item: QueuedTurn; lane: 'steer' | 'follow_up' };

interface ComposerQueueProps {
  kernel: KernelQueueItem[];
  local: { id: string; text: string }[];
  onRemoveKernel: (id: string) => Promise<unknown> | unknown;
  onRemoveLocal: (id: string) => void;
  onClear: () => void;
  onSendNow: () => Promise<unknown> | unknown;
  busy: boolean;
}

/**
 * Kernel steer/follow-up entries and the local FIFO (attachments, references)
 * in one list. Text clamps to two lines with the full value in the title, so a
 * long queued prompt stays readable at 390px instead of scrolling sideways.
 */
export function ComposerQueue({ kernel, local, onRemoveKernel, onRemoveLocal, onClear, onSendNow, busy }: ComposerQueueProps) {
  const { t } = useTranslation();
  const total = kernel.length + local.length;
  if (!total) return null;

  return (
    <div className="flex items-start gap-2 border-t border-line bg-sunken/40 px-3 py-1.5 text-meta text-fg-muted">
      <Clock className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
      <span className="shrink-0 font-medium">{t('chatInput.queuedCount', { count: total })}</span>
      <div className="flex min-w-0 flex-1 flex-wrap gap-1.5" aria-live="polite">
        {kernel.map(({ item, lane }) => (
          <span key={item.id} title={item.text} className="flex max-w-full items-start gap-1 rounded-lg bg-sunken px-2 py-0.5">
            <span className={cn('mt-0.5 shrink-0 rounded px-1 text-2xs font-semibold', lane === 'steer' ? 'bg-brand-soft text-brand-ink' : 'bg-selected text-fg-muted')}>
              {t(lane === 'steer' ? 'chatInput.queueLaneSteer' : 'chatInput.queueLaneFollowUp')}
            </span>
            <span className="line-clamp-2 min-w-0 break-words">{item.text}</span>
            <button type="button" onClick={() => void onRemoveKernel(item.id)} title={t('chatInput.removeQueued')} aria-label={`${t('chatInput.removeQueued')}: ${item.text}`} className="shrink-0 rounded p-0.5 transition-colors hover:bg-hover hover:text-fg"><X className="h-3 w-3" aria-hidden="true" /></button>
          </span>
        ))}
        {local.map((item) => (
          <span key={item.id} title={item.text} className="flex max-w-full items-start gap-1 rounded-lg bg-sunken px-2 py-0.5">
            <span className="line-clamp-2 min-w-0 break-words">{item.text}</span>
            <button type="button" onClick={() => onRemoveLocal(item.id)} title={t('chatInput.removeQueued')} aria-label={`${t('chatInput.removeQueued')}: ${item.text}`} className="shrink-0 rounded p-0.5 transition-colors hover:bg-hover hover:text-fg"><X className="h-3 w-3" aria-hidden="true" /></button>
          </span>
        ))}
      </div>
      <button type="button" onClick={() => void onSendNow()} disabled={busy} className="shrink-0 rounded-lg px-2 py-0.5 font-medium text-brand-ink transition-colors hover:bg-hover disabled:opacity-50" title={t('chatInput.sendQueuedNow')} aria-label={t('chatInput.sendQueuedNow')}>{t('chatInput.sendQueuedNow')}</button>
      <button type="button" onClick={onClear} className="shrink-0 rounded-lg px-2 py-0.5 transition-colors hover:bg-hover hover:text-fg" title={t('chatInput.clearQueue')} aria-label={t('chatInput.clearQueue')}>{t('chatInput.clearQueue')}</button>
    </div>
  );
}
