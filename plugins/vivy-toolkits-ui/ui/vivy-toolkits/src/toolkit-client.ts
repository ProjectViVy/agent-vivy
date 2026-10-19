import { createModuleActionClient, type FaceClientRPC } from '@vivy/ui-sdk';

export const TOOLKIT_MODULE_ID = 'vivy/toolkits' as const;
export const TOOLKIT_ACTIONS = Object.freeze({
  list: 'vivy.bundles.list',
  get: 'vivy.bundles.get',
  inspect: 'vivy.bundles.inspect',
  preview: 'vivy.bundles.preview',
  previewGit: 'vivy.bundles.preview-git',
  install: 'vivy.bundles.install',
  prepareUpdate: 'vivy.bundles.prepare-update',
  prepareUpdateGit: 'vivy.bundles.prepare-update-git',
  enable: 'vivy.bundles.enable',
  disable: 'vivy.bundles.disable',
  uninstall: 'vivy.bundles.uninstall',
  rollback: 'vivy.bundles.rollback',
  operationGet: 'vivy.bundles.operation.get',
  operationPublish: 'vivy.bundles.operation.publish',
  operationCancel: 'vivy.bundles.operation.cancel',
  componentGet: 'vivy.bundles.component.get',
  hookArm: 'vivy.bundles.hook.arm',
  hookDisarm: 'vivy.bundles.hook.disarm',
  mcpAuthorize: 'vivy.bundles.mcp.authorize',
  mcpRevoke: 'vivy.bundles.mcp.revoke',
  bindingsSet: 'vivy.bundles.bindings.set',
  sourceRebind: 'vivy.bundles.source.rebind',
  sourceRebindGit: 'vivy.bundles.source.rebind-git',
} as const);

export interface CatalogIdentity {
  readonly epoch: number;
  readonly config_revision: number;
  readonly effective_digest: string;
  readonly generation_id: string;
}
export interface ToolkitInstallation {
  readonly install_id: string;
  readonly bundle_id: string;
  readonly state: string;
  readonly source: { readonly kind: string; readonly origin: string; readonly requested_ref?: string; readonly resolved_commit?: string };
  readonly release_digest: string;
  readonly revision: number;
  readonly tombstone: boolean;
}
export interface ToolkitPageResult {
  readonly items: readonly ToolkitInstallation[];
  readonly next_after_id: string;
  readonly catalog: CatalogIdentity;
  readonly capabilities: { readonly owners: Readonly<Record<string, boolean>> };
}
export interface ToolkitManifest {
  readonly id: string;
  readonly version: string;
  readonly name: string;
  readonly description?: string;
  readonly components: {
    readonly skills: readonly { readonly id: string; readonly path: string }[];
    readonly masks: readonly { readonly id: string; readonly name: string; readonly description?: string; readonly path: string; readonly recommendedSkills: readonly string[] }[];
    readonly rules: readonly { readonly id: string; readonly path: string; readonly faces: readonly string[] }[];
    readonly hooks: readonly { readonly id: string; readonly event: string; readonly command: string; readonly matcher?: string; readonly cwd: string; readonly timeoutMs: number; readonly envFrom?: Readonly<Record<string, string>> }[];
    readonly mcp: readonly { readonly id: string; readonly endpoint?: string; readonly command?: string; readonly cwd?: string; readonly authEnv?: string; readonly args: readonly string[]; readonly envFrom?: Readonly<Record<string, string>>; readonly resourceBridge: boolean }[];
  };
  readonly resources: readonly string[];
}
export interface ExecutableInspection {
  readonly kind: 'hook' | 'mcp';
  readonly local_id: string;
  readonly release_digest: string;
  readonly declaration_digest: string;
  readonly consent_current: boolean;
  readonly consent_allowed: boolean;
  readonly consent_revision: number;
  readonly missing_bindings: readonly string[];
  readonly runtime_state?: 'not-compiled' | 'unconfigured' | 'inactive' | 'ready' | 'unavailable' | 'deferred' | string;
  readonly runtime_initialized?: boolean;
  readonly runtime_tool_count?: number;
}
export interface ToolkitInspection {
  readonly installation: ToolkitInstallation;
  readonly release: { readonly install_id: string; readonly bundle_id: string; readonly release_digest: string; readonly release_sequence: number };
  readonly releases: readonly { readonly install_id: string; readonly bundle_id: string; readonly release_digest: string; readonly release_sequence: number }[];
  readonly manifest?: ToolkitManifest;
  readonly consents: readonly { readonly kind: string; readonly local_id: string; readonly declaration_digest: string; readonly allowed: boolean; readonly revision: number; readonly subject: string }[];
  readonly executables: readonly ExecutableInspection[];
  readonly bindings: readonly { readonly install_id: string; readonly kind: 'hook' | 'mcp'; readonly local_id: string; readonly name: string; readonly reference: string }[];
}
export interface ToolkitOperation {
  readonly id: string;
  readonly kind: string;
  readonly state: string;
  readonly revision: number;
  readonly expected_epoch: number;
  readonly committed_epoch: number;
  readonly error_code?: string;
  readonly retryable: boolean;
  readonly progress?: string;
  readonly blockers: readonly { readonly run_id: string; readonly session_id: string; readonly status: string }[];
  readonly prepared?: {
    readonly input_digest: string;
    readonly expected_catalog: CatalogIdentity;
    readonly candidate_digest: string;
    readonly releases: readonly unknown[];
    readonly new_releases: readonly unknown[];
    readonly installations: readonly unknown[];
    readonly mutation_kind: string;
    readonly mutation_install_id: string;
    readonly changes: readonly { readonly kind: string; readonly local_id: string; readonly change: 'added' | 'removed' | 'changed'; readonly before_digest?: string; readonly after_digest?: string; readonly requires_fresh_consent?: boolean }[];
  };
}
export interface PreparedCandidate {
  readonly operation: ToolkitOperation;
  readonly candidate_digest: string;
  readonly expected_catalog: CatalogIdentity;
  readonly releases: readonly unknown[];
}
export interface ToolkitPreview {
  readonly operation_id: string;
  readonly install_id: string;
  readonly release_digest: string;
  readonly source: ToolkitInstallation['source'];
  readonly inventory: readonly { readonly path: string; readonly size: number; readonly sha256: string }[];
  readonly manifest: ToolkitManifest;
}

export interface ToolkitActionTransport {
  invoke<Input = unknown, Result = unknown>(request: { readonly moduleId: string; readonly actionId: string; readonly input: Input }): Promise<Result>;
}

export class ToolkitClient {
  constructor(private readonly actions: ToolkitActionTransport) {}
  static fromRPC(rpc: FaceClientRPC): ToolkitClient { return new ToolkitClient(createModuleActionClient(rpc)); }

  list(input: { readonly after_id?: string; readonly limit?: number; readonly state?: string } = {}): Promise<ToolkitPageResult> {
    return this.invoke(TOOLKIT_ACTIONS.list, { after_id: input.after_id ?? '', limit: input.limit ?? 50, state: input.state ?? '' });
  }
  inspect(installId: string): Promise<ToolkitInspection> { return this.invoke(TOOLKIT_ACTIONS.inspect, { install_id: installId }); }
  preview(kind: 'local' | 'git', input: { readonly operation_id: string; readonly expected_catalog: CatalogIdentity; readonly location: string; readonly ref?: string }): Promise<ToolkitPreview> {
    return this.invoke(kind === 'git' ? TOOLKIT_ACTIONS.previewGit : TOOLKIT_ACTIONS.preview, input);
  }
  install(input: { readonly operation_id: string; readonly expected_catalog: CatalogIdentity; readonly preview_operation_id: string; readonly expected_release_digest: string }): Promise<ToolkitOperation> { return this.invoke(TOOLKIT_ACTIONS.install, input); }
  prepare(kind: 'enable' | 'disable' | 'uninstall' | 'rollback', input: Record<string, unknown>): Promise<PreparedCandidate> { return this.invoke(TOOLKIT_ACTIONS[kind], input); }
  prepareSource(kind: 'local' | 'git', operation: 'update' | 'rebind', input: { readonly operation_id: string; readonly preview_operation_id: string; readonly expected_catalog: CatalogIdentity; readonly install_id: string; readonly expected_install_revision: number; readonly location: string; readonly ref?: string }): Promise<PreparedCandidate> {
    const action = operation === 'update'
      ? kind === 'git' ? TOOLKIT_ACTIONS.prepareUpdateGit : TOOLKIT_ACTIONS.prepareUpdate
      : kind === 'git' ? TOOLKIT_ACTIONS.sourceRebindGit : TOOLKIT_ACTIONS.sourceRebind;
    return this.invoke(action, input);
  }
  publish(input: { readonly operation_id: string; readonly candidate_digest: string; readonly expected_catalog: CatalogIdentity }): Promise<ToolkitOperation> { return this.invoke(TOOLKIT_ACTIONS.operationPublish, input); }
  operation(operationId: string): Promise<ToolkitOperation> { return this.invoke(TOOLKIT_ACTIONS.operationGet, { operation_id: operationId }); }
  cancel(input: { readonly operation_id: string; readonly expected_operation_revision: number }): Promise<ToolkitOperation> { return this.invoke(TOOLKIT_ACTIONS.operationCancel, input); }
  component(input: { readonly install_id: string; readonly kind: string; readonly local_id: string; readonly expected_release_digest: string; readonly offset?: number; readonly max_bytes?: number }): Promise<{ readonly content: string; readonly digest: string; readonly offset: number; readonly next_offset: number; readonly total_bytes: number }> { return this.invoke(TOOLKIT_ACTIONS.componentGet, input); }
  consent(kind: 'hook' | 'mcp', allow: boolean, input: Record<string, unknown>): Promise<ToolkitOperation> {
    const id = kind === 'hook' ? (allow ? TOOLKIT_ACTIONS.hookArm : TOOLKIT_ACTIONS.hookDisarm) : (allow ? TOOLKIT_ACTIONS.mcpAuthorize : TOOLKIT_ACTIONS.mcpRevoke);
    return this.invoke(id, input);
  }
  setBindings(input: { readonly operation_id: string; readonly expected_catalog: CatalogIdentity; readonly expected_config_revision: number; readonly install_id: string; readonly kind: 'hook' | 'mcp'; readonly component_id: string; readonly bindings: readonly { readonly name: string; readonly reference: string }[] }): Promise<ToolkitOperation> {
    return this.invoke(TOOLKIT_ACTIONS.bindingsSet, input);
  }
  async componentContent(input: { readonly install_id: string; readonly kind: string; readonly local_id: string; readonly expected_release_digest: string }): Promise<{ readonly content: string; readonly digest: string; readonly total_bytes: number }> {
    const pages: string[] = [];
    let offset = 0;
    let digest = '';
    let totalBytes = 0;
    for (let pageIndex = 0; pageIndex < 16; pageIndex += 1) {
      const page = await this.component({ ...input, offset, max_bytes: 64 * 1024 });
      if (page.offset !== offset || page.next_offset < offset || page.total_bytes > 512 * 1024 || page.next_offset > page.total_bytes || (digest && digest !== page.digest) || (totalBytes && totalBytes !== page.total_bytes)) {
        throw new Error('Toolkit component page changed or exceeded its read bound.');
      }
      digest = page.digest;
      totalBytes = page.total_bytes;
      pages.push(page.content);
      if (page.next_offset === page.total_bytes) return { content: pages.join(''), digest, total_bytes: totalBytes };
      if (page.next_offset <= offset) throw new Error('Toolkit component page did not advance.');
      offset = page.next_offset;
    }
    throw new Error('Toolkit component is larger than the UI read bound.');
  }
  private invoke<Input, Result>(actionId: string, input: Input): Promise<Result> { return this.actions.invoke<Input, Result>({ moduleId: TOOLKIT_MODULE_ID, actionId, input }); }
}

export function newOperationID(): string {
  if (typeof crypto === 'undefined' || typeof crypto.randomUUID !== 'function') throw new Error('This browser cannot create a secure operation ID.');
  return crypto.randomUUID();
}
