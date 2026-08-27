import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  CHANNELS_KEY,
  channelStatusFor,
  getChannelStatuses,
  getChannels,
  normalizeDiscordConfig,
  removeChannel,
  saveChannel,
  toggleChannel,
} from './channel-store';

type ListenerMap = Record<string, Array<() => void>>;

function stubWindow(initial: Record<string, string> = {}) {
  const storage = new Map(Object.entries(initial));
  const listeners: ListenerMap = {};
  vi.stubGlobal('window', {
    localStorage: {
      getItem: (key: string) => (storage.has(key) ? storage.get(key)! : null),
      setItem: (key: string, value: string) => { storage.set(key, value); },
    },
    addEventListener: (type: string, onChange: () => void) => {
      (listeners[type] ??= []).push(onChange);
    },
    removeEventListener: (type: string, onChange: () => void) => {
      listeners[type] = (listeners[type] ?? []).filter((fn) => fn !== onChange);
    },
    dispatchEvent: (event: Event) => {
      for (const listener of listeners[event.type] ?? []) listener();
      return true;
    },
  });
  return { storage, listeners };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('getChannels 读取与坏数据过滤', () => {
  it('SSR guard：无 window 时返回空表且写操作不抛错', () => {
    vi.stubGlobal('window', undefined);
    expect(getChannels()).toEqual({});
    expect(() => saveChannel('telegram', { token: 'x' })).not.toThrow();
    expect(() => toggleChannel('telegram')).not.toThrow();
    expect(() => removeChannel('telegram')).not.toThrow();
  });

  it('存储为空返回空表', () => {
    stubWindow();
    expect(getChannels()).toEqual({});
  });

  it('坏 JSON / 非对象返回空表', () => {
    stubWindow({ [CHANNELS_KEY]: '{broken json' });
    expect(getChannels()).toEqual({});
    stubWindow({ [CHANNELS_KEY]: JSON.stringify([1, 2, 3]) });
    expect(getChannels()).toEqual({});
  });

  it('非对象条目整条丢弃', () => {
    stubWindow({
      [CHANNELS_KEY]: JSON.stringify({ telegram: 'not-an-object', feishu: { enabled: true } }),
    });
    const channels = getChannels();
    expect(Object.keys(channels)).toEqual(['feishu']);
  });

  it('读入时补齐 Discord 默认值（normalizeDiscordConfig 应用）', () => {
    stubWindow({
      [CHANNELS_KEY]: JSON.stringify({
        discord: { enabled: true, token: 'x', allow_from: ['1', '2'] },
      }),
    });
    const config = getChannels().discord;
    expect(config.allow_from).toEqual(['1', '2']);
    expect(config.gateway_url).toBe('wss://gateway.discord.gg/?v=10&encoding=json');
    expect(config.intents).toBe(37377);
    expect(config.guild_id).toBeNull();
    expect(config.mention_only).toBe(false);
    expect(config.listen_to_bots).toBe(false);
    expect(config.group_reply_allowed_sender_ids).toEqual([]);
  });

  it('读入时隐藏下架通道', () => {
    stubWindow({
      [CHANNELS_KEY]: JSON.stringify({ slack: { enabled: true }, telegram: { enabled: false } }),
    });
    expect(Object.keys(getChannels())).toEqual(['telegram']);
  });
});

describe('saveChannel / toggleChannel / removeChannel', () => {
  it('save 写入侧深拷贝（改传入配置不影响已存值），存储落盘正确', () => {
    const { storage } = stubWindow();
    const config = { enabled: true, token: 'abc' };
    saveChannel('telegram', config);
    config.token = 'mutated-input';
    expect(getChannels().telegram).toEqual({ enabled: true, token: 'abc' });
    expect(JSON.parse(storage.get(CHANNELS_KEY)!)).toEqual({
      telegram: { enabled: true, token: 'abc' },
    });
  });

  it('toggle 切换 enabled，其余字段不动；缺失通道为 no-op', () => {
    stubWindow();
    saveChannel('telegram', { enabled: false, token: 'x' });
    toggleChannel('telegram');
    expect(getChannels().telegram.enabled).toBe(true);
    toggleChannel('telegram');
    expect(getChannels().telegram.enabled).toBe(false);
    expect(() => toggleChannel('missing')).not.toThrow();
  });

  it('remove 删除通道；缺失通道为 no-op', () => {
    stubWindow();
    saveChannel('telegram', { enabled: true });
    removeChannel('telegram');
    expect(getChannels()).toEqual({});
    expect(() => removeChannel('telegram')).not.toThrow();
  });

  it('写操作触发自定义事件（跨订阅者刷新）', () => {
    const { listeners } = stubWindow();
    const onChange = vi.fn();
    listeners['vivy.ui.channels.changed'] = [onChange];
    saveChannel('telegram', { enabled: true });
    expect(onChange).toHaveBeenCalledTimes(1);
  });
});

describe('normalizeDiscordConfig', () => {
  it('仅补齐缺失键，不动已填值', () => {
    const config: Record<string, unknown> = { token: 'x', intents: 100, gateway_url: 'wss://custom' };
    normalizeDiscordConfig(config);
    expect(config.intents).toBe(100);
    expect(config.gateway_url).toBe('wss://custom');
    expect(config.allow_from).toEqual([]);
    expect(config.mention_only).toBe(false);
  });

  it('非对象输入静默返回', () => {
    expect(() => normalizeDiscordConfig(undefined)).not.toThrow();
  });
});

describe('就绪状态（纯前端近似）', () => {
  it('必填齐全 → ready；缺失列出', () => {
    expect(channelStatusFor('telegram', { enabled: true, token: 'x' })).toEqual({
      name: 'telegram',
      enabled: true,
      ready: true,
      missing_fields: [],
      notes: [],
    });
    const status = channelStatusFor('telegram', { enabled: false, token: '' });
    expect(status.ready).toBe(false);
    expect(status.missing_fields).toEqual(['token']);
    expect(status.enabled).toBe(false);
  });

  it('getChannelStatuses 按可见通道产出并排除下架通道', () => {
    stubWindow();
    saveChannel('telegram', { enabled: true, token: 'x' });
    saveChannel('slack', { enabled: true });
    const statuses = getChannelStatuses();
    expect(statuses.map((s) => s.name)).toEqual(['telegram']);
    expect(statuses[0].ready).toBe(true);
  });
});