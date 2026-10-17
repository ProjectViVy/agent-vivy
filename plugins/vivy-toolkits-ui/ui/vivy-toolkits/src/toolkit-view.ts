import type { UITranslator } from '@vivy/ui-sdk';
import type {
  PreparedCandidate,
  ToolkitInstallation,
  ToolkitInspection,
  ToolkitManifest,
} from './toolkit-client';

/**
 * Presentation vocabulary for the Toolkit page.
 *
 * Every value the backend sends as a machine token is translated here, once, so
 * no page component renders an enum. The catalog key is written literally at
 * each `t()` call because the repository's i18n gate checks those call sites;
 * a key that is missing from `i18n/catalog.json` therefore fails the gate
 * instead of rendering a raw key at runtime.
 */

export type SourceKind = 'local' | 'git';
export type ComponentKind = 'skills' | 'masks' | 'rules' | 'hooks' | 'mcp';

/**
 * The mutation in flight. `key` is the install id or operation id the request
 * belongs to, so a control can express loading only for its own object.
 */
export type MutationAction = 'prepare' | 'source' | 'rollback' | 'bindings' | 'consent' | 'publish' | 'cancel';
export interface PendingAction {
  readonly action: MutationAction;
  readonly key: string;
}

export function isPending(pending: PendingAction | null, action: MutationAction, key?: string): boolean {
  if (!pending || pending.action !== action) return false;
  return key === undefined || pending.key === key;
}

export const COMPONENT_KINDS: readonly ComponentKind[] = ['skills', 'masks', 'rules', 'hooks', 'mcp'];

export function sourceKindOf(kind: string): SourceKind {
  return kind === 'git' ? 'git' : 'local';
}

export function sourceKindLabel(t: UITranslator, kind: string): string {
  return sourceKindOf(kind) === 'git'
    ? t('plugin.vivy/toolkits-ui.gitSource')
    : t('plugin.vivy/toolkits-ui.localSource');
}

export function installationStateLabel(t: UITranslator, state: string): string {
  switch (state) {
    case 'enabled':
      return t('plugin.vivy/toolkits-ui.stateEnabled');
    case 'disabled':
      return t('plugin.vivy/toolkits-ui.stateDisabled');
    case 'orphaned':
      return t('plugin.vivy/toolkits-ui.stateOrphaned');
    default:
      return t('plugin.vivy/toolkits-ui.stateUnknown');
  }
}

export function installationStateTone(state: string): 'default' | 'secondary' | 'outline' | 'destructive' {
  switch (state) {
    case 'enabled':
      return 'default';
    case 'orphaned':
      return 'destructive';
    case 'disabled':
      return 'secondary';
    default:
      return 'outline';
  }
}

export function operationStateLabel(t: UITranslator, state: string): string {
  switch (state) {
    case 'pending':
      return t('plugin.vivy/toolkits-ui.operationStatePreparing');
    case 'committed':
      return t('plugin.vivy/toolkits-ui.operationStatePublished');
    case 'cancelled':
      return t('plugin.vivy/toolkits-ui.operationStateCancelled');
    case 'failed':
      return t('plugin.vivy/toolkits-ui.operationStateFailed');
    default:
      return t('plugin.vivy/toolkits-ui.stateUnknown');
  }
}

export function changeVerbLabel(t: UITranslator, change: string): string {
  switch (change) {
    case 'added':
      return t('plugin.vivy/toolkits-ui.changeAdded');
    case 'removed':
      return t('plugin.vivy/toolkits-ui.changeRemoved');
    case 'changed':
      return t('plugin.vivy/toolkits-ui.changeChanged');
    default:
      return change;
  }
}

export function mutationLabel(t: UITranslator, mutationKind: string): string {
  switch (mutationKind) {
    case 'install':
      return t('plugin.vivy/toolkits-ui.mutationInstall');
    case 'enable':
      return t('plugin.vivy/toolkits-ui.mutationEnable');
    case 'disable':
      return t('plugin.vivy/toolkits-ui.mutationDisable');
    case 'uninstall':
      return t('plugin.vivy/toolkits-ui.mutationUninstall');
    case 'update':
      return t('plugin.vivy/toolkits-ui.mutationUpdate');
    case 'rebind':
      return t('plugin.vivy/toolkits-ui.mutationRebind');
    case 'rollback':
      return t('plugin.vivy/toolkits-ui.mutationRollback');
    default:
      return mutationKind;
  }
}

export function mcpRuntimeLabel(t: UITranslator, state: string): string {
  switch (state) {
    case 'not-compiled':
      return t('plugin.vivy/toolkits-ui.mcpNotCompiled');
    case 'unconfigured':
      return t('plugin.vivy/toolkits-ui.mcpUnconfigured');
    case 'inactive':
      return t('plugin.vivy/toolkits-ui.mcpInactive');
    case 'ready':
      return t('plugin.vivy/toolkits-ui.mcpReady');
    case 'unavailable':
      return t('plugin.vivy/toolkits-ui.mcpUnavailable');
    case 'deferred':
      return t('plugin.vivy/toolkits-ui.mcpDeferred');
    default:
      return t('plugin.vivy/toolkits-ui.mcpUnknown');
  }
}

export function componentKindLabel(t: UITranslator, kind: ComponentKind): string {
  switch (kind) {
    case 'skills':
      return t('plugin.vivy/toolkits-ui.skills');
    case 'masks':
      return t('plugin.vivy/toolkits-ui.masks');
    case 'rules':
      return t('plugin.vivy/toolkits-ui.rules');
    case 'hooks':
      return t('plugin.vivy/toolkits-ui.hooks');
    default:
      return t('plugin.vivy/toolkits-ui.mcp');
  }
}

export function emptyKindLabel(t: UITranslator, kind: ComponentKind): string {
  switch (kind) {
    case 'skills':
      return t('plugin.vivy/toolkits-ui.emptySkills');
    case 'masks':
      return t('plugin.vivy/toolkits-ui.emptyMasks');
    case 'rules':
      return t('plugin.vivy/toolkits-ui.emptyRules');
    case 'hooks':
      return t('plugin.vivy/toolkits-ui.emptyHooks');
    default:
      return t('plugin.vivy/toolkits-ui.emptyMcp');
  }
}

/** Owner availability is keyed by the singular capability name the backend reports. */
export function ownerKey(kind: ComponentKind): string {
  return kind === 'skills' ? 'skill' : kind === 'masks' ? 'mask' : kind === 'rules' ? 'rule' : kind === 'hooks' ? 'hook' : 'mcp';
}

/** Component changes and component reads report the singular capability name; sections are plural. */
export function componentKindOf(kind: string): ComponentKind | null {
  switch (kind) {
    case 'skill':
      return 'skills';
    case 'mask':
      return 'masks';
    case 'rule':
      return 'rules';
    case 'hook':
      return 'hooks';
    case 'mcp':
      return 'mcp';
    default:
      return null;
  }
}

export function componentCount(manifest: ToolkitManifest, kind: ComponentKind): number {
  return manifest.components[kind].length;
}

/** Digests are identity, so they stay exact; only the display is abbreviated. */
export function shortDigest(value: string, length = 12): string {
  return value.length > length ? value.slice(0, length) : value;
}

export interface CandidateTarget {
  readonly mutationKind: string;
  readonly installId: string;
  /** Empty when the catalog has no matching installation (for example a lost receipt). */
  readonly name: string;
}

export function candidateTarget(
  candidate: PreparedCandidate,
  installations: readonly ToolkitInstallation[],
  selected: ToolkitInspection | null,
): CandidateTarget {
  const prepared = candidate.operation.prepared;
  const installId = prepared?.mutation_install_id ?? '';
  const selectedID = selected?.installation.install_id ?? '';
  if (installId && selectedID === installId) {
    return { mutationKind: prepared?.mutation_kind ?? '', installId, name: selected?.manifest?.name || selected?.installation.bundle_id || '' };
  }
  const match = installations.find((item) => item.install_id === installId);
  return { mutationKind: prepared?.mutation_kind ?? '', installId, name: match?.bundle_id ?? '' };
}
