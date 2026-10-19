import { useEffect, useRef, type PointerEvent as ReactPointerEvent } from 'react';
import { GripVertical, X } from 'lucide-react';
import type { ReviewItem } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { useTranslation } from '@/i18n';

/** Card placement relative to its default anchor (above the composer, right aligned). */
export type QuickOffset = { x: number; y: number };

/** Viewport edges of the card without any drag offset, in viewport coordinates. */
export type NaturalRect = { left: number; right: number; top: number; bottom: number };

const VIEWPORT_MARGIN_PX = 8;

/**
 * The pending item the card decides: the active session's oldest pending review
 * first (that is what the composer is about), otherwise the oldest pending
 * review overall. Returns null when nothing is pending.
 */
export function pickPendingReview(reviews: ReviewItem[], activeSessionId: string | null): ReviewItem | null {
  const pending = reviews.filter((review) => review.status === 'pending');
  const scoped = activeSessionId === null ? [] : pending.filter((review) => review.session_id === activeSessionId);
  const pool = scoped.length ? scoped : pending;
  if (!pool.length) return null;
  return pool.reduce((oldest, review) => (review.created_at < oldest.created_at ? review : oldest));
}

/** Keeps a dragged card fully inside the viewport, given its offset-free edges. */
export function clampOffset(natural: NaturalRect, viewport: { width: number; height: number }, offset: QuickOffset): QuickOffset {
  const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), Math.max(min, max));
  return {
    x: clamp(offset.x, VIEWPORT_MARGIN_PX - natural.left, viewport.width - VIEWPORT_MARGIN_PX - natural.right),
    y: clamp(offset.y, VIEWPORT_MARGIN_PX - natural.top, viewport.height - VIEWPORT_MARGIN_PX - natural.bottom),
  };
}

/** One pending review with inline Approve/Deny, floating above the composer. */
export function ApprovalQuickCard({
  review, activeSessionId = null, pendingCount, busy, error, offset, onOffsetChange, onRespond, onOpenCenter, onClose,
}: {
  review: ReviewItem | null;
  /** The session the composer belongs to; a review from any other session is labelled. */
  activeSessionId?: string | null;
  pendingCount: number;
  busy: boolean;
  error: string | null;
  offset: QuickOffset;
  onOffsetChange: (offset: QuickOffset) => void;
  onRespond: (action: 'approve' | 'deny') => Promise<void> | void;
  onOpenCenter: () => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const cardRef = useRef<HTMLDivElement>(null);
  const dragRef = useRef<{
    pointerId: number;
    startX: number;
    startY: number;
    origin: QuickOffset;
    natural: NaturalRect;
  } | null>(null);

  // A shrunk window can strand a previously dragged card outside the viewport;
  // the default anchor is always safe, so fall back to it.
  useEffect(() => {
    const onResize = () => {
      const rect = cardRef.current?.getBoundingClientRect();
      if (!rect) return;
      if (rect.left < 0 || rect.top < 0 || rect.right > window.innerWidth || rect.bottom > window.innerHeight) onOffsetChange({ x: 0, y: 0 });
    };
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, [onOffsetChange]);

  const handlePointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) return;
    const rect = cardRef.current?.getBoundingClientRect();
    if (!rect) return;
    dragRef.current = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      origin: offset,
      natural: { left: rect.left - offset.x, right: rect.right - offset.x, top: rect.top - offset.y, bottom: rect.bottom - offset.y },
    };
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const handlePointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) return;
    onOffsetChange(clampOffset(drag.natural, { width: window.innerWidth, height: window.innerHeight }, {
      x: drag.origin.x + event.clientX - drag.startX,
      y: drag.origin.y + event.clientY - drag.startY,
    }));
  };

  const handlePointerUp = (event: ReactPointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) return;
    dragRef.current = null;
    event.currentTarget.releasePointerCapture(event.pointerId);
  };

  // The store records respond failures in `reviewsError`; swallow the rejection here
  // so a failed decision never becomes an unhandled promise.
  const respond = async (action: 'approve' | 'deny') => {
    try {
      await onRespond(action);
    } catch {
      return;
    }
  };

  const title = review === null
    ? t('approvals.quick.title')
    : review.kind === 'approval'
      ? review.tool_name || review.action || t('approvals.toolApproval')
      : t('approvals.userQuestion');
  const command = review === null ? '' : review.preview?.trim() || review.target || review.action || '';
  const risk = review?.risk_findings?.[0];
  const remaining = Math.max(0, pendingCount - 1);
  const otherSession = review !== null && review.session_id !== activeSessionId ? review.session_title || review.session_id : null;

  return (
    <div
      ref={cardRef}
      id="approval-quick-card"
      role="region"
      aria-label={t('approvals.quick.title')}
      onKeyDown={(event) => { if (event.key === 'Escape') onClose(); }}
      style={{ transform: `translate(${offset.x}px, ${offset.y}px)` }}
      className="absolute right-0 bottom-full z-30 mb-3 w-[min(22rem,calc(100vw-2rem))] rounded-xl border border-border bg-card shadow-lg"
    >
      <div
        className="flex cursor-grab touch-none select-none items-center gap-2 rounded-t-xl border-b border-border/60 px-3 py-2"
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerUp}
      >
        <GripVertical className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <span className="min-w-0 flex-1 truncate text-sm font-medium" title={t('approvals.quick.drag')}>{title}</span>
        {remaining > 0 ? <span className="shrink-0 text-xs text-muted-foreground">{t('approvals.quick.remaining', { count: remaining })}</span> : null}
        <button
          type="button"
          onPointerDown={(event) => event.stopPropagation()}
          onClick={onClose}
          title={t('approvals.quick.close')}
          aria-label={t('approvals.quick.close')}
          className="shrink-0 rounded-lg p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      </div>
      <div className="space-y-2 px-3 py-2.5">
        {otherSession ? <p className="truncate text-xs font-medium text-amber-600 dark:text-amber-400" title={otherSession}>{t('approvals.quick.otherSession', { session: otherSession })}</p> : null}
        {review === null ? (
          <p className="text-sm text-muted-foreground">{t('approvals.quick.empty')}</p>
        ) : review.kind === 'question' ? (
          <>
            <p className="text-xs text-muted-foreground">{t('approvals.quick.questionHint')}</p>
            {review.prompt ? <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-muted p-2 font-mono text-xs">{review.prompt}</pre> : null}
          </>
        ) : (
          <>
            <p className="text-xs text-muted-foreground">{t('approvals.quick.command')}</p>
            {command ? <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-muted p-2 font-mono text-xs">{command}</pre> : null}
            {risk ? <p className="text-xs text-destructive">{t('approvals.risk')}: {risk}</p> : null}
          </>
        )}
        {error ? <p className="text-xs text-destructive" role="alert">{error}</p> : null}
      </div>
      <div className="flex items-center gap-2 border-t border-border/60 px-3 py-2">
        {review?.kind === 'approval' ? (
          <>
            <Button size="sm" disabled={busy} onClick={() => void respond('approve')}>{busy ? t('approvals.processing') : t('approvals.approve')}</Button>
            <Button size="sm" variant="destructive" disabled={busy} onClick={() => void respond('deny')}>{t('approvals.deny')}</Button>
          </>
        ) : null}
        <Button size="sm" variant="ghost" className="ml-auto" onClick={onOpenCenter}>{t('approvals.quick.openCenter')}</Button>
      </div>
    </div>
  );
}
