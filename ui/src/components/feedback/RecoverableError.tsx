import { Link } from '@tanstack/react-router';
import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { classifyFailure, type ClassifiedFailure } from '@/lib/failure';
import { useTranslation } from '@/i18n';
import { cn } from '@/lib/utils';

export function RecoverableError({
  error,
  onRetry,
  retryBusy,
  retryLabel,
  compact,
  className,
}: {
  error: unknown;
  onRetry?: () => void;
  retryBusy?: boolean;
  retryLabel?: string;
  compact?: boolean;
  className?: string;
}) {
  const failure = classifyFailure(error);
  return (
    <RecoverableFailure
      failure={failure}
      onRetry={onRetry}
      retryBusy={retryBusy}
      retryLabel={retryLabel}
      compact={compact}
      className={className}
    />
  );
}

export function RecoverableFailure({
  failure,
  onRetry,
  retryBusy,
  retryLabel,
  compact,
  className,
}: {
  failure: ClassifiedFailure;
  onRetry?: () => void;
  retryBusy?: boolean;
  retryLabel?: string;
  compact?: boolean;
  className?: string;
}) {
  const { t } = useTranslation();
  const showRetry = Boolean(onRetry);
  const showSettings = failure.action === 'open_model_settings';
  return (
    <div
      role="alert"
      className={cn(
        'rounded-xl border border-destructive/30 bg-destructive/5 text-left text-destructive',
        compact ? 'p-3' : 'p-6',
        className,
      )}
    >
      <div className={cn('flex gap-3', compact ? 'items-start' : 'flex-col items-center text-center')}>
        <AlertTriangle className={cn('shrink-0', compact ? 'mt-0.5 h-4 w-4' : 'h-5 w-5')} aria-hidden="true" />
        <div className={cn('min-w-0', compact ? 'flex-1' : 'w-full')}>
          <h2 className={cn('font-medium text-foreground', compact ? 'text-sm' : 'text-lg')}>{t(failure.titleKey)}</h2>
          <p className={cn('text-sm text-muted-foreground', compact ? 'mt-1' : 'mt-2')}>{failure.detail}</p>
          {failure.hintKey ? <p className={cn('text-xs text-muted-foreground/80', compact ? 'mt-1' : 'mt-2')}>{t(failure.hintKey)}</p> : null}
          {showRetry || showSettings ? (
            <div className={cn('flex flex-wrap gap-2', compact ? 'mt-3' : 'mt-4 justify-center')}>
              {showRetry ? (
                <Button size={compact ? 'sm' : 'default'} disabled={retryBusy} onClick={onRetry}>
                  {retryBusy ? t('errors.retrying') : retryLabel ?? t('common.retry')}
                </Button>
              ) : null}
              {showSettings ? (
                <Button asChild size={compact ? 'sm' : 'default'}>
                  <Link to="/settings" search={{ tab: 'model' }}>{t('errors.openModelSettings')}</Link>
                </Button>
              ) : null}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
