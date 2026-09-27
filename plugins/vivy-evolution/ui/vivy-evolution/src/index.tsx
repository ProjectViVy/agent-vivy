/**
 * 进化 sidebar Module entry.
 *
 * The Module owns both halves of its presence: the `/evolution` route and the
 * grouped navigation entry the main sidebar assembles. Removing this Module
 * from the Recipe removes the page and the entry together.
 */
import { defineNavigationItem, defineUIRoute, defineUIExtension, type FullUIHost } from '@vivy/ui-sdk';
import { EvolutionPage } from './page';

const ROUTE = '/evolution';

export const extension = defineUIExtension({
  id: 'vivy.evolution.extension',
  install: (host: FullUIHost) => {
    const registrations = [
      host.composition.routes.register('vivy-evolution', defineUIRoute({
        path: ROUTE,
        titleKey: 'plugin.vivy/evolution.title',
        subtitleKey: 'plugin.vivy/evolution.subtitle',
        demo: true,
        render: () => <EvolutionPage />,
      })),
      host.composition.navigation.register('vivy-evolution', defineNavigationItem({
        group: 'vivy',
        to: ROUTE,
        labelKey: 'plugin.vivy/evolution.nav',
        icon: 'dna',
        order: 30,
      })),
    ];
    return () => {
      for (const registration of registrations.reverse()) registration.dispose();
    };
  },
});
