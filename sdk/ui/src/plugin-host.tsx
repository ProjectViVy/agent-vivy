/**
 * Host binding for selected UI Modules.
 *
 * The PresentationHost mounts the provider once around everything an extension
 * renders, so Module code reads the live host through a hook instead of
 * receiving host internals (or a translator copy) through props or a private
 * per-Module shim. This lives in the SDK because it is part of the Module ABI:
 * an out-of-tree Module localizes its own sealed `plugin.<module-id>.*` catalog
 * with nothing but this package.
 */
import { createContext, useContext, useMemo, type ReactNode } from 'react';
import type { FullUIHost, UITranslator } from './module';

const PluginHostContext = createContext<FullUIHost | undefined>(undefined);

/** Publishes the live host to everything the assembled Generation renders. */
export function PluginHostProvider({ host, children }: {
  readonly host: FullUIHost;
  readonly children: ReactNode;
}): ReactNode {
  return <PluginHostContext.Provider value={host}>{children}</PluginHostContext.Provider>;
}

/** The live host when rendering inside the assembled Face, otherwise undefined. */
export function usePluginHost(): FullUIHost | undefined {
  return useContext(PluginHostContext);
}

/** Unresolved keys stay visible instead of throwing outside a host. */
function unresolvedKey(key: string): string {
  return key;
}

/**
 * Translation binding of the host this code renders inside. It resolves the
 * Module's sealed `plugin.*` units and falls back to the core dictionary the
 * shell and the Module share, so Module copy has exactly one home: its catalog.
 * The returned object stays referentially stable while the host does, so it is
 * safe to list as a memo dependency.
 */
export function usePluginTranslation(): { readonly t: UITranslator } {
  const t = usePluginHost()?.t ?? unresolvedKey;
  return useMemo(() => ({ t }), [t]);
}