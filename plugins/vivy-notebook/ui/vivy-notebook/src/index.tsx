/**
 * 记事本 sidebar Module entry.
 *
 * The Module owns both halves of its presence: the `/notebook` route and the
 * grouped navigation entry the main sidebar assembles. Removing this Module
 * from the Recipe removes the page and the entry together.
 */
import { defineNavigationItem, defineUIRoute, defineUIExtension, type FullUIHost } from '@vivy/ui-sdk';
import { NotebookPage } from './page';

const ROUTE = '/notebook';

export const extension = defineUIExtension({
  id: 'vivy.notebook.extension',
  install: (host: FullUIHost) => {
    const registrations = [
      host.composition.routes.register('vivy-notebook', defineUIRoute({
        path: ROUTE,
        titleKey: 'plugin.vivy/notebook.title',
        subtitleKey: 'plugin.vivy/notebook.subtitle',
        demo: true,
        render: () => <NotebookPage />,
      })),
      host.composition.navigation.register('vivy-notebook', defineNavigationItem({
        group: 'vivy',
        to: ROUTE,
        labelKey: 'plugin.vivy/notebook.nav',
        icon: 'notebook-pen',
        order: 50,
      })),
    ];
    return () => {
      for (const registration of registrations.reverse()) registration.dispose();
    };
  },
});
