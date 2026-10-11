import type { UITranslator } from '@vivy/ui-sdk';
import { Check, Copy, Pencil, Trash2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import type { MaskDraft } from './mask-state';
import { MaskIdentity } from './MaskIdentity';
import { isCustomMask, maskKind, type MaskChoice } from './mask-view';

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
  const editable = isCustomMask(mask);
  const originHint = mask.id === '' ? 'defaultHint'
    : mask.origin === 'bundle' ? 'bundleHint'
      : editable ? 'customHint' : 'builtInHint';
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
      {mask.bundle ? (
        <div className="space-y-2 rounded-xl border bg-muted/25 p-3 text-sm">
          <p className="font-medium">{t(mask.origin === 'bundle' ? 'plugin.vivy/masks-ui.bundleSource' : 'plugin.vivy/masks-ui.copiedFromToolkit')}</p>
          <p className="break-all text-xs text-muted-foreground">
            {mask.bundle.release.bundle_id} · {mask.bundle.local_id} · {t('plugin.vivy/masks-ui.release')} {mask.bundle.release.release_sequence}
          </p>
          <a className="inline-flex min-h-10 items-center text-primary underline-offset-4 hover:underline" href="/toolkits">
            {t('plugin.vivy/masks-ui.openToolkits')}
          </a>
        </div>
      ) : null}
      {mask.recommended_skills && mask.recommended_skills.length > 0 ? (
        <div className="rounded-xl border bg-muted/25 p-3 text-sm">
          <p className="font-medium">{t('plugin.vivy/masks-ui.recommendedSkills')}</p>
          <ul className="mt-1 list-inside list-disc text-xs text-muted-foreground">
            {mask.recommended_skills.map((skill) => (
              <li key={`${skill.ref.install_id}/${skill.ref.local_id}`}>
                {skill.name} · {skill.available ? t('plugin.vivy/masks-ui.skillAvailable') : t('plugin.vivy/masks-ui.skillUnavailable')}
              </li>
            ))}
          </ul>
          <p className="mt-1 text-xs text-muted-foreground">{t('plugin.vivy/masks-ui.recommendationOnly')}</p>
          <a className="inline-flex min-h-10 items-center text-primary underline-offset-4 hover:underline" href="/skills">
            {t('plugin.vivy/masks-ui.openRecommendedSkills')}
          </a>
        </div>
      ) : null}
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
            `plugin.vivy/masks-ui.${originHint}`,
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
            onClick={editable ? onEdit : onCopy}
          >
            {!editable ? (
              <Copy className="h-4 w-4" />
            ) : (
              <Pencil className="h-4 w-4" />
            )}
            {t(`plugin.vivy/masks-ui.${editable ? 'edit' : mask.origin === 'bundle' ? 'copyToCustom' : 'duplicate'}`)}
          </Button>
          {editable ? (
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
