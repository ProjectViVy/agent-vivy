// @vitest-environment happy-dom
import { describe, expect, it, vi } from 'vitest';
import type { Deliverable, DeliveryChunk } from './api';
import { DeliverableTransferError, downloadDeliverable, isTextPreviewable, readDeliveryPreview } from './deliverable-download';

const item = (overrides: Partial<Deliverable> = {}): Deliverable => ({
  id: 'itm_1',
  session_id: 'ses_1',
  run_id: 'run_1',
  workspace_id: 'ws_1',
  path: 'docs/报告.txt',
  name: '报告.txt',
  description: 'final report',
  size: 3,
  sha256: 'd'.repeat(64),
  media_type: 'text/plain',
  captured_at: 1700000000,
  origin_tool_call_id: 'call_1',
  ...overrides,
});

const chunk = (overrides: Partial<DeliveryChunk>): DeliveryChunk => ({
  transfer_id: 'xfr_1',
  item_id: 'itm_1',
  digest: 'd'.repeat(64),
  offset: 0,
  data_base64: '',
  eof: false,
  expires_at: 0,
  ...overrides,
});

const b64 = (text: string) => btoa(text);
const b64u = (text: string) => btoa(String.fromCharCode(...new TextEncoder().encode(text)));

function makeRpc(readImpl: (sessionId: string, req: { offset: number; length: number; transfer_id?: string }) => Promise<DeliveryChunk>) {
  const read = vi.fn(readImpl);
  const close = vi.fn(async () => undefined);
  return { read, close };
}

async function sha256Hex(text: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text));
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, '0')).join('');
}

describe('isTextPreviewable', () => {
  it('admits text types and structured text, rejects html/svg/binary', () => {
    expect(isTextPreviewable('text/plain')).toBe(true);
    expect(isTextPreviewable('text/markdown; charset=utf-8')).toBe(true);
    expect(isTextPreviewable('application/json')).toBe(true);
    expect(isTextPreviewable('text/html')).toBe(false);
    expect(isTextPreviewable('image/svg+xml')).toBe(false);
    expect(isTextPreviewable('application/pdf')).toBe(false);
  });
});

describe('downloadDeliverable', () => {
  it('fetches chunks sequentially, verifies EOF+hash, then creates and revokes the object URL', async () => {
    const body = 'hello world';
    const meta = item({ size: body.length, sha256: await sha256Hex(body) });
    const rpc = makeRpc(async (_sid, req) => {
      if (req.offset === 0) return chunk({ transfer_id: 'xfr_1', digest: meta.sha256, data_base64: b64(body.slice(0, 5)) });
      if (req.offset === 5) return chunk({ transfer_id: 'xfr_1', digest: meta.sha256, offset: 5, data_base64: b64(body.slice(5)), eof: true });
      throw new Error(`unexpected offset ${req.offset}`);
    });
    const createURL = vi.fn(() => 'blob:fake');
    const revokeURL = vi.fn();
    vi.stubGlobal('URL', { ...URL, createObjectURL: createURL, revokeObjectURL: revokeURL });
    const clicks: string[] = [];
    const realCreate = document.createElement.bind(document);
    vi.spyOn(document, 'createElement').mockImplementation(((tag: string) => {
      const el = realCreate(tag);
      if (tag === 'a') vi.spyOn(el as HTMLAnchorElement, 'click').mockImplementation(() => clicks.push((el as HTMLAnchorElement).download));
      return el;
    }) as typeof document.createElement);

    await downloadDeliverable(meta, 'ses_1', rpc, new AbortController().signal);

    expect(rpc.read).toHaveBeenCalledTimes(2);
    expect(rpc.read.mock.calls[1]?.[1]).toMatchObject({ offset: 5, transfer_id: 'xfr_1', expected_digest: meta.sha256 });
    expect(createURL).toHaveBeenCalledTimes(1);
    expect(revokeURL).toHaveBeenCalledWith('blob:fake');
    expect(clicks).toEqual(['报告.txt']);
    expect(rpc.close).toHaveBeenCalledWith('ses_1', 'xfr_1');
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('aborts mid-transfer: closes the active transfer and never creates an object URL', async () => {
    const controller = new AbortController();
    const meta = item();
    const rpc = makeRpc(async () => {
      controller.abort();
      return chunk({ transfer_id: 'xfr_active', digest: meta.sha256, data_base64: b64('abc') });
    });
    const createURL = vi.fn(() => 'blob:fake');
    vi.stubGlobal('URL', { ...URL, createObjectURL: createURL, revokeObjectURL: vi.fn() });

    await expect(downloadDeliverable(meta, 'ses_1', rpc, controller.signal)).rejects.toMatchObject({ reason: 'aborted' });
    expect(createURL).not.toHaveBeenCalled();
    expect(rpc.close).toHaveBeenCalledWith('ses_1', 'xfr_active');
    vi.unstubAllGlobals();
  });

  it('maps a changed read error to the typed reason and still closes the transfer', async () => {
    const meta = item();
    const rpc = makeRpc(async () => { throw new Error('deliverables/read failed: changed: workspace file differs'); });
    rpc.read.mockImplementationOnce(async () => chunk({ transfer_id: 'xfr_1', digest: meta.sha256, data_base64: b64('abc') }));
    vi.stubGlobal('URL', { ...URL, createObjectURL: vi.fn(), revokeObjectURL: vi.fn() });

    await expect(downloadDeliverable(meta, 'ses_1', rpc)).rejects.toMatchObject({ reason: 'changed' });
    expect(rpc.close).toHaveBeenCalledWith('ses_1', 'xfr_1');
    vi.unstubAllGlobals();
  });

  it('rejects a frame whose offset/digest does not match the request', async () => {
    const meta = item();
    const rpc = makeRpc(async () => chunk({ transfer_id: 'xfr_1', digest: meta.sha256, offset: 99, data_base64: b64('abc'), eof: true }));
    vi.stubGlobal('URL', { ...URL, createObjectURL: vi.fn(), revokeObjectURL: vi.fn() });
    await expect(downloadDeliverable(meta, 'ses_1', rpc)).rejects.toMatchObject({ reason: 'mismatch' });
    expect(rpc.close).toHaveBeenCalledWith('ses_1', 'xfr_1');
    vi.unstubAllGlobals();
  });

  it('rejects when assembled bytes fail the committed sha256', async () => {
    const meta = item({ size: 3 });
    const rpc = makeRpc(async () => chunk({ transfer_id: 'xfr_1', digest: meta.sha256, data_base64: b64('abc'), eof: true }));
    vi.stubGlobal('URL', { ...URL, createObjectURL: vi.fn(), revokeObjectURL: vi.fn() });
    await expect(downloadDeliverable(meta, 'ses_1', rpc)).rejects.toMatchObject({ reason: 'mismatch' });
    vi.unstubAllGlobals();
  });
});

describe('readDeliveryPreview', () => {
  it('returns decoded utf-8 text and closes its transfer', async () => {
    const meta = item({ size: 3 });
    const rpc = makeRpc(async () => chunk({ transfer_id: 'xfr_p', digest: meta.sha256, data_base64: b64u('你好'), eof: false }));
    const preview = await readDeliveryPreview(meta, 'ses_1', rpc);
    expect(preview).toBe('你好');
    expect(rpc.close).toHaveBeenCalledWith('ses_1', 'xfr_p');
    expect(rpc.read.mock.calls[0]?.[1].length).toBe(3);
  });

  it('propagates a missing file as a typed reason', async () => {
    const meta = item();
    const rpc = makeRpc(async () => { throw new Error('deliverables/read failed: missing'); });
    await expect(readDeliveryPreview(meta, 'ses_1', rpc)).rejects.toBeInstanceOf(DeliverableTransferError);
    await expect(readDeliveryPreview(meta, 'ses_1', rpc)).rejects.toMatchObject({ reason: 'missing' });
  });
});
