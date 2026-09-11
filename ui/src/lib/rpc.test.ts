import { resetLocaleForTests } from '@/i18n';
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { getRpcCapabilitiesSnapshot, resetRpcClient, RpcClient, RpcClientError } from './rpc';

beforeEach(() => resetLocaleForTests());

describe('RpcClient bootstrap', () => {
  afterEach(() => {
    resetRpcClient();
    vi.unstubAllGlobals();
  });
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

  it('clears negotiated capabilities when an idle WebSocket closes', async () => {
    class FakeWebSocket {
      static latest: FakeWebSocket | undefined;
      onopen: (() => void) | null = null;
      onerror: (() => void) | null = null;
      onmessage: ((message: { data: string }) => void) | null = null;
      onclose: (() => void) | null = null;
      constructor(_url: string) {
        FakeWebSocket.latest = this;
        queueMicrotask(() => this.onopen?.());
      }
      send(payload: string): void {
        const request = JSON.parse(payload) as { id: string };
        queueMicrotask(() => this.onmessage?.({
          data: JSON.stringify({
            jsonrpc: '2.0',
            id: request.id,
            result: { protocol_version: 'vivy.rpc.v1', capabilities: ['session.read'] },
          }),
        }));
      }
      close(): void { this.onclose?.(); }
    }
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).includes('vivy-config')) return new Response('', { status: 404 });
      return new Response(JSON.stringify({ protocol_version: 'vivy.rpc.v1', websocket_path: '/rpc/ws', token: 'fixture-token' }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      });
    }));
    vi.stubGlobal('WebSocket', FakeWebSocket);

    await RpcClient.connect();
    expect(getRpcCapabilitiesSnapshot()).toEqual({ protocol_version: 'vivy.rpc.v1', capabilities: ['session.read'] });
    FakeWebSocket.latest?.onclose?.();
    expect(getRpcCapabilitiesSnapshot()).toEqual({ protocol_version: '', capabilities: [] });
  });
});
