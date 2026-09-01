import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ChannelEnvelope, ChannelStatus } from '../../lib/api';

// api 模块整体 mock：通道真源在服务端，store 只做状态编排。
const inspectChannels = vi.fn<() => Promise<ChannelStatus[]>>();
const getChannel = vi.fn<(name: string) => Promise<ChannelEnvelope>>();
const updateChannel = vi.fn<(name: string, patch: Record<string, unknown>) => Promise<ChannelEnvelope>>();

vi.mock('../../lib/api', () => ({
  inspectChannels: () => inspectChannels(),
  getChannel: (name: string) => getChannel(name),
  updateChannel: (name: string, patch: Record<string, unknown>) => updateChannel(name, patch),
}));

import {
  channelPendingRestart,
  disableChannel,
  getChannelsState,
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
};

const telegramEnvelope: ChannelEnvelope = {
  name: 'telegram',
  enabled: false,
  allow_from: ['alice'],
  token_env: 'TELEGRAM_BOT_TOKEN',
  configured: true,
};

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
  seedStatuses([], {});
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('refreshChannels（服务端真源）', () => {
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

  it('遗留 localStorage（vivy.ui.channels）不再是真源：email/neuro-link 等历史副本被忽略', async () => {
    const storage = new Map<string, string>();
    storage.set(
      'vivy.ui.channels',
      JSON.stringify({ email: { enabled: true }, 'neuro-link': { enabled: true }, telegram: { enabled: true, token: 'legacy' } }),
    );
    vi.stubGlobal('window', {
      localStorage: {
        getItem: (key: string) => storage.get(key) ?? null,
        setItem: (key: string, value: string) => { storage.set(key, value); },
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
    // localStorage 从未被读取，也就从未被写入。
    expect(storage.get('vivy.ui.channels')).toBe(
      JSON.stringify({ email: { enabled: true }, 'neuro-link': { enabled: true }, telegram: { enabled: true, token: 'legacy' } }),
    );
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
