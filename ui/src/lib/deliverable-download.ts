// 显式交付传输（SC-D4 §12）：下载只走 digest 绑定的 deliverables/read +
// deliverables/close，序列化分片直到 EOF，组装后校验字节数与 sha256，
// 只有全部成立才触发浏览器保存；没有成功前不产生 object URL。
// HTML/SVG 只以文本形式预览或作为附件落盘，从不在应用源内执行。

import type { Deliverable, DeliveryChunk, DeliveryReadRequest } from './api';

export type DeliverableTransferReason = 'changed' | 'missing' | 'forbidden' | 'unavailable' | 'aborted' | 'mismatch';

export class DeliverableTransferError extends Error {
  readonly reason: DeliverableTransferReason;

  constructor(reason: DeliverableTransferReason, message?: string) {
    super(message ?? reason);
    this.name = 'DeliverableTransferError';
    this.reason = reason;
  }
}

/** 传输 RPC 接缝：由 api.deliverablesRead / deliverablesClose 填充。 */
export interface DeliverableTransferRpc {
  read: (sessionId: string, request: DeliveryReadRequest) => Promise<DeliveryChunk>;
  close: (sessionId: string, transferId: string) => Promise<unknown>;
}

const CHUNK_LENGTH = 256 * 1024;
export const DELIVERY_PREVIEW_BYTES = 64 * 1024;

const TEXT_PREVIEW_TYPES = new Set([
  'application/json',
  'application/ld+json',
  'application/x-ndjson',
  'application/xml',
  'application/javascript',
  'application/typescript',
  'application/x-yaml',
  'application/yaml',
  'application/toml',
  'application/x-sh',
]);

/** 有界文本预览只面向明确的文本体裁；SVG/HTML 永远不预览渲染。 */
export function isTextPreviewable(mediaType: string): boolean {
  const base = (mediaType.split(';')[0] ?? '').trim().toLowerCase();
  if (base === 'image/svg+xml' || base === 'text/html') return false;
  return base.startsWith('text/') || TEXT_PREVIEW_TYPES.has(base);
}

function decodeBase64(value: string): Uint8Array {
  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}

export function mapDeliverableReason(message: string): DeliverableTransferReason {
  const lower = message.toLowerCase();
  if (lower.includes('changed')) return 'changed';
  if (lower.includes('missing')) return 'missing';
  if (lower.includes('forbidden')) return 'forbidden';
  if (lower.includes('abort')) return 'aborted';
  return 'unavailable';
}

function fail(err: unknown): DeliverableTransferError {
  if (err instanceof DeliverableTransferError) return err;
  const message = err instanceof Error ? err.message : String(err);
  return new DeliverableTransferError(mapDeliverableReason(message), message);
}

function aborted(signal?: AbortSignal): boolean {
  return signal?.aborted === true;
}

async function sha256Hex(data: Uint8Array): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', data.slice().buffer);
  return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

async function readChunk(item: Deliverable, sessionId: string, rpc: DeliverableTransferRpc, request: { offset: number; length: number; transferId?: string }): Promise<DeliveryChunk> {
  try {
    return await rpc.read(sessionId, {
      item_id: item.id,
      expected_digest: item.sha256,
      transfer_id: request.transferId === '' || request.transferId === undefined ? undefined : request.transferId,
      offset: request.offset,
      length: request.length,
    });
  } catch (err) {
    throw fail(err);
  }
}

function assertChunkFrame(chunk: DeliveryChunk, item: Deliverable, offset: number): void {
  if (chunk.item_id !== item.id || chunk.digest !== item.sha256 || chunk.offset !== offset) {
    throw new DeliverableTransferError('mismatch');
  }
}

/** 保存下载：顺序取片到 EOF，整体验证后走 object URL；任何中止/不匹配
 * 都关传输且不留半成品。 */
export async function downloadDeliverable(item: Deliverable, sessionId: string, rpc: DeliverableTransferRpc, signal?: AbortSignal): Promise<void> {
  if (aborted(signal)) throw new DeliverableTransferError('aborted');
  let transferId = '';
  let offset = 0;
  let total = 0;
  const parts: Uint8Array[] = [];
  try {
    for (;;) {
      if (aborted(signal)) throw new DeliverableTransferError('aborted');
      const chunk = await readChunk(item, sessionId, rpc, { offset, length: CHUNK_LENGTH, transferId });
      transferId = chunk.transfer_id;
      if (aborted(signal)) throw new DeliverableTransferError('aborted');
      assertChunkFrame(chunk, item, offset);
      const bytes = decodeBase64(chunk.data_base64);
      if (bytes.length === 0 && !chunk.eof) throw new DeliverableTransferError('mismatch');
      parts.push(bytes);
      total += bytes.length;
      offset += bytes.length;
      if (chunk.eof) break;
    }
  } finally {
    if (transferId !== '') await rpc.close(sessionId, transferId).catch(() => undefined);
  }
  if (total !== item.size) throw new DeliverableTransferError('mismatch');
  const assembled = new Uint8Array(total);
  let at = 0;
  for (const part of parts) {
    assembled.set(part, at);
    at += part.length;
  }
  if ((await sha256Hex(assembled)) !== item.sha256) throw new DeliverableTransferError('mismatch');
  const url = URL.createObjectURL(new Blob([assembled.slice().buffer], { type: item.media_type }));
  try {
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = item.name;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
  } finally {
    URL.revokeObjectURL(url);
  }
}

/** 有界文本预览：只读文件头一段；失败原因与下载一致。 */
export async function readDeliveryPreview(item: Deliverable, sessionId: string, rpc: DeliverableTransferRpc, signal?: AbortSignal): Promise<string> {
  if (aborted(signal)) throw new DeliverableTransferError('aborted');
  let transferId = '';
  try {
    const chunk = await readChunk(item, sessionId, rpc, { offset: 0, length: Math.min(Math.max(item.size, 1), DELIVERY_PREVIEW_BYTES) });
    transferId = chunk.transfer_id;
    assertChunkFrame(chunk, item, 0);
    return new TextDecoder('utf-8', { fatal: false }).decode(decodeBase64(chunk.data_base64));
  } finally {
    if (transferId !== '') await rpc.close(sessionId, transferId).catch(() => undefined);
  }
}

/** 可用性探针：一个 digest 绑定的起始读，成功即证明当前内容与快照一致。 */
export async function checkDeliverable(item: Deliverable, sessionId: string, rpc: DeliverableTransferRpc, signal?: AbortSignal): Promise<void> {
  if (aborted(signal)) throw new DeliverableTransferError('aborted');
  let transferId = '';
  try {
    const chunk = await readChunk(item, sessionId, rpc, { offset: 0, length: Math.min(Math.max(item.size, 1), 4096) });
    transferId = chunk.transfer_id;
    assertChunkFrame(chunk, item, 0);
  } finally {
    if (transferId !== '') await rpc.close(sessionId, transferId).catch(() => undefined);
  }
}
