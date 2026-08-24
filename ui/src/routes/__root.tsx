import { useEffect } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Navigate, Outlet, createRootRouteWithContext, useRouterState } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { useVivyStore } from '@/lib/store';

function NotFound() { const pathname = useRouterState({ select: (state) => state.location.pathname }); return pathname === '/' ? null : <Navigate to="/" replace/>; }
function RouteError() { return <Navigate to="/" replace/>; }
export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({ component: Root, notFoundComponent: NotFound, errorComponent: RouteError });

function Root() {
  const { queryClient } = Route.useRouteContext();
  const initialized = useVivyStore((state) => state.initialized);
  const error = useVivyStore((state) => state.initializationError);
  const initialize = useVivyStore((state) => state.initialize);
  useEffect(() => { void initialize(); }, [initialize]);
  return <QueryClientProvider client={queryClient}>{!initialized ? <div className="flex h-dvh items-center justify-center bg-background" role="status" aria-label="Vivy 加载中"><div className="animate-pulse bg-gradient-to-r from-sky-500 via-blue-600 to-indigo-500 bg-clip-text text-3xl font-semibold tracking-[0.22em] text-transparent">VIVY</div></div> : error ? <div className="flex h-dvh items-center justify-center bg-background p-6"><div className="max-w-lg rounded-xl border bg-card p-6 text-center"><h1 className="text-lg font-semibold">无法连接 Vivy</h1><p className="mt-2 text-sm text-muted-foreground">{error}</p><Button className="mt-4" onClick={() => window.location.reload()}>重试</Button></div></div> : <Outlet/>}</QueryClientProvider>;
}
