/**
 * 会话树 sidebar Module entry.
 *
 * The Module owns both halves of its presence: the `/session-tree` route and
 * the grouped navigation entry the main sidebar assembles. Removing this
 * Module from the Recipe removes the page and the entry together.
 */
import { defineNavigationItem, defineUIRoute, defineUIExtension, type FullUIHost } from '@vivy/ui-sdk';
import { SessionTreePage } from './page';

const ROUTE = '/session-tree';

export const extension = defineUIExtension({
  id: 'vivy.session-tree.page',
  install: (host: FullUIHost) => {
    const registrations = [
      host.composition.routes.register('vivy/session-tree', defineUIRoute({
        path: ROUTE,
        titleKey: 'plugin.vivy/session-tree.title',
        subtitleKey: 'plugin.vivy/session-tree.subtitle',
        render: () => <SessionTreePage />,
      })),
      host.composition.navigation.register('vivy/session-tree', defineNavigationItem({
        group: 'vivy',
        to: ROUTE,
        labelKey: 'plugin.vivy/session-tree.nav',
        icon: 'workflow',
        order: 45,
      })),
    ];
    return () => {
      for (const registration of registrations.reverse()) registration.dispose();
    };
  },
});
