import { createPortal } from 'react-dom';
import { AlertTriangle, CheckCircle2, Info, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { dismissToast, pauseToasts, resumeToasts, useToasts, type ToastRecord } from '@/lib/toasts';
import { useTranslation } from '@/i18n';
import { cn } from '@/lib/utils';
import { DiagnosticsDetails } from './FailureNotice';

const TONE = {
  danger: { rail: 'border-l-danger', icon: 'text-danger', Icon: AlertTriangle },
  warn: { rail: 'border-l-warn', icon: 'text-warn', Icon: AlertTriangle },
  ok: { rail: 'border-l-ok', icon: 'text-ok', Icon: CheckCircle2 },
  info: { rail: 'border-l-info', icon: 'text-info', Icon: Info },
} as const;

function ToastCard({ toast }: { toast: ToastRecord }) {
  const { t } = useTranslation();
  const tone = TONE[toast.tone];
  const Icon = tone.Icon;
  const urgent = toast.tone === 'danger' || toast.tone === 'warn';
  return (
    <div
      role={urgent ? 'alert' : 'status'}
      className={cn(
        'pointer-events-auto rounded-xl border border-line border-l-2 bg-raised p-3 shadow-lg motion-safe:animate-in motion-safe:fade-in-0 motion-safe:slide-in-from-right-2 motion-safe:duration-150',
        tone.rail,
      )}
    >
      <div className="flex gap-2.5">
        <Icon className={cn('mt-0.5 h-4 w-4 shrink-0', tone.icon)} aria-hidden="true" />
        <div className="min-w-0 flex-1">
          <div className="flex items-start gap-2">
            <p className="min-w-0 flex-1 break-words text-ui font-medium text-fg">{toast.title}</p>
            {toast.count > 1 ? (
              <span className="shrink-0 rounded-full bg-background px-1.5 text-2xs leading-5 text-fg-muted">×{toast.count}</span>
            ) : null}
            <Button
              type="button"
              size="icon"
              variant="ghost"
              className="-mr-1 -mt-1 h-6 w-6 shrink-0"
              aria-label={t('feedback.dismiss')}
              title={t('feedback.dismiss')}
              onClick={() => dismissToast(toast.id)}
            >
              <X className="h-3.5 w-3.5" />
            </Button>
          </div>
          {toast.detail ? <p className="mt-1 line-clamp-2 break-words text-meta text-fg-muted">{toast.detail}</p> : null}
          {toast.action ? (
            <Button type="button" size="sm" variant="outline" className="mt-2" onClick={toast.action.onClick}>
              {toast.action.label}
            </Button>
          ) : null}
          {toast.technical ? <DiagnosticsDetails technical={toast.technical} /> : null}
        </div>
      </div>
    </div>
  );
}

export function ToastViewport() {
  const toasts = useToasts();
  if (typeof document === 'undefined' || toasts.length === 0) return null;
  return createPortal(
    <div
      className="pointer-events-none fixed right-3 top-16 z-[300] flex w-[min(380px,calc(100vw-1.5rem))] flex-col gap-2"
      onMouseEnter={pauseToasts}
      onMouseLeave={resumeToasts}
      onFocus={pauseToasts}
      onBlur={resumeToasts}
      onKeyDown={(event) => {
        if (event.key === 'Escape' && toasts[0]) {
          event.preventDefault();
          dismissToast(toasts[0].id);
        }
      }}
    >
      {toasts.map((toast) => <ToastCard key={toast.id} toast={toast} />)}
    </div>,
    document.body,
  );
}
