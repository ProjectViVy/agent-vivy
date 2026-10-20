import type { RunEvent } from './run-subscription';

/** Check framing from the accepted event sequence; text remains delta-owned. */
export function modelStreamError(events: RunEvent[]): string | null {
  let bytes = 0;
  const encoder = new TextEncoder();
  for (const event of events) {
    const payload = event.payload;
    if (event.type === 'model.request' || event.type === 'tool.requested') bytes = 0;
    else if (event.type === 'model.delta') {
      if (!payload || Object.keys(payload).length !== 1 || typeof payload.delta !== 'string') return 'model.delta: invalid payload';
      bytes += encoder.encode(payload.delta).byteLength;
    } else if (event.type === 'model.completed') {
      if (event.payload_version !== 3) return 'model.completed: unsupported payload version';
      if (!payload || Object.keys(payload).length !== 1 || !Number.isSafeInteger(payload.byte_len) || (payload.byte_len as number) < 0) return 'model.completed: invalid byte_len';
      if (payload.byte_len !== bytes) return 'model.completed: byte_len mismatch';
      bytes = 0;
    }
  }
  return null;
}
