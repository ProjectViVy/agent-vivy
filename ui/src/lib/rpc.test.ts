import { afterEach, describe, expect, it, vi } from 'vitest';
import { RpcClient, RpcClientError } from './rpc';

describe('RpcClient bootstrap', () => {
  afterEach(() => vi.unstubAllGlobals());
  it('reports an actionable error when the dev server returns HTML', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<!doctype html>', { status: 200, headers: { 'content-type': 'text/html' } })));
    await expect(RpcClient.connect()).rejects.toMatchObject({ code: -32098 } satisfies Partial<RpcClientError>);
    await expect(RpcClient.connect()).rejects.toThrow('/rpc/bootstrap 返回了非 JSON 响应');
  });
});
