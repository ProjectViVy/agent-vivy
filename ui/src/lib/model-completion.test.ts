import { describe, expect, it } from 'vitest';
import { modelStreamError } from './model-completion';
import type { RunEvent } from './run-subscription';

const event = (type: string, payload: Record<string, unknown>, version = 1): RunEvent =>
  ({ run_id: 'r', seq: 1, type, payload, payload_version: version, created_at: 1 });

describe('model completion framing', () => {
  it('counts Unicode by step and keeps tool preambles out of the final count', () => {
    expect(modelStreamError([
      event('model.delta', { delta: 'preamble' }), event('tool.requested', {}),
      event('model.request', {}, 4), event('model.delta', { delta: '你好' }), event('model.delta', { delta: '🌱' }),
      event('model.completed', { byte_len: 10 }, 3), event('model.completed', { byte_len: 0 }, 3),
    ])).toBeNull();
  });
  it.each([{}, { byte_len: null }, { byte_len: -1 }, { byte_len: 1.5 }, { byte_len: '0' }, { byte_len: 0, content_sha256: 'old' }])('rejects invalid metadata %j', (payload) => {
    expect(modelStreamError([event('model.completed', payload, 3)])).not.toBeNull();
  });
  it('rejects an old live version and malformed deltas', () => {
    expect(modelStreamError([event('model.completed', { byte_len: 0 }, 2)])).not.toBeNull();
    expect(modelStreamError([event('model.delta', { delta: null })])).not.toBeNull();
  });
});
