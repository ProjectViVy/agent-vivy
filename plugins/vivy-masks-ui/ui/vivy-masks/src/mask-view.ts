import type { UITranslator } from '@vivy/ui-sdk';
import type { MaskMetadata } from './mask-client';

export type MaskChoice = Pick<
  MaskMetadata,
  'id' | 'name' | 'description' | 'built_in'
>;

/** The empty selection is a presentation choice, never a catalog definition. */
export function defaultIdentity(t: UITranslator): MaskChoice {
  return {
    id: '',
    name: t('plugin.vivy/masks-ui.unmasked'),
    description: t('plugin.vivy/masks-ui.unmaskedHint'),
    built_in: false,
  };
}

export function maskKind(t: UITranslator, mask: MaskChoice): string {
  return t(
    `plugin.vivy/masks-ui.${mask.id === '' ? 'defaultIdentity' : mask.built_in ? 'builtIn' : 'custom'}`,
  );
}
