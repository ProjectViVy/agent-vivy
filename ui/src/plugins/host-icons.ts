/**
 * The host icon set.
 *
 * A Module names an icon (`HOST_ICON_NAMES` in `@vivy/ui-sdk`); the shell owns
 * what that name looks like. That keeps one icon implementation in the bundle
 * — an entry cannot drag its own icon library in — and it keeps the sidebar,
 * the group surfaces, and the Module page header visually identical.
 *
 * `host-icons.test.ts` fails if the SDK's name list and this table ever drift.
 */
import {
  Brain,
  Clock,
  Dna,
  LayoutDashboard,
  NotebookPen,
  Plug,
  Settings,
  ShieldCheck,
  Sparkles,
  UserRound,
  VenetianMask,
  Wrench,
  Zap,
  type LucideIcon,
} from 'lucide-react';
import { HOST_ICON_NAMES, type HostIconName } from '@vivy/ui-sdk';

/** Every host icon name resolved to the host's own icon implementation. */
export const HOST_ICONS: Readonly<Record<HostIconName, LucideIcon>> = Object.freeze({
  dashboard: LayoutDashboard,
  wrench: Wrench,
  sparkles: Sparkles,
  settings: Settings,
  clock: Clock,
  plug: Plug,
  zap: Zap,
  'shield-check': ShieldCheck,
  'venetian-mask': VenetianMask,
  'user-round': UserRound,
  dna: Dna,
  brain: Brain,
  'notebook-pen': NotebookPen,
});

/**
 * Resolves a contribution's icon name. An unknown name — a hand-written string
 * that is not in the SDK list — falls back to the group icon rather than
 * rendering nothing, so a typo cannot leave an entry unlabelled.
 */
export function resolveHostIcon(name: HostIconName | string | undefined): LucideIcon {
  if (typeof name === 'string' && name in HOST_ICONS) return HOST_ICONS[name as HostIconName];
  return Sparkles;
}

/** The icon names this host implements, for diagnostics and tests. */
export function hostIconNames(): readonly string[] {
  return HOST_ICON_NAMES;
}