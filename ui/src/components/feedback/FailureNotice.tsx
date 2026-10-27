import { useEffect, useRef, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { classifyFailure } from '@/lib/failure';
import { copyTextToClipboard } from '@/lib/clipboard';
import { useTranslation } from '@/i18n';
import { cn } from '@/lib/utils';

export type FailureNoticeVariant = 'full' | 'compact' | 'inline';

export function DiagnosticsDetails({ technical, className }: { technical: string; className?: string }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => () => { if (timer.current) clearTimeout(timer.current); }, []);
  const copy = async () => {
    if (!(await copyTextToClipboard(technical))) return;
    setCopied(true);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(false), 2000);
  };
  return (
    <details className={cn('group mt-2 text-meta text-fg-muted', className)}>
      <summary className="cursor-pointer select-none text-fg-muted hover:text-fg">{t('feedback.details')}</summary>
      <pre className="mt-1 max-h-32 overflow-auto whitespace-pre-wrap break-all rounded-md border border-line bg-background p-2 font-mono text-2xs text-fg-muted">{technical}</pre>
      <Button type="button" size="sm" variant="ghost" className="mt-1" onClick={() => void copy()}>
        {copied ? t('feedback.copied') : t('feedback.copyDiagnostics')}
      </Button>
    </details>
  );
}

export function FailureNotice({
  error,
  title,
  detail,
  technical,
  variant = 'compact',
  onRetry,
  retryBusy,
  retryLabel,
  className,
}: {
  error?: unknown;
  title?: string;
  detail?: string;
  technical?: string;
  variant?: FailureNoticeVariant;
  onRetry?: () => void;
  retryBusy?: boolean;
  retryLabel?: string;
  className?: string;
}) {
  const { t } = useTranslation();
  const failure = error === undefined ? undefined : classifyFailure(error);
  const resolvedTitle = title ?? (failure ? t(failure.titleKey) : undefined);
  const resolvedDetail = detail ?? failure?.detail;
  const resolvedTechnical = technical ?? failure?.technical;
  const showRetry = Boolean(onRetry);
  const showSettings = failure?.action === 'open_model_settings';
  const controlSize = variant === 'full' ? 'default' : 'sm';
  if (!resolvedTitle && !resolvedDetail) return null;
  return (
    <div
      role="alert"
      className={cn(
        'rounded-xl border border-line border-l-2 border-l-danger bg-raised text-left',
        variant === 'full' ? 'p-4' : variant === 'compact' ? 'p-3' : 'px-2 py-1.5',
        className,
      )}
    >
      <div className="flex gap-2.5">
        <AlertTriangle
          className={cn('shrink-0 text-danger', variant === 'full' ? 'mt-0.5 h-5 w-5' : 'mt-0.5 h-4 w-4')}
          aria-hidden="true"
        />
        <div className="min-w-0 flex-1">
          {resolvedTitle ? <h2 className={cn('font-medium text-fg', variant === 'inline' ? 'text-meta' : 'text-ui')}>{resolvedTitle}</h2> : null}
          {resolvedDetail ? <p className={cn('break-words text-fg-muted', variant === 'inline' ? 'text-meta' : 'mt-1 text-meta')}>{resolvedDetail}</p> : null}
          {failure?.hintKey && !title ? <p className="mt-1 text-meta text-fg-muted">{t(failure.hintKey)}</p> : null}
          {showRetry || showSettings ? (
            <div className="mt-2 flex flex-wrap items-center gap-2">
              {showRetry ? (
                <Button type="button" size={controlSize} variant="outline" disabled={retryBusy} onClick={onRetry}>
                  {retryBusy ? t('errors.retrying') : retryLabel ?? t('common.retry')}
                </Button>
              ) : null}
              {showSettings ? (
                <Button asChild size={controlSize} variant="outline">
                  <Link to="/settings" search={{ tab: 'model' }}>{t('errors.openModelSettings')}</Link>
                </Button>
              ) : null}
            </div>
          ) : null}
          {resolvedTechnical ? <DiagnosticsDetails technical={resolvedTechnical} /> : null}
        </div>
      </div>
    </div>
  );
}
