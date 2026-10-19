import { useCallback, useEffect, useRef, useState } from 'react';
import { usePluginTranslation, type UITranslator } from '@vivy/ui-sdk';
import { AlertTriangle, RefreshCw } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  ToolkitClient,
  newOperationID,
  type PreparedCandidate,
  type ToolkitInstallation,
  type ToolkitInspection,
  type ToolkitOperation,
  type ToolkitPageResult,
  type ToolkitPreview,
} from './toolkit-client';
import { ToolkitCandidate } from './ToolkitCandidate';
import { ToolkitContentDialog, type ComponentBody } from './ToolkitContentDialog';
import { ToolkitDetails, type SourceChange } from './ToolkitDetails';
import { ToolkitInstallDialog } from './ToolkitInstallDialog';
import { ToolkitInventory } from './ToolkitInventory';
import { ToolkitLibrary } from './ToolkitLibrary';
import {
  candidateTarget,
  operationStateLabel,
  shortDigest,
  type PendingAction,
  type SourceKind,
} from './toolkit-view';

const LAST_OPERATION_KEY = 'vivy.toolkits.last-operation-id';
/** Marks the one refresh failure that means "the catalog was republished", not "the read failed". */
const CATALOG_CHANGED_CODE = 'catalog_changed';

/**
 * Toolkit (工具包) management.
 *
 * The page owns the async orchestration and the catalog/selection state; the
 * child components own the input they collect. A machine token from the backend
 * is translated through `toolkit-view` before it reaches the screen, and a
 * mutation is only ever expressed on the control that triggered it.
 */
export function ToolkitPage({ client }: { readonly client: ToolkitClient }) {
  const { t } = usePluginTranslation();
  const [page, setPage] = useState<ToolkitPageResult | null>(null);
  const [installations, setInstallations] = useState<readonly ToolkitInstallation[]>([]);
  const [selected, setSelected] = useState<ToolkitInspection | null>(null);
  const [candidate, setCandidate] = useState<PreparedCandidate | null>(null);
  const [operation, setOperation] = useState<ToolkitOperation | null>(null);
  const [body, setBody] = useState<ComponentBody | null>(null);
  const [installOpen, setInstallOpen] = useState(false);
  const [firstLoading, setFirstLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [pending, setPending] = useState<PendingAction | null>(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const bodyTriggerRef = useRef<HTMLButtonElement | null>(null);
  const detailsRef = useRef<HTMLElement | null>(null);
  const requestSequence = useRef(0);
  const detailSequence = useRef(0);
  const loaded = useRef(false);

  const refresh = useCallback(async (): Promise<void> => {
    const sequence = ++requestSequence.current;
    if (loaded.current) setRefreshing(true);
    else setFirstLoading(true);
    setError('');
    try {
      const items: ToolkitInstallation[] = [];
      let after = '';
      let first: ToolkitPageResult | null = null;
      for (let pageIndex = 0; pageIndex < 100; pageIndex += 1) {
        const next = await client.list({ after_id: after, limit: 100 });
        if (first && catalogKey(first.catalog) !== catalogKey(next.catalog)) {
          throw Object.assign(new Error('Toolkit catalog changed while paging.'), { code: CATALOG_CHANGED_CODE });
        }
        first ??= next;
        items.push(...next.items);
        if (!next.next_after_id || next.next_after_id === after) break;
        after = next.next_after_id;
      }
      if (sequence === requestSequence.current) {
        setPage(first ?? null);
        setInstallations(items);
        loaded.current = true;
        setFirstLoading(false);
        setRefreshing(false);
      }
    } catch (cause) {
      if (sequence === requestSequence.current) {
        setFirstLoading(false);
        setRefreshing(false);
        // A catalog republished mid-page is not a transport failure: the list was
        // assembled from two different catalog revisions, so say so.
        const republished = (cause as { readonly code?: string } | null)?.code === CATALOG_CHANGED_CODE;
        setError(
          republished
            ? t('plugin.vivy/toolkits-ui.catalogChanged')
            : t('plugin.vivy/toolkits-ui.catalogReadError'),
        );
      }
    }
  }, [client, t]);

  const inspect = useCallback(
    async (item: ToolkitInstallation): Promise<void> => {
      const sequence = ++detailSequence.current;
      setError('');
      try {
        const detail = await client.inspect(item.install_id);
        if (sequence === detailSequence.current && detail.installation.install_id === item.install_id) {
          setSelected(detail);
        }
      } catch {
        if (sequence === detailSequence.current) setError(t('plugin.vivy/toolkits-ui.detailReadError'));
      }
    },
    [client, t],
  );

  const showOperation = useCallback(
    async (operationID: string): Promise<ToolkitOperation | null> => {
      if (!operationID) return null;
      try {
        const latest = await client.operation(operationID);
        setOperation(latest);
        if (latest.state === 'pending' && latest.prepared) {
          setCandidate({
            operation: latest,
            candidate_digest: latest.prepared.candidate_digest,
            expected_catalog: latest.prepared.expected_catalog,
            releases: latest.prepared.releases,
          });
        } else if (latest.state !== 'pending') {
          setCandidate(null);
          clearLastOperationID();
          if (latest.state === 'committed') await refresh();
        } else {
          setCandidate(null);
          setNotice(t('plugin.vivy/toolkits-ui.operationPreparing'));
        }
        return latest;
      } catch {
        // A session-storage value is only a recovery hint. The backend remains
        // the authority; an unavailable receipt is shown as unknown.
        setNotice(t('plugin.vivy/toolkits-ui.operationRecovery'));
        return null;
      }
    },
    [client, refresh, t],
  );

  useEffect(() => {
    void refresh();
    const onFocus = () => {
      void refresh();
    };
    window.addEventListener('focus', onFocus);
    const operationID = readLastOperationID();
    if (operationID) void showOperation(operationID);
    return () => {
      requestSequence.current += 1;
      window.removeEventListener('focus', onFocus);
    };
  }, [refresh, showOperation]);

  const selectFromLibrary = (item: ToolkitInstallation) => {
    void inspect(item).finally(() => {
      // Below the side-by-side breakpoint the details sit under the library, so
      // selecting a Toolkit has to bring the details into view.
      if (typeof window !== 'undefined' && window.innerWidth < 1280) {
        detailsRef.current?.scrollIntoView?.({ block: 'start' });
      }
    });
  };

  /** Validates a source and returns the preview, or `null` when it failed. */
  async function previewSource(input: {
    readonly kind: SourceKind;
    readonly location: string;
    readonly ref: string;
  }): Promise<ToolkitPreview | null> {
    if (!page?.catalog || !input.location.trim() || candidate) return null;
    try {
      return await client.preview(input.kind, {
        operation_id: newOperationID(),
        expected_catalog: page.catalog,
        location: input.location.trim(),
        ...(input.kind === 'git' ? { ref: input.ref.trim() || 'HEAD' } : {}),
      });
    } catch {
      return null;
    }
  }

  /**
   * Installs the previewed release. `unknown` means the outcome could not be
   * confirmed: the page then shows the operation receipt instead of claiming
   * either result.
   */
  async function installPreview(preview: ToolkitPreview): Promise<'ok' | 'failed' | 'unknown'> {
    if (!page?.catalog || candidate) return 'failed';
    const operationID = newOperationID();
    rememberOperationID(operationID);
    setError('');
    setNotice('');
    try {
      const result = await client.install({
        operation_id: operationID,
        expected_catalog: page.catalog,
        preview_operation_id: preview.operation_id,
        expected_release_digest: preview.release_digest,
      });
      setOperation(result);
      clearLastOperationID();
      setNotice(t('plugin.vivy/toolkits-ui.installReady'));
      await refresh();
      await inspect({ install_id: preview.install_id } as ToolkitInstallation);
      setInstallOpen(false);
      return 'ok';
    } catch {
      const recovered = await showOperation(operationID);
      if (recovered?.state === 'committed') {
        setError('');
        setNotice(t('plugin.vivy/toolkits-ui.installReady'));
        await refresh();
        await inspect({ install_id: preview.install_id } as ToolkitInstallation);
        setInstallOpen(false);
        return 'ok';
      }
      setNotice(t('plugin.vivy/toolkits-ui.installUnknown'));
      setInstallOpen(false);
      return 'unknown';
    }
  }

  async function prepareMutation(mutation: 'enable' | 'disable' | 'uninstall') {
    const catalog = page?.catalog;
    const item = selected?.installation;
    if (!catalog || !item || candidate) return;
    const operationID = newOperationID();
    rememberOperationID(operationID);
    setPending({ action: 'prepare', key: mutation });
    setError('');
    setNotice('');
    try {
      const result = await client.prepare(mutation, {
        operation_id: operationID,
        expected_catalog: catalog,
        install_id: item.install_id,
        expected_install_revision: item.revision,
      });
      setCandidate(result);
      setOperation(result.operation);
      setNotice(t('plugin.vivy/toolkits-ui.candidateReady'));
    } catch {
      const recovered = await showOperation(operationID);
      if (recovered?.state === 'pending' && recovered.prepared) return;
      setError(t('plugin.vivy/toolkits-ui.mutationFailed'));
    } finally {
      setPending(null);
    }
  }

  async function prepareSource(mutation: 'update' | 'rebind', change: SourceChange) {
    const catalog = page?.catalog;
    const item = selected?.installation;
    if (!catalog || !item || !change.location.trim() || candidate) return;
    const operationID = newOperationID();
    const previewOperationID = newOperationID();
    rememberOperationID(operationID);
    setPending({ action: 'source', key: mutation });
    setError('');
    setNotice('');
    try {
      const result = await client.prepareSource(change.kind, mutation, {
        operation_id: operationID,
        preview_operation_id: previewOperationID,
        expected_catalog: catalog,
        install_id: item.install_id,
        expected_install_revision: item.revision,
        location: change.location.trim(),
        ...(change.kind === 'git' ? { ref: change.ref.trim() || 'HEAD' } : {}),
      });
      setCandidate(result);
      setOperation(result.operation);
      setNotice(t('plugin.vivy/toolkits-ui.candidateReady'));
    } catch {
      const recovered = await showOperation(operationID);
      if (recovered?.state === 'pending' && recovered.prepared) return;
      setError(t('plugin.vivy/toolkits-ui.sourcePrepareFailed'));
    } finally {
      setPending(null);
    }
  }

  async function prepareRollback(releaseDigest: string) {
    const catalog = page?.catalog;
    const item = selected?.installation;
    if (!catalog || !item || !releaseDigest || candidate) return;
    const operationID = newOperationID();
    rememberOperationID(operationID);
    setPending({ action: 'rollback', key: releaseDigest });
    setError('');
    setNotice('');
    try {
      const result = await client.prepare('rollback', {
        operation_id: operationID,
        expected_catalog: catalog,
        install_id: item.install_id,
        expected_install_revision: item.revision,
        release_digest: releaseDigest,
      });
      setCandidate(result);
      setOperation(result.operation);
      setNotice(t('plugin.vivy/toolkits-ui.rollbackReady'));
    } catch {
      const recovered = await showOperation(operationID);
      if (recovered?.state === 'pending' && recovered.prepared) return;
      setError(t('plugin.vivy/toolkits-ui.rollbackFailed'));
    } finally {
      setPending(null);
    }
  }

  async function saveBindings(
    kind: 'hook' | 'mcp',
    componentID: string,
    bindings: readonly { readonly name: string; readonly reference: string }[],
  ) {
    const catalog = page?.catalog;
    const item = selected?.installation;
    if (!catalog || !item || candidate) return;
    const operationID = newOperationID();
    rememberOperationID(operationID);
    setPending({ action: 'bindings', key: `${kind}/${componentID}` });
    setError('');
    setNotice('');
    try {
      const result = await client.setBindings({
        operation_id: operationID,
        expected_catalog: catalog,
        expected_config_revision: catalog.config_revision,
        install_id: item.install_id,
        kind,
        component_id: componentID,
        bindings,
      });
      setOperation(result);
      clearLastOperationID();
      setNotice(t('plugin.vivy/toolkits-ui.bindingsSaved'));
      await inspect(item);
      await refresh();
    } catch {
      const recovered = await showOperation(operationID);
      if (recovered?.state === 'committed') {
        await inspect(item);
        setError('');
        setNotice(t('plugin.vivy/toolkits-ui.bindingsSaved'));
        return;
      }
      setError(t('plugin.vivy/toolkits-ui.bindingsConflict'));
    } finally {
      setPending(null);
    }
  }

  async function publishCandidate() {
    if (!candidate) return;
    const operationIDToPublish = candidate.operation.id;
    const selectedID = selected?.installation.install_id ?? '';
    setPending({ action: 'publish', key: operationIDToPublish });
    setError('');
    setNotice('');
    try {
      const result = await client.publish({
        operation_id: operationIDToPublish,
        candidate_digest: candidate.candidate_digest,
        expected_catalog: candidate.expected_catalog,
      });
      setOperation(result);
      clearLastOperationID();
      setNotice(t('plugin.vivy/toolkits-ui.catalogUpdated'));
      // The candidate stays on screen until the commit is confirmed and the
      // list and details reflect it.
      await refresh();
      if (selectedID) await inspect({ install_id: selectedID } as ToolkitInstallation);
      setCandidate(null);
    } catch {
      const recovered = await showOperation(operationIDToPublish);
      if (recovered?.state === 'committed') {
        setError('');
        setNotice(t('plugin.vivy/toolkits-ui.catalogUpdated'));
        if (selectedID) await inspect({ install_id: selectedID } as ToolkitInstallation);
        setCandidate(null);
        return;
      }
      setError(t('plugin.vivy/toolkits-ui.publishUnknown'));
    } finally {
      setPending(null);
    }
  }

  async function cancelCandidate() {
    if (!candidate) return;
    const operationIDToCancel = candidate.operation.id;
    setPending({ action: 'cancel', key: operationIDToCancel });
    setError('');
    try {
      const result = await client.cancel({
        operation_id: operationIDToCancel,
        expected_operation_revision: operation?.revision ?? candidate.operation.revision,
      });
      setOperation(result);
      setCandidate(null);
      clearLastOperationID();
      setNotice(t('plugin.vivy/toolkits-ui.candidateCancelled'));
    } catch {
      const recovered = await showOperation(operationIDToCancel);
      if (recovered?.state === 'cancelled') {
        setError('');
        setNotice(t('plugin.vivy/toolkits-ui.candidateCancelled'));
        return;
      }
      setError(t('plugin.vivy/toolkits-ui.cancelUnknown'));
    } finally {
      setPending(null);
    }
  }

  async function changeConsent(kind: 'hook' | 'mcp', allow: boolean, componentID: string) {
    const catalog = page?.catalog;
    const item = selected?.installation;
    if (!catalog || !item || !selected || candidate) return;
    const identity = selected.executables.find((entry) => entry.kind === kind && entry.local_id === componentID);
    if (!identity) {
      setError(t('plugin.vivy/toolkits-ui.consentIdentityMissing'));
      return;
    }
    const current = selected.consents.find((entry) => entry.kind === kind && entry.local_id === componentID);
    const operationID = newOperationID();
    rememberOperationID(operationID);
    setPending({ action: 'consent', key: `${kind}/${componentID}` });
    setError('');
    try {
      const result = await client.consent(kind, allow, {
        operation_id: operationID,
        expected_catalog: catalog,
        install_id: item.install_id,
        component_id: componentID,
        release_digest: identity.release_digest,
        declaration_digest: identity.declaration_digest,
        expected_consent_revision: current?.revision ?? 0,
      });
      setOperation(result);
      clearLastOperationID();
      setNotice(allow ? t('plugin.vivy/toolkits-ui.consentGranted') : t('plugin.vivy/toolkits-ui.consentRevoked'));
      await inspect(item);
    } catch {
      const recovered = await showOperation(operationID);
      if (recovered?.state === 'committed') {
        clearLastOperationID();
        setError('');
        setNotice(allow ? t('plugin.vivy/toolkits-ui.consentGranted') : t('plugin.vivy/toolkits-ui.consentRevoked'));
        await inspect(item);
        return;
      }
      setError(t('plugin.vivy/toolkits-ui.consentUnknown'));
    } finally {
      setPending(null);
    }
  }

  async function readBody(kind: string, id: string, trigger: HTMLButtonElement) {
    if (!selected?.manifest || !selected.release.release_digest) return;
    bodyTriggerRef.current = trigger;
    try {
      const result = await client.componentContent({
        install_id: selected.installation.install_id,
        kind,
        local_id: id,
        expected_release_digest: selected.release.release_digest,
      });
      setBody({ title: `${kind}/${id}`, content: result.content, digest: result.digest });
    } catch {
      setError(t('plugin.vivy/toolkits-ui.componentReadError'));
    }
  }

  // Every mutation consumes the shared catalog identity (epoch + configuration
  // revision), so one in-flight mutation must finish before another starts.
  // Reads, selection and refresh stay available: only mutations serialize.
  const mutationsLocked = Boolean(candidate) || pending !== null;

  return (
    <main className="mx-auto w-full max-w-6xl space-y-6 p-4 sm:p-6 lg:p-8" data-testid="toolkit-page">
      {error ? (
        <div
          role="alert"
          className="flex flex-wrap items-start justify-between gap-3 rounded-xl border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive"
        >
          <p className="flex min-w-0 items-start gap-2">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
            <span className="break-words">{error}</span>
          </p>
          <Button type="button" size="sm" variant="outline" data-action="retry" onClick={() => void refresh()}>
            <RefreshCw className="h-3.5 w-3.5" aria-hidden />
            {t('plugin.vivy/toolkits-ui.retry')}
          </Button>
        </div>
      ) : null}

      {notice ? (
        <div role="status" className="rounded-xl border bg-muted/40 p-3 text-sm">
          {notice}
        </div>
      ) : null}

      {operation ? <OperationStrip operation={operation} t={t} /> : null}

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1.05fr)_minmax(0,0.95fr)]">
        <ToolkitLibrary
          items={installations}
          selectedId={selected?.installation.install_id ?? null}
          catalog={page?.catalog ?? null}
          firstLoad={firstLoading}
          refreshing={refreshing}
          mutationsLocked={mutationsLocked}
          onSelect={selectFromLibrary}
          onRefresh={() => void refresh()}
          onInstall={() => setInstallOpen(true)}
          t={t}
        />

        <section
          ref={detailsRef}
          aria-label={t('plugin.vivy/toolkits-ui.details')}
          data-testid="toolkit-detail"
          className="min-w-0 scroll-mt-4 space-y-6"
        >
          {candidate ? (
            <ToolkitCandidate
              candidate={candidate}
              target={candidateTarget(candidate, installations, selected)}
              pending={pending}
              onPublish={() => void publishCandidate()}
              onCancel={() => void cancelCandidate()}
              t={t}
            />
          ) : null}

          {!selected ? (
            <p className="rounded-xl border border-dashed p-8 text-sm text-muted-foreground">
              {t('plugin.vivy/toolkits-ui.selectToolkit')}
            </p>
          ) : (
            <>
              <ToolkitDetails
                key={selected.installation.install_id}
                inspection={selected}
                pending={pending}
                mutationsLocked={mutationsLocked}
                onPrepare={(mutation) => void prepareMutation(mutation)}
                onPrepareSource={(mutation, change) => void prepareSource(mutation, change)}
                onPrepareRollback={(digest) => void prepareRollback(digest)}
                t={t}
              />
              {selected.manifest ? (
                <ToolkitInventory
                  key={`inventory-${selected.installation.install_id}`}
                  manifest={selected.manifest}
                  inspection={selected}
                  owners={page?.capabilities.owners ?? {}}
                  pending={pending}
                  mutationsLocked={mutationsLocked}
                  onRead={(kind, id, trigger) => void readBody(kind, id, trigger)}
                  onConsent={(kind, allow, componentID) => void changeConsent(kind, allow, componentID)}
                  onSaveBindings={(kind, componentID, bindings) =>
                    void saveBindings(kind, componentID, bindings)
                  }
                  t={t}
                />
              ) : null}
            </>
          )}
        </section>
      </div>

      <ToolkitInstallDialog
        open={installOpen}
        catalog={page?.catalog ?? null}
        mutationsLocked={mutationsLocked}
        onClose={() => setInstallOpen(false)}
        onPreview={previewSource}
        onInstall={installPreview}
        t={t}
      />

      <ToolkitContentDialog
        body={body}
        triggerRef={bodyTriggerRef}
        onClose={() => setBody(null)}
        t={t}
      />
    </main>
  );
}

function OperationStrip({
  operation,
  t,
}: {
  readonly operation: ToolkitOperation;
  readonly t: UITranslator;
}) {
  return (
    <div className="space-y-1 rounded-xl border bg-muted/30 p-3 text-xs" data-testid="toolkit-operation">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-medium">{t('plugin.vivy/toolkits-ui.operationStatus')}</span>
        <Badge variant="outline">{operationStateLabel(t, operation.state)}</Badge>
        <span className="font-mono text-muted-foreground" title={operation.id}>
          {shortDigest(operation.id, 8)}
        </span>
        <span className="text-muted-foreground">
          {t('plugin.vivy/toolkits-ui.revision')} {operation.revision}
        </span>
      </div>
      {operation.error_code ? (
        <p className="break-all">
          {t('plugin.vivy/toolkits-ui.errorCode')}: <code className="font-mono">{operation.error_code}</code>
        </p>
      ) : null}
    </div>
  );
}

function catalogKey(catalog: ToolkitPageResult['catalog']): string {
  return `${catalog.epoch}/${catalog.config_revision}/${catalog.effective_digest}/${catalog.generation_id}`;
}

function readLastOperationID(): string {
  try {
    return sessionStorage.getItem(LAST_OPERATION_KEY) ?? '';
  } catch {
    return '';
  }
}

function rememberOperationID(id: string): void {
  try {
    sessionStorage.setItem(LAST_OPERATION_KEY, id);
  } catch {
    /* recovery hint is optional; backend stays authoritative */
  }
}

function clearLastOperationID(): void {
  try {
    sessionStorage.removeItem(LAST_OPERATION_KEY);
  } catch {
    /* ignore storage policy */
  }
}
