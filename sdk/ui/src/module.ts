import type * as React from "react";

/** The public Port identities consumed by the full-code UI compiler. */
export const UI_EXTENSION_PORT = "std/ui-extension@v1" as const;
export const UI_ROOT_PORT = "std/ui-root@v1" as const;
export const UI_PORTS = Object.freeze({
  extension: UI_EXTENSION_PORT,
  root: UI_ROOT_PORT,
});

export type UIPort = (typeof UI_PORTS)[keyof typeof UI_PORTS];

/** The only forms accepted by the shared Web/TUI localization Host. */
export type UIMessageForm = "" | "short" | "long";
export type UITranslationForm = UIMessageForm;
export type TranslationForm = UIMessageForm;
export type UITranslationArgs = Readonly<Record<string, string | number>>;
export type UITranslator = (
  key: string,
  args?: UITranslationArgs,
  form?: UIMessageForm,
) => string;

/**
 * A cleanup callback returned from a UI extension. Cleanup is deliberately
 * synchronous: the PresentationHost can reverse registrations deterministically
 * during a generation transition.
 */
export type UICleanup = () => void;
export type Cleanup = UICleanup;

/** An idempotent, explicit lifecycle handle owned by the PresentationHost. */
export interface CleanupHandle {
  readonly active: boolean;
  dispose(): void;
}

export type UICleanupHandle = CleanupHandle;
export type UIExtensionCleanupHandle = CleanupHandle;

/**
 * Creates an idempotent cleanup handle. The callback is called at most once,
 * including when a callback throws. The first throw is rethrown to the caller
 * of dispose so the Host can report it without invoking cleanup twice.
 */
export function createCleanupHandle(cleanup: UICleanup): CleanupHandle {
  if (typeof cleanup !== "function") {
    throw new TypeError("UI cleanup must be a function");
  }

  let active = true;
  return {
    get active() {
      return active;
    },
    dispose() {
      if (!active) return;
      active = false;
      cleanup();
    },
  };
}

/**
 * Converts the return value of UIExtension.install into an explicit handle.
 * `undefined` is a valid no-op installation; every other non-function value
 * is rejected so malformed Modules cannot silently leak registrations.
 */
export function cleanupHandleFromInstall(result: void | UICleanup): CleanupHandle;
export function cleanupHandleFromInstall(result: unknown): CleanupHandle {
  if (result === undefined) return createCleanupHandle(() => undefined);
  if (typeof result !== "function") {
    throw new TypeError("UI cleanup must be a function");
  }
  return createCleanupHandle(result as UICleanup);
}

export const cleanupFromInstall = cleanupHandleFromInstall;
export const normalizeCleanup = cleanupHandleFromInstall;
export const toCleanupHandle = cleanupHandleFromInstall;

/**
 * A full-code UI extension receives the complete Web Face Host. Selection into
 * a Generation is the authority to run this trusted frontend code.
 */
export interface UIExtension extends UIContributionRelations {
  readonly id: string;
  install(host: FullUIHost): void | UICleanup;
}

/** An exclusive Web Face root supplied by a selected Module. */
export interface UIRoot extends UIContributionRelations {
  readonly id: string;
  render(host: FullUIHost): React.ReactNode;
}

type UIExtensionDefinition = Omit<UIExtension, "install"> & {
  install(host: FullUIHost): unknown;
};
type ValidInstall<T extends UIExtensionDefinition> =
  Exclude<ReturnType<T["install"]>, void | undefined | UICleanup> extends never
    ? unknown
    : { readonly install: never };

/** Type-safe authoring helper that catches malformed cleanup return values. */
export function defineUIExtension<T extends UIExtensionDefinition>(
  extension: T & ValidInstall<T>,
): T & UIExtension {
  return extension as T & UIExtension;
}

/** Type-safe authoring helper for exclusive root providers. */
export function defineUIRoot<T extends UIRoot>(root: T): T {
  return root;
}

export interface UIContributionRelations {
  /** IDs that must be installed before this contribution. */
  readonly before?: readonly string[] | string;
  /** IDs that must be installed before this contribution. */
  readonly after?: readonly string[] | string;
  /** IDs this contribution explicitly replaces. */
  readonly replaces?: readonly string[] | string;
}

/** Metadata attached to an input contribution instead of the entry itself. */
export interface UIContributionOptions extends UIContributionRelations {}

/** Canonical extension provider input for composition and Assembly. */
export interface UIExtensionProvider extends UIContributionOptions {
  readonly extension: UIExtension;
  readonly port: typeof UI_EXTENSION_PORT;
}

/** Canonical exclusive-root provider input for composition and Assembly. */
export interface UIRootProvider extends UIContributionOptions {
  readonly root: UIRoot;
  readonly port: typeof UI_ROOT_PORT;
}

/** Generic provider envelope accepted by generated Assembly inputs. */
export type UIProvider<T extends UIExtension | UIRoot> =
  T extends UIExtension
    ? UIContributionOptions & { readonly provider: T; readonly port: typeof UI_EXTENSION_PORT }
    : UIContributionOptions & { readonly provider: T; readonly port: typeof UI_ROOT_PORT };

/** Alternate names used by generators when describing selected contributions. */
export type UIExtensionContribution = UIExtensionProvider;
export type UIRootContribution = UIRootProvider;

export type UIExtensionInput = UIExtension | UIExtensionProvider | UIProvider<UIExtension>;
export type UIRootInput = UIRoot | UIRootProvider | UIProvider<UIRoot>;

/** Pure input consumed by the deterministic UI composition validator. */
export interface UICompositionInput {
  /** At most one root provider may be selected. */
  readonly roots?: readonly UIRootInput[];
  /** Explicit provider-envelope spelling used by generated Assembly code. */
  readonly rootProviders?: readonly UIRootInput[];
  /** Singular convenience form for generated compositions with one root. */
  readonly root?: UIRootInput;
  /** Ordered extension providers; before/after edges may further constrain it. */
  readonly extensions?: readonly UIExtensionInput[];
  /** Explicit provider-envelope spelling used by generated Assembly code. */
  readonly extensionProviders?: readonly UIExtensionInput[];
}

/** A normalized, validated composition plan; composing it does not execute code. */
export interface UIComposition {
  readonly root?: UIRoot;
  readonly roots: readonly UIRoot[];
  readonly extensions: readonly UIExtension[];
  readonly rootProviders: readonly UIRootProvider[];
  readonly extensionProviders: readonly UIExtensionProvider[];
}

export class UICompositionError extends Error {
  readonly code: UICompositionErrorCode;

  constructor(code: UICompositionErrorCode, message: string) {
    super(message);
    this.name = "UICompositionError";
    this.code = code;
  }
}

export type UICompositionErrorCode =
  | "missing_id"
  | "duplicate_id"
  | "duplicate_root"
  | "missing_port"
  | "unresolved_before"
  | "unresolved_after"
  | "unresolved_replaces"
  | "port_mismatch"
  | "invalid_provider"
  | "composition_cycle";

/**
 * Validates and normalizes a UI composition. This is intentionally a pure
 * build-time operation: it never imports, installs, discovers, or evaluates
 * a Module and it never performs network or backend operations.
 */
export function composeUI(input: UICompositionInput): UIComposition {
  const roots = normalizeRoots(input);
  const extensions = normalizeExtensions([...(input.extensions ?? []), ...(input.extensionProviders ?? [])]);

  if (roots.length > 1) {
    throw new UICompositionError(
      "duplicate_root",
      "duplicate root provider: std/ui-root@v1 accepts at most one provider",
    );
  }

  const entries: NormalizedContribution[] = [
    ...roots.map((provider) => ({ kind: "root" as const, provider })),
    ...extensions.map((provider) => ({ kind: "extension" as const, provider })),
  ];
  validateIDs(entries);
  validateRelations(entries);

  const orderedExtensions = orderExtensions(extensions);
  return {
    root: roots[0]?.root,
    roots: roots.map((provider) => provider.root),
    extensions: orderedExtensions.map((provider) => provider.extension),
    rootProviders: roots,
    extensionProviders: orderedExtensions,
  };
}

/** Descriptive alias for callers that do not need the normalized result. */
export const validateUIComposition = composeUI;
export const validateComposition = composeUI;
export const createUIComposition = composeUI;

function normalizeRoots(input: UICompositionInput): UIRootProvider[] {
  const roots: UIRootInput[] = [];
  if (input.root !== undefined) roots.push(input.root);
  roots.push(...(input.roots ?? []));
  roots.push(...(input.rootProviders ?? []));
  return roots.map((value) => normalizeRoot(value));
}

function normalizeExtensions(input: readonly UIExtensionInput[]): UIExtensionProvider[] {
  return input.map((value) => normalizeExtension(value));
}

function normalizeRoot(value: UIRootInput): UIRootProvider {
  if (isRootProvider(value)) {
    assertPort(value.port, UI_ROOT_PORT);
    const root = "root" in value ? value.root : value.provider;
    return {
      root,
      before: value.before,
      after: value.after,
      replaces: value.replaces,
      port: value.port,
    };
  }
  const relations = contributionRelations(value);
  return {
    root: value,
    port: UI_ROOT_PORT,
    ...relations,
  };
}

function normalizeExtension(value: UIExtensionInput): UIExtensionProvider {
  if (isExtensionProvider(value)) {
    assertPort(value.port, UI_EXTENSION_PORT);
    const extension = "extension" in value ? value.extension : value.provider;
    return {
      extension,
      before: value.before,
      after: value.after,
      replaces: value.replaces,
      port: value.port,
    };
  }
  const relations = contributionRelations(value);
  return {
    extension: value,
    port: UI_EXTENSION_PORT,
    ...relations,
  };
}

function assertPort(actual: unknown, expected: UIPort): void {
  if (actual === undefined) {
    throw new UICompositionError(
      "missing_port",
      `provider envelope must declare ${expected}`,
    );
  }
  if (actual !== expected) {
    throw new UICompositionError(
      "port_mismatch",
      `provider envelope declares ${String(actual)} but this input requires ${expected}`,
    );
  }
}

function contributionRelations(value: unknown): UIContributionOptions {
  if (!isRecord(value)) return {};
  return {
    before: relationValue(value.before),
    after: relationValue(value.after),
    replaces: relationValue(value.replaces),
  };
}

function relationValue(value: unknown): readonly string[] | string | undefined {
  if (typeof value === "string") return value;
  if (Array.isArray(value) && value.every((item) => typeof item === "string")) return value;
  return undefined;
}

function isRootProvider(value: UIRootInput): value is UIRootProvider | UIProvider<UIRoot> {
  return isRecord(value) && ("root" in value || "provider" in value);
}

function isExtensionProvider(value: UIExtensionInput): value is UIExtensionProvider | UIProvider<UIExtension> {
  return isRecord(value) && ("extension" in value || "provider" in value);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

type NormalizedContribution =
  | { readonly kind: "root"; readonly provider: UIRootProvider }
  | { readonly kind: "extension"; readonly provider: UIExtensionProvider };

function validateIDs(entries: readonly NormalizedContribution[]): void {
  const seen = new Set<string>();
  for (const entry of entries) {
    const candidate = entry.kind === "root" ? entry.provider.root : entry.provider.extension;
    const id = isRecord(candidate) && typeof candidate.id === "string" ? candidate.id : undefined;
    if (typeof id !== "string" || id.trim() === "") {
      throw new UICompositionError("missing_id", "UI provider id is required");
    }
    if (seen.has(id)) {
      throw new UICompositionError("duplicate_id", `duplicate UI provider id ${id}`);
    }
    seen.add(id);

    const behavior = isRecord(candidate) ? candidate : undefined;
    if (entry.kind === "root" && typeof behavior?.render !== "function") {
      throw new UICompositionError(
        "invalid_provider",
        `UI root provider ${id} must expose a render(host) function`,
      );
    }
    if (entry.kind === "extension" && typeof behavior?.install !== "function") {
      throw new UICompositionError(
        "invalid_provider",
        `UI extension provider ${id} must expose an install(host) function`,
      );
    }
  }
}

function validateRelations(entries: readonly NormalizedContribution[]): void {
  const known = new Set(entries.map((entry) => {
    const candidate = entry.kind === "root" ? entry.provider.root : entry.provider.extension;
    return isRecord(candidate) && typeof candidate.id === "string" ? candidate.id : "";
  }));
  for (const entry of entries) {
    const candidate = entry.kind === "root" ? entry.provider.root : entry.provider.extension;
    const id = isRecord(candidate) && typeof candidate.id === "string" ? candidate.id : "";
    const relations = entry.provider;
    for (const relation of ["before", "after", "replaces"] as const) {
      const code = `unresolved_${relation}` as UICompositionErrorCode;
      for (const reference of relationValues(relations[relation])) {
        if (!known.has(reference)) {
          throw new UICompositionError(code, `unresolved ${relation} reference ${reference} for ${id}`);
        }
        if (reference === id) {
          throw new UICompositionError(code, `${relation} reference ${reference} cannot target ${id}`);
        }
      }
    }
  }
}

function relationValues(value: readonly string[] | string | undefined): readonly string[] {
  if (value === undefined) return [];
  return typeof value === "string" ? [value] : value;
}

function orderExtensions(providers: readonly UIExtensionProvider[]): UIExtensionProvider[] {
  const byID = new Map(providers.map((provider) => [provider.extension.id, provider]));
  const edges = new Map<string, Set<string>>();
  const indegree = new Map<string, number>();
  for (const provider of providers) {
    const id = provider.extension.id;
    edges.set(id, new Set());
    indegree.set(id, 0);
  }

  for (const provider of providers) {
    const id = provider.extension.id;
    for (const target of relationValues(provider.before)) {
      if (!byID.has(target)) continue;
      addEdge(id, target, edges, indegree);
    }
    for (const target of relationValues(provider.after)) {
      if (!byID.has(target)) continue;
      addEdge(target, id, edges, indegree);
    }
  }

  const originalIndex = new Map(providers.map((provider, index) => [provider.extension.id, index]));
  const ready = providers.filter((provider) => indegree.get(provider.extension.id) === 0).map((provider) => provider.extension.id);
  const result: UIExtensionProvider[] = [];
  while (ready.length > 0) {
    ready.sort((left, right) => (originalIndex.get(left) ?? 0) - (originalIndex.get(right) ?? 0));
    const id = ready.shift()!;
    result.push(byID.get(id)!);
    for (const next of edges.get(id) ?? []) {
      const degree = (indegree.get(next) ?? 0) - 1;
      indegree.set(next, degree);
      if (degree === 0) ready.push(next);
    }
  }
  if (result.length !== providers.length) {
    throw new UICompositionError("composition_cycle", "UI extension before/after relationships contain a cycle");
  }
  return result;
}

function addEdge(from: string, to: string, edges: Map<string, Set<string>>, indegree: Map<string, number>): void {
  const successors = edges.get(from)!;
  if (successors.has(to)) return;
  successors.add(to);
  indegree.set(to, (indegree.get(to) ?? 0) + 1);
}

/** Generic registration handle used by full-code composition registries. */
export interface UIRegistrationHandle extends CleanupHandle {
  readonly id: string;
}

export interface UIRegistry<T = unknown> {
  register(id: string, value: T): UIRegistrationHandle;
  unregister(id: string): void;
}

/**
 * The broad, host-owned composition surface. Concrete Web Face registries can
 * specialize these values without changing the public Module ABI.
 */
export interface UICompositionHost {
  readonly routes: UIRegistry<unknown>;
  readonly navigation: UIRegistry<unknown>;
  readonly pages: UIRegistry<unknown>;
  readonly components: UIRegistry<unknown>;
  readonly styles: UIRegistry<unknown>;
  readonly themes: UIRegistry<unknown>;
  readonly shortcuts: UIRegistry<unknown>;
  readonly commands: UIRegistry<unknown>;
  registerCleanup(cleanup: UICleanup): CleanupHandle;
}

/** Core session data returned by the existing Web Face API. */
export type FaceRunMode = "normal" | "plan";
export type FaceName = "web" | "tui" | "code";
export type FaceTodoStatus = "pending" | "in_progress" | "completed" | "cancelled";
export type FacePermissionPreset = "cautious" | "smart" | "trusted" | "custom";
export type FaceSandboxMode = "read_only" | "workspace_write" | "danger_full_access";
export type FaceApprovalPolicy = "ask" | "never" | "auto";

export interface FaceSession {
  readonly id: string;
  readonly title: string;
  readonly created_at: number;
  readonly sandbox_mode?: FaceSandboxMode;
  readonly approval_policy?: FaceApprovalPolicy;
  readonly permission_preset?: FacePermissionPreset;
}

export type FaceMessageRole = "user" | "assistant" | "system" | "tool";

/** Core message data returned by the existing Web Face API. */
export interface FaceMessage {
  readonly id: string;
  readonly run_id?: string;
  readonly role: FaceMessageRole;
  readonly content: string;
  readonly created_at: number;
  readonly attachments?: FaceMessageAttachment[];
  readonly provenance?: FaceMessageProvenance;
}

export interface FaceMessageProvenance {
  readonly source: string;
  readonly channel?: string;
  readonly chat_id?: string;
  readonly channel_message_id?: string;
}

export interface FaceMessageAttachment {
  readonly name?: string;
  readonly mime_type: string;
  readonly data_url: string;
}

export interface FaceAttachmentInput {
  readonly name?: string;
  readonly mime_type: string;
  readonly data: string;
}

export type FaceThinkingMode = "auto" | "on" | "off";

export type FaceRunStatus = "accepted" | "queued" | "active" | "completed" | "failed" | "cancelled";

/** Core run data returned by the existing Web Face API. */
export interface FaceRun {
  readonly id: string;
  readonly session_id: string;
  readonly status: FaceRunStatus;
  readonly created_at: number;
}

export interface FaceTodo {
  readonly id: string;
  readonly session_id: string;
  readonly subject: string;
  readonly description: string;
  readonly status: FaceTodoStatus;
  readonly blocks: string[];
  readonly blocked_by: string[];
  readonly active_form?: string;
  readonly owner?: string;
  readonly position: number;
  readonly created_at: number;
  readonly updated_at: number;
}

export interface FaceSessionContext {
  readonly session_id: string;
  readonly total_messages: number;
  readonly feed_messages: number;
  readonly feed_bytes: number;
  readonly feed_tokens: number;
  readonly limit_bytes: number;
  readonly model_limit_tokens: number;
  readonly thinking_supported: boolean;
  readonly compaction_enabled: boolean;
  readonly trigger_tokens: number;
  readonly would_compact: boolean;
  readonly has_compaction_summary: boolean;
  readonly last_compaction?: {
    readonly mode: "reduction" | "summarization" | "session";
    readonly before_tokens: number;
    readonly after_tokens: number;
    readonly at: number;
  } | null;
}

export interface FaceCompactResult {
  readonly before_tokens: number;
  readonly after_tokens: number;
  readonly folded_messages: number;
  readonly skipped: boolean;
}

export interface FaceSessionCompactionRecord {
  readonly run_id: string;
  readonly created_at: number;
  readonly tail_from: number;
  readonly dropped_count: number;
  readonly summary: string;
}

export interface FaceBackgroundRun extends FaceRun {
  readonly workspace_id?: string;
}

export interface FaceChildRun extends FaceRun {
  readonly parent_run_id: string;
  readonly root_run_id: string;
  readonly depth: number;
  readonly workspace_id?: string;
  readonly result?: string;
  readonly error?: string;
}

export type FaceReviewKind = "approval" | "question";
export type FaceReviewStatus = "pending" | "approved" | "denied" | "answered" | "cancelled" | "expired" | "stale";

export interface FaceReviewItem {
  readonly id: string;
  readonly kind: FaceReviewKind;
  readonly status: FaceReviewStatus;
  readonly session_id: string;
  readonly session_title?: string;
  readonly run_id: string;
  readonly tool_call_id?: string;
  readonly tool_name?: string;
  readonly source?: string;
  readonly actor?: string;
  readonly created_at: number;
  readonly expires_at: number;
  readonly decided_at?: number;
  readonly action?: string;
  readonly target?: string;
  readonly precondition_hash?: string;
  readonly preview?: string;
  readonly risk_findings?: string[];
  readonly arguments?: Record<string, unknown>;
  readonly prompt?: string;
  readonly decision_reason?: string;
  readonly stale_reason?: string;
  readonly error?: string;
  readonly effect?: string;
  readonly reversibility?: string;
  readonly scope?: string;
  readonly trust?: string;
}

export interface FaceWorkspaceFile {
  readonly path: string;
  readonly size: number;
}

export interface FaceWorkspaceFileContent extends FaceWorkspaceFile {
  readonly content: string;
  readonly truncated: boolean;
  readonly binary: boolean;
}

export interface FaceRewindResult {
  readonly cutoff_message_id: string;
  readonly remaining_count: number;
}

export interface FaceForkResult {
  readonly session_id: string;
  readonly fork_point_message_id: string;
  readonly copied_count: number;
}

export interface FaceRunStartResult {
  readonly run_id: string;
  readonly status: FaceRunStatus;
}

export interface FaceRunInterruptResult {
  readonly run_id: string;
  readonly status: string;
}

export interface FaceChildStartInput {
  readonly parent_run_id: string;
  readonly text: string;
  readonly policy_profile?: string;
  readonly tool_names?: string[];
}

export interface FaceReviewListInput {
  readonly kind?: FaceReviewKind;
  readonly status?: FaceReviewStatus;
  readonly session_id?: string;
  readonly limit?: number;
}

export interface FaceReviewResponse {
  readonly action: "approve" | "deny" | "answer" | "cancel";
  readonly reason?: string;
  readonly answer?: string;
}

export interface FaceSessionList {
  readonly sessions: readonly FaceSession[];
}

export interface FaceSessionDetail {
  readonly session: FaceSession;
  readonly messages: readonly FaceMessage[];
}

export interface FaceMessageList {
  readonly messages: readonly FaceMessage[];
}

export interface FaceRunLogEvent {
  readonly run_id: string;
  readonly seq: number;
  readonly type: string;
  readonly created_at: number;
  readonly payload_version: number;
  readonly payload: Readonly<Record<string, unknown>>;
}

export interface FaceRunLog {
  readonly events: readonly FaceRunLogEvent[];
}

export type FaceLocale = "en" | "zh";

export interface FaceLocaleSettings {
  readonly locale: FaceLocale;
  readonly generation_locale: FaceLocale;
  readonly workspace_locale: FaceLocale | "";
  readonly locale_read_only: boolean;
}

export interface FaceSettings extends FaceLocaleSettings {
  readonly provider: string;
  readonly default_model: string;
  readonly base_url: string;
  readonly read_only: boolean;
  readonly config_provider: string;
  readonly config_model: string;
  readonly frozen?: boolean;
  readonly api_key_set?: boolean;
  readonly network_search?: FaceNetworkSearchSettings;
  readonly execute_max_timeout_seconds?: number;
  readonly config_execute_max_timeout_seconds?: number;
  readonly sandbox?: FaceSandboxSettings;
  readonly compaction?: FaceCompactionSettings;
  readonly http?: FaceHTTPSettings;
}

export interface FaceNetworkSearchProviderInfo {
  readonly name: string;
  readonly keyless: boolean;
  readonly configured: boolean;
  readonly env_key?: string;
}

export interface FaceNetworkSearchSettings {
  readonly provider: string;
  readonly config_provider: string;
  readonly providers: FaceNetworkSearchProviderInfo[];
}

export interface FaceCompactionSettings {
  readonly enabled: boolean;
  readonly max_tokens: number;
  readonly trigger_percent: number;
  readonly keep_recent: number;
  readonly config_enabled: boolean;
  readonly config_max_tokens: number;
  readonly config_trigger_percent: number;
  readonly config_keep_recent: number;
}

export interface FaceHTTPSettings {
  readonly allowed_hosts: string[];
  readonly timeout_seconds: number;
  readonly config_allowed_hosts: string[];
  readonly config_timeout_seconds: number;
  readonly overlay_set: boolean;
}

export interface FaceSandboxSettings {
  readonly default_preset: FacePermissionPreset;
  readonly config_default_preset: FacePermissionPreset;
  readonly deny_private_ips: boolean;
  readonly allowed_domains: string[];
  readonly workspace_root?: string;
  readonly execute_allowed_commands?: string[];
}

export interface FaceSettingsUpdate {
  readonly provider: string;
  readonly default_model: string;
  readonly base_url: string;
  readonly api_key?: string;
  readonly network_search?: { readonly provider: string };
  readonly execute_max_timeout_seconds?: number;
  readonly sandbox?: {
    readonly default_preset: Exclude<FacePermissionPreset, "custom">;
    readonly deny_private_ips: boolean;
    readonly allowed_domains: string[];
  };
  readonly compaction?: {
    readonly enabled: boolean;
    readonly max_tokens: number;
    readonly trigger_percent: number;
    readonly keep_recent: number;
  };
  readonly http?: {
    readonly allowed_hosts: string[];
    readonly timeout_seconds: number;
  };
}

export interface FaceSpeciesInspect {
  readonly protocol_version: string;
  readonly binary_id: string;
  readonly generation_id: string;
  readonly artifact_sha256?: string;
  /** Final UI artifact hash from the sealed runtime Generation Manifest. */
  readonly ui_artifact_sha256?: string;
  readonly recipe: FaceRecipe;
  readonly policy_profile: string;
  readonly policy_hash: string;
  readonly tools: { readonly name: string; readonly readonly: boolean }[];
  readonly grants: string[];
}

export interface FaceRecipe {
  readonly loop?: string;
  readonly world?: string;
  readonly providers?: string[];
  readonly tools?: string[];
  readonly plugins?: string[];
}

export type FaceGenerationPhase = "built" | "eval_pending" | "evaluated" | "promoted" | "released" | "rejected";

export interface FaceGeneration {
  readonly id: string;
  readonly parent_id?: string;
  readonly artifact_sha256: string;
  readonly source_ref?: string;
  readonly recipe: FaceRecipe;
  readonly phase: FaceGenerationPhase;
  readonly created_at: number;
}

export type FaceEvalVerdict = "better" | "worse" | "mixed" | "failed_to_run";

export interface FaceEvalRun {
  readonly id: string;
  readonly candidate_id: string;
  readonly baseline_id?: string;
  readonly suite: string;
  readonly verdict: FaceEvalVerdict;
  readonly journal_ref?: string;
  readonly created_at: number;
}

export interface FacePromotion {
  readonly id: string;
  readonly from_id: string;
  readonly to_id: string;
  readonly eval_id?: string;
  readonly actor: string;
  readonly phase: string;
  readonly applies_at: string;
  readonly created_at: number;
}

export interface FaceApproval {
  readonly id: string;
  readonly run_id: string;
  readonly tool_call_id: string;
  readonly decision?: string;
  readonly expires_at: number;
}

export interface FaceQuestion {
  readonly id: string;
  readonly run_id: string;
  readonly tool_call_id: string;
  readonly prompt: string;
  readonly status: string;
  readonly expires_at: number;
}

export interface FaceApprovalResponse {
  readonly approval_id: string;
  readonly decision: "approved" | "denied";
}

export interface FaceQuestionResponse {
  readonly question_id: string;
  readonly answer: string;
}

export interface FaceToolCatalogEntry {
  readonly name: string;
  readonly description: string;
  readonly readonly: boolean;
  readonly active: boolean;
}

export interface FaceToolsCatalogView {
  readonly tools: readonly FaceToolCatalogEntry[];
  readonly active: readonly string[];
  readonly config_enabled: readonly string[];
  readonly overlay_written: boolean;
}

export interface FaceProviderEntry {
  readonly id: string;
  readonly display_name: string;
  readonly bundle: "openai" | "anthropic";
  readonly base_url: string;
  readonly default_model: string;
  readonly models: string[];
  readonly api_key_set: boolean;
}

export interface FaceProviderEntryInput {
  readonly id?: string;
  readonly display_name: string;
  readonly bundle: "openai" | "anthropic";
  readonly base_url: string;
  readonly default_model: string;
  readonly models: string[];
  readonly api_key?: string;
}

export interface FaceProvidersView {
  readonly entries: readonly FaceProviderEntry[];
  readonly active_provider: string;
  readonly active_model: string;
  readonly active_base_url: string;
  readonly read_only: boolean;
  readonly frozen?: boolean;
  readonly config_provider: string;
  readonly config_model: string;
}

export interface FaceProviderRefreshInput {
  readonly id?: string;
  readonly bundle?: "openai";
  readonly base_url?: string;
  readonly display_name?: string;
  readonly default_model?: string;
}

export type FaceMcpStatus = "idle" | "ok" | "error";
export type FaceMcpTransport = "http" | "stdio";

export interface FaceMcpServer {
  readonly name: string;
  readonly transport: FaceMcpTransport;
  readonly endpoint?: string;
  readonly command?: string;
  readonly args?: readonly string[];
  readonly env_from?: Readonly<Record<string, string>>;
  readonly cwd?: string;
  readonly auth_env?: string;
  readonly auth_env_set: boolean;
  readonly env_missing?: readonly string[];
  readonly enabled: boolean;
  readonly tool_count: number;
  readonly status: FaceMcpStatus;
  readonly error?: string;
}

export interface FaceMcpServerInput {
  readonly name: string;
  readonly transport: FaceMcpTransport;
  readonly endpoint?: string;
  readonly command?: string;
  readonly args?: string[];
  readonly env_from?: Record<string, string>;
  readonly cwd?: string;
  readonly auth_env?: string;
  readonly enabled?: boolean;
}

export interface FaceMcpServersView {
  readonly servers: readonly FaceMcpServer[];
  readonly read_only: boolean;
}

export interface FaceChannelCapabilities {
  readonly typing: boolean;
  readonly edit: boolean;
  readonly delete: boolean;
  readonly reaction: boolean;
  readonly placeholder: boolean;
  readonly media: boolean;
  readonly media_store: boolean;
  readonly webhook: boolean;
  readonly listen: boolean;
  readonly stream: boolean;
  readonly health: boolean;
}

export interface FaceChannelStatus {
  readonly name: string;
  readonly capabilities: FaceChannelCapabilities;
  readonly configured: boolean;
  readonly enabled: boolean;
  readonly allow_from: readonly string[];
  readonly started: boolean;
  readonly token_env: string;
  readonly token_env_set: boolean;
  readonly note: string;
}

export interface FaceChannelEnvelope {
  readonly name: string;
  readonly enabled: boolean;
  readonly allow_from: readonly string[];
  readonly token_env: string;
  readonly configured: boolean;
}

export interface FaceChannelUpdateInput {
  readonly enabled?: boolean;
  readonly allow_from?: string[];
  readonly token_env?: string;
}

export type FaceTokenUsagePeriod = "1d" | "3d" | "1w" | "1m" | "6m" | "1y";

export interface FaceTokenUsageTotal {
  readonly total_input: number;
  readonly total_output: number;
  readonly total_tokens: number;
  readonly total_reasoning: number;
  readonly total_cached: number;
  readonly request_count: number;
  readonly total_cost_usd: number;
  readonly cost_known: boolean;
}

export interface FaceTokenModelShare {
  readonly model: string;
  readonly percentage: number;
  readonly total_tokens: number;
  readonly cost_usd: number;
  readonly cost_known: boolean;
}

export interface FaceTokenProviderGroup {
  readonly key: string;
  readonly total_tokens: number;
  readonly request_count: number;
}

export interface FaceTokenTimelinePoint {
  readonly time_bucket: string;
  readonly label: string;
  readonly total_input: number;
  readonly total_output: number;
  readonly total_tokens: number;
}

export interface FaceTokenSessionUsage {
  readonly id: string;
  readonly title: string;
  readonly model: string;
  readonly request_count: number;
  readonly total_input: number;
  readonly total_output: number;
  readonly total_tokens: number;
  readonly cost_usd: number;
  readonly cost_known: boolean;
}

export interface FaceTokenUsageSnapshot {
  readonly period: FaceTokenUsagePeriod;
  readonly scope: "chat_runs";
  readonly total: FaceTokenUsageTotal;
  readonly models: readonly FaceTokenModelShare[];
  readonly providers: readonly FaceTokenProviderGroup[];
  readonly timeline: readonly FaceTokenTimelinePoint[];
  readonly sessions: readonly FaceTokenSessionUsage[];
}

export interface FaceTokenUsageParams {
  readonly period: FaceTokenUsagePeriod;
  readonly tz_offset_minutes: number;
  readonly session_limit?: number;
}

export interface FaceSkillSummary {
  readonly name: string;
  readonly description: string;
  readonly context?: string;
  readonly agent?: string;
  readonly model?: string;
  readonly origin?: "user" | "project";
  readonly enabled: boolean;
  readonly hash: string;
  readonly warnings: readonly string[];
}

export interface FaceSkillView extends FaceSkillSummary {
  readonly content: string;
  readonly relative_path: string;
  readonly supporting_files: readonly string[];
}

export interface FaceMarketplaceSkill {
  readonly id: string;
  readonly name: string;
  readonly source: string;
  readonly installs: number;
}

export interface FaceMarketplaceFeatured {
  readonly generated_at: string;
  readonly source: string;
  readonly metric: string;
  readonly skills: readonly FaceMarketplaceSkill[];
}

export interface FaceMarketplaceInstallResult {
  readonly skill: FaceSkillView;
  readonly outcome: "created" | "upgraded" | "up_to_date";
  readonly skipped_files?: readonly string[];
  readonly warnings?: readonly string[];
}

export interface FaceMarketplaceUpdateCheck {
  readonly name: string;
  readonly status: "not_installed" | "unmanaged" | "up_to_date" | "upgrade_available";
  readonly marketplace_id?: string;
  readonly snapshot_hash?: string;
}

export type FaceScheduleKind = "at" | "every" | "cron";
export type FaceCronStatus = "running" | "scheduled" | "paused" | "completed" | "failed";

export interface FaceCronSchedule {
  readonly kind: FaceScheduleKind;
  readonly atMs?: number;
  readonly everyMs?: number;
  readonly expr?: string;
  readonly tz?: string | null;
}

export interface FaceCronPayload {
  readonly kind: string;
  readonly message: string;
  readonly deliver: boolean;
  readonly channel?: string | null;
  readonly to?: string | null;
}

export interface FaceCronRunSnapshot {
  readonly run_id: string;
  readonly job_id: string;
  readonly startedAtMs: number;
  readonly lastHeartbeatAtMs: number;
  readonly trigger: "scheduled" | "manual";
  readonly cancelable: boolean;
}

export interface FaceCronJob {
  readonly id: string;
  readonly name: string;
  readonly enabled: boolean;
  readonly schedule: FaceCronSchedule;
  readonly payload: FaceCronPayload;
  readonly sessionId?: string | null;
  readonly state: {
    readonly nextRunAtMs?: number | null;
    readonly lastRunAtMs?: number | null;
    readonly lastStatus?: string | null;
    readonly lastError?: string | null;
  };
  readonly createdAtMs: number;
  readonly updatedAtMs: number;
  readonly deleteAfterRun: boolean;
  readonly isRunning: boolean;
  readonly activeRun?: FaceCronRunSnapshot | null;
  readonly computedStatus: FaceCronStatus;
}

export interface FaceCronJobInput {
  readonly name: string;
  readonly enabled: boolean;
  readonly schedule: FaceCronSchedule;
  readonly payload: FaceCronPayload;
  readonly delete_after_run?: boolean;
}

export interface FaceTrajectoryTokens {
  readonly input?: number;
  readonly output?: number;
  readonly think?: number;
  readonly cache_read?: number;
  readonly cache_write?: number;
}

export interface FaceTrajectoryRecord {
  readonly index: number;
  readonly id: string;
  readonly turn: number | null;
  readonly group: string;
  readonly kind: string;
  readonly text: string;
  readonly time_seconds?: number | null;
  readonly started_at?: number | null;
  readonly tokens?: FaceTrajectoryTokens;
  readonly result?: string;
  readonly is_error?: boolean;
  readonly input_detail?: string;
  readonly output_detail?: string;
  readonly call_id?: string;
  readonly provider?: string;
  readonly model?: string;
  readonly opens_turn?: boolean;
}

export interface FaceTrajectoryRequest {
  readonly number: number;
  readonly turn: number | null;
  readonly group: string;
  readonly status: string;
  readonly started_at: number;
  readonly completed_at: number;
  readonly provider?: string;
  readonly model?: string;
  readonly usage: FaceTrajectoryTokens;
  readonly retry?: number;
  readonly messages?: number;
  readonly preamble_bytes?: number;
  readonly error?: string;
}

export interface FaceTrajectorySession {
  readonly session_id: string;
  readonly turns: number;
  readonly records: readonly FaceTrajectoryRecord[];
  readonly requests: readonly FaceTrajectoryRequest[];
}

export interface FaceGenerationInput {
  readonly id?: string;
  readonly parent_id?: string;
  readonly artifact_sha256: string;
  readonly source_ref?: string;
  readonly recipe: {
    readonly loop?: string;
    readonly world?: string;
    readonly providers?: string[];
    readonly tools?: string[];
    readonly plugins?: string[];
  };
}

export interface FaceEvalRecordInput {
  readonly candidate_id: string;
  readonly baseline_id?: string;
  readonly suite: string;
  readonly verdict: FaceEvalVerdict;
  readonly journal_ref?: string;
}

export interface FaceEvalStartInput {
  readonly candidate_id: string;
  readonly baseline_id?: string;
  readonly suite: string;
}

export interface FacePromotionInput {
  readonly from_id: string;
  readonly to_id: string;
  readonly eval_id?: string;
  readonly actor?: string;
}

export interface FaceActionResult {
  readonly ok: boolean;
}

export interface FaceWorkspaceFileList {
  readonly files: readonly FaceWorkspaceFile[];
  readonly truncated: boolean;
}

export interface FaceBackgroundRecoveryResult {
  readonly recovered: boolean;
}

export interface FaceChildList {
  readonly children: readonly FaceChildRun[];
}

export interface FaceApprovalList {
  readonly approvals: readonly FaceApproval[];
}

export interface FaceQuestionList {
  readonly questions: readonly FaceQuestion[];
}

export interface FaceReviewList {
  readonly reviews: readonly FaceReviewItem[];
}

export interface FaceGenerationList {
  readonly generations: readonly FaceGeneration[];
}

export interface FaceEvalList {
  readonly evals: readonly FaceEvalRun[];
}

export interface FacePromotionList {
  readonly promotions: readonly FacePromotion[];
}

export interface FaceProviderDeleteResult {
  readonly deleted: boolean;
  readonly id: string;
}

export interface FaceMcpDeleteResult {
  readonly deleted: boolean;
  readonly name: string;
}

export interface FaceChannelList extends ReadonlyArray<FaceChannelStatus> {}

export interface FaceCronJobList {
  readonly jobs: readonly FaceCronJob[];
}

export interface FaceCronJobResult {
  readonly job: FaceCronJob;
}

export interface FaceSkillList {
  readonly skills: readonly FaceSkillSummary[];
}

export interface FaceMarketplaceSkillList {
  readonly skills: readonly FaceMarketplaceSkill[];
}

export interface FaceRunEvent {
  readonly run_id: string;
  readonly seq: number;
  readonly type: string;
  readonly created_at: number;
  readonly payload_version: number;
  readonly payload: Readonly<Record<string, unknown>>;
}

/** Structural client surface aligned with the current Web Face API module. */
export interface FaceClientAPI {
  request<T = unknown>(method: string, params?: unknown): Promise<T>;
  settingsUpdateFrom(settings: FaceSettings | null, patch?: Partial<FaceSettingsUpdate>): FaceSettingsUpdate;
  initialize(): Promise<FaceRPCCapabilities>;
  listWorkspaceFiles(runId: string): Promise<FaceWorkspaceFileList>;
  readWorkspaceFile(runId: string, path: string): Promise<FaceWorkspaceFileContent>;
  listSessions(): Promise<FaceSessionList>;
  getSession(id: string): Promise<FaceSessionDetail>;
  createSession(title: string): Promise<FaceSession>;
  renameSession(id: string, title: string): Promise<FaceSession>;
  setSessionPermission(id: string, preset: Exclude<FacePermissionPreset, "custom">): Promise<FaceSession>;
  deleteSession(id: string): Promise<void>;
  listMessages(sessionId: string): Promise<FaceMessageList>;
  getSessionContext(sessionId: string): Promise<FaceSessionContext>;
  compactSession(sessionId: string): Promise<FaceCompactResult>;
  rewindSession(sessionId: string, messageId: string): Promise<FaceRewindResult>;
  forkSession(sessionId: string, messageId: string, title?: string): Promise<FaceForkResult>;
  editSession(
    sessionId: string,
    messageId: string,
    text: string,
    mode?: FaceRunMode,
    face?: FaceName,
    thinking?: FaceThinkingMode,
  ): Promise<FaceRunStartResult>;
  listSessionCompactions(sessionId: string, limit?: number): Promise<{ readonly compactions: readonly FaceSessionCompactionRecord[] }>;
  listTodos(sessionId: string): Promise<{ readonly todos: readonly FaceTodo[] }>;
  updateTodo(sessionId: string, id: string, status: FaceTodoStatus): Promise<{ readonly todo: FaceTodo }>;
  startTurn(
    sessionId: string,
    text: string,
    mode?: FaceRunMode,
    face?: FaceName,
    attachments?: FaceAttachmentInput[],
    thinking?: FaceThinkingMode,
  ): Promise<FaceRunStartResult>;
  interruptRun(runId: string): Promise<FaceRunInterruptResult>;
  cancelRun(runId: string): Promise<FaceRunInterruptResult>;
  getRun(runId: string): Promise<FaceRun>;
  getRunLog(runId: string, afterSeq?: number): Promise<FaceRunLog>;
  recoverBackgroundRuns(): Promise<FaceBackgroundRecoveryResult>;
  listBackgroundRuns(): Promise<{ readonly runs: readonly FaceBackgroundRun[] }>;
  attachBackgroundRun(runId: string): Promise<FaceBackgroundRun>;
  startChild(params: FaceChildStartInput): Promise<FaceChildRun>;
  getChild(runId: string): Promise<FaceChildRun>;
  listChildren(parentRunId: string, tree?: boolean): Promise<FaceChildList>;
  waitChild(runId: string): Promise<FaceChildRun>;
  cancelChild(runId: string): Promise<FaceChildRun>;
  listApprovals(): Promise<FaceApprovalList>;
  respondApproval(approvalId: string, decision: "approved" | "denied", reason?: string): Promise<FaceApprovalResponse>;
  listQuestions(): Promise<FaceQuestionList>;
  respondQuestion(questionId: string, answer: string): Promise<FaceQuestionResponse>;
  listReviews(params?: FaceReviewListInput): Promise<FaceReviewList>;
  getReview(reviewId: string): Promise<FaceReviewItem>;
  respondReview(reviewId: string, response: FaceReviewResponse): Promise<{ readonly review_id: string; readonly status: string }>;
  inspectSpecies(): Promise<FaceSpeciesInspect>;
  listGenerations(): Promise<FaceGenerationList>;
  getGeneration(id: string): Promise<FaceGeneration>;
  createGeneration(params: FaceGenerationInput): Promise<FaceGeneration>;
  rejectGeneration(id: string): Promise<FaceGeneration>;
  listEvals(): Promise<FaceEvalList>;
  recordEval(params: FaceEvalRecordInput): Promise<FaceEvalRun>;
  startEval(params: FaceEvalStartInput): Promise<FaceEvalRun>;
  listPromotions(): Promise<FacePromotionList>;
  promoteGeneration(params: FacePromotionInput): Promise<FacePromotion>;
  getSettings(): Promise<FaceSettings>;
  updateLocale(locale: FaceLocale): Promise<FaceLocaleSettings>;
  listTools(): Promise<FaceToolsCatalogView>;
  setActiveTools(tools: string[]): Promise<FaceToolsCatalogView>;
  updateSettings(params: FaceSettingsUpdate): Promise<FaceSettings>;
  listProviders(): Promise<FaceProvidersView>;
  upsertProvider(input: FaceProviderEntryInput): Promise<FaceProviderEntry>;
  deleteProvider(id: string): Promise<FaceProviderDeleteResult>;
  refreshProviderModels(input: FaceProviderRefreshInput): Promise<FaceProviderEntry>;
  listMcpServers(): Promise<FaceMcpServersView>;
  upsertMcpServer(input: FaceMcpServerInput): Promise<FaceMcpServer>;
  deleteMcpServer(name: string): Promise<FaceMcpDeleteResult>;
  probeMcpServer(name: string): Promise<FaceMcpServer>;
  inspectChannels(): Promise<readonly FaceChannelStatus[]>;
  getChannel(name: string): Promise<FaceChannelEnvelope>;
  updateChannel(name: string, patch: FaceChannelUpdateInput): Promise<FaceChannelEnvelope>;
  getTokenUsage(params: FaceTokenUsageParams): Promise<FaceTokenUsageSnapshot>;
  listSkills(): Promise<FaceSkillList>;
  getSkill(name: string, path?: string): Promise<FaceSkillView>;
  setSkillEnabled(name: string, enabled: boolean, base_hash: string): Promise<FaceSkillSummary>;
  searchMarketplaceSkills(q: string, limit?: number): Promise<FaceMarketplaceSkillList>;
  featuredMarketplaceSkills(): Promise<FaceMarketplaceFeatured>;
  installMarketplaceSkill(id: string, mode?: "create" | "upgrade"): Promise<FaceMarketplaceInstallResult>;
  checkMarketplaceUpdate(name: string): Promise<FaceMarketplaceUpdateCheck>;
  listCronJobs(): Promise<FaceCronJobList>;
  createCronJob(input: FaceCronJobInput): Promise<FaceCronJobResult>;
  updateCronJob(id: string, input: FaceCronJobInput): Promise<FaceCronJobResult>;
  deleteCronJob(id: string): Promise<{ readonly deleted: boolean }>;
  triggerCronJob(id: string): Promise<FaceCronJobResult>;
  stopCronJob(id: string): Promise<{ readonly stopped: boolean }>;
  fetchSessionTrajectory(sessionId: string, limit?: number): Promise<FaceTrajectorySession>;
}

export type FaceAPI = FaceClientAPI;

/** Capabilities negotiated by the existing Face JSON-RPC client. */
export interface FaceRPCCapabilities {
  readonly protocol_version: string;
  readonly capabilities: readonly string[];
}
export type FaceRpcCapabilities = FaceRPCCapabilities;

/** The authenticated JSON-RPC client already owned by the Web Face. */
export interface FaceClientRPC {
  readonly capabilities: FaceRPCCapabilities;
  call<T = unknown>(method: string, params?: unknown): Promise<T>;
  onNotification(method: string, listener: (params: unknown) => void): () => void;
  onClose(listener: () => void): () => void;
  close(): void;
}

export type FaceRPC = FaceClientRPC;
export type FaceClientRpc = FaceClientRPC;

export type FaceConnectionState = "idle" | "connecting" | "connected" | "reconnecting" | "error";

export type FacePhase = "idle" | "loading" | "refreshing" | "ready" | "empty" | "error" | "processing";

export interface FaceQueuedMessage {
  readonly id: string;
  readonly text: string;
  readonly mode: FaceRunMode;
  readonly face?: FaceName;
  readonly attachments?: FaceAttachmentInput[];
  readonly thinking?: FaceThinkingMode;
}

/** Complete current Zustand-backed Face state exposed to UI Modules. */
export interface FaceStoreState {
  readonly initialized: boolean;
  readonly initializationError: string | null;
  readonly capabilities: string[];
  readonly connection: FaceConnectionState;
  readonly sessions: FaceSession[];
  readonly sessionsPhase: FacePhase;
  readonly sessionsError: string | null;
  readonly sessionBusyId: string | null;
  readonly activeSessionId: string | null;
  readonly messages: FaceMessage[];
  readonly messagesPhase: FacePhase;
  readonly messagesError: string | null;
  readonly sessionContext: FaceSessionContext | null;
  readonly todos: FaceTodo[];
  readonly todosPhase: FacePhase;
  readonly todosError: string | null;
  readonly todoPanelOpen: boolean;
  readonly currentRun: FaceRun | null;
  readonly runEvents: FaceRunEvent[];
  readonly streamingText: string;
  readonly streamingReasoning: string;
  readonly runError: string | null;
  readonly runBusy: boolean;
  readonly queuedMessages: FaceQueuedMessage[];
  readonly backgroundRuns: FaceBackgroundRun[];
  readonly backgroundPhase: FacePhase;
  readonly backgroundError: string | null;
  readonly backgroundBusyId: string | null;
  readonly children: FaceChildRun[];
  readonly childrenPhase: FacePhase;
  readonly childrenError: string | null;
  readonly childBusyId: string | null;
  readonly selectedChild: FaceChildRun | null;
  readonly reviews: FaceReviewItem[];
  readonly reviewsPhase: FacePhase;
  readonly reviewsError: string | null;
  readonly reviewBusyIds: string[];
  readonly reviewCenterOpen: boolean;
  readonly filesPanelOpen: boolean;
  readonly sessionDrawerOpen: boolean;
  readonly settings: FaceSettings | null;
  readonly settingsPhase: FacePhase;
  readonly settingsError: string | null;
  readonly providers: FaceProviderEntry[];
  readonly providersPhase: FacePhase;
  readonly providersError: string | null;
  readonly species: FaceSpeciesInspect | null;
  readonly generations: FaceGeneration[];
  readonly evals: FaceEvalRun[];
  readonly promotions: FacePromotion[];
  readonly lifecyclePhase: FacePhase;
  readonly lifecycleError: string | null;
  readonly lifecycleBusy: boolean;
  initialize(): Promise<void>;
  retryInitialize(): Promise<void>;
  loadSessions(): Promise<void>;
  createSession(title?: string): Promise<FaceSession>;
  renameSession(id: string, title: string): Promise<void>;
  setSessionPermission(id: string, preset: Exclude<FacePermissionPreset, "custom">): Promise<void>;
  deleteSession(id: string): Promise<void>;
  selectSession(id: string): Promise<void>;
  startRun(sessionId: string, text: string, mode?: FaceRunMode, face?: FaceName, attachments?: FaceAttachmentInput[], thinking?: FaceThinkingMode): Promise<void>;
  editSession(sessionId: string, messageId: string, text: string, mode?: FaceRunMode, face?: FaceName, thinking?: FaceThinkingMode): Promise<void>;
  enqueueMessage(text: string, mode?: FaceRunMode, face?: FaceName, attachments?: FaceAttachmentInput[], thinking?: FaceThinkingMode): void;
  removeQueuedMessage(id: string): void;
  clearQueue(): void;
  cancelCurrentRun(): Promise<void>;
  openRun(runId: string, sessionId: string): Promise<void>;
  loadBackgroundRuns(): Promise<void>;
  attachBackgroundRun(runId: string): Promise<void>;
  loadChildren(parentRunId?: string): Promise<void>;
  startChild(text: string, policyProfile?: string, toolNames?: string[]): Promise<void>;
  openChild(runId: string): Promise<void>;
  waitChild(runId: string): Promise<void>;
  cancelChild(runId: string): Promise<void>;
  loadReviews(): Promise<void>;
  respondReview(id: string, response: FaceReviewResponse): Promise<void>;
  setReviewCenterOpen(open: boolean): void;
  setFilesPanelOpen(open: boolean): void;
  setSessionDrawerOpen(open: boolean): void;
  loadTodos(sessionId?: string): Promise<void>;
  updateTodoStatus(todoId: string, status: FaceTodoStatus): Promise<void>;
  setTodoPanelOpen(open: boolean): void;
  loadSettings(): Promise<void>;
  saveSettings(value: FaceSettingsUpdate): Promise<void>;
  saveLocale(locale: FaceLocale): Promise<void>;
  loadSessionContext(sessionId?: string): Promise<void>;
  compactSession(sessionId: string): Promise<FaceCompactResult>;
  rewindSession(sessionId: string, messageId: string): Promise<FaceMessage[]>;
  forkSession(sessionId: string, messageId: string, title?: string): Promise<string>;
  loadProviders(): Promise<void>;
  saveProvider(input: FaceProviderEntryInput): Promise<void>;
  removeProvider(id: string): Promise<void>;
  refreshProvider(input: FaceProviderRefreshInput): Promise<FaceProviderEntry>;
  loadLifecycle(): Promise<void>;
  createGeneration(params: FaceGenerationInput): Promise<void>;
  rejectGeneration(id: string): Promise<void>;
  startEval(params: FaceEvalStartInput): Promise<void>;
  recordEval(params: FaceEvalRecordInput): Promise<void>;
  promoteGeneration(params: FacePromotionInput): Promise<void>;
}

/** Read/write observation surface for the current Zustand-backed Face store. */
export interface FaceClientStore<State extends object = FaceStoreState> {
  getState(): State;
  getInitialState(): State;
  setState(
    partial: State | Partial<State> | ((state: State) => State | Partial<State>),
    replace?: false,
  ): void;
  setState(state: State | ((state: State) => State), replace: true): void;
  subscribe(listener: (state: State, previousState: State) => void): () => void;
}

export type FaceStore<State extends object = FaceStoreState> = FaceClientStore<State>;

/** Common navigation options accepted by the current TanStack Face router. */
export type FaceNavigationScalar = string | number | boolean;
export type FaceNavigationParams = Readonly<Record<string, FaceNavigationScalar>>;
export type FaceNavigationSearch = Readonly<Record<string, FaceNavigationScalar | readonly FaceNavigationScalar[]>>;
export type FaceNavigationUpdater<T> = T | ((previous?: T) => T);

export interface FaceNavigationParsedHistoryState {
  readonly __TSR_index: number;
  readonly key?: string;
  readonly __TSR_key?: string;
}

export type FaceNavigationState = object;
export type FaceNavigationStateUpdater =
  | true
  | FaceNavigationState
  | ((previous: FaceNavigationParsedHistoryState) => FaceNavigationState);
export type FaceNavigationHash = true | FaceNavigationUpdater<string>;
export type FaceNavigationParamsOption =
  | true
  | FaceNavigationParams
  | ((current: FaceNavigationParams) => FaceNavigationParams);
export type FaceNavigationSearchOption =
  | true
  | FaceNavigationSearch
  | ((current: FaceNavigationSearch) => FaceNavigationSearch);

export interface FaceNavigationLocation {
  readonly href: string;
  readonly pathname: string;
  readonly search: FaceNavigationSearch;
  readonly searchStr: string;
  readonly state: FaceNavigationParsedHistoryState;
  readonly hash: string;
  readonly maskedLocation?: FaceNavigationLocation;
  readonly unmaskOnReload?: boolean;
  readonly publicHref: string;
  readonly external: boolean;
}

export interface FaceNavigationLocationChangeInfo {
  readonly fromLocation?: FaceNavigationLocation;
  readonly toLocation: FaceNavigationLocation;
  readonly pathChanged: boolean;
  readonly hrefChanged: boolean;
  readonly hashChanged: boolean;
}

export interface FaceViewTransitionOptions {
  readonly types: readonly string[] | ((locationChangeInfo: FaceNavigationLocationChangeInfo) => readonly string[] | false);
}

export interface FaceNavigationMaskOptions {
  readonly to?: string;
  readonly from?: string;
  readonly params?: FaceNavigationParamsOption;
  readonly search?: FaceNavigationSearchOption;
  readonly hash?: FaceNavigationHash;
  readonly state?: FaceNavigationStateUpdater;
  readonly unsafeRelative?: "path";
  readonly unmaskOnReload?: boolean;
}

export interface FaceNavigationOptions {
  readonly to?: string;
  readonly from?: string;
  readonly params?: FaceNavigationParamsOption;
  readonly search?: FaceNavigationSearchOption;
  readonly hash?: FaceNavigationHash;
  readonly state?: FaceNavigationStateUpdater;
  readonly unsafeRelative?: "path";
  readonly _fromLocation?: FaceNavigationLocation;
  readonly mask?: FaceNavigationMaskOptions;
  readonly hashScrollIntoView?: boolean | ScrollIntoViewOptions;
  readonly replace?: boolean;
  readonly resetScroll?: boolean;
  readonly startTransition?: boolean;
  readonly viewTransition?: boolean | FaceViewTransitionOptions;
  readonly ignoreBlocker?: boolean;
  readonly reloadDocument?: boolean;
  readonly href?: string;
}

/** Router surface intentionally leaves concrete TanStack route types to host. */
export interface FaceClientRouter {
  navigate(options: FaceNavigationOptions): Promise<void>;
  invalidate(): Promise<void>;
}

export type FaceRouter = FaceClientRouter;

export interface FaceClient {
  readonly api: FaceClientAPI;
  readonly rpc: FaceClientRPC;
  readonly store: FaceClientStore<FaceStoreState>;
  readonly router: FaceClientRouter;
}

/** Complete host passed to both root renderers and extension installers. */
export interface FullUIHost {
  readonly face?: FaceClient;
  readonly api: FaceClientAPI;
  readonly rpc: FaceClientRPC;
  readonly store: FaceClientStore<FaceStoreState>;
  readonly router: FaceClientRouter;
  readonly composition: UICompositionHost;
  readonly t: UITranslator;
  readonly registerCleanup: (cleanup: UICleanup) => CleanupHandle;
}
