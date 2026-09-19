/**
 * 进化 sidebar Module entry.
 *
 * The Module owns both halves of its presence: the `/evolution` route and the
 * grouped navigation entry the main sidebar assembles. Removing this Module
 * from the Recipe removes the page and the entry together.
 */
import { defineNavigationItem, defineUIExtension, type FullUIHost } from '@vivy/ui-sdk';
import { Dna } from 'lucide-react';
import { EvolutionPage } from './page';

const ROUTE = '/evolution';

export const extension = defineUIExtension({
  id: 'vivy.evolution.extension',
  install: (host: FullUIHost) => {
    const registrations = [
      host.composition.routes.register('vivy-evolution', {
        path: ROUTE,
        render: () => <EvolutionPage />,
      }),
      host.composition.navigation.register('vivy-evolution', defineNavigationItem({
        group: 'vivy',
        to: ROUTE,
        labelKey: 'plugin.vivy/evolution.nav',
        icon: Dna,
        order: 30,
      })),
    ];
    return () => {
      for (const registration of registrations.reverse()) registration.dispose();
    };
  },
});
