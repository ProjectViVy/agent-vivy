import { resetLocaleForTests } from '@/i18n';
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { RpcClient, RpcClientError } from './rpc';

beforeEach(() => resetLocaleForTests());

describe('RpcClient bootstrap', () => {
  afterEach(() => vi.unstubAllGlobals());
  it('reports an actionable error when the dev server returns HTML', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<!doctype html>', { status: 200, headers: { 'content-type': 'text/html' } })));
    await expect(RpcClient.connect()).rejects.toMatchObject({ code: -32098 } satisfies Partial<RpcClientError>);
    await expect(RpcClient.connect()).rejects.toThrow('/rpc/bootstrap returned a non-JSON response');
  });

  it('reports an actionable error when the control plane is unreachable', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).includes('vivy-config')) return new Response('', { status: 404 });
      throw new TypeError('Failed to fetch');
    }));
    await expect(RpcClient.connect()).rejects.toMatchObject({ code: -32098 } satisfies Partial<RpcClientError>);
    await expect(RpcClient.connect()).rejects.toThrow('the backend is not listening');
  });
});
