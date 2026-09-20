import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ChannelDelivery, ChannelEnvelope, ChannelStatus } from '../../lib/api';
import type { RpcCapabilities } from '../../lib/rpc';

// api 模块整体 mock：通道真源在服务端，store 只做状态编排。
const inspectChannels = vi.fn<() => Promise<ChannelStatus[]>>();
const getChannel = vi.fn<(name: string) => Promise<ChannelEnvelope>>();
const updateChannel = vi.fn<(name: string, patch: Record<string, unknown>) => Promise<ChannelEnvelope>>();
const listChannelDeliveries = vi.fn<() => Promise<{ deliveries: ChannelDelivery[] }>>();
const redeliverChannelDelivery = vi.fn<(runId: string) => Promise<{ run_id: string; redelivered: boolean }>>();
const readRpcCapabilities = vi.fn<() => RpcCapabilities>();

vi.mock('../../lib/api', () => ({
  inspectChannels: () => inspectChannels(),
  getChannel: (name: string) => getChannel(name),
  updateChannel: (name: string, patch: Record<string, unknown>) => updateChannel(name, patch),
  listChannelDeliveries: () => listChannelDeliveries(),
  redeliverChannelDelivery: (runId: string) => redeliverChannelDelivery(runId),
}));

vi.mock('../../lib/rpc', () => ({
  getRpcCapabilitiesSnapshot: () => readRpcCapabilities(),
}));

import {
  channelPendingRestart,
  disableChannel,
  getChannelsState,
  redeliverDelivery,
  refreshChannels,
  saveChannel,
  toggleChannel,
} from './channel-store';

const telegramStatus: ChannelStatus = {
  name: 'telegram',
  capabilities: {
    typing: false, edit: false, delete: false, reaction: false, placeholder: false,
    media: false, media_store: false, webhook: false, listen: false, stream: false, health: false,
  },
  configured: true,
  enabled: false,
  allow_from: ['alice'],
  started: false,
  token_env: 'TELEGRAM_BOT_TOKEN',
  token_env_set: false,
  note: 'disabled',
  health: null,
};

const telegramEnvelope: ChannelEnvelope = {
  name: 'telegram',
  enabled: false,
  allow_from: ['alice'],
  token_env: 'TELEGRAM_BOT_TOKEN',
  configured: true,
};

const combinedCapabilities = {
  protocol_version: 'vivy.rpc.v1',
  capabilities: ['channel.deliveries.list', 'channel.deliveries.redeliver'],
  ui_extensions: [{ id: 'vivy/channel-ui', enabled: true }],
} satisfies RpcCapabilities;

const failedDelivery = {
  run_id: 'run-1',
  session_id: 'session-1',
  channel: 'telegram',
  chat_id: 'chat-1',
  topic_id: '',
  state: 'failed',
  attempts: 3,
  created_at_ms: 1,
  updated_at_ms: 2,
} satisfies ChannelDelivery;

function seedStatuses(statuses: ChannelStatus[], envelopes: Record<string, ChannelEnvelope> = {}): void {
  inspectChannels.mockResolvedValue(statuses);
  for (const status of statuses) {
    getChannel.mockImplementation(async (name: string) => {
      const envelope = envelopes[name];
      if (!envelope) throw new Error(`no envelope fixture for ${name}`);
      return envelope;
    });
  }
}

beforeEach(() => {
  vi.clearAllMocks();
  readRpcCapabilities.mockReturnValue({ protocol_version: 'vivy.rpc.v1', capabilities: [] });
  listChannelDeliveries.mockResolvedValue({ deliveries: [] });
  redeliverChannelDelivery.mockResolvedValue({ run_id: 'run-1', redelivered: true });
  seedStatuses([], {});
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('refreshChannels（服务端真源）', () => {
  it('能力缺失时不请求失败投递接口', async () => {
    await refreshChannels();

    expect(readRpcCapabilities).toHaveBeenCalledTimes(1);
    expect(listChannelDeliveries).not.toHaveBeenCalled();
    expect(getChannelsState().failedDeliveries).toEqual([]);
  });

  it('同一能力投影保留 UI extension、投递能力、健康态与失败投递', async () => {
    const healthyStatus: ChannelStatus = {
      ...telegramStatus,
      started: true,
      health: { ok: true, detail: 'connected' },
    };
    readRpcCapabilities.mockReturnValue(combinedCapabilities);
    seedStatuses([healthyStatus], { telegram: telegramEnvelope });
    listChannelDeliveries.mockResolvedValue({ deliveries: [failedDelivery] });

    await refreshChannels();

    expect(combinedCapabilities.ui_extensions?.[0]?.id).toBe('vivy/channel-ui');
    expect(combinedCapabilities.capabilities).toContain('channel.deliveries.list');
    expect(combinedCapabilities.capabilities).toContain('channel.deliveries.redeliver');
    expect(getChannelsState().statuses[0]?.health).toEqual({ ok: true, detail: 'connected' });
    expect(getChannelsState().failedDeliveries).toEqual([failedDelivery]);
    expect(listChannelDeliveries).toHaveBeenCalledTimes(1);
  });

  it('拉取 inspect 列表与各通道 envelope（编译内通道，按服务端排序）', async () => {
    seedStatuses([telegramStatus], { telegram: telegramEnvelope });
    await refreshChannels();
    const state = getChannelsState();
    expect(state.loaded).toBe(true);
    expect(state.error).toBeNull();
    expect(state.statuses.map((s) => s.name)).toEqual(['telegram']);
    expect(state.envelopes.telegram).toEqual(telegramEnvelope);
    expect(getChannel).toHaveBeenCalledWith('telegram');
  });

  it('空 host：statuses 为空表（这一代没有耳朵）', async () => {
    await refreshChannels();
    const state = getChannelsState();
    expect(state.loaded).toBe(true);
    expect(state.statuses).toEqual([]);
  });

  it('inspect 失败 → error，列表不可信', async () => {
    inspectChannels.mockRejectedValue(new Error('rpc down'));
    await refreshChannels();
    const state = getChannelsState();
    expect(state.loaded).toBe(true);
    expect(state.statuses).toEqual([]);
    expect(state.error).toContain('rpc down');
  });

  it('单通道 envelope 失败不拖垮列表（该通道暂无文档真值）', async () => {
    seedStatuses([telegramStatus]);
    getChannel.mockRejectedValue(new Error('not found'));
    await refreshChannels();
    const state = getChannelsState();
    expect(state.error).toBeNull();
    expect(state.statuses).toHaveLength(1);
    expect(state.envelopes.telegram).toBeUndefined();
  });

  it('重投失败投递后刷新台账', async () => {
    readRpcCapabilities.mockReturnValue(combinedCapabilities);
    listChannelDeliveries
      .mockResolvedValueOnce({ deliveries: [failedDelivery] })
      .mockResolvedValue({ deliveries: [] });
    await refreshChannels();

    await redeliverDelivery(failedDelivery.run_id);

    expect(redeliverChannelDelivery).toHaveBeenCalledWith('run-1');
    expect(listChannelDeliveries).toHaveBeenCalledTimes(2);
    expect(getChannelsState().failedDeliveries).toEqual([]);
  });

  it('重投前能力消失时不发请求并清除旧台账', async () => {
    readRpcCapabilities.mockReturnValue(combinedCapabilities);
    listChannelDeliveries.mockResolvedValue({ deliveries: [failedDelivery] });
    await refreshChannels();
    readRpcCapabilities.mockReturnValue({ protocol_version: 'vivy.rpc.v1', capabilities: [] });

    await redeliverDelivery(failedDelivery.run_id);

    expect(redeliverChannelDelivery).not.toHaveBeenCalled();
    expect(getChannelsState().failedDeliveries).toEqual([]);
  });

  it('遗留 localStorage（vivy.ui.channels）只删不迁：首次拉取即清除，且从不写回', async () => {
    const storage = new Map<string, string>();
    const written: string[] = [];
    storage.set(
      'vivy.ui.channels',
      JSON.stringify({ email: { enabled: true }, 'neuro-link': { enabled: true }, telegram: { enabled: true, token: 'legacy' } }),
    );
    vi.stubGlobal('window', {
      localStorage: {
        getItem: (key: string) => storage.get(key) ?? null,
        setItem: (key: string, value: string) => {
          storage.set(key, value);
          written.push(key);
        },
        removeItem: (key: string) => {
          storage.delete(key);
        },
      },
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => true,
    });
    seedStatuses([telegramStatus], { telegram: telegramEnvelope });
    await refreshChannels();
    const state = getChannelsState();
    expect(state.statuses.map((s) => s.name)).toEqual(['telegram']);
    expect(state.envelopes.telegram).toEqual(telegramEnvelope);
    // 残留副本被删除（可能含 token 残迹），且没有任何迁移/写回。
    expect(storage.get('vivy.ui.channels')).toBeUndefined();
    expect(written).toEqual([]);
  });
});

describe('saveChannel / toggleChannel / disableChannel', () => {
  it('整条 overlay entry 写回：enabled/allow_from 合并当前 envelope，token_env 不被发明', async () => {
    seedStatuses([telegramStatus], { telegram: telegramEnvelope });
    await refreshChannels();
    const saved: ChannelEnvelope = { ...telegramEnvelope, enabled: true, allow_from: [] };
    updateChannel.mockResolvedValue(saved);
    // 保存后的重拉会重新读取文档真值，此时服务端已返回 saved。
    getChannel.mockResolvedValue(saved);

    const result = await saveChannel('telegram', { enabled: true, allow_from: [] });

    expect(updateChannel).toHaveBeenCalledWith('telegram', {
      enabled: true,
      allow_from: [],
    });
    expect('token_env' in (updateChannel.mock.calls[0][1] as Record<string, unknown>)).toBe(false);
    expect(result).toEqual(saved);
    expect(getChannelsState().envelopes.telegram).toEqual(saved);
    // 保存后重拉 inspect（进程真值不变，pending 状态可见）。
    expect(inspectChannels).toHaveBeenCalledTimes(2);
  });

  it('仅给 patch 字段时其余字段回落当前文档真值（不会意外清空 allow_from）', async () => {
    seedStatuses([telegramStatus], { telegram: telegramEnvelope });
    await refreshChannels();
    updateChannel.mockResolvedValue(telegramEnvelope);

    await saveChannel('telegram', { enabled: true });

    expect(updateChannel).toHaveBeenCalledWith('telegram', {
      enabled: true,
      allow_from: ['alice'],
    });
  });

  it('toggle/disable 映射到 updateChannel 的 enabled 指针（删除 = 关耳朵）', async () => {
    seedStatuses([telegramStatus], { telegram: telegramEnvelope });
    await refreshChannels();
    updateChannel.mockResolvedValue({ ...telegramEnvelope, enabled: true });

    await toggleChannel('telegram', true);
    expect(updateChannel).toHaveBeenLastCalledWith('telegram', { enabled: true, allow_from: ['alice'] });

    await disableChannel('telegram');
    expect(updateChannel).toHaveBeenLastCalledWith('telegram', { enabled: false, allow_from: ['alice'] });
  });

  it('当前文档真值缺失时先 getChannel 再写回', async () => {
    seedStatuses([telegramStatus]);
    getChannel.mockResolvedValue(telegramEnvelope);
    updateChannel.mockResolvedValue(telegramEnvelope);

    await saveChannel('telegram', { enabled: true });

    expect(getChannel).toHaveBeenCalledWith('telegram');
    expect(updateChannel).toHaveBeenCalledWith('telegram', { enabled: true, allow_from: ['alice'] });
  });
});

describe('channelPendingRestart（文档真值 vs 进程真值）', () => {
  it('enabled/configured/token_env 出现差异 → true', () => {
    expect(
      channelPendingRestart(telegramStatus, { ...telegramEnvelope, enabled: true }),
    ).toBe(true);
    expect(
      channelPendingRestart({ ...telegramStatus, configured: false }, telegramEnvelope),
    ).toBe(true);
    expect(
      channelPendingRestart(telegramStatus, { ...telegramEnvelope, token_env: '' }),
    ).toBe(true);
  });

  it('纯 allow_from 编辑（enabled/configured/token_env 全同）→ true', () => {
    expect(
      channelPendingRestart(telegramStatus, { ...telegramEnvelope, allow_from: ['alice', 'bob'] }),
    ).toBe(true);
    expect(
      channelPendingRestart(telegramStatus, { ...telegramEnvelope, allow_from: [] }),
    ).toBe(true);
  });

  it('一致 → false；无文档真值（未拉取）→ false', () => {
    expect(channelPendingRestart(telegramStatus, telegramEnvelope)).toBe(false);
    expect(channelPendingRestart(telegramStatus, undefined)).toBe(false);
  });
});
