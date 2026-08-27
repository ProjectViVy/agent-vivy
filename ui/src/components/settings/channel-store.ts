import { useSyncExternalStore } from 'react';
import { isRetiredChannel } from './channel-platforms';
import { validateConfig } from './channel-schema';

/**
 * 通道配置本地存储（Agent-Diva ChannelsSettings 的 vivy 纯前端适配）。
 *
 * - 唯一存储于 localStorage key `vivy.ui.channels`（真实功能，禁用 vivy.demo.*）。
 * - 存储形状 = Diva `get_channels` wire 格式：`Record<通道名, { enabled, ...字段 }>`，
 *   便于未来接入后端通道读写时直接替换读写层（无需迁移 UI）。
 * - 与 custom-providers.ts / saved-models.ts 同款持久化样板：模块级缓存 +
 *   `useSyncExternalStore` + 自定义事件 / `storage` 事件广播，不进 zustand store。
 * - 快照标识稳定：每次写入以不可变方式生成新记录（新对象引用），未写入时
 *   `getChannels()` 返回同一缓存引用，满足 `useSyncExternalStore` 的稳定快照要求。
 * - 就绪状态（ready / missing_fields）为纯前端近似：按 schema 必填字段存在性
 *   计算，替代 Diva 的服务端 `getConfigStatus` 通道报告；后端接入后以服务端
 *   报告为准。
 */

export const CHANNELS_KEY = 'vivy.ui.channels';

const CHANNELS_CHANGED_EVENT = 'vivy.ui.channels.changed';

/** 单个通道配置：与 Diva wire 形状一致（`enabled` 为保留字段，不出现在表单）。 */
export type ChannelConfig = Record<string, unknown>;

/** 通道名 → 配置 的映射（与 Diva `get_channels` 返回形状一致）。 */
export type StoredChannels = Record<string, ChannelConfig>;

/** 就绪报告（对齐 Diva `ChannelStatusSummary` 的 GUI 用法）。 */
export interface ChannelStatusSummary {
  name: string;
  enabled: boolean;
  ready: boolean;
  missing_fields: string[];
  notes: string[];
}

function cloneConfig(config: ChannelConfig): ChannelConfig {
  return JSON.parse(JSON.stringify(config)) as ChannelConfig;
}

/** 移植自 Diva ChannelsSettings.loadChannels：读入时补齐 Discord 默认值。 */
export function normalizeDiscordConfig(d: Record<string, unknown> | undefined): void {
  if (!d || typeof d !== 'object') return;
  if (!Array.isArray(d.allow_from)) d.allow_from = [];
  if (d.gateway_url === undefined || d.gateway_url === '') {
    d.gateway_url = 'wss://gateway.discord.gg/?v=10&encoding=json';
  }
  if (d.intents === undefined || d.intents === null) d.intents = 37377;
  if (d.guild_id === undefined) d.guild_id = null;
  if (d.mention_only === undefined) d.mention_only = false;
  if (d.listen_to_bots === undefined) d.listen_to_bots = false;
  if (!Array.isArray(d.group_reply_allowed_sender_ids)) d.group_reply_allowed_sender_ids = [];
}

/** 读入时按平台补齐默认值；坏条目（非对象）整条丢弃、下架通道直接隐藏。 */
function normalizeChannels(raw: unknown): StoredChannels {
  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) return {};
  const next: StoredChannels = {};
  for (const [name, config] of Object.entries(raw as Record<string, unknown>)) {
    if (typeof config !== 'object' || config === null || Array.isArray(config)) continue;
    if (isRetiredChannel(name)) continue;
    const entry = { ...(config as Record<string, unknown>) };
    if (name === 'discord') normalizeDiscordConfig(entry);
    next[name] = entry;
  }
  return next;
}

let cachedRaw: string | null = null;
let cachedChannels: StoredChannels = {};

function readChannels(): StoredChannels {
  if (typeof window === 'undefined') return cachedChannels;
  let raw: string | null = null;
  try {
    raw = window.localStorage.getItem(CHANNELS_KEY);
  } catch {
    // Local UI preference is best effort.
    return cachedChannels;
  }
  if (raw === cachedRaw) return cachedChannels;
  cachedRaw = raw;
  if (!raw) {
    cachedChannels = {};
    return cachedChannels;
  }
  try {
    cachedChannels = normalizeChannels(JSON.parse(raw));
  } catch {
    cachedChannels = {};
  }
  return cachedChannels;
}

function writeChannels(next: StoredChannels): void {
  if (typeof window === 'undefined') return;
  cachedChannels = next;
  cachedRaw = JSON.stringify(next);
  try {
    window.localStorage.setItem(CHANNELS_KEY, cachedRaw);
  } catch {
    // Local UI preference is best effort.
  }
  window.dispatchEvent(new Event(CHANNELS_CHANGED_EVENT));
}

/** 可见通道表（读入时已剔除下架通道）：快照引用稳定（useSyncExternalStore 要求）。 */
export function getChannels(): StoredChannels {
  return readChannels();
}

/** 就绪报告：必填字段齐全 → ready；缺失列于 missing_fields（纯前端近似）。 */
export function channelStatusFor(name: string, config: ChannelConfig): ChannelStatusSummary {
  const { valid, missing } = validateConfig(name, config);
  return {
    name,
    enabled: Boolean(config.enabled),
    ready: valid,
    missing_fields: missing,
    notes: [],
  };
}

/** 全通道就绪报告（与 Diva `getConfigStatus().channels` 形状对齐）。 */
export function getChannelStatuses(): ChannelStatusSummary[] {
  return Object.entries(getChannels()).map(([name, config]) => channelStatusFor(name, config));
}

/** 保存（新建或整表替换）单个通道配置；返回保存后的副本。下架通道写入为 no-op。 */
export function saveChannel(name: string, config: ChannelConfig): ChannelConfig {
  if (isRetiredChannel(name)) return cloneConfig(config);
  const channels = readChannels();
  const next: StoredChannels = { ...channels, [name]: cloneConfig(config) };
  writeChannels(next);
  return cloneConfig(next[name]);
}

/** 启用/停用切换（不可变更新，保持其它字段不动）。 */
export function toggleChannel(name: string): void {
  if (isRetiredChannel(name)) return;
  const channels = readChannels();
  const config = channels[name];
  if (!config) return;
  writeChannels({ ...channels, [name]: { ...config, enabled: !config.enabled } });
}

/** 删除通道（本地存储）。 */
export function removeChannel(name: string): void {
  if (isRetiredChannel(name)) return;
  const channels = readChannels();
  if (!(name in channels)) return;
  const next: StoredChannels = { ...channels };
  delete next[name];
  writeChannels(next);
}

function subscribeToChannels(onChange: () => void): () => void {
  if (typeof window === 'undefined') return () => undefined;
  window.addEventListener(CHANNELS_CHANGED_EVENT, onChange);
  window.addEventListener('storage', onChange);
  return () => {
    window.removeEventListener(CHANNELS_CHANGED_EVENT, onChange);
    window.removeEventListener('storage', onChange);
  };
}

export function useChannels(): StoredChannels {
  return useSyncExternalStore(subscribeToChannels, getChannels, () => ({}));
}