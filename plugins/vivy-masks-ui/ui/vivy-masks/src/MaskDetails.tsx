import type { UITranslator } from '@vivy/ui-sdk';
import { Check, Copy, Pencil, Trash2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import type { MaskDraft } from './mask-state';
import { MaskIdentity } from './MaskIdentity';
import { maskKind, type MaskChoice } from './mask-view';

export function MaskDetails({
  mask,
  draft,
  active,
  pending,
  canSelect,
  onUse,
  onEdit,
  onCopy,
  onDelete,
  t,
}: {
  readonly mask: MaskChoice;
  readonly draft: MaskDraft | null;
  readonly active: boolean;
  readonly pending: boolean;
  readonly canSelect: boolean;
  readonly onUse: () => void;
  readonly onEdit: () => void;
  readonly onCopy: () => void;
  readonly onDelete: () => void;
  readonly t: UITranslator;
}) {
  return (
    <section
      className="min-w-0 space-y-5"
      aria-label={t('plugin.vivy/masks-ui.editorTitle')}
    >
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-xs font-medium text-muted-foreground">
          {t('plugin.vivy/masks-ui.editorTitle')}
        </span>
        <Badge variant="secondary">{maskKind(t, mask)}</Badge>
      </div>
      <MaskIdentity id={mask.id} large />
      <div>
        <h2 className="break-words text-2xl font-semibold tracking-tight">
          {mask.name}
        </h2>
        <p className="mt-2 break-words text-sm leading-relaxed text-muted-foreground">
          {mask.description}
        </p>
      </div>
      <div className="space-y-3 border-t pt-5">
        <h3 className="text-sm font-medium">
          {t(`plugin.vivy/masks-ui.${mask.id ? 'body' : 'defaultIdentity'}`)}
        </h3>
        <div
          data-mask-instructions
          className="max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-xl bg-muted/45 p-4 text-sm leading-relaxed text-muted-foreground"
        >
          {mask.id
            ? pending
              ? t('plugin.vivy/masks-ui.loading')
              : draft?.body || t('plugin.vivy/masks-ui.detailsUnavailable')
            : t('plugin.vivy/masks-ui.defaultBody')}
        </div>
        <p className="text-xs leading-relaxed text-muted-foreground">
          {t(
            `plugin.vivy/masks-ui.${mask.id ? (mask.built_in ? 'builtInHint' : 'customHint') : 'defaultHint'}`,
          )}
        </p>
      </div>
      <Button
        className="w-full"
        variant={active ? 'secondary' : 'default'}
        disabled={!canSelect || active}
        onClick={onUse}
      >
        {active ? <Check className="h-4 w-4" /> : null}
        {active
          ? t('plugin.vivy/masks-ui.inUse')
          : t('plugin.vivy/masks-ui.useMask', { name: mask.name })}
      </Button>
      {mask.id ? (
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            disabled={pending || !draft}
            onClick={mask.built_in ? onCopy : onEdit}
          >
            {mask.built_in ? (
              <Copy className="h-4 w-4" />
            ) : (
              <Pencil className="h-4 w-4" />
            )}
            {t(`plugin.vivy/masks-ui.${mask.built_in ? 'duplicate' : 'edit'}`)}
          </Button>
          {!mask.built_in ? (
            <Button
              variant="ghost"
              className="text-destructive hover:text-destructive"
              disabled={pending || !draft}
              onClick={onDelete}
            >
              <Trash2 className="h-4 w-4" />
              {t('plugin.vivy/masks-ui.delete')}
            </Button>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
