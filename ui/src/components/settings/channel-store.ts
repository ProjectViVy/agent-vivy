import { useSyncExternalStore } from 'react';
import {
  getChannel as fetchChannelEnvelope,
  inspectChannels,
  updateChannel,
  type ChannelEnvelope,
  type ChannelStatus,
  type ChannelUpdateInput,
} from '../../lib/api';

/**
 * 通道注册表（服务端真源）。
 *
 * - 编译进当前代的通道集合与每个通道的 envelope（enabled / allow_from /
 *   token_env）以后端为唯一真源：channel/inspect 报告进程真值（上次
 *   StartAll 的决策），channel/get 报告文档真值（config.yaml envelope
 *   ⊕ settings overlay）。
 * - 写入走 channel/update（settings overlay 条目），进程重启后才生效；
 *   UI 用文档真值与进程真值的差异展示"待重启"。
 * - 遗留 localStorage key `vivy.ui.channels` 不再读取：服务端是唯一
 *   真源，历史前端副本不做迁移（其中从未有过可信的密钥存储）。
 */

export interface ChannelsState {
  /** 编译进当前代的通道（服务端按名称排序）；未加载完成时为空表。 */
  statuses: ChannelStatus[];
  /** 各通道 envelope（文档真值）；单通道读取失败时缺省。 */
  envelopes: Record<string, ChannelEnvelope>;
  /** 首次 inspect 是否已返回（区分"加载中"与"这一代没有耳朵"）。 */
  loaded: boolean;
  /** inspect 失败原因；非空时列表不可信。 */
  error: string | null;
}

let state: ChannelsState = { statuses: [], envelopes: {}, loaded: false, error: null };
const listeners = new Set<() => void>();
let refreshInFlight: Promise<void> | null = null;

function emit(): void {
  for (const listener of listeners) listener();
}

function setState(patch: Partial<ChannelsState>): void {
  state = { ...state, ...patch };
  emit();
}

function getSnapshot(): ChannelsState {
  return state;
}

function subscribe(listener: () => void): () => void {
  const first = listeners.size === 0;
  listeners.add(listener);
  // 首个订阅者触发首次拉取；后续订阅沿用同一份状态。
  if (first && !state.loaded) void refreshChannels();
  return () => {
    listeners.delete(listener);
  };
}

/** 拉取（或重拉）通道列表与各通道 envelope 并广播。 */
export function refreshChannels(): Promise<void> {
  if (refreshInFlight) return refreshInFlight;
  refreshInFlight = inspectChannels()
    .then(async (statuses) => {
      const entries = await Promise.all(
        statuses.map(async (status): Promise<readonly [string, ChannelEnvelope?]> => {
          try {
            return [status.name, await fetchChannelEnvelope(status.name)] as const;
          } catch {
            // 单通道 envelope 读取失败不拖垮列表；该通道暂无文档真值。
            return [status.name, undefined] as const;
          }
        }),
      );
      const envelopes: Record<string, ChannelEnvelope> = {};
      for (const [name, envelope] of entries) {
        if (envelope) envelopes[name] = envelope;
      }
      setState({ statuses, envelopes, loaded: true, error: null });
    })
    .catch((error) => {
      setState({ loaded: true, error: error instanceof Error ? error.message : String(error) });
    })
    .finally(() => {
      refreshInFlight = null;
    });
  return refreshInFlight;
}

/** 订阅通道状态（首个订阅者触发首次 inspect）。 */
export function useChannelsState(): ChannelsState {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

/** 非 hook 读取当前状态（SSR / 纯函数测试 / 一次性检查用）。 */
export function getChannelsState(): ChannelsState {
  return state;
}

/**
 * 保存一个通道：以当前文档真值合并 patch 后整条 overlay entry 写回。
 * channel/update 是指针语义的整条替换——只发部分字段会把未携带的字段
 * 回落到 config.yaml 值，因此这里始终带上 enabled 与 allow_from。
 * token_env 只在显式给定（环境变量名）时发送；界面从不发明密钥值。
 */
export async function saveChannel(name: string, patch: ChannelUpdateInput): Promise<ChannelEnvelope> {
  const current = state.envelopes[name] ?? (await fetchChannelEnvelope(name));
  const envelope = await updateChannel(name, {
    enabled: patch.enabled ?? current.enabled,
    allow_from: patch.allow_from ?? current.allow_from,
    ...(patch.token_env !== undefined ? { token_env: patch.token_env } : {}),
  });
  setState({ envelopes: { ...state.envelopes, [name]: envelope } });
  await refreshChannels();
  return envelope;
}

/** 启用/停用切换（写入 overlay，重启后生效）。 */
export async function toggleChannel(name: string, enabled: boolean): Promise<ChannelEnvelope> {
  return saveChannel(name, { enabled });
}

/**
 * "删除" = 关耳朵（enabled=false）：settings overlay 在本代没有删除
 * 条目的语义，通道保持编译可见、重启进程后不再启动。
 */
export async function disableChannel(name: string): Promise<ChannelEnvelope> {
  return saveChannel(name, { enabled: false });
}

/**
 * "待重启"判定：文档真值与进程真值出现差异。allow_from 在 inspect 表面
 * （启动生效摘要），纯 allow_from 编辑同样判 pending；顺序敏感——两侧都
 * 保存写入顺序，仅重排也算差异，重启后自然消除。状态或 envelope 未就绪
 * 时不判 pending。
 */
export function channelPendingRestart(
  status: ChannelStatus | undefined,
  envelope: ChannelEnvelope | undefined,
): boolean {
  if (!status || !envelope) return false;
  return (
    envelope.enabled !== status.enabled ||
    envelope.configured !== status.configured ||
    envelope.token_env !== status.token_env ||
    !allowFromEqual(envelope.allow_from, status.allow_from)
  );
}

function allowFromEqual(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((id, i) => id === b[i]);
}
