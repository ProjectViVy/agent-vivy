import { Check, Pencil, Trash2, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/i18n';

interface RenameState {
  editing: boolean;
  draft: string;
  onDraftChange: (value: string) => void;
  onStart: () => void;
  onSave: () => void;
  onCancel: () => void;
}

interface Props {
  title: string;
  /** Untitled sessions render their localized placeholder muted. */
  untitled?: boolean;
  subtitle?: string;
  active?: boolean;
  busy?: boolean;
  onSelect: () => void;
  /** Surfaces that cannot rename in place omit this. */
  rename?: RenameState;
  onDelete?: () => void;
}

/**
 * One session identity row for every list that shows sessions: the sidebar and
 * the history picker. Selection, busy state and the placeholder title are the
 * row's; a surface only decides whether it also offers rename and delete.
 */
export function SessionRow({ title, untitled, subtitle, active, busy, onSelect, rename, onDelete }: Props) {
  const { t } = useTranslation();
  const shell = cn(
    'group relative flex min-h-8 rounded-lg text-sm transition-colors',
    active ? 'bg-sidebar-accent text-sidebar-accent-foreground' : 'hover:bg-sidebar-accent/50',
    busy && 'opacity-60',
  );

  if (rename?.editing) {
    return (
      <div className={cn(shell, 'items-center gap-1 px-2.5 py-1.5')}>
        <input
          autoFocus
          value={rename.draft}
          onChange={(event) => rename.onDraftChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') rename.onSave();
            if (event.key === 'Escape') rename.onCancel();
          }}
          className="min-w-0 flex-1 rounded border bg-background px-2 py-0.5 text-sm outline-none focus:border-primary"
        />
        <button type="button" onClick={rename.onSave} title={t('common.save')} aria-label={t('common.save')} className="rounded p-0.5 hover:bg-sidebar-accent">
          <Check className="h-3.5 w-3.5" />
        </button>
        <button type="button" onClick={rename.onCancel} title={t('common.cancel')} aria-label={t('common.cancel')} className="rounded p-0.5 hover:bg-sidebar-accent">
          <X className="h-3.5 w-3.5" />
        </button>
      </div>
    );
  }

  return (
    <div className={shell}>
      <button type="button" onClick={onSelect} className="flex min-w-0 flex-1 flex-col justify-center gap-0.5 px-2.5 py-1.5 text-left">
        <span
          title={title}
          className={cn('min-w-0 truncate', (rename || onDelete) && 'group-hover:pr-14', untitled && 'text-muted-foreground')}
        >
          {title}
        </span>
        {subtitle ? <span className="min-w-0 truncate text-xs text-muted-foreground">{subtitle}</span> : null}
      </button>
      {rename || onDelete ? (
        <div className="pointer-events-none absolute inset-y-0 right-1 flex items-center gap-0.5 opacity-0 group-hover:pointer-events-auto group-hover:opacity-100 group-focus-within:pointer-events-auto group-focus-within:opacity-100">
          {rename ? (
            <button type="button" onClick={rename.onStart} title={t('common.edit')} aria-label={t('common.edit')} className="rounded p-1 hover:bg-sidebar-accent">
              <Pencil className="h-3.5 w-3.5" />
            </button>
          ) : null}
          {onDelete ? (
            <button type="button" onClick={onDelete} title={t('common.delete')} aria-label={t('common.delete')} className="rounded p-1 text-destructive hover:bg-destructive/10">
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
