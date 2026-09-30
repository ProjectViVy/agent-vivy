/**
 * INOFY workflow editor Module entry.
 *
 * The page hosts the Module's own VIVY-native editor over the host `inofy.*`
 * action surface: the backend's session-scoped handlers remain the single
 * authorization path, and the journal is the sole event authority. Editor
 * state supplies data only — never policy or tool authority.
 */
import { defineNavigationItem, defineUIExtension, defineUIRoute, type FullUIHost } from '@vivy/ui-sdk';
import { WorkflowPage } from './WorkflowPage';

const ROUTE = '/workflows';

export const extension = defineUIExtension({
  id: 'vivy.workflow-ui.extension',
  install: (host: FullUIHost) => {
    const registrations = [
      host.composition.routes.register('vivy-workflow-ui', defineUIRoute({
        path: ROUTE,
        titleKey: 'plugin.vivy/workflow-ui.title',
        subtitleKey: 'plugin.vivy/workflow-ui.subtitle',
        render: () => <WorkflowPage />,
      })),
      host.composition.navigation.register('vivy-workflow-ui', defineNavigationItem({
        group: 'vivy',
        to: ROUTE,
        labelKey: 'plugin.vivy/workflow-ui.nav',
        icon: 'workflow',
        order: 70,
      })),
    ];
    return () => {
      for (const registration of registrations.reverse()) registration.dispose();
    };
  },
});
