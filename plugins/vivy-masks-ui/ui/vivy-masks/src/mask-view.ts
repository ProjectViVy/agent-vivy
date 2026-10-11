import type { UITranslator } from '@vivy/ui-sdk';
import type { MaskMetadata } from './mask-client';

export type MaskChoice = Pick<
  MaskMetadata,
  'id' | 'name' | 'description' | 'built_in' | 'origin' | 'bundle' | 'recommended_skills' | 'available' | 'inactive_reason'
>;

/** The empty selection is a presentation choice, never a catalog definition. */
export function defaultIdentity(t: UITranslator): MaskChoice {
  return {
    id: '',
    name: t('plugin.vivy/masks-ui.unmasked'),
    description: t('plugin.vivy/masks-ui.unmaskedHint'),
    built_in: false,
    origin: 'custom',
  };
}

export function maskKind(t: UITranslator, mask: MaskChoice): string {
  const kind = mask.id === '' ? 'defaultIdentity'
    : mask.origin === 'bundle' ? 'bundle'
      : mask.origin === 'builtin' || (mask.origin === undefined && mask.built_in) ? 'builtIn'
        : 'custom';
  return t(
    `plugin.vivy/masks-ui.${kind}`,
  );
}

export function isCustomMask(mask: MaskChoice): boolean {
  // Older schema-1 service generations omit origin. Preserve their custom
  // editing behavior while treating any explicit non-custom origin as read-only.
  return mask.origin === 'custom' || (mask.origin === undefined && !mask.built_in);
}
