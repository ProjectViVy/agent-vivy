/**
 * 人格 sidebar Module entry.
 *
 * The Module owns both halves of its presence: the `/persona` route and the
 * grouped navigation entry the main sidebar assembles. Removing this Module
 * from the Recipe removes the page and the entry together.
 */
import { defineNavigationItem, defineUIRoute, defineUIExtension, type FullUIHost } from '@vivy/ui-sdk';
import { PersonaPage } from './page';

const ROUTE = '/persona';

export const extension = defineUIExtension({
  id: 'vivy.persona.extension',
  install: (host: FullUIHost) => {
    const registrations = [
      host.composition.routes.register('vivy-persona', defineUIRoute({
        path: ROUTE,
        titleKey: 'plugin.vivy/persona.title',
        subtitleKey: 'plugin.vivy/persona.subtitle',
        demo: true,
        render: () => <PersonaPage />,
      })),
      host.composition.navigation.register('vivy-persona', defineNavigationItem({
        group: 'vivy',
        to: ROUTE,
        labelKey: 'plugin.vivy/persona.nav',
        icon: 'user-round',
        order: 10,
      })),
    ];
    return () => {
      for (const registration of registrations.reverse()) registration.dispose();
    };
  },
});