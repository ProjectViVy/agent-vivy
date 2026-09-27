/**
 * Backend-driven mask Module entry.
 *
 * The UI is a removable extension. It calls the separately selected
 * `vivy/masks` action owner and remains useful as a catalog editor even when
 * no session is active; the selector itself is disabled until the host has a
 * session-bound backend selection.
 */
import { defineNavigationItem, defineUIExtension, defineUIRoute, type ChatHeaderContribution, type FullUIHost } from '@vivy/ui-sdk';
import { MaskPage } from './MaskPage';
import { MaskHeader } from './MaskHeader';

const ROUTE = '/masks';

export const extension = defineUIExtension({
  id: 'vivy.masks-ui.extension',
  install: (host: FullUIHost) => {
    const registrations = [
      host.composition.routes.register('vivy-masks-ui', defineUIRoute({
        path: ROUTE,
        titleKey: 'plugin.vivy/masks-ui.title',
        subtitleKey: 'plugin.vivy/masks-ui.subtitle',
        render: () => <MaskPage />,
      })),
      host.composition.navigation.register('vivy-masks-ui', defineNavigationItem({
        group: 'vivy',
        to: ROUTE,
        labelKey: 'plugin.vivy/masks-ui.nav',
        icon: 'venetian-mask',
        order: 60,
      })),
      host.composition.components.register('vivy-masks-ui.header', {
        slot: 'chat.header',
        render: (context) => <MaskHeader context={context} />,
      } satisfies ChatHeaderContribution),
    ];
    return () => {
      for (const registration of registrations.reverse()) registration.dispose();
    };
  },
});
