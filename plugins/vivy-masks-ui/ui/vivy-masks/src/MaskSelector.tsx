import type { UITranslator } from '@vivy/ui-sdk';
import type { MaskMetadata } from './mask-client';

export interface MaskSelectorProps {
  readonly catalog: readonly MaskMetadata[];
  readonly selectedMaskId: string;
  readonly activeSessionId: string | null;
  readonly inactiveReason?: string;
  readonly disabled: boolean;
  readonly running: boolean;
  readonly loading: boolean;
  readonly onSelect: (maskId: string) => void;
  readonly t: UITranslator;
}

/** The session-bound selector is intentionally a controlled surface. */
export function MaskSelector({
  catalog,
  selectedMaskId,
  activeSessionId,
  inactiveReason,
  disabled,
  running,
  loading,
  onSelect,
  t,
}: MaskSelectorProps) {
  const unavailable = activeSessionId === null;
  const selectDisabled = disabled || unavailable || loading;
  return (
    <section className="rounded-lg border bg-card p-4" aria-label={t('plugin.vivy/masks-ui.selectorLabel')}>
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h2 className="text-sm font-semibold">{t('plugin.vivy/masks-ui.selectorTitle')}</h2>
          <p className="mt-1 text-xs text-muted-foreground">
            {unavailable
              ? t('plugin.vivy/masks-ui.noActiveSession')
              : inactiveReason
                ? t('plugin.vivy/masks-ui.errors.unavailable')
                : running
                  ? t('plugin.vivy/masks-ui.nextRun')
                  : t('plugin.vivy/masks-ui.currentSession')}
          </p>
        </div>
        {loading ? <span className="text-xs text-muted-foreground">{t('plugin.vivy/masks-ui.loading')}</span> : null}
      </div>
      <select
        className="mt-3 w-full rounded-md border bg-background px-3 py-2 text-sm"
        value={selectedMaskId}
        disabled={selectDisabled}
        onChange={(event) => onSelect(event.target.value)}
        aria-label={t('plugin.vivy/masks-ui.selectorLabel')}
      >
        <option value="">{t('plugin.vivy/masks-ui.unmasked')}</option>
        {catalog.map((mask) => (
          <option key={mask.id} value={mask.id}>
            {mask.name}{mask.built_in ? '' : ` · ${t('plugin.vivy/masks-ui.custom')}`}
          </option>
        ))}
      </select>
    </section>
  );
}
