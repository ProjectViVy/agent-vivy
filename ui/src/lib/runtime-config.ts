import { t } from '@/i18n';

export interface VivyRuntimeConfig {
  /** Absolute HTTP(S) origin of the Vivy control plane; empty means same-origin. */
  controlPlaneUrl: string;
}

const defaultRuntimeConfig: VivyRuntimeConfig = { controlPlaneUrl: '' };

export async function loadRuntimeConfig(): Promise<VivyRuntimeConfig> {
  let response: Response;
  try {
    response = await fetch('/vivy-config.json', { cache: 'no-store' });
  } catch {
    throw new Error(t('errors.runtimeConfigUnavailable'));
  }
  // Vite's dev fallback can return the app shell when the public file is not
  // present. Treat that as the documented same-origin default.
  if (response.status === 404) {
    return defaultRuntimeConfig;
  }
  if (!response.ok) {
    throw new Error(t('errors.runtimeConfigHttp', { status: response.status }));
  }
  if (!(response.headers.get('content-type') ?? '').toLowerCase().includes('application/json')) {
    return defaultRuntimeConfig;
  }
  let raw: unknown;
  try {
    raw = await response.json();
  } catch {
    throw new Error(t('errors.runtimeConfigNotJson'));
  }
  if (!raw || typeof raw !== 'object' || typeof (raw as { controlPlaneUrl?: unknown }).controlPlaneUrl !== 'string') {
    throw new Error(t('errors.runtimeConfigMissingUrl'));
  }
  return { controlPlaneUrl: (raw as { controlPlaneUrl: string }).controlPlaneUrl.trim() };
}

export function resolveControlPlaneOrigin(controlPlaneUrl: string, pageOrigin = pageOriginFromEnvironment()): URL {
  const value = controlPlaneUrl.trim();
  const origin = new URL(value || pageOrigin);
  if (origin.protocol !== 'http:' && origin.protocol !== 'https:') {
    throw new Error(t('errors.controlPlaneScheme'));
  }
  if (origin.username || origin.password || origin.search || origin.hash || (origin.pathname !== '' && origin.pathname !== '/')) {
    throw new Error(t('errors.controlPlaneHostOnly'));
  }
  origin.pathname = '/';
  return origin;
}

function pageOriginFromEnvironment(): string {
  if (typeof window !== 'undefined' && window.location.origin) return window.location.origin;
  return 'http://127.0.0.1';
}
