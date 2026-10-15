/**
 * 人格 sidebar Module entry.
 *
 * The Module owns both halves of its presence: the `/persona` route and the
 * grouped navigation entry the main sidebar assembles. Removing this Module
 * from the Recipe removes the page and the entry together.
 *
 * It also declares the first-run gate: the host owns the `shell.onboarding`
 * slot and its timing (it renders the slot once its own welcome flow closes),
 * and the Module owns whether anything appears.
 */
import { defineNavigationItem, defineUIRoute, defineUIExtension, type FullUIHost, type ShellOnboardingContribution } from '@vivy/ui-sdk';
import { PersonaPage } from './page';
import { PersonaOnboardingGate } from './gate';

const ROUTE = '/persona';

export const extension = defineUIExtension({
  id: 'vivy.persona.extension',
  install: (host: FullUIHost) => {
    const gate: ShellOnboardingContribution = {
      slot: 'shell.onboarding',
      render: () => <PersonaOnboardingGate />,
    };
    const registrations = [
      host.composition.routes.register('vivy-persona', defineUIRoute({
        path: ROUTE,
        titleKey: 'plugin.vivy/persona.title',
        subtitleKey: 'plugin.vivy/persona.subtitle',
        demo: false,
        render: () => <PersonaPage />,
      })),
      host.composition.navigation.register('vivy-persona', defineNavigationItem({
        group: 'vivy',
        to: ROUTE,
        labelKey: 'plugin.vivy/persona.nav',
        icon: 'user-round',
        order: 10,
      })),
      host.composition.components.register('vivy-persona-onboarding', gate),
    ];
    return () => {
      for (const registration of registrations.reverse()) registration.dispose();
    };
  },
});
