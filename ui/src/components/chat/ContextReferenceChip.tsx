import { Eye, X } from 'lucide-react';
import type { ReferencePreview } from '@/lib/api';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { dateTimeLocale, useTranslation } from '@/i18n';

export interface ContextReferenceChipProps {
  preview: ReferencePreview;
  onView?: () => void;
  onRemove?: () => void;
}

/**
 * Composer chip for one previewed reference attachment (SC-D4 §13.1). The
 * chip is metadata only: source session, record count and capture time —
 * never trusted excerpt authority.
 */
export function ContextReferenceChip({ preview, onView, onRemove }: ContextReferenceChipProps) {
  const { t } = useTranslation();
  const kinds = [...new Set(preview.items.map((item) => item.ref.kind))];
  const captured = new Date(preview.captured_at).toLocaleTimeString(dateTimeLocale(), { hour: '2-digit', minute: '2-digit' });
  return (
    <Badge
      variant="secondary"
      className="flex max-w-full items-center gap-1.5 py-1 pl-2 pr-1 font-normal"
      data-testid="context-reference-chip"
    >
      <span className="truncate text-xs" title={preview.selection.source_session_id}>
        {t('contextRef.chip', { session: preview.selection.source_session_id })}
      </span>
      <span className="shrink-0 text-[10px] text-muted-foreground">
        {t('contextRef.chipMeta', { count: preview.items.length, kinds: kinds.join('/'), time: captured })}
      </span>
      {onView ? (
        <Button
          type="button" variant="ghost" size="icon"
          className="h-5 w-5 shrink-0"
          aria-label={t('contextRef.view')}
          onClick={onView}
        >
          <Eye className="h-3 w-3" />
        </Button>
      ) : null}
      {onRemove ? (
        <Button
          type="button" variant="ghost" size="icon"
          className="h-5 w-5 shrink-0"
          aria-label={t('contextRef.remove')}
          onClick={onRemove}
        >
          <X className="h-3 w-3" />
        </Button>
      ) : null}
    </Badge>
  );
}
