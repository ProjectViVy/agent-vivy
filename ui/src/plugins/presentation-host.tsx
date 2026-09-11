import {
  Component,
  Fragment,
  createElement,
  isValidElement,
  useMemo,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type ComponentType,
  type ErrorInfo,
  type MouseEvent,
  type PropsWithChildren,
  type ReactNode,
} from 'react';
import {
  cleanupHandleFromInstall,
  createCleanupHandle,
  type CleanupHandle,
  type FaceClient,
  type FaceClientAPI,
  type FaceClientRPC,
  type FaceClientRouter,
  type FaceClientStore,
  type FaceNavigationOptions,
  type FaceStoreState,
  type FullUIHost,
  type UICompositionHost,
  type UIExtension,
  type UIMessageForm,
  type UIRegistrationHandle,
  type UIRegistry,
  type UIRoot,
  type UITranslationArgs,
  type UITranslator,
} from '@vivy/ui-sdk';
import { generatedUIExtensions, generatedUIRoot, UI_ASSEMBLY_MANIFEST } from '@vivy/generated-assembly';
import * as faceAPI from '@/lib/api';
import * as rpcTransport from '@/lib/rpc';
import { useVivyStore } from '@/lib/store';
import { getLocale, t as translate } from '@/i18n';

/** Build metadata shown in diagnostics; it never controls whether a Module runs. */
export interface PresentationProvenance {
  readonly generationId?: string;
  readonly artifactSha256?: string;
  readonly uiArtifactSha256?: string;
  readonly rootId?: string;
  readonly extensionIds?: readonly string[];
  readonly root?: string;
  readonly extensions?: readonly string[];
  readonly sourceHashes?: Readonly<Record<string, string>>;
  readonly dependencyLockHashes?: Readonly<Record<string, string>>;
  readonly sdkPackage?: string;
  readonly sdkVersion?: string;
  readonly assetHashes?: Readonly<Record<string, string>>;
  readonly [key: string]: unknown;
}

/** The strict, compiler-owned projection consumed by the Web Host resolver. */
export interface PluginCatalogUnitProjection {
  readonly description?: string;
  readonly placeholders?: readonly string[];
  readonly messages: Readonly<Record<string, string>>;
  readonly short?: Readonly<Record<string, string>>;
  readonly long?: Readonly<Record<string, string>>;
}

export interface PluginCatalogProjectionEntry {
  readonly apiVersion?: string;
  readonly schemaVersion?: string;
  readonly path?: string;
  readonly defaultLocale?: string;
  readonly locales?: readonly string[];
  readonly module?: string;
  readonly moduleId?: string;
  readonly digest?: string;
  readonly completeness?: Readonly<Record<string, string>>;
  readonly evidence?: readonly string[];
  readonly units: Readonly<Record<string, PluginCatalogUnitProjection>>;
}

export type PluginCatalogProjection = Readonly<Record<string, PluginCatalogProjectionEntry>>;
export type PluginCatalogInput = PluginCatalogProjection | readonly PluginCatalogProjectionEntry[];

export interface PresentationHostProps {
  /** The real Web Face Host. Tests and alternate Faces can inject their own. */
  readonly host: FullUIHost;
  /** The generated exclusive root; omitted means the established route tree. */
  readonly root?: UIRoot;
  /** The generated extension list, already ordered by the Recipe compiler. */
  readonly extensions?: readonly UIExtension[];
  /** Sealed Generation metadata used only for diagnostics. */
  readonly provenance?: PresentationProvenance;
  /** Existing route-tree content used when no replacement root is selected. */
  readonly children?: ReactNode;
  /** Current TanStack path, supplied by the host route tree when available. */
  readonly path?: string;
}

type PresentationPhase =
  | { readonly status: 'pending' }
  | { readonly status: 'ready'; readonly node: ReactNode }
  | { readonly status: 'error'; readonly diagnostic: PresentationDiagnostic };

export interface PresentationDiagnostic {
  readonly stage: 'host' | 'extension-install' | 'root-render' | 'root-component';
  readonly message: string;
  readonly moduleId?: string;
  readonly cleanupErrors: readonly string[];
  readonly cause: unknown;
}

interface PresentationControllerOptions {
  readonly host: FullUIHost;
  readonly root?: UIRoot;
  readonly extensions: readonly UIExtension[];
  readonly provenance: PresentationProvenance;
}

interface StartResult {
  readonly node?: ReactNode;
  readonly diagnostic?: PresentationDiagnostic;
}

type RegistryKind = Exclude<keyof UICompositionHost, 'registerCleanup'>;
const REGISTRY_KINDS: readonly RegistryKind[] = [
  'routes', 'navigation', 'pages', 'components', 'styles', 'themes', 'shortcuts', 'commands',
];

const LIVE_COMPOSITION = Symbol('vivy.presentation.live-composition');
const DEFAULT_COMPOSITION_OWNER = Symbol('vivy.presentation.default-owner');
const COMMAND_EVENT = 'vivy:command';

interface LiveCompositionHost extends UICompositionHost {
  readonly [LIVE_COMPOSITION]: LiveCompositionRuntime;
}

interface CompositionEntry {
  readonly kind: RegistryKind;
  readonly id: string;
  readonly token: symbol;
  readonly owner: symbol;
  readonly ownerId?: string;
  readonly value: unknown;
  active: boolean;
  effectCleanup?: () => void;
}

interface ThemeProjection {
  readonly variables: Readonly<Record<string, string>>;
  readonly classes: readonly string[];
  readonly attributes: Readonly<Record<string, string | null>>;
  readonly css?: string;
}

interface NavigationProjection {
  readonly target?: string;
  readonly label?: ReactNode;
  readonly render?: unknown;
  readonly component?: ComponentType<Record<string, unknown>>;
  readonly onClick?: (event: MouseEvent<HTMLElement>) => unknown;
}

interface RouteProjection {
  readonly path: string;
  readonly render: unknown;
}

interface ShortcutProjection {
  readonly shortcut: string;
  readonly handler?: (event: KeyboardEvent) => unknown;
  readonly command?: string;
}

/** Runtime composition owned by one PresentationHost mount. */
class LiveCompositionRuntime {
  private readonly entries = new Map<RegistryKind, Map<string, CompositionEntry[]>>();
  private readonly registrationOrder: CompositionEntry[] = [];
  private readonly listeners = new Set<() => void>();
  private readonly themeVariableBaseline = new Map<string, string | undefined>();
  private readonly themeClassBaseline = new Map<string, boolean>();
  private readonly themeAttributeBaseline = new Map<string, string | null>();
  private readonly revokedOwners = new Set<symbol>();
  private currentPath = browserPath();
  private version = 0;
  private attached = false;
  private acceptingRegistrations = true;
  private disposed = false;
  private errorHandler: ((cause: unknown, moduleId?: string) => void) | undefined;

  constructor() {
    for (const kind of REGISTRY_KINDS) this.entries.set(kind, new Map());
  }

  setErrorHandler(handler: (cause: unknown, moduleId?: string) => void): void { this.errorHandler = handler; }

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  getSnapshot = (): number => this.version;

  getCurrentPath(): string { return this.currentPath; }
  reportError(cause: unknown, moduleId?: string): void { this.errorHandler?.(cause, moduleId); }
  revokeOwner(owner: symbol): void { this.revokedOwners.add(owner); }

  setPath(path: string): void {
    if (!this.acceptingRegistrations) return;
    const next = normalizePath(path);
    if (next === this.currentPath) return;
    this.currentPath = next;
    this.bump();
  }

  attach(): void {
    if (this.attached || !this.acceptingRegistrations) return;
    this.attached = true;
    if (typeof window === 'undefined') return;
    window.addEventListener('popstate', this.handleBrowserLocation);
    window.addEventListener('hashchange', this.handleBrowserLocation);
    window.addEventListener(COMMAND_EVENT, this.handleCommandEvent as EventListener);
    window.addEventListener('keydown', this.handleKeyDown);
  }

  detach(): void {
    if (!this.attached) return;
    this.attached = false;
    if (typeof window === 'undefined') return;
    window.removeEventListener('popstate', this.handleBrowserLocation);
    window.removeEventListener('hashchange', this.handleBrowserLocation);
    window.removeEventListener(COMMAND_EVENT, this.handleCommandEvent as EventListener);
    window.removeEventListener('keydown', this.handleKeyDown);
  }

  /** Stop accepting new work while already-owned cleanup callbacks unwind. */
  beginDispose(): void {
    if (!this.acceptingRegistrations) return;
    this.acceptingRegistrations = false;
    this.detach();
  }

  dispose(): void {
    if (this.disposed) return;
    this.beginDispose();
    const failures: unknown[] = [];
    for (const entry of this.registrationOrder.slice().reverse()) {
      if (!entry.active) continue;
      try { this.removeEntry(entry); }
      catch (cause) { failures.push(ownedError(entry.ownerId, cause)); }
    }
    this.registrationOrder.length = 0;
    this.listeners.clear();
    this.disposed = true;
    if (failures.length === 1) throw failures[0];
    if (failures.length > 1) throw new AggregateError(failures, `UI composition cleanup failed: ${failures.map(errorMessage).join('; ')}`);
  }

  register(kind: RegistryKind, id: string, value: unknown, owner = DEFAULT_COMPOSITION_OWNER, ownerId?: string): UIRegistrationHandle {
    if (!this.acceptingRegistrations || this.revokedOwners.has(owner)) throw new Error(`UI composition is disposed; late ${kind} registration "${id}" was rejected`);
    const entry: CompositionEntry = { kind, id, token: Symbol(id), owner, ownerId, value, active: true };
    const byID = this.entries.get(kind)!;
    const stack = byID.get(id) ?? [];
    stack.push(entry);
    byID.set(id, stack);
    this.registrationOrder.push(entry);
    try {
      entry.effectCleanup = this.applyEntry(entry);
    } catch (cause) {
      try { this.removeEntry(entry); }
      catch (cleanupCause) { throw new AggregateError([cause, cleanupCause], 'UI composition registration failed and rollback failed'); }
      throw cause;
    }
    this.bump();
    return {
      id,
      get active() { return entry.active; },
      dispose: () => this.removeEntry(entry),
    };
  }

  unregister(kind: RegistryKind, id: string, owner = DEFAULT_COMPOSITION_OWNER): void {
    const entry = this.registrationOrder.slice().reverse().find((item) => item.active && item.kind === kind && item.id === id && item.owner === owner);
    if (entry) this.removeEntry(entry);
  }

  getEntries(kind: RegistryKind): readonly CompositionEntry[] {
    return this.registrationOrder.filter((entry) => entry.active && entry.kind === kind);
  }

  getActiveRoute(): CompositionEntry | undefined {
    const candidates = this.registrationOrder.filter((entry) => entry.active && (entry.kind === 'routes' || entry.kind === 'pages'))
      .map((entry) => ({ entry, projection: routeProjection(entry.id, entry.value) }))
      .filter((item): item is { entry: CompositionEntry; projection: RouteProjection } => Boolean(item.projection))
      .filter(({ projection }) => normalizePath(projection.path) === this.currentPath);
    return candidates.at(-1)?.entry;
  }

  async navigate(options: FaceNavigationOptions, delegate: FaceClientRouter): Promise<void> {
    // TanStack Router is the sole navigation authority. Module routes are a
    // presentation registry, so even a claimed path must pass through the
    // host router to update its real location, matches, history, blockers,
    // and loaders. The runtime observes that state via `path`/history events;
    // it never maintains a private browser-history shadow.
    await delegate.navigate(options);
  }

  async dispatchCommand(id: string, payload?: unknown): Promise<boolean> {
    const entry = this.getEntries('commands').slice().reverse().find((item) => item.id === id);
    if (!entry) return false;
    const handler = commandHandler(entry.value);
    if (!handler) return false;
    try {
      await handler(payload);
      return true;
    } catch (cause) {
      this.errorHandler?.(cause, entry.ownerId);
      return false;
    }
  }

  private applyEntry(entry: CompositionEntry): (() => void) | undefined {
    if (typeof document === 'undefined') return undefined;
    if (entry.kind === 'styles') return this.applyStyle(entry.id, entry.value);
    if (entry.kind === 'themes') return this.applyTheme(entry.value);
    return undefined;
  }

  private applyStyle(id: string, value: unknown): (() => void) | undefined {
    const css = cssText(value);
    if (!css) return undefined;
    const style = document.createElement('style');
    style.dataset.vivyUiStyle = id;
    style.textContent = css;
    document.head.append(style);
    return () => style.remove();
  }

  private applyTheme(value: unknown): (() => void) | undefined {
    const theme = themeProjection(value);
    if (!theme) return undefined;
    for (const name of Object.keys(theme.variables)) {
      if (!this.themeVariableBaseline.has(name)) this.themeVariableBaseline.set(name, document.documentElement.style.getPropertyValue(name) || undefined);
    }
    for (const name of theme.classes) {
      if (!this.themeClassBaseline.has(name)) this.themeClassBaseline.set(name, document.documentElement.classList.contains(name));
    }
    for (const name of Object.keys(theme.attributes)) {
      if (!this.themeAttributeBaseline.has(name)) this.themeAttributeBaseline.set(name, document.documentElement.getAttribute(name));
    }
    let styleCleanup: (() => void) | undefined;
    if (theme.css) {
      const style = document.createElement('style');
      style.dataset.vivyUiTheme = 'true';
      style.textContent = theme.css;
      document.head.append(style);
      styleCleanup = () => style.remove();
    }
    this.recomputeThemes();
    return () => {
      let failure: unknown;
      try { styleCleanup?.(); }
      catch (cause) { failure = cause; }
      finally { this.recomputeThemes(); }
      if (failure !== undefined) throw failure;
    };
  }

  private recomputeThemes(): void {
    if (typeof document === 'undefined') return;
    const root = document.documentElement;
    const themes = this.getEntries('themes').map((entry) => themeProjection(entry.value)).filter((item): item is ThemeProjection => Boolean(item));
    const variableNames = new Set([...this.themeVariableBaseline.keys(), ...themes.flatMap((theme) => Object.keys(theme.variables))]);
    for (const name of variableNames) {
      const winner = themes.slice().reverse().find((theme) => Object.prototype.hasOwnProperty.call(theme.variables, name));
      if (winner) root.style.setProperty(name, winner.variables[name]);
      else {
        const baseline = this.themeVariableBaseline.get(name);
        if (baseline === undefined) root.style.removeProperty(name);
        else root.style.setProperty(name, baseline);
        this.themeVariableBaseline.delete(name);
      }
    }
    const classNames = new Set([...this.themeClassBaseline.keys(), ...themes.flatMap((theme) => theme.classes)]);
    for (const name of classNames) {
      const claimed = themes.some((theme) => theme.classes.includes(name));
      if (claimed) root.classList.add(name);
      else {
        if (this.themeClassBaseline.get(name)) root.classList.add(name);
        else root.classList.remove(name);
        this.themeClassBaseline.delete(name);
      }
    }
    const attributes = new Set([...this.themeAttributeBaseline.keys(), ...themes.flatMap((theme) => Object.keys(theme.attributes))]);
    for (const name of attributes) {
      const winner = themes.slice().reverse().find((theme) => Object.prototype.hasOwnProperty.call(theme.attributes, name));
      if (winner) {
        const value = winner.attributes[name];
        if (value === null || value === undefined) root.removeAttribute(name);
        else root.setAttribute(name, value);
      } else {
        const baseline = this.themeAttributeBaseline.get(name);
        if (baseline === null || baseline === undefined) root.removeAttribute(name);
        else root.setAttribute(name, baseline);
        this.themeAttributeBaseline.delete(name);
      }
    }
  }

  private removeEntry(entry: CompositionEntry): void {
    if (!entry.active) return;
    entry.active = false;
    let failure: unknown;
    try { entry.effectCleanup?.(); }
    catch (cause) { failure = cause; }
    entry.effectCleanup = undefined;
    const byID = this.entries.get(entry.kind);
    const stack = byID?.get(entry.id);
    if (stack) {
      const remaining = stack.filter((candidate) => candidate.token !== entry.token);
      if (remaining.length) byID!.set(entry.id, remaining);
      else byID!.delete(entry.id);
    }
    const orderIndex = this.registrationOrder.indexOf(entry);
    if (orderIndex >= 0) this.registrationOrder.splice(orderIndex, 1);
    this.bump();
    if (failure !== undefined) throw failure;
  }

  private bump(): void {
    this.version += 1;
    for (const listener of this.listeners) listener();
  }

  private handleBrowserLocation = (): void => { this.setPath(browserPath()); };

  private handleCommandEvent = (event: CustomEvent<{ id?: string; command?: string; payload?: unknown }>): void => {
    const id = event.detail?.id ?? event.detail?.command;
    if (id) void this.dispatchCommand(id, event.detail?.payload);
  };

  private handleKeyDown = (event: KeyboardEvent): void => {
    for (const entry of this.getEntries('shortcuts').slice().reverse()) {
      const shortcut = shortcutProjection(entry.id, entry.value);
      if (!shortcut || !matchesShortcut(shortcut.shortcut, event)) continue;
      event.preventDefault();
      try {
        if (shortcut.command) void this.dispatchCommand(shortcut.command, event);
        else void Promise.resolve(shortcut.handler?.(event)).catch((cause) => this.reportError(cause, entry.ownerId));
      } catch (cause) {
        this.reportError(cause, entry.ownerId);
      }
      return;
    }
  };
}

/** Host controller for one generated UI tree. */
class PresentationController {
  private readonly cleanupHandles: Array<{ readonly handle: CleanupHandle; readonly ownerId?: string }> = [];
  private readonly host: FullUIHost;
  private readonly sourceHost: FullUIHost;
  private readonly root?: UIRoot;
  private readonly extensions: readonly UIExtension[];
  private readonly runtime: LiveCompositionRuntime;
  private readonly ownerTokens = new Set<symbol>();
  private provenance: PresentationProvenance;
  private readonly diagnosticListeners = new Set<(diagnostic: PresentationDiagnostic) => void>();
  private readonly handleRuntimeError = (cause: unknown, moduleId?: string): void => {
    if (!this.disposed && !this.diagnostic) this.fail('host', moduleId, cause);
  };
  private started = false;
  private disposed = false;
  private mounts = 0;
  private disposeQueued = false;
  private renderedNode: ReactNode | undefined;
  private diagnostic: PresentationDiagnostic | undefined;
  private readonly cleanupErrors: string[] = [];
  private cleanupReportedCount = 0;

  constructor(options: PresentationControllerOptions) {
    this.sourceHost = options.host;
    this.root = options.root;
    this.extensions = options.extensions;
    this.provenance = options.provenance;
    this.runtime = compositionRuntime(options.host.composition) ?? new LiveCompositionRuntime();
    this.host = this.createManagedHost(this.sourceHost, this.root?.id ?? 'root');
  }

  mount(): () => void {
    this.mounts += 1;
    this.runtime.setErrorHandler(this.handleRuntimeError);
    this.runtime.attach();
    let released = false;
    return () => {
      if (released) return;
      released = true;
      this.mounts = Math.max(0, this.mounts - 1);
      this.queueDispose();
    };
  }

  start(): StartResult {
    if (this.started) return this.diagnostic ? { diagnostic: this.diagnostic } : { node: this.renderedNode };
    this.started = true;
    if (this.disposed) {
      this.diagnostic = this.makeDiagnostic('host', 'PresentationHost was started after disposal', undefined);
      return { diagnostic: this.diagnostic };
    }
    for (const extension of this.extensions) {
      try {
        // Each Module receives its own registry facade. The shared runtime
        // remains live, while unregister(id) can only see that Module's
        // registrations and cannot remove a colliding sibling entry.
        const extensionHost = this.createManagedHost(this.sourceHost, extension.id);
        const cleanup = cleanupHandleFromInstall(extension.install(extensionHost));
        this.track(cleanup, extension.id);
      } catch (cause) {
        this.diagnostic = this.fail('extension-install', extension.id, cause);
        return { diagnostic: this.diagnostic };
      }
    }
    if (this.root) {
      try { this.renderedNode = this.root.render(this.host); }
      catch (cause) {
        this.diagnostic = this.fail('root-render', this.root.id, cause);
        return { diagnostic: this.diagnostic };
      }
    }
    return { node: this.renderedNode };
  }

  getNode(): ReactNode | undefined { return this.renderedNode; }
  getRuntime(): LiveCompositionRuntime { return this.runtime; }
  getRouter(): FaceClientRouter { return this.host.router; }
  getDiagnostic(): PresentationDiagnostic | undefined { return this.diagnostic; }
  setProvenance(provenance: PresentationProvenance): void { this.provenance = provenance; }
  onDiagnostic(listener: (diagnostic: PresentationDiagnostic) => void): () => void {
    this.diagnosticListeners.add(listener);
    return () => this.diagnosticListeners.delete(listener);
  }

  failRootComponent(cause: unknown): PresentationDiagnostic {
    return this.diagnostic ?? this.fail('root-component', this.root?.id, cause);
  }

  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    this.runtime.beginDispose();
    for (const owner of this.ownerTokens) this.runtime.revokeOwner(owner);
    this.ownerTokens.clear();
    for (let index = this.cleanupHandles.length - 1; index >= 0; index -= 1) {
      const { handle, ownerId } = this.cleanupHandles[index];
      try { handle.dispose(); }
      catch (cause) { this.cleanupErrors.push(errorMessage(ownedError(ownerId, cause))); }
    }
    this.cleanupHandles.length = 0;
    try { this.runtime.dispose(); }
    catch (cause) { this.cleanupErrors.push(errorMessage(cause)); }
    if (this.cleanupErrors.length > 0) {
      const existing = this.diagnostic;
      this.diagnostic = {
        ...(existing ?? this.makeDiagnostic('host', 'PresentationHost cleanup failed', undefined)),
        cleanupErrors: [...(existing?.cleanupErrors ?? []), ...this.cleanupErrors],
      };
      this.reportCleanupErrors();
    }
  }

  private queueDispose(): void {
    if (this.disposeQueued || this.mounts !== 0) return;
    this.disposeQueued = true;
    queueMicrotask(() => {
      this.disposeQueued = false;
      if (this.mounts === 0) this.dispose();
    });
  }

  private track(handle: CleanupHandle, ownerId?: string): CleanupHandle {
    if (!handle || typeof handle.dispose !== 'function' || typeof handle.active !== 'boolean') throw new TypeError('UI Module cleanup handle must expose dispose()');
    if (this.disposed) {
      try { handle.dispose(); }
      catch (cause) { this.recordCleanupFailure(ownedError(ownerId, cause)); }
      throw new Error(ownerId ? `PresentationHost is disposed; late registration from ${ownerId} was rejected` : 'PresentationHost is disposed; the late registration was disposed');
    }
    if (handle.active) this.cleanupHandles.push({ handle, ownerId });
    return handle;
  }

  private fail(stage: PresentationDiagnostic['stage'], moduleId: string | undefined, cause: unknown): PresentationDiagnostic {
    const diagnostic = this.makeDiagnostic(stage, `${stageLabel(stage)} failed${moduleId ? ` for ${moduleId}` : ''}: ${errorMessage(cause)}`, moduleId, cause);
    this.diagnostic = diagnostic;
    this.dispose();
    const result = { ...diagnostic, cleanupErrors: [...this.cleanupErrors] };
    for (const listener of this.diagnosticListeners) listener(result);
    return result;
  }

  private makeDiagnostic(stage: PresentationDiagnostic['stage'], message: string, moduleId?: string, cause: unknown = new Error(message)): PresentationDiagnostic {
    return { stage, moduleId, message, cleanupErrors: [...this.cleanupErrors], cause };
  }

  private reportCleanupErrors(): void {
    if (this.cleanupErrors.length <= this.cleanupReportedCount) return;
    const errors = this.cleanupErrors.slice(this.cleanupReportedCount);
    this.cleanupReportedCount = this.cleanupErrors.length;
    console.error('[Vivy UI] cleanup failed', { provenance: this.provenance, errors });
  }

  private recordCleanupFailure(cause: unknown): void {
    this.cleanupErrors.push(errorMessage(cause));
    if (this.disposed) this.reportCleanupErrors();
  }

  private createManagedHost(source: FullUIHost, ownerId?: string): FullUIHost {
    const owner = Symbol(ownerId ?? 'vivy.presentation.owner');
    this.ownerTokens.add(owner);
    const adapters = createCompositionAdapters(source.composition, this.runtime, owner, ownerId, () => this.disposed, (handle) => this.track(handle, ownerId), (cause) => this.recordCleanupFailure(cause));
    const managedRouter: FaceClientRouter = {
      navigate: (options) => this.runtime.navigate(options, source.router),
      invalidate: () => source.router.invalidate(),
    };
    const registerCleanup = (cleanup: () => void): CleanupHandle => {
      if (this.disposed) throw new Error('PresentationHost is disposed; late cleanup registration was rejected');
      const handle = source.registerCleanup(cleanup);
      try { return this.track(handle, ownerId); }
      catch (cause) {
        try { handle.dispose(); } catch (cleanupCause) { this.recordCleanupFailure(cleanupCause); }
        throw cause;
      }
    };
    const face = source.face ? { ...source.face, router: managedRouter } : source.face;
    return {
      ...source,
      face,
      router: managedRouter,
      composition: adapters,
      // The source host already owns the compiler-projected catalog body. Do
      // not accept per-mount catalog props or inspect/provenance metadata as a
      // runtime discovery channel.
      t: source.t,
      registerCleanup,
      dispatchCommand: (id: string, payload?: unknown) => this.runtime.dispatchCommand(id, payload),
    } as FullUIHost;
  }
}

function compositionRuntime(composition: UICompositionHost): LiveCompositionRuntime | undefined {
  return (composition as Partial<LiveCompositionHost>)[LIVE_COMPOSITION];
}

function createCompositionAdapters(source: UICompositionHost, runtime: LiveCompositionRuntime, owner: symbol, ownerId: string | undefined, isDisposed: () => boolean, track: (handle: CleanupHandle) => CleanupHandle, reportCleanupFailure: (cause: unknown) => void): UICompositionHost {
  const sourceRuntime = compositionRuntime(source);
  const wrap = (kind: RegistryKind, registry: UIRegistry<unknown>): UIRegistry<unknown> => {
    const owned = new Map<string, UIRegistrationHandle[]>();
    return {
      register(id, value) {
        let runtimeHandle: UIRegistrationHandle | undefined;
        let sourceHandle: UIRegistrationHandle | undefined;
        try {
          runtimeHandle = runtime.register(kind, id, value, owner, ownerId);
          if (sourceRuntime !== runtime) sourceHandle = registry.register(id, value);
          const composite = registrationHandle(id, sourceHandle, runtimeHandle);
          const tracked = track(composite) as UIRegistrationHandle;
          const stack = owned.get(id) ?? [];
          stack.push(tracked);
          owned.set(id, stack);
          return tracked;
        } catch (cause) {
          try { sourceHandle?.dispose(); } catch (cleanupCause) { reportCleanupFailure(cleanupCause); }
          try { runtimeHandle?.dispose(); } catch (cleanupCause) { reportCleanupFailure(cleanupCause); }
          throw cause;
        }
      },
      unregister(id) {
        const stack = owned.get(id);
        if (!stack) return;
        while (stack.length > 0) {
          const handle = stack.pop()!;
          if (!handle.active) continue;
          handle.dispose();
          break;
        }
        if (stack.length === 0) owned.delete(id);
      },
    };
  };
  return {
    routes: wrap('routes', source.routes),
    navigation: wrap('navigation', source.navigation),
    pages: wrap('pages', source.pages),
    components: wrap('components', source.components),
    styles: wrap('styles', source.styles),
    themes: wrap('themes', source.themes),
    shortcuts: wrap('shortcuts', source.shortcuts),
    commands: wrap('commands', source.commands),
    registerCleanup(cleanup) {
      if (isDisposed()) throw new Error(ownerId ? `PresentationHost is disposed; late cleanup registration from ${ownerId} was rejected` : 'PresentationHost is disposed; late cleanup registration was rejected');
      const handle = source.registerCleanup(cleanup);
      return track(handle);
    },
  };
}

function registrationHandle(id: string, source: UIRegistrationHandle | undefined, runtime: UIRegistrationHandle | undefined): UIRegistrationHandle {
  const handles = [source, runtime].filter((handle): handle is UIRegistrationHandle => Boolean(handle));
  let active = true;
  return {
    id,
    get active() { return active && handles.some((handle) => handle.active); },
    dispose() {
      if (!active) return;
      active = false;
      const failures: unknown[] = [];
      for (const handle of handles) {
        try { handle.dispose(); } catch (cause) { failures.push(cause); }
      }
      if (failures.length === 1) throw failures[0];
      if (failures.length > 1) throw new AggregateError(failures, 'UI registration cleanup failed');
    },
  };
}

function stageLabel(stage: PresentationDiagnostic['stage']): string {
  switch (stage) {
    case 'extension-install': return 'UI extension installation';
    case 'root-render': return 'UI root render';
    case 'root-component': return 'UI root component';
    default: return 'UI host';
  }
}

function errorMessage(cause: unknown): string {
  const message = cause instanceof Error ? cause.message : String(cause);
  return message.length > 1024 ? `${message.slice(0, 1021)}...` : message;
}

function ownedError(ownerId: string | undefined, cause: unknown): Error {
  const message = errorMessage(cause);
  return new Error(ownerId ? `${ownerId}: ${message}` : message, { cause });
}

function provenanceFor(provenance: PresentationProvenance | undefined, root: UIRoot | undefined, extensions: readonly UIExtension[]): PresentationProvenance {
  return {
    ...(provenance ?? {}),
    rootId: provenance?.rootId ?? provenance?.root ?? root?.id ?? 'default',
    extensionIds: provenance?.extensionIds ?? provenance?.extensions ?? extensions.map((extension) => extension.id),
  };
}

function serializedProvenance(provenance: PresentationProvenance): string {
  try { return JSON.stringify(provenance); }
  catch { return '{"error":"unserializable UI provenance"}'; }
}

function diagnosticsNode(diagnostic: PresentationDiagnostic, provenance: PresentationProvenance): ReactNode {
  return createElement(
    'div',
    {
      className: 'vivy-presentation-diagnostic',
      role: 'alert',
      'data-vivy-presentation-error': diagnostic.stage,
      'data-vivy-presentation-provenance': serializedProvenance(provenance),
    },
    createElement('h2', null, 'Vivy UI Module failed'),
    createElement('p', null, diagnostic.message),
    diagnostic.cleanupErrors.length > 0 ? createElement('p', { className: 'vivy-presentation-diagnostic-cleanup' }, `Cleanup: ${diagnostic.cleanupErrors.join('; ')}`) : null,
    createElement('pre', { className: 'vivy-presentation-diagnostic-provenance' }, serializedProvenance(provenance)),
  );
}

class PresentationErrorBoundary extends Component<PropsWithChildren<{ readonly controller: PresentationController; readonly provenance: PresentationProvenance }>, { readonly error: unknown; readonly diagnostic?: PresentationDiagnostic }> {
  state: { readonly error: unknown; readonly diagnostic?: PresentationDiagnostic } = { error: undefined };
  static getDerivedStateFromError(error: unknown): { readonly error: unknown } { return { error }; }
  componentDidCatch(error: unknown, _info: ErrorInfo): void {
    // React renders the derived fallback before componentDidCatch runs. Keep
    // the post-rollback diagnostic in boundary state so cleanup failures are
    // visible on that same contained error surface.
    this.setState({ error, diagnostic: this.props.controller.failRootComponent(error) });
  }
  render(): ReactNode {
    return this.state.error === undefined ? this.props.children : diagnosticsNode(this.state.diagnostic ?? this.props.controller.getDiagnostic() ?? {
      stage: 'root-component', message: `UI root component failed: ${errorMessage(this.state.error)}`, moduleId: undefined, cleanupErrors: [], cause: this.state.error,
    }, this.props.provenance);
  }
}

/** Host one generated Web Face tree with exactly one React root. */
export function PresentationHost({
  host,
  root = generatedUIRoot,
  extensions = generatedUIExtensions,
  provenance = UI_ASSEMBLY_MANIFEST,
  children,
  path,
}: PresentationHostProps): ReactNode {
  const controllerRef = useRef<PresentationController | undefined>(undefined);
  if (!controllerRef.current) controllerRef.current = new PresentationController({ host, root, extensions, provenance: provenanceFor(provenance, root, extensions) });
  const controller = controllerRef.current;
  const resolvedProvenance = useMemo(() => provenanceFor(provenance, root, extensions), [provenance, root, extensions]);
  const runtime = controller.getRuntime();
  useSyncExternalStore(runtime.subscribe, runtime.getSnapshot, runtime.getSnapshot);
  const [phase, setPhase] = useState<PresentationPhase>({ status: 'pending' });

  useEffect(() => { controller.setProvenance(resolvedProvenance); }, [controller, resolvedProvenance]);
  useEffect(() => { runtime.setPath(path ?? browserPath()); }, [runtime, path]);
  useEffect(() => {
    let active = true;
    const unsubscribe = controller.onDiagnostic((diagnostic) => {
      if (active) setPhase({ status: 'error', diagnostic });
    });
    const release = controller.mount();
    const result = controller.start();
    if (result.diagnostic) setPhase({ status: 'error', diagnostic: result.diagnostic });
    else setPhase({ status: 'ready', node: result.node });
    return () => {
      active = false;
      unsubscribe();
      release();
    };
  }, [controller]);

  const diagnostic = phase.status === 'error' ? phase.diagnostic : controller.getDiagnostic();
  const route = runtime.getActiveRoute();
  const routeNode = route
    ? createElement('div', { 'data-vivy-presentation-route': route.id }, renderContribution(routeProjection(route.id, route.value)?.render))
    : null;
  const selectedNode = routeNode ?? (root ? phase.status === 'ready' ? phase.node : null : children);
  const hostedContent = diagnostic
    ? diagnosticsNode(diagnostic, resolvedProvenance)
    : phase.status === 'ready'
      ? createElement(PresentationErrorBoundary, { controller, provenance: resolvedProvenance }, createElement(Fragment, null, navigationNode(runtime, controller.getRouter()), selectedNode))
      : null;
  return createElement('div', { className: 'vivy-presentation-host', 'data-vivy-presentation-tree': '', 'data-vivy-presentation-provenance': serializedProvenance(resolvedProvenance) }, hostedContent);
}

/** Options for the production Face adapter; all values are runtime facts. */
export interface WebFaceHostOptions {
  readonly capabilities?: { readonly protocol_version?: string; readonly capabilities?: readonly string[] };
  readonly catalogs?: PluginCatalogInput;
}

export function createWebFaceHost(router: FaceClientRouter, options: WebFaceHostOptions = {}): FullUIHost {
  const api = faceAPI as unknown as FaceClientAPI;
  const store = useVivyStore as unknown as FaceClientStore<FaceStoreState>;
  const snapshot = rpcTransport.getRpcCapabilitiesSnapshot?.();
  const storeState = store.getState();
  const storeCapabilities = storeState.capabilities ?? [];
  const negotiated = options.capabilities?.capabilities !== undefined
    ? options.capabilities.capabilities
    : storeState.initialized || storeCapabilities.length > 0
      ? storeCapabilities
      : snapshot?.capabilities ?? [];
  const rpc = createLazyRPC({
    protocol_version: options.capabilities?.protocol_version ?? snapshot?.protocol_version ?? '',
    capabilities: [...negotiated],
  });
  const runtime = new LiveCompositionRuntime();
  const composition = createLiveCompositionHost(runtime, Symbol('face-host'), 'face-host');
  const managedRouter: FaceClientRouter = { navigate: (navigation) => runtime.navigate(navigation, router), invalidate: () => router.invalidate() };
  const face: FaceClient = { api, rpc, store, router: managedRouter };
  const generatedCatalogs = (UI_ASSEMBLY_MANIFEST as typeof UI_ASSEMBLY_MANIFEST & { readonly catalogs?: PluginCatalogInput }).catalogs;
  return { face, api, rpc, store, router: managedRouter, composition, t: createPluginTranslator(translate, options.catalogs ?? generatedCatalogs), registerCleanup: (cleanup) => createCleanupHandle(cleanup) };
}

function createLiveCompositionHost(runtime: LiveCompositionRuntime, owner = DEFAULT_COMPOSITION_OWNER, ownerId?: string): UICompositionHost {
  const registry = (kind: RegistryKind): UIRegistry<unknown> => ({ register: (id, value) => runtime.register(kind, id, value, owner, ownerId), unregister: (id) => runtime.unregister(kind, id, owner) });
  const host = {
    routes: registry('routes'), navigation: registry('navigation'), pages: registry('pages'), components: registry('components'), styles: registry('styles'), themes: registry('themes'), shortcuts: registry('shortcuts'), commands: registry('commands'),
    registerCleanup: (cleanup: () => void) => createCleanupHandle(cleanup),
  } as LiveCompositionHost;
  Object.defineProperty(host, LIVE_COMPOSITION, { value: runtime, enumerable: false });
  return host;
}

function createLazyRPC(initial: { protocol_version: string; capabilities: readonly string[] }): FaceClientRPC {
  const capabilities: { protocol_version: string; capabilities: string[] } = { protocol_version: initial.protocol_version, capabilities: [...initial.capabilities] };
  const resolve = async (): Promise<rpcTransport.RpcClient> => {
    const client = await rpcTransport.getRpcClient();
    capabilities.protocol_version = client.capabilities.protocol_version;
    capabilities.capabilities = [...client.capabilities.capabilities];
    return client;
  };
  return {
    capabilities,
    call: (method, params) => resolve().then((client) => client.call(method, params)),
    onNotification: (method, listener) => {
      let active = true;
      let unsubscribe: (() => void) | undefined;
      void resolve().then((client) => { if (active) unsubscribe = client.onNotification(method, listener); });
      return () => { active = false; unsubscribe?.(); };
    },
    onClose: (listener) => {
      let active = true;
      let unsubscribe: (() => void) | undefined;
      void resolve().then((client) => { if (active) unsubscribe = client.onClose(listener); });
      return () => { active = false; unsubscribe?.(); };
    },
    close: () => { void resolve().then((client) => client.close()); },
  };
}

/** Apply the approved Web/TUI plugin fallback contract to one explicit projection. */
export function createPluginTranslator(base: UITranslator, input?: PluginCatalogInput, owner?: string): UITranslator {
  const catalogs = normalizeCatalogs(input, owner);
  return (key: string, args?: UITranslationArgs, form?: UIMessageForm): string => {
    if (!key.startsWith('plugin.')) return base(key, args, form);
    const catalog = findCatalogForKey(catalogs, key);
    const unit = catalog?.units[key];
    if (!unit) return missingTranslation(key);
    const locale = getLocale();
    const candidates = form ? [unit[form]?.[locale], unit.messages[locale], unit[form]?.en, unit.messages.en] : [unit.messages[locale], unit.messages.en];
    const template = candidates.find((candidate): candidate is string => typeof candidate === 'string' && candidate.length > 0);
    if (template === undefined) return missingTranslation(key);
    const rendered = interpolate(template, args);
    const missing = rendered.match(/\{\{\w+\}\}/g);
    return missing ? `${rendered} [missing translation arguments: ${diagnosticKey(key)}]` : rendered;
  };
}

const TRANSLATION_DIAGNOSTIC_KEY_LIMIT = 512;

function diagnosticKey(key: string): string {
  return key.length > TRANSLATION_DIAGNOSTIC_KEY_LIMIT
    ? `${key.slice(0, TRANSLATION_DIAGNOSTIC_KEY_LIMIT - 3)}...`
    : key;
}

function missingTranslation(key: string): string {
  return `[missing translation: ${diagnosticKey(key)}]`;
}

function normalizeCatalogs(input: PluginCatalogInput | undefined, owner?: string): PluginCatalogProjectionEntry[] {
  if (!input) return [];
  const candidates = Array.isArray(input)
    ? input.map((catalog) => ({ catalog, hintedOwner: owner }))
    : Object.entries(input).map(([hintedOwner, catalog]) => ({ catalog, hintedOwner }));
  const normalized: PluginCatalogProjectionEntry[] = [];
  const claimedKeys = new Map<string, string>();
  const claimedModules = new Set<string>();
  for (const candidate of candidates) {
    const catalog = validateCatalogProjection(candidate.catalog, candidate.hintedOwner, candidates.length === 1);
    if (!catalog) return [];
    const module = catalog.module;
    if (!module || claimedModules.has(module)) return [];
    claimedModules.add(module);
    const prefix = `plugin.${module}.`;
    for (const key of Object.keys(catalog.units)) {
      if (!key.startsWith(prefix) || key.length === prefix.length) return [];
      const previous = claimedKeys.get(key);
      if (previous && previous !== module) return [];
      claimedKeys.set(key, module);
    }
    normalized.push(catalog);
  }
  return normalized.sort((left, right) => (left.module ?? '').localeCompare(right.module ?? ''));
}

const catalogOwnerPattern = /^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?\/[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$/;
const catalogLocalePattern = /^[a-z]{2,3}(?:-[a-z0-9]{2,8})*$/;
const catalogPlaceholderPattern = /^[a-z][a-z0-9_]*$/;
const catalogPlaceholderTokenPattern = /\{\{([^{}]*)\}\}/g;
const catalogDigestPattern = /^[0-9a-f]{64}$/;
const catalogAPI = 'vivy.i18n/v1';
const maxCatalogBytes = 1 << 20;
const maxCatalogUnits = 4096;
const maxCatalogMessageBytes = 8 << 10;
const maxCatalogPlaceholders = 32;

type CatalogRecord = Record<string, unknown>;

function isPlainRecord(value: unknown): value is CatalogRecord {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return false;
  const prototype = Object.getPrototypeOf(value);
  return prototype === Object.prototype || prototype === null;
}

function hasOnlyCatalogKeys(value: CatalogRecord, allowed: readonly string[]): boolean {
  return Object.keys(value).every((key) => allowed.includes(key));
}

function normalizeCatalogLocale(value: string): string {
  return value.trim().toLowerCase().replaceAll('_', '-');
}

function inferCatalogOwner(units: CatalogRecord): string | undefined {
  const keys = Object.keys(units);
  if (keys.length === 0) return undefined;
  const prefix = 'plugin.';
  if (!keys.every((key) => key.startsWith(prefix))) return undefined;
  const owners = new Set(keys.map((key) => {
    const body = key.slice(prefix.length);
    const separator = body.lastIndexOf('.');
    return separator > 0 ? body.slice(0, separator) : '';
  }));
  return owners.size === 1 ? [...owners][0] : undefined;
}

function validateCatalogProjection(value: unknown, hintedOwner: string | undefined, allowInferredOwner: boolean): PluginCatalogProjectionEntry | undefined {
  if (!isPlainRecord(value) || !hasOnlyCatalogKeys(value, ['apiVersion', 'schemaVersion', 'path', 'defaultLocale', 'locales', 'module', 'moduleId', 'digest', 'completeness', 'evidence', 'units'])) return undefined;
  if (!isPlainRecord(value.units) || Object.keys(value.units).length === 0 || Object.keys(value.units).length > maxCatalogUnits) return undefined;

  const rawModule = value.module ?? value.moduleId ?? (allowInferredOwner ? hintedOwner ?? inferCatalogOwner(value.units) : undefined);
  if (typeof rawModule !== 'string' || !catalogOwnerPattern.test(rawModule)) return undefined;
  if (value.module !== undefined && typeof value.module !== 'string') return undefined;
  if (value.moduleId !== undefined && typeof value.moduleId !== 'string') return undefined;
  if (typeof value.module === 'string' && typeof value.moduleId === 'string' && value.module !== value.moduleId) return undefined;

  if (value.apiVersion !== catalogAPI || value.schemaVersion !== catalogAPI) return undefined;
  if (typeof value.path !== 'string' || !isCatalogPath(value.path)) return undefined;
  if (typeof value.defaultLocale !== 'string' || normalizeCatalogLocale(value.defaultLocale) !== 'en') return undefined;
  if (!Array.isArray(value.locales) || value.locales.length === 0) return undefined;
  const locales = value.locales.map((locale) => typeof locale === 'string' ? normalizeCatalogLocale(locale) : '');
  if (locales.some((locale) => !catalogLocalePattern.test(locale))) return undefined;
  const uniqueLocales = new Set(locales);
  if (uniqueLocales.size !== locales.length || !uniqueLocales.has('en')) return undefined;
  locales.sort(compareCatalogStrings);

  if (typeof value.digest !== 'string' || !catalogDigestPattern.test(value.digest)) return undefined;
  const units: Record<string, PluginCatalogUnitProjection> = {};
  for (const key of Object.keys(value.units).sort(compareCatalogStrings)) {
    if (!key.startsWith(`plugin.${rawModule}.`) || key.length === `plugin.${rawModule}.`.length) return undefined;
    const unit = normalizeCatalogUnit(value.units[key], locales);
    if (!unit) return undefined;
    units[key] = unit;
  }

  let completeness: Record<string, string> | undefined;
  if (value.completeness !== undefined) {
    if (!isPlainRecord(value.completeness)) return undefined;
    completeness = {};
    for (const [locale, state] of Object.entries(value.completeness)) {
      const normalizedLocale = normalizeCatalogLocale(locale);
      if (!catalogLocalePattern.test(normalizedLocale) || Object.prototype.hasOwnProperty.call(completeness, normalizedLocale) || typeof state !== 'string' || (state !== 'COMPLETE' && state !== 'INCOMPLETE')) return undefined;
      completeness[normalizedLocale] = state;
    }
  }
  let evidence: string[] | undefined;
  if (value.evidence !== undefined) {
    if (!Array.isArray(value.evidence) || value.evidence.some((entry) => typeof entry !== 'string' || entry.length === 0)) return undefined;
    evidence = [...new Set(value.evidence)].sort(compareCatalogStrings);
  }
  const normalized: PluginCatalogProjectionEntry = {
    apiVersion: catalogAPI,
    schemaVersion: catalogAPI,
    path: value.path,
    defaultLocale: 'en',
    locales,
    module: rawModule,
    digest: value.digest,
    ...(completeness ? { completeness } : {}),
    ...(evidence ? { evidence } : {}),
    units,
  };

  const canonicalProjection = { apiVersion: catalogAPI, units };
  if (sha256Hex(goJSONString(canonicalProjection)) !== value.digest) return undefined;
  if (utf8ByteLength(goJSONString(normalized)) > maxCatalogBytes) return undefined;
  return normalized;
}

function isCatalogPath(path: string): boolean {
  if (!path || path.includes('\\') || path.startsWith('/') || /^[A-Za-z]:/.test(path) || !path.endsWith('.json')) return false;
  const segments = path.split('/');
  return segments.every((segment) => segment.length > 0 && segment !== '.' && segment !== '..');
}

function compareCatalogStrings(left: string, right: string): number {
  return left < right ? -1 : left > right ? 1 : 0;
}

function normalizeCatalogUnit(value: unknown, locales: readonly string[]): PluginCatalogUnitProjection | undefined {
  if (!isPlainRecord(value) || !hasOnlyCatalogKeys(value, ['description', 'placeholders', 'messages', 'short', 'long'])) return undefined;
  if (typeof value.description !== 'string' || value.description.trim().length === 0) return undefined;
  if (!Array.isArray(value.placeholders) || value.placeholders.length > maxCatalogPlaceholders || value.placeholders.some((placeholder) => typeof placeholder !== 'string' || !catalogPlaceholderPattern.test(placeholder))) return undefined;
  if (!value.placeholders.every((placeholder, index, all) => index === 0 || compareCatalogStrings(all[index - 1], placeholder) <= 0)) return undefined;
  if (new Set(value.placeholders).size !== value.placeholders.length) return undefined;
  const declared = new Set(value.placeholders);
  const normalizedMessages = normalizeCatalogMessages(value.messages, locales, declared);
  if (!normalizedMessages) return undefined;
  let short: Readonly<Record<string, string>> | undefined;
  let long: Readonly<Record<string, string>> | undefined;
  if (value.short !== undefined) {
    short = normalizeCatalogMessages(value.short, locales, declared);
    if (!short) return undefined;
  }
  if (value.long !== undefined) {
    long = normalizeCatalogMessages(value.long, locales, declared);
    if (!long) return undefined;
  }
  const unit: PluginCatalogUnitProjection = {
    description: value.description,
    placeholders: [...value.placeholders],
    messages: normalizedMessages,
    ...(short ? { short } : {}),
    ...(long ? { long } : {}),
  };
  return unit;
}

function normalizeCatalogMessages(value: unknown, locales: readonly string[], declared: ReadonlySet<string>): Readonly<Record<string, string>> | undefined {
  if (!isPlainRecord(value) || Object.keys(value).length === 0) return undefined;
  const allowed = new Set(locales);
  const normalized: Record<string, string> = {};
  for (const [rawLocale, message] of Object.entries(value)) {
    const locale = normalizeCatalogLocale(rawLocale);
    if (!allowed.has(locale) || Object.prototype.hasOwnProperty.call(normalized, locale) || typeof message !== 'string' || (locale === 'en' && message.trim().length === 0)) return undefined;
    if (utf8ByteLength(message) > maxCatalogMessageBytes || !messageHasPlaceholders(message, declared)) return undefined;
    normalized[locale] = message;
  }
  if (typeof normalized.en !== 'string' || normalized.en.trim().length === 0) return undefined;
  return Object.fromEntries(Object.entries(normalized).sort(([left], [right]) => compareCatalogStrings(left, right)));
}

function messageHasPlaceholders(message: string, declared: ReadonlySet<string>): boolean {
  const found = new Set<string>();
  let malformed = false;
  const consumed = message.replace(catalogPlaceholderTokenPattern, (token, name: string) => {
    if (!catalogPlaceholderPattern.test(name)) {
      malformed = true;
      return token;
    }
    found.add(name);
    return '';
  });
  if (malformed || consumed.includes('{{') || consumed.includes('}}') || found.size !== declared.size) return false;
  for (const name of declared) if (!found.has(name)) return false;
  return true;
}

function goJSONString(value: unknown): string {
  return JSON.stringify(value).replace(/[<>&\u2028\u2029]/g, (character) => {
    switch (character) {
      case '<': return '\\u003c';
      case '>': return '\\u003e';
      case '&': return '\\u0026';
      case '\u2028': return '\\u2028';
      default: return '\\u2029';
    }
  });
}

function utf8ByteLength(value: string): number {
  return new TextEncoder().encode(value).byteLength;
}

function sha256Hex(value: string): string {
  const bytes = new TextEncoder().encode(value);
  const bitLength = bytes.length * 8;
  const paddedLength = Math.ceil((bytes.length + 9) / 64) * 64;
  const padded = new Uint8Array(paddedLength);
  padded.set(bytes);
  padded[bytes.length] = 0x80;
  const view = new DataView(padded.buffer);
  view.setUint32(paddedLength - 8, Math.floor(bitLength / 0x100000000), false);
  view.setUint32(paddedLength - 4, bitLength >>> 0, false);
  const constants = [
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
    0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
    0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
    0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
    0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
    0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
    0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
    0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
  ];
  let h = [0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19];
  for (let offset = 0; offset < padded.length; offset += 64) {
    const words = new Array<number>(64).fill(0);
    for (let index = 0; index < 16; index += 1) words[index] = view.getUint32(offset + index * 4, false);
    for (let index = 16; index < 64; index += 1) {
      const first = words[index - 15];
      const second = words[index - 2];
      const sigma0 = ((first >>> 7) | (first << 25)) ^ ((first >>> 18) | (first << 14)) ^ (first >>> 3);
      const sigma1 = ((second >>> 17) | (second << 15)) ^ ((second >>> 19) | (second << 13)) ^ (second >>> 10);
      words[index] = (words[index - 16] + sigma0 + words[index - 7] + sigma1) >>> 0;
    }
    let [a, b, c, d, e, f, g, currentH] = h;
    for (let index = 0; index < 64; index += 1) {
      const sigma1 = ((e >>> 6) | (e << 26)) ^ ((e >>> 11) | (e << 21)) ^ ((e >>> 25) | (e << 7));
      const choice = (e & f) ^ (~e & g);
      const temp1 = (currentH + sigma1 + choice + constants[index] + words[index]) >>> 0;
      const sigma0 = ((a >>> 2) | (a << 30)) ^ ((a >>> 13) | (a << 19)) ^ ((a >>> 22) | (a << 10));
      const majority = (a & b) ^ (a & c) ^ (b & c);
      const temp2 = (sigma0 + majority) >>> 0;
      currentH = g;
      g = f;
      f = e;
      e = (d + temp1) >>> 0;
      d = c;
      c = b;
      b = a;
      a = (temp1 + temp2) >>> 0;
    }
    h = h.map((word, index) => (word + [a, b, c, d, e, f, g, currentH][index]) >>> 0);
  }
  return h.map((word) => word.toString(16).padStart(8, '0')).join('');
}

function findCatalogForKey(catalogs: readonly PluginCatalogProjectionEntry[], key: string): PluginCatalogProjectionEntry | undefined {
  return catalogs.filter((catalog) => {
    const owner = catalog.module ?? catalog.moduleId;
    return Boolean(owner && key.startsWith(`plugin.${owner}.`));
  }).sort((left, right) => (right.module ?? right.moduleId ?? '').length - (left.module ?? left.moduleId ?? '').length)[0];
}

function interpolate(template: string, args?: UITranslationArgs): string {
  return template.replace(/\{\{(\w+)\}\}/g, (match, name: string) => args && Object.prototype.hasOwnProperty.call(args, name) ? String(args[name]) : match);
}

function navigationNode(runtime: LiveCompositionRuntime, router: FaceClientRouter): ReactNode {
  const entries = runtime.getEntries('navigation');
  if (entries.length === 0) return null;
  return createElement('nav', { 'data-vivy-presentation-navigation': '', 'aria-label': 'Module navigation' }, entries.map((entry, index) => createElement(NavigationItem, { key: `${entry.id}-${index}`, entry, runtime, router })));
}

function NavigationItem({ entry, runtime, router }: { readonly entry: CompositionEntry; readonly runtime: LiveCompositionRuntime; readonly router: FaceClientRouter }): ReactNode {
  const projection = navigationProjection(entry.id, entry.value);
  if (projection?.render !== undefined) return renderContribution(projection.render);
  if (projection?.component) return createElement(projection.component, { id: entry.id });
  if (isValidElement(entry.value) || typeof entry.value === 'function') return renderContribution(entry.value);
  const label = projection?.label ?? entry.id;
  const target = projection?.target;
  const onClick = (event: MouseEvent<HTMLElement>) => {
    if (projection?.onClick) {
      void Promise.resolve(projection.onClick(event)).catch((cause) => runtime.reportError(cause, entry.ownerId));
      return;
    }
    if (target) {
      event.preventDefault();
      void runtime.navigate({ to: target }, router).catch((cause) => runtime.reportError(cause, entry.ownerId));
    }
  };
  return target ? createElement('a', { href: target, onClick }, label) : createElement('button', { type: 'button', onClick }, label);
}

function navigationProjection(id: string, value: unknown): NavigationProjection | undefined {
  if (!value || typeof value !== 'object') return undefined;
  const object = value as Record<string, unknown>;
  const target = typeof object.to === 'string' ? object.to : typeof object.href === 'string' ? object.href : typeof object.path === 'string' ? object.path : undefined;
  const render = object.render ?? object.element;
  const component = typeof object.component === 'function' ? object.component as ComponentType<Record<string, unknown>> : undefined;
  const onClick = typeof object.onClick === 'function' ? object.onClick as NavigationProjection['onClick'] : undefined;
  return { target, label: object.label as ReactNode ?? object.title as ReactNode ?? id, render, component, onClick };
}

function routeProjection(id: string, value: unknown): RouteProjection | undefined {
  if (typeof value === 'function' || isValidElement(value)) return id.startsWith('/') ? { path: id, render: value } : undefined;
  if (!value || typeof value !== 'object') return id.startsWith('/') ? { path: id, render: value } : undefined;
  const object = value as Record<string, unknown>;
  const path = typeof object.path === 'string' ? object.path : typeof object.to === 'string' ? object.to : typeof object.href === 'string' ? object.href : id.startsWith('/') ? id : undefined;
  const render = object.render ?? object.component ?? object.element ?? value;
  return path && render !== undefined ? { path, render } : undefined;
}

function renderContribution(value: unknown): ReactNode {
  if (isValidElement(value)) return value;
  if (typeof value === 'function') return createElement(value as ComponentType);
  if (value && typeof value === 'object') {
    const object = value as Record<string, unknown>;
    if (isValidElement(object.element)) return object.element;
    if (typeof object.render === 'function') return createElement(object.render as ComponentType);
    if (typeof object.component === 'function') return createElement(object.component as ComponentType);
  }
  return typeof value === 'string' || typeof value === 'number' ? value : null;
}

function cssText(value: unknown): string | undefined {
  if (typeof value === 'string') return value;
  if (!value || typeof value !== 'object') return undefined;
  const object = value as Record<string, unknown>;
  for (const key of ['css', 'cssText', 'text', 'textContent']) if (typeof object[key] === 'string') return object[key] as string;
  return undefined;
}

function themeProjection(value: unknown): ThemeProjection | undefined {
  if (typeof value === 'string') return { variables: {}, classes: [], attributes: {}, css: value };
  if (!value || typeof value !== 'object') return undefined;
  const object = value as Record<string, unknown>;
  const rawVariables = object.variables ?? object.cssVariables;
  const variables: Record<string, string> = {};
  if (rawVariables && typeof rawVariables === 'object') for (const [name, raw] of Object.entries(rawVariables)) if (typeof raw === 'string' || typeof raw === 'number') variables[name] = String(raw);
  const rawClasses = object.classes ?? object.className;
  const classes = typeof rawClasses === 'string' ? rawClasses.split(/\s+/).filter(Boolean) : Array.isArray(rawClasses) ? rawClasses.filter((item): item is string => typeof item === 'string') : [];
  const rawAttributes = object.attributes;
  const attributes: Record<string, string | null> = {};
  if (rawAttributes && typeof rawAttributes === 'object') for (const [name, raw] of Object.entries(rawAttributes)) if (typeof raw === 'string' || raw === null) attributes[name] = raw;
  const css = cssText(value);
  return { variables, classes, attributes, css };
}

function commandHandler(value: unknown): ((payload?: unknown) => unknown) | undefined {
  if (typeof value === 'function') return value as (payload?: unknown) => unknown;
  if (!value || typeof value !== 'object') return undefined;
  const object = value as Record<string, unknown>;
  for (const key of ['execute', 'handler', 'run', 'onInvoke']) if (typeof object[key] === 'function') return object[key] as (payload?: unknown) => unknown;
  return undefined;
}

function shortcutProjection(id: string, value: unknown): ShortcutProjection | undefined {
  if (typeof value === 'string') return { shortcut: value };
  if (!value || typeof value !== 'object') return undefined;
  const object = value as Record<string, unknown>;
  const shortcut = typeof object.shortcut === 'string' ? object.shortcut : typeof object.key === 'string' ? object.key : id;
  const handler = typeof object.handler === 'function' ? object.handler as (event: KeyboardEvent) => unknown : typeof object.onTrigger === 'function' ? object.onTrigger as (event: KeyboardEvent) => unknown : undefined;
  const command = typeof object.command === 'string' ? object.command : undefined;
  return { shortcut, handler, command };
}

function matchesShortcut(shortcut: string, event: KeyboardEvent): boolean {
  const tokens = shortcut.toLowerCase().split('+').map((token) => token.trim()).filter(Boolean);
  const key = tokens.at(-1);
  if (!key) return false;
  const wants = (name: string): boolean => tokens.includes(name);
  if (wants('ctrl') !== event.ctrlKey || wants('alt') !== event.altKey || wants('shift') !== event.shiftKey || (wants('meta') || wants('cmd')) !== event.metaKey) return false;
  return event.key.toLowerCase() === key || event.code.toLowerCase() === key;
}

function browserPath(): string { return typeof window === 'undefined' ? '/' : normalizePath(window.location.pathname || '/'); }

function normalizePath(path: string): string {
  const value = path.trim() || '/';
  const withSlash = value.startsWith('/') ? value : `/${value}`;
  return withSlash.length > 1 ? withSlash.replace(/\/+$/, '') : '/';
}

export { COMMAND_EVENT as PRESENTATION_COMMAND_EVENT };
