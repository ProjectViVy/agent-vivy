import { resetLocaleForTests } from '@/i18n';
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { loadRuntimeConfig, resolveControlPlaneOrigin } from './runtime-config';

beforeEach(() => resetLocaleForTests());

describe('Vivy runtime config', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('loads a cross-origin control plane from JSON', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('{"controlPlaneUrl":"http://127.0.0.1:8787"}', {
      status: 200,
      headers: { 'content-type': 'application/json' },
    })));
    await expect(loadRuntimeConfig()).resolves.toEqual({ controlPlaneUrl: 'http://127.0.0.1:8787' });
  });

  it('uses the page origin when the runtime file is absent', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('', { status: 404 })));
    await expect(loadRuntimeConfig()).resolves.toEqual({ controlPlaneUrl: '' });
  });

  it('reports an actionable error when the runtime file cannot be fetched', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('Failed to fetch'); }));
    await expect(loadRuntimeConfig()).rejects.toThrow('Vivy runtime config failed to load');
  });

  it('rejects credentials and paths in the control plane URL', () => {
    expect(() => resolveControlPlaneOrigin('http://user:pass@127.0.0.1:8787')).toThrow('Vivy controlPlaneUrl may contain only the scheme, host, and port');
    expect(() => resolveControlPlaneOrigin('http://127.0.0.1:8787/app')).toThrow('Vivy controlPlaneUrl may contain only the scheme, host, and port');
    expect(resolveControlPlaneOrigin('', 'http://127.0.0.1:3015').origin).toBe('http://127.0.0.1:3015');
  });
});
