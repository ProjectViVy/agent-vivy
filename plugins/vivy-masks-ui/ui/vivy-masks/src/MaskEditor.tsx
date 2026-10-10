import type { UITranslator } from '@vivy/ui-sdk';
import { useId } from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { MaskIdentity } from './MaskIdentity';
import type { MaskDraft } from './mask-state';

export function MaskEditor({
  draft,
  open,
  mobile,
  saving,
  canUse,
  error,
  onClose,
  onChange,
  onSave,
  onReload,
  t,
}: {
  readonly draft: MaskDraft | null;
  readonly open: boolean;
  readonly mobile: boolean;
  readonly saving: boolean;
  readonly canUse: boolean;
  readonly error: string | null;
  readonly onClose: () => void;
  readonly onChange: (
    field: 'name' | 'description' | 'body',
    value: string,
  ) => void;
  readonly onSave: (use: boolean) => void;
  readonly onReload?: () => void;
  readonly t: UITranslator;
}) {
  const fieldId = useId();
  const retrying = Boolean(draft && !draft.id && draft.operationId);
  const disabled =
    saving || Boolean(onReload) || !draft?.name.trim() || !draft?.body.trim();
  return (
    <Sheet
      open={open}
      onOpenChange={(next) => {
        if (!next && !saving) onClose();
      }}
    >
      <SheetContent
        side={mobile ? 'bottom' : 'right'}
        className={mobile ? 'h-[92dvh] rounded-t-2xl' : 'w-full sm:max-w-xl'}
        closeDisabled={saving}
      >
        <SheetHeader className="border-b px-6 py-5 text-left">
          <SheetTitle>
            {t(
              `plugin.vivy/masks-ui.${draft?.id ? 'editTitle' : 'createTitle'}`,
            )}
          </SheetTitle>
          <SheetDescription>
            {t('plugin.vivy/masks-ui.editorDescription')}
          </SheetDescription>
        </SheetHeader>
        <SheetBody className="space-y-6 p-6" data-mask-editor>
          <div className="flex min-w-0 items-center gap-3 rounded-xl border bg-muted/35 p-4">
            <MaskIdentity id={draft?.id ?? 'custom/draft'} />
            <div className="min-w-0">
              <p className="break-words font-medium">
                {draft?.name.trim() || t('plugin.vivy/masks-ui.untitled')}
              </p>
              <Badge variant="secondary" className="mt-1">
                {t('plugin.vivy/masks-ui.custom')}
              </Badge>
            </div>
          </div>
          {error ? (
            <div
              className="space-y-2 rounded-xl border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"
              role="alert"
            >
              {error}
              {onReload ? (
                <Button variant="outline" size="sm" onClick={onReload}>
                  {t('plugin.vivy/masks-ui.reloadLatest')}
                </Button>
              ) : null}
            </div>
          ) : null}
          {retrying && !saving ? (
            <p className="rounded-xl bg-amber-500/10 p-3 text-sm text-amber-700 dark:text-amber-300">
              {t('plugin.vivy/masks-ui.retryCreate')}
            </p>
          ) : null}
          <div className="space-y-2 text-sm">
            <label htmlFor={`${fieldId}-name`} className="block font-medium">
              {t('plugin.vivy/masks-ui.name')}
            </label>
            <input
              id={`${fieldId}-name`}
              autoFocus
              readOnly={saving || retrying}
              className="w-full rounded-lg border bg-card px-3 py-2.5"
              value={draft?.name ?? ''}
              placeholder={t('plugin.vivy/masks-ui.namePlaceholder')}
              onChange={(e) => onChange('name', e.target.value)}
            />
          </div>
          <div className="space-y-2 text-sm">
            <label
              htmlFor={`${fieldId}-description`}
              className="block font-medium"
            >
              {t('plugin.vivy/masks-ui.description')}
            </label>
            <input
              id={`${fieldId}-description`}
              readOnly={saving || retrying}
              className="w-full rounded-lg border bg-card px-3 py-2.5"
              value={draft?.description ?? ''}
              placeholder={t('plugin.vivy/masks-ui.descriptionPlaceholder')}
              onChange={(e) => onChange('description', e.target.value)}
            />
          </div>
          <div className="space-y-2 text-sm">
            <label htmlFor={`${fieldId}-body`} className="block font-medium">
              {t('plugin.vivy/masks-ui.body')}
            </label>
            <textarea
              id={`${fieldId}-body`}
              readOnly={saving || retrying}
              className="min-h-56 w-full resize-y rounded-lg border bg-card px-3 py-3 text-sm leading-relaxed"
              value={draft?.body ?? ''}
              placeholder={t('plugin.vivy/masks-ui.bodyPlaceholder')}
              onChange={(e) => onChange('body', e.target.value)}
            />
          </div>
          <p className="text-xs leading-relaxed text-muted-foreground">
            {t('plugin.vivy/masks-ui.roleHint')}
          </p>
        </SheetBody>
        <SheetFooter className="border-t p-5">
          <Button variant="outline" disabled={saving} onClick={onClose}>
            {t('plugin.vivy/masks-ui.cancel')}
          </Button>
          <Button
            variant="secondary"
            disabled={disabled}
            onClick={() => onSave(false)}
          >
            {t(`plugin.vivy/masks-ui.${saving ? 'saving' : 'save'}`)}
          </Button>
          <Button disabled={disabled || !canUse} onClick={() => onSave(true)}>
            {t('plugin.vivy/masks-ui.saveAndUse')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
