/**
 * Grouped navigation assembly.
 *
 * A selected UI Module may claim a navigation entry for a named host surface
 * (`group`) instead of the PresentationHost's own top navigation. This core
 * shell seam projects those live contributions into the surface that owns the
 * group, so the surface renders exactly what the Recipe assembled and nothing
 * else. It reads the composition registry; it never registers into it.
 */
import { useMemo, useSyncExternalStore, type ComponentType } from 'react';
import type { FullUIHost } from '@vivy/ui-sdk';
import { compositionRuntimeOf } from './presentation-host';

/** The sidebar group assembled on the main page. */
export const SIDEBAR_VIVY_GROUP = 'vivy';

/** One projected grouped navigation entry, already normalized for rendering. */
export interface GroupedNavigationEntry {
  readonly id: string;
  readonly ownerId?: string;
  readonly group: string;
  readonly to: string;
  readonly labelKey: string;
  readonly label?: string;
  readonly icon?: ComponentType<{ readonly className?: string }>;
  readonly order: number;
  readonly exact: boolean;
}

const EMPTY_ENTRIES: readonly GroupedNavigationEntry[] = Object.freeze([]);
const noSubscribe = (): (() => void) => () => undefined;

/**
 * Projects composition entries into one group's renderable entries. Entries
 * without a usable `to`/`labelKey` pair are dropped: the surface cannot render
 * an entry that cannot be navigated to or labelled.
 */
export function projectGroupedNavigation(
  entries: readonly { readonly id: string; readonly ownerId?: string; readonly value: unknown }[],
  group: string,
): readonly GroupedNavigationEntry[] {
  const projected: GroupedNavigationEntry[] = [];
  for (const entry of entries) {
    const value = entry.value;
    if (!value || typeof value !== 'object') continue;
    const object = value as Record<string, unknown>;
    const rawGroup = object.group ?? object.surface;
    if (typeof rawGroup !== 'string' || rawGroup.trim() !== group) continue;
    const to = typeof object.to === 'string' ? object.to : typeof object.path === 'string' ? object.path : undefined;
    const labelKey = typeof object.labelKey === 'string' ? object.labelKey : undefined;
    if (!to || !labelKey) continue;
    const rawOrder = object.order;
    const order = typeof rawOrder === 'number' && Number.isFinite(rawOrder) ? rawOrder : Number.MAX_SAFE_INTEGER;
    projected.push({
      id: entry.id,
      ownerId: entry.ownerId,
      group,
      to,
      labelKey,
      label: typeof object.label === 'string' ? object.label : undefined,
      icon: typeof object.icon === 'function' ? object.icon as GroupedNavigationEntry['icon'] : undefined,
      order,
      exact: object.exact === true,
    });
  }
  // Stable sort: equal `order` values keep Recipe (registration) order.
  return projected
    .map((entry, index) => ({ entry, index }))
    .sort((left, right) => left.entry.order - right.entry.order || left.index - right.index)
    .map((item) => item.entry);
}

/** Live grouped entries for one surface; re-renders as Modules install. */
export function useGroupedNavigation(group: string, host?: FullUIHost): readonly GroupedNavigationEntry[] {
  const runtime = host ? compositionRuntimeOf(host) : undefined;
  const version = useSyncExternalStore(
    runtime ? runtime.subscribe : noSubscribe,
    runtime ? runtime.getSnapshot : () => 0,
    () => 0,
  );
  return useMemo(
    () => (runtime ? projectGroupedNavigation(runtime.getEntries('navigation'), group) : EMPTY_ENTRIES),
    // `version` is the registry revision the projection must be rebuilt for.
    [runtime, version, group],
  );
}