import { useEffect } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Navigate, Outlet, createRootRouteWithContext, useRouterState } from '@tanstack/react-router';
import { RecoverableError } from '@/components/feedback/RecoverableError';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';

function NotFound() { const pathname = useRouterState({ select: (state) => state.location.pathname }); return pathname === '/' ? null : <Navigate to="/" replace/>; }
function RouteError() { return <Navigate to="/" replace/>; }
export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({ component: Root, notFoundComponent: NotFound, errorComponent: RouteError });

function Root() {
  const { queryClient } = Route.useRouteContext();
  const initialized = useVivyStore((state) => state.initialized);
  const error = useVivyStore((state) => state.initializationError);
  const initialize = useVivyStore((state) => state.initialize);
  const retryInitialize = useVivyStore((state) => state.retryInitialize);
  const { t, locale } = useTranslation();
  useEffect(() => { void initialize(); }, [initialize]);
  useEffect(() => { document.title = t('app.documentTitle'); }, [t, locale]);
  return <QueryClientProvider client={queryClient}>{!initialized ? <div className="flex h-dvh items-center justify-center bg-background" role="status" aria-label={t('app.loading')}><div className="animate-pulse bg-gradient-to-r from-sky-500 via-blue-600 to-indigo-500 bg-clip-text text-3xl font-semibold tracking-[0.22em] text-transparent">VIVY</div></div> : error ? <div className="flex h-dvh items-center justify-center bg-background p-6"><RecoverableError className="max-w-lg" error={error} onRetry={() => void retryInitialize()} /></div> : <Outlet/>}</QueryClientProvider>;
}
