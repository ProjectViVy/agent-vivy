/**
 * 人格 sidebar Module entry.
 *
 * The Module owns both halves of its presence: the `/persona` route and the
 * grouped navigation entry the main sidebar assembles. Removing this Module
 * from the Recipe removes the page and the entry together.
 */
import { defineNavigationItem, defineUIExtension, type FullUIHost } from '@vivy/ui-sdk';
import { UserRound } from 'lucide-react';
import { PersonaPage } from './page';

const ROUTE = '/persona';

export const extension = defineUIExtension({
  id: 'vivy.persona.extension',
  install: (host: FullUIHost) => {
    const registrations = [
      host.composition.routes.register('vivy-persona', {
        path: ROUTE,
        render: () => <PersonaPage />,
      }),
      host.composition.navigation.register('vivy-persona', defineNavigationItem({
        group: 'vivy',
        to: ROUTE,
        labelKey: 'plugin.vivy/persona.nav',
        icon: UserRound,
        order: 10,
      })),
    ];
    return () => {
      for (const registration of registrations.reverse()) registration.dispose();
    };
  },
});