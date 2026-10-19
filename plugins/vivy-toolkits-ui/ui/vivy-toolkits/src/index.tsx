import { defineNavigationItem, defineUIExtension, defineUIRoute, type FullUIHost } from '@vivy/ui-sdk';
import { ToolkitPage } from './ToolkitPage';
import { ToolkitClient } from './toolkit-client';

const ROUTE = '/toolkits';
export const extension = defineUIExtension({
  id: 'vivy.toolkits-ui.extension',
  install: (host: FullUIHost) => {
    const client = ToolkitClient.fromRPC(host.rpc);
    const registrations = [
      host.composition.routes.register('vivy-toolkits-ui', defineUIRoute({
        path: ROUTE,
        titleKey: 'plugin.vivy/toolkits-ui.title',
        subtitleKey: 'plugin.vivy/toolkits-ui.subtitle',
        render: () => <ToolkitPage client={client} />,
      })),
      host.composition.navigation.register('vivy-toolkits-ui', defineNavigationItem({
        group: 'vivy', to: ROUTE, labelKey: 'plugin.vivy/toolkits-ui.nav', icon: 'wrench', order: 65,
      })),
    ];
    return () => { for (const registration of registrations.reverse()) registration.dispose(); };
  },
});
