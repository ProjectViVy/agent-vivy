import { useEffect, useMemo } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Navigate, Outlet, createRootRouteWithContext, useRouter, useRouterState } from '@tanstack/react-router';
import type { FaceClientRouter } from '@vivy/ui-sdk';
import { RecoverableError } from '@/components/feedback/RecoverableError';
import { generatedUIExtensions, generatedUIRoot, UI_ASSEMBLY_MANIFEST } from '@vivy/generated-assembly';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';
import { createWebFaceHost, PresentationHost } from '@/plugins/presentation-host';

function NotFound() { const pathname = useRouterState({ select: (state) => state.location.pathname }); return pathname === '/' ? null : <Navigate to="/" replace/>; }
function RouteError() { return <Navigate to="/" replace/>; }
export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({ component: Root, notFoundComponent: NotFound, errorComponent: RouteError });

function Root() {
  const { queryClient } = Route.useRouteContext();
  const initialized = useVivyStore((state) => state.initialized);
  const error = useVivyStore((state) => state.initializationError);
  const species = useVivyStore((state) => state.species);
  const initialize = useVivyStore((state) => state.initialize);
  const retryInitialize = useVivyStore((state) => state.retryInitialize);
  const { t, locale } = useTranslation();
  const router = useRouter();
  const pathname = useRouterState({ select: (state) => state.location.pathname });
  const faceRouter = useMemo<FaceClientRouter>(() => ({
    navigate: (options) => router.navigate(options as never),
    invalidate: () => router.invalidate(),
  }), [router]);
  // Construct the Face adapter only after initialize() has projected the
  // negotiated client/store state. This keeps install-time capabilities
  // truthful without recreating a live host while its extensions run.
  const faceHost = useMemo(() => initialized ? createWebFaceHost(faceRouter) : undefined, [faceRouter, initialized]);
  const runtimeProvenance = useMemo(() => {
    const generated = UI_ASSEMBLY_MANIFEST as typeof UI_ASSEMBLY_MANIFEST & {
      readonly generationId?: string;
      readonly artifactSha256?: string;
      readonly uiArtifactSha256?: string;
    };
    return {
      ...generated,
      ...(species?.generation_id ? { generationId: species.generation_id } : {}),
      ...(species?.artifact_sha256 ? { artifactSha256: species.artifact_sha256 } : {}),
      ...(species?.ui_artifact_sha256 ? { uiArtifactSha256: species.ui_artifact_sha256 } : {}),
    };
  }, [species]);
  useEffect(() => { void initialize(); }, [initialize]);
  useEffect(() => { document.title = t('app.documentTitle'); }, [t, locale]);
  useEffect(() => {
    const rootElement = document.getElementById('root');
    if (!rootElement) return;
    const attributes: Readonly<Record<string, string | undefined>> = {
      'vivyGenerationId': runtimeProvenance.generationId,
      'vivyArtifactSha256': runtimeProvenance.artifactSha256,
      'vivyUiArtifactSha256': runtimeProvenance.uiArtifactSha256,
    };
    for (const [name, value] of Object.entries(attributes)) {
      if (value) rootElement.dataset[name] = value;
      else delete rootElement.dataset[name];
    }
    rootElement.dataset.vivyUiProvenance = JSON.stringify(runtimeProvenance);
  }, [runtimeProvenance]);
  return <QueryClientProvider client={queryClient}>{!initialized ? <div className="flex h-dvh items-center justify-center bg-background" role="status" aria-label={t('app.loading')}><div className="animate-pulse bg-gradient-to-r from-sky-500 via-blue-600 to-indigo-500 bg-clip-text text-3xl font-semibold tracking-[0.22em] text-transparent">VIVY</div></div> : error ? <div className="flex h-dvh items-center justify-center bg-background p-6"><RecoverableError className="max-w-lg" error={error} onRetry={() => void retryInitialize()} /></div> : faceHost ? <PresentationHost host={faceHost} root={generatedUIRoot} extensions={generatedUIExtensions} provenance={runtimeProvenance} path={pathname}><Outlet /></PresentationHost> : null}</QueryClientProvider>;
}
