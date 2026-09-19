/**
 * The frame's route slot.
 *
 * Every path the core route tree does not own lands here, so a Module route
 * claimed by the selected Generation renders inside the app frame — beside the
 * sidebar and panels — instead of replacing it. A path no selected Module owns
 * returns to the app root, which is also what an unselected entry must do.
 *
 * While the router is still finishing a transition this match still belongs to
 * the previous page, so the slot renders nothing rather than redirecting the
 * navigation that is already in flight.
 */
import { Navigate, createFileRoute, useRouterState } from '@tanstack/react-router';
import { usePluginHost } from '@vivy/ui-sdk';
import { useActivePresentationRoute } from '@/plugins/presentation-host';

export const Route = createFileRoute('/_layout/$')({ component: AssembledRoute });

function AssembledRoute() {
  const host = usePluginHost();
  const route = useActivePresentationRoute(host);
  const match = Route.useMatch();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  if (route) return route;
  if (match.pathname !== pathname) return null;
  return <Navigate to="/" replace />;
}