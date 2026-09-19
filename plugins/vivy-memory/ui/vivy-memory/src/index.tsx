/**
 * 记忆 sidebar Module entry.
 *
 * The Module owns both halves of its presence: the `/memory` route and the
 * grouped navigation entry the main sidebar assembles. Removing this Module
 * from the Recipe removes the page and the entry together.
 */
import { defineNavigationItem, defineUIRoute, defineUIExtension, type FullUIHost } from '@vivy/ui-sdk';
import { MemoryPage } from './page';

const ROUTE = '/memory';

export const extension = defineUIExtension({
  id: 'vivy.memory.extension',
  install: (host: FullUIHost) => {
    const registrations = [
      host.composition.routes.register('vivy-memory', defineUIRoute({
        path: ROUTE,
        titleKey: 'plugin.vivy/memory.title',
        subtitleKey: 'plugin.vivy/memory.subtitle',
        demo: true,
        render: () => <MemoryPage />,
      })),
      host.composition.navigation.register('vivy-memory', defineNavigationItem({
        group: 'vivy',
        to: ROUTE,
        labelKey: 'plugin.vivy/memory.nav',
        icon: 'brain',
        order: 40,
      })),
    ];
    return () => {
      for (const registration of registrations.reverse()) registration.dispose();
    };
  },
});
