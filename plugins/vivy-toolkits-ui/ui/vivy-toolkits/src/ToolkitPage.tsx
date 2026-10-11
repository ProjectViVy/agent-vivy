import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { usePluginTranslation } from '@vivy/ui-sdk';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ToolkitClient, newOperationID, type ExecutableInspection, type PreparedCandidate, type ToolkitInstallation, type ToolkitInspection, type ToolkitOperation, type ToolkitPageResult, type ToolkitPreview } from './toolkit-client';

const LAST_OPERATION_KEY = 'vivy.toolkits.last-operation-id';
type SourceKind = 'local' | 'git';

export function ToolkitPage({ client }: { readonly client: ToolkitClient }) {
  const { t } = usePluginTranslation();
  const tx = (key: string, fallback: string) => toolkitText(t, key, fallback);
  const [page, setPage] = useState<ToolkitPageResult | null>(null);
  const [installations, setInstallations] = useState<readonly ToolkitInstallation[]>([]);
  const [selected, setSelected] = useState<ToolkitInspection | null>(null);
  const [preview, setPreview] = useState<ToolkitPreview | null>(null);
  const [body, setBody] = useState<{ readonly title: string; readonly content: string; readonly digest: string } | null>(null);
  const bodyTriggerRef = useRef<HTMLButtonElement | null>(null);
  const [candidate, setCandidate] = useState<PreparedCandidate | null>(null);
  const [operation, setOperation] = useState<ToolkitOperation | null>(null);
  const [sourceKind, setSourceKind] = useState<SourceKind>('local');
  const [sourceLocation, setSourceLocation] = useState('');
  const [sourceRef, setSourceRef] = useState('main');
  const [rollbackDigest, setRollbackDigest] = useState('');
  const [loading, setLoading] = useState(true);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const requestSequence = useRef(0);
  const detailSequence = useRef(0);

  const refresh = useCallback(async () => {
    const sequence = ++requestSequence.current;
    setLoading(true);
    setError('');
    try {
      const items: ToolkitInstallation[] = [];
      let after = '';
      let first: ToolkitPageResult | null = null;
      for (let pageIndex = 0; pageIndex < 100; pageIndex += 1) {
        const next = await client.list({ after_id: after, limit: 100 });
        if (first && catalogKey(first.catalog) !== catalogKey(next.catalog)) {
          throw new Error('Toolkit catalog changed while paging.');
        }
        first ??= next;
        items.push(...next.items);
        if (!next.next_after_id || next.next_after_id === after) break;
        after = next.next_after_id;
      }
      if (sequence === requestSequence.current) {
        setPage(first ?? null);
        setInstallations(items);
        setLoading(false);
      }
    } catch {
      if (sequence === requestSequence.current) {
        setLoading(false);
        setError(toolkitText(t,'catalogReadError','无法读取工具包目录。请检查连接后重试。'));
      }
    }
  }, [client, t]);

  const setInspection = useCallback((detail: ToolkitInspection) => {
    setSelected(detail);
    const source = detail.installation.source;
    if (source.kind === 'local' || source.kind === 'git') setSourceKind(source.kind);
    setSourceLocation(source.origin);
    setSourceRef(source.requested_ref || 'main');
    const priorRelease = detail.releases.find((release) => release.release_digest !== detail.installation.release_digest);
    setRollbackDigest(priorRelease?.release_digest ?? '');
  }, []);

  const inspect = useCallback(async (item: ToolkitInstallation) => {
    const sequence = ++detailSequence.current;
    setError('');
    try {
      const detail = await client.inspect(item.install_id);
      if (sequence === detailSequence.current && detail.installation.install_id === item.install_id) setInspection(detail);
    } catch {
      if (sequence === detailSequence.current) setError(toolkitText(t,'detailReadError','无法读取这个工具包的详情。请刷新后重试。'));
    }
  }, [client, setInspection, t]);

  const showOperation = useCallback(async (operationID: string): Promise<ToolkitOperation | null> => {
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
        setNotice(tx('operationPreparing','操作仍在准备中，候选信息尚未返回。目录状态以服务端为准。'));
      }
      return latest;
    } catch {
      // A session-storage value is only a recovery hint. The backend remains
      // the authority; an unavailable receipt is shown as unknown.
      setNotice(tx('operationRecovery','上次操作暂时无法恢复。目录仍以服务端状态为准。'));
      return null;
    }
  }, [client, refresh, t]);

  useEffect(() => {
    void refresh();
    const onFocus = () => { void refresh(); };
    window.addEventListener('focus', onFocus);
    const operationID = readLastOperationID();
    if (operationID) void showOperation(operationID);
    return () => {
      requestSequence.current += 1;
      window.removeEventListener('focus', onFocus);
    };
  }, [refresh, showOperation]);

  async function previewSource() {
    if (!page?.catalog || !sourceLocation.trim() || candidate) return;
    setWorking(true); setError(''); setNotice('');
    try {
      const result = await client.preview(sourceKind, {
        operation_id: newOperationID(), expected_catalog: page.catalog,
        location: sourceLocation.trim(), ...(sourceKind === 'git' ? { ref: sourceRef.trim() || 'HEAD' } : {}),
      });
      setPreview(result); setNotice(tx('previewReady','已验证工具包内容。检查来源和摘要后再安装。'));
    } catch {
      setError(tx('previewFailed','预览失败。请检查主机路径或 Git 来源，并重试。'));
    } finally { setWorking(false); }
  }

  async function installPreview() {
    if (!preview || !page?.catalog || candidate) return;
    const operationID = newOperationID();
    rememberOperationID(operationID);
    setWorking(true); setError(''); setNotice('');
    try {
      const result = await client.install({ operation_id: operationID, expected_catalog: page.catalog, preview_operation_id: preview.operation_id, expected_release_digest: preview.release_digest });
      setOperation(result); setCandidate(null); setPreview(null); setNotice(tx('installReady','工具包已安装为停用状态。启用和可执行权限需要分别确认。'));
      clearLastOperationID(); await refresh();
      const detail = await client.inspect(preview.install_id);
      setInspection(detail);
    } catch {
      const recovered = await showOperation(operationID);
      if (recovered?.state === 'committed') {
        setError(''); setNotice(tx('installReady','工具包已安装为停用状态。启用和可执行权限需要分别确认。'));
        const detail = await client.inspect(preview.install_id); setInspection(detail); return;
      }
      setError(tx('installUnknown','安装结果尚未确认。已尝试按同一操作编号向服务端核实；请查看操作状态后再继续。'));
    } finally { setWorking(false); }
  }

  async function prepareMutation(kind: 'enable' | 'disable' | 'uninstall', item: ToolkitInstallation) {
    const catalog = page?.catalog;
    if (!catalog || candidate) return;
    const operationID = newOperationID();
    rememberOperationID(operationID); setWorking(true); setError(''); setNotice('');
    try {
      const result = await client.prepare(kind, { operation_id: operationID, expected_catalog: catalog, install_id: item.install_id, expected_install_revision: item.revision });
      setCandidate(result); setOperation(result.operation); setNotice(tx('candidateReady','候选变更已准备。检查后发布才会生效。'));
    } catch {
      const recovered = await showOperation(operationID);
      if (recovered?.state === 'pending' && recovered.prepared) return;
      setError(tx('mutationFailed','无法准备这项变更。请检查工具包状态和当前操作。'));
    } finally { setWorking(false); }
  }

  async function prepareSource(operationKind: 'update' | 'rebind') {
    if (!selected || !page?.catalog || !sourceLocation.trim() || candidate) return;
    const operationID = newOperationID();
    const previewOperationID = newOperationID();
    rememberOperationID(operationID);
    setWorking(true); setError(''); setNotice('');
    try {
      const result = await client.prepareSource(sourceKind, operationKind, {
        operation_id: operationID,
        preview_operation_id: previewOperationID,
        expected_catalog: page.catalog,
        install_id: selected.installation.install_id,
        expected_install_revision: selected.installation.revision,
        location: sourceLocation.trim(),
        ...(sourceKind === 'git' ? { ref: sourceRef.trim() || 'HEAD' } : {}),
      });
      setCandidate(result); setOperation(result.operation);
      setNotice(tx('candidateReady', '已准备候选变更。检查差异后发布才会生效。'));
    } catch {
      const recovered = await showOperation(operationID);
      if (recovered?.state === 'pending' && recovered.prepared) return;
      setError(tx('sourcePrepareFailed', '无法准备来源变更。请检查来源、工具包状态和当前操作。'));
    } finally { setWorking(false); }
  }

  async function prepareRollback() {
    if (!selected || !page?.catalog || !rollbackDigest || candidate) return;
    const operationID = newOperationID();
    rememberOperationID(operationID); setWorking(true); setError(''); setNotice('');
    try {
      const result = await client.prepare('rollback', {
        operation_id: operationID, expected_catalog: page.catalog,
        install_id: selected.installation.install_id,
        expected_install_revision: selected.installation.revision,
        release_digest: rollbackDigest,
      });
      setCandidate(result); setOperation(result.operation);
      setNotice(tx('rollbackReady', '已准备回滚候选。检查差异后发布才会生效。'));
    } catch {
      const recovered = await showOperation(operationID);
      if (recovered?.state === 'pending' && recovered.prepared) return;
      setError(tx('rollbackFailed', '无法准备回滚。请重新检查保留版本和工具包状态。'));
    } finally { setWorking(false); }
  }

  async function saveBindings(kind: 'hook' | 'mcp', componentID: string, bindings: readonly { readonly name: string; readonly reference: string }[]) {
    if (!selected || !page?.catalog || candidate) return;
    const operationID = newOperationID();
    rememberOperationID(operationID);
    setWorking(true); setError(''); setNotice('');
    try {
      const result = await client.setBindings({
        operation_id: operationID,
        expected_catalog: page.catalog,
        expected_config_revision: page.catalog.config_revision,
        install_id: selected.installation.install_id,
        kind, component_id: componentID, bindings,
      });
      setOperation(result);
      clearLastOperationID();
      setNotice(tx('bindingsSaved', '主机环境变量引用已保存。相关 Hook/MCP 授权需要重新确认。'));
      const updated = await client.inspect(selected.installation.install_id);
      setInspection(updated);
      await refresh();
    } catch {
      const recovered = await showOperation(operationID);
      if (recovered?.state === 'committed') {
        const updated = await client.inspect(selected.installation.install_id); setInspection(updated);
        setError(''); setNotice(tx('bindingsSaved', '主机环境变量引用已保存。相关 Hook/MCP 授权需要重新确认。')); return;
      }
      setError(tx('bindingsConflict', '绑定未保存。配置可能已在其他位置更新；表单内容已保留，请刷新后核对。'));
    } finally { setWorking(false); }
  }

  async function publishCandidate() {
    if (!candidate) return;
    setWorking(true); setError(''); setNotice('');
    try {
      const result = await client.publish({ operation_id: candidate.operation.id, candidate_digest: candidate.candidate_digest, expected_catalog: candidate.expected_catalog });
      setOperation(result); setCandidate(null); clearLastOperationID(); setNotice(tx('catalogUpdated','工具包目录已更新。'));
      const selectedID = selected?.installation.install_id;
      await refresh();
      if (selectedID) await inspect({ install_id: selectedID } as ToolkitInstallation);
    } catch {
      const recovered = await showOperation(candidate.operation.id);
      if (recovered?.state === 'committed') {
        setError(''); setNotice(tx('catalogUpdated','工具包目录已更新。'));
        const selectedID = selected?.installation.install_id;
        if (selectedID) await inspect({ install_id: selectedID } as ToolkitInstallation);
        return;
      }
      setError(tx('publishUnknown','发布结果尚未确认。请核对操作状态；不要重复创建新的变更。'));
    } finally { setWorking(false); }
  }

  async function cancelCandidate() {
    if (!candidate) return;
    setWorking(true); setError('');
    try {
      const result = await client.cancel({ operation_id: candidate.operation.id, expected_operation_revision: operation?.revision ?? candidate.operation.revision });
      setOperation(result); setCandidate(null); clearLastOperationID(); setNotice(tx('candidateCancelled','待发布变更已取消。'));
    } catch {
      const recovered = await showOperation(candidate.operation.id);
      if (recovered?.state === 'cancelled') { setError(''); setNotice(tx('candidateCancelled','待发布变更已取消。')); return; }
      setError(tx('cancelUnknown','无法确认取消结果。请检查操作状态后再决定是否继续。'));
    }
    finally { setWorking(false); }
  }

  async function changeConsent(kind: 'hook' | 'mcp', allow: boolean, item: ToolkitInstallation, componentID: string) {
    if (!page?.catalog || !selected || candidate) return;
    const identity = selected.executables.find((entry) => entry.kind === kind && entry.local_id === componentID);
    if (!identity) { setError(tx('consentIdentityMissing','缺少已检查的可执行声明身份，无法提交权限变更。')); return; }
    const current = selected.consents.find((entry) => entry.kind === kind && entry.local_id === componentID);
    const operationID = newOperationID(); rememberOperationID(operationID); setWorking(true); setError('');
    try {
      const result = await client.consent(kind, allow, {
        operation_id: operationID, expected_catalog: page.catalog, install_id: item.install_id,
        component_id: componentID, release_digest: identity.release_digest,
        declaration_digest: identity.declaration_digest, expected_consent_revision: current?.revision ?? 0,
      });
      setOperation(result); clearLastOperationID(); setNotice(allow ? tx('consentGranted','权限已记录。运行时还会检查绑定和已编译能力。') : tx('consentRevoked','权限已撤销。'));
      await inspect(item);
    } catch {
      const recovered = await showOperation(operationID);
      if (recovered?.state === 'committed') {
        clearLastOperationID(); setError(''); setNotice(allow ? tx('consentGranted','权限已记录。运行时还会检查绑定和已编译能力。') : tx('consentRevoked','权限已撤销。'));
        await inspect(item); return;
      }
      setError(tx('consentUnknown','权限状态尚未确认。请重新检查工具包后再决定是否重试。'));
    } finally { setWorking(false); }
  }

  async function readBody(kind: string, id: string, trigger: HTMLButtonElement) {
    if (!selected?.manifest || !selected.release.release_digest) return;
    bodyTriggerRef.current = trigger;
    try {
      const result = await client.componentContent({ install_id: selected.installation.install_id, kind, local_id: id, expected_release_digest: selected.release.release_digest });
      setBody({ title: `${kind}/${id}`, content: result.content, digest: result.digest });
    } catch { setError(tx('componentReadError','无法读取组件内容，可能该版本已更新。请重新打开详情。')); }
  }

  return <main className="mx-auto grid max-w-6xl gap-6 p-4 md:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)] md:p-8" data-testid="toolkit-page">
    <section className="space-y-5" aria-label={tx('library', '工具包库')}>
      <header className="flex items-center justify-between gap-3">
        <div><h1 className="text-2xl font-semibold">{tx('title', '工具包')}</h1><p className="text-sm text-muted-foreground">{tx('subtitle', '检查来源、组件和运行时能力；启用与可执行授权分别管理。')}</p></div>
        <button type="button" className="min-h-10 rounded-md border px-3 py-2 text-sm" onClick={() => void refresh()} disabled={loading}>{tx('refresh', '刷新')}</button>
      </header>
      {page && <p className="text-xs text-muted-foreground" data-testid="catalog-epoch">{tx('catalogEpoch','目录 epoch')} {page.catalog.epoch} · {tx('configRevision','配置修订')} {page.catalog.config_revision}</p>}
      {error && <p role="alert" className="rounded-md border border-destructive p-3 text-sm">{error}</p>}
      {notice && <pre role="status" className="whitespace-pre-wrap rounded-md border p-3 text-sm">{notice}</pre>}
      {loading && <p role="status">{tx('loading', '正在读取工具包…')}</p>}
      {!loading && installations.length === 0 && <p className="rounded-lg border border-dashed p-6 text-sm">{tx('empty', '还没有安装工具包。使用下面的表单预览主机目录或 Git 来源。')}</p>}
      <ul className="space-y-2" aria-label={tx('installed', '已安装工具包')}>
        {installations.map((item) => <li key={item.install_id}>
          <button type="button" className={`w-full rounded-lg border p-3 text-left ${selected?.installation.install_id === item.install_id ? 'border-primary' : ''}`} disabled={working||Boolean(candidate)} onClick={() => void inspect(item)}>
            <span className="block font-medium">{item.bundle_id}</span>
            <span className="mt-1 block text-xs text-muted-foreground">{item.state} · {item.source.kind}: {item.source.origin}</span>
          </button>
        </li>)}
      </ul>
      <section className="space-y-3 rounded-xl border p-4" aria-labelledby="toolkit-source-title">
        <h2 id="toolkit-source-title" className="font-semibold">{tx('sourceTitle', '预览并安装')}</h2>
        <label className="block text-sm">{tx('sourceKind', '来源类型')}
          <select className="mt-1 block w-full rounded-md border bg-background p-2" value={sourceKind} onChange={(event) => { setSourceKind(event.target.value as SourceKind); setPreview(null); }}>
            <option value="local">{tx('localSource', '主机上的本地目录')}</option><option value="git">{tx('gitSource', 'Git 仓库')}</option>
          </select>
        </label>
        <label className="block text-sm">{sourceKind === 'local' ? tx('localPath', '主机目录路径') : tx('gitAddress', 'Git 仓库地址')}
          <input className="mt-1 block w-full rounded-md border bg-background p-2" value={sourceLocation} onChange={(event) => setSourceLocation(event.target.value)} autoComplete="off" />
        </label>
        {sourceKind === 'git' && <label className="block text-sm">{tx('gitRef', '分支、标签或提交')}
          <input className="mt-1 block w-full rounded-md border bg-background p-2" value={sourceRef} onChange={(event) => setSourceRef(event.target.value)} autoComplete="off" />
        </label>}
        <p className="text-xs text-muted-foreground">{tx('hostPathHint', '本地路径由运行 Vivy 的主机读取。浏览器文件选择和上传不会被当作主机目录。')}</p>
        <button type="button" className="min-h-10 rounded-md border px-3 py-2 text-sm" disabled={working || Boolean(candidate) || loading || !sourceLocation.trim()} onClick={() => void previewSource()}>{tx('previewSource', '预览来源')}</button>
        {preview && <div className="space-y-2 rounded-lg bg-muted/40 p-3" data-testid="toolkit-preview">
          <h3 className="font-medium">{preview.manifest.name} · {preview.manifest.version}</h3>
          <p className="break-all text-xs">{preview.source.kind}: {preview.source.origin}</p>
          <p className="break-all text-xs">Release SHA-256: {preview.release_digest}</p>
          <ComponentCounts manifest={preview.manifest} />
          <button type="button" className="min-h-10 rounded-md bg-primary px-3 py-2 text-sm text-primary-foreground" disabled={working||Boolean(candidate)} onClick={() => void installPreview()}>{tx('installDisabled', '安装为停用状态')}</button>
        </div>}
      </section>
      {operation && <section className="space-y-2 rounded-xl border p-4" data-testid="toolkit-operation">
        <h2 className="font-semibold">{tx('operationStatus', '操作状态')}</h2><p>{operation.id} · {operation.kind} · {operation.state} · {tx('revision', '修订')} {operation.revision}</p>
        {operation.error_code && <p role="alert">{operation.error_code}</p>}
        {operation.blockers?.length > 0 && <ul>{operation.blockers.map((blocker) => <li key={blocker.run_id}>{blocker.run_id} · {blocker.status}</li>)}</ul>}
      </section>}
    </section>

    <section className="space-y-4" aria-label={tx('details', '工具包详情')}>
      {!selected && <div className="rounded-xl border border-dashed p-8 text-sm text-muted-foreground">{tx('selectToolkit', '选择一个工具包查看详情。')}</div>}
      {selected && <>
        <header className="rounded-xl border p-4">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div><h2 className="text-xl font-semibold">{selected.manifest?.name ?? selected.installation.bundle_id}</h2><p className="text-sm text-muted-foreground">{selected.installation.bundle_id} · {selected.installation.state}</p></div>
            <div className="flex flex-wrap gap-2">
              {selected.installation.state === 'disabled' && <button type="button" className="min-h-10 rounded-md bg-primary px-3 py-2 text-sm text-primary-foreground" disabled={working||Boolean(candidate)} onClick={() => void prepareMutation('enable', selected.installation)}>{tx('prepareEnable', '准备启用')}</button>}
              {selected.installation.state === 'enabled' && <button type="button" className="min-h-10 rounded-md border px-3 py-2 text-sm" disabled={working||Boolean(candidate)} onClick={() => void prepareMutation('disable', selected.installation)}>{tx('prepareDisable', '准备停用')}</button>}
              {selected.installation.state === 'disabled' && <button type="button" className="min-h-10 rounded-md border px-3 py-2 text-sm" disabled={working||Boolean(candidate)} onClick={() => void prepareMutation('uninstall', selected.installation)}>{tx('prepareUninstall', '准备卸载')}</button>}
            </div>
          </div>
          <p className="mt-3 break-all text-xs">{selected.installation.source.kind}: {selected.installation.source.origin}</p>
          <p className="break-all text-xs">Release SHA-256: {selected.installation.release_digest}</p>
          {selected.manifest?.description && <p className="mt-2 text-sm">{selected.manifest.description}</p>}
        </header>
        <section className="space-y-3 rounded-xl border p-4" aria-labelledby="toolkit-source-change-title">
          <h3 id="toolkit-source-change-title" className="font-semibold">{tx('sourceChangeTitle', '更新或重新绑定来源')}</h3>
          <p className="text-xs text-muted-foreground">{tx('sourceChangeHint', '上方来源字段已载入当前来源。更新会准备新版本；重新绑定只会修复来源记录。检查候选差异后再发布。')}</p>
          <div className="flex flex-wrap gap-2">
            <button type="button" className="min-h-10 rounded-md border px-3 py-2 text-sm" disabled={working || Boolean(candidate) || !sourceLocation.trim()} onClick={() => void prepareSource('update')}>{tx('prepareUpdate', '准备更新')}</button>
            <button type="button" className="min-h-10 rounded-md border px-3 py-2 text-sm" disabled={working || Boolean(candidate) || !sourceLocation.trim()} onClick={() => void prepareSource('rebind')}>{tx('prepareRebind', '预览并重新绑定来源')}</button>
          </div>
          {selected.releases.some((release) => release.release_digest !== selected.installation.release_digest) && <div className="flex flex-wrap items-end gap-2 border-t pt-3">
            <label className="min-w-0 flex-1 text-sm">{tx('rollbackRelease', '回滚到保留版本')}
              <select className="mt-1 block min-h-10 w-full rounded-md border bg-background p-2" value={rollbackDigest} onChange={(event) => setRollbackDigest(event.target.value)}>
                {selected.releases.filter((release) => release.release_digest !== selected.installation.release_digest).map((release) => <option key={release.release_digest} value={release.release_digest}>#{release.release_sequence} · {release.release_digest.slice(0, 12)}</option>)}
              </select>
            </label>
            <button type="button" className="min-h-10 rounded-md border px-3 py-2 text-sm" disabled={working || Boolean(candidate) || !rollbackDigest} onClick={() => void prepareRollback()}>{tx('prepareRollback', '准备回滚')}</button>
          </div>}
        </section>
        {candidate && <section className="space-y-2 rounded-xl border border-primary p-4" data-testid="prepared-candidate">
          <h3 className="font-semibold">{tx('candidateTitle', '待审核候选')}</h3><p className="break-all text-xs">{tx('candidateDigest', '候选摘要')}: {candidate.candidate_digest}</p><p className="text-xs">{tx('basedOnEpoch', '基于目录 epoch')} {candidate.expected_catalog.epoch}；{tx('operationID', '操作 ID')} {candidate.operation.id}</p>
          {candidate.operation.prepared?.changes && candidate.operation.prepared.changes.length > 0 ? <div><h4 className="text-sm font-medium">{tx('changeList', '组件差异')}</h4><ul className="mt-1 space-y-1 text-xs">{candidate.operation.prepared.changes.map((change) => <li key={`${change.kind}/${change.local_id}/${change.change}`} className="break-all">{change.kind}/{change.local_id}: {change.change}{change.before_digest||change.after_digest?` · ${(change.before_digest??'∅').slice(0,12)} → ${(change.after_digest??'∅').slice(0,12)}`:''}{change.requires_fresh_consent ? ` · ${tx('freshConsent', '需要重新确认授权')}` : ''}{change.kind==='mask'&&change.change==='removed'?<span> · {tx('maskRemovalHint','若会话正在使用此面具，请在面具管理中重新选择。')} <a className="text-primary underline" href="/masks">{tx('openMasks','面具管理')}</a></span>:null}</li>)}</ul></div> : <p className="text-xs text-muted-foreground">{tx('noComponentChanges', '本次候选未报告组件内容差异。')}</p>}
          {candidate.operation.blockers?.length > 0 && <div><h4 className="text-sm font-medium">{tx('blockers', '等待中的运行')}</h4><ul className="text-xs">{candidate.operation.blockers.map((blocker) => <li key={blocker.run_id}>{blocker.run_id} · {blocker.session_id} · {blocker.status}</li>)}</ul><p className="text-xs text-muted-foreground">{tx('blockerHint', '运行完成前不会自动取消或发布此变更。')}</p></div>}
          <div className="flex flex-wrap gap-2"><button type="button" className="min-h-10 rounded-md bg-primary px-3 py-2 text-sm text-primary-foreground" disabled={working} onClick={() => void publishCandidate()}>{tx('publish', '发布变更')}</button><button type="button" className="min-h-10 rounded-md border px-3 py-2 text-sm" disabled={working} onClick={() => void cancelCandidate()}>{tx('cancelCandidate', '取消候选')}</button></div>
        </section>}
        {selected.manifest && <div className="space-y-4">
          <ComponentSection title={tx('skills','Skills')} available={page?.capabilities.owners.skill !== false} empty={tx('emptySkills','没有 Skill。')} count={selected.manifest.components.skills.length}>
            <p className="text-xs"><a className="text-primary underline-offset-4 hover:underline" href="/skills">{tx('openSkills','在 Skills 管理中查看和使用')}</a></p>
            {selected.manifest.components.skills.map((entry) => <ComponentRow key={entry.id} id={entry.id} kind="skill" onRead={(trigger) => void readBody('skill',entry.id,trigger)} />)}
          </ComponentSection>
          <ComponentSection title={tx('masks','面具')} available={page?.capabilities.owners.mask !== false} empty={tx('emptyMasks','没有面具。')} count={selected.manifest.components.masks.length}>
            <p className="text-xs"><a className="text-primary underline-offset-4 hover:underline" href="/masks">{tx('openMasks', '在面具管理中查看来源、复制和使用')}</a></p>
            {selected.manifest.components.masks.map((entry) => <div key={entry.id} className="space-y-1"><ComponentRow id={entry.id} label={entry.name} kind="mask" onRead={(trigger) => void readBody('mask',entry.id,trigger)} />{entry.description && <p className="pl-3 text-xs text-muted-foreground">{entry.description}</p>}{entry.recommendedSkills.length > 0 && <p className="pl-3 text-xs">{tx('recommendedSkills', '推荐 Skill')}：{entry.recommendedSkills.join(', ')}（{tx('recommendationOnly', '不会自动启用')}）</p>}</div>)}
          </ComponentSection>
          <ComponentSection title={tx('rules','Rules')} available={page?.capabilities.owners.rule !== false} empty={tx('emptyRules','没有规则。')} count={selected.manifest.components.rules.length}>
            {selected.manifest.components.rules.map((entry) => <ComponentRow key={entry.id} id={entry.id} kind="rule" onRead={(trigger) => void readBody('rule',entry.id,trigger)} />)}
          </ComponentSection>
          <ComponentSection title={tx('hooks','Hooks')} available={page?.capabilities.owners.hook !== false} empty={tx('emptyHooks','没有 Hook。')} count={selected.manifest.components.hooks.length}>
            {selected.manifest.components.hooks.map((entry) => <ExecutableRow key={entry.id} kind="hook" id={entry.id} detail={entry.event} item={selected.installation} inspection={selected} working={working||Boolean(candidate)} available={page?.capabilities.owners.hook !== false} onConsent={(allow) => void changeConsent('hook',allow,selected.installation,entry.id)} onSaveBindings={(bindings) => void saveBindings('hook',entry.id,bindings)} slots={Object.keys(entry.envFrom ?? {}).map((target) => `envFrom:${target}`)} />)}
          </ComponentSection>
          <ComponentSection title={tx('mcp','MCP')} available={page?.capabilities.owners.mcp !== false} empty={tx('emptyMcp','没有 MCP 服务。')} count={selected.manifest.components.mcp.length}>
            {selected.manifest.components.mcp.map((entry) => <ExecutableRow key={entry.id} kind="mcp" id={entry.id} detail={entry.endpoint || entry.command || ''} item={selected.installation} inspection={selected} working={working||Boolean(candidate)} available={page?.capabilities.owners.mcp !== false} onConsent={(allow) => void changeConsent('mcp',allow,selected.installation,entry.id)} onSaveBindings={(bindings) => void saveBindings('mcp',entry.id,bindings)} slots={[...(entry.authEnv ? ['authEnv'] : []), ...Object.keys(entry.envFrom ?? {}).map((target) => `envFrom:${target}`)]} />)}
          </ComponentSection>
        </div>}
      </>}
    </section>
    {body && <Dialog open onOpenChange={(open)=>{if(!open)setBody(null);}}>
      <DialogContent className="max-h-[85vh] max-w-3xl" onCloseAutoFocus={(event) => { event.preventDefault(); bodyTriggerRef.current?.focus(); }}>
        <DialogHeader><DialogTitle>{tx('componentContent', '组件内容')}: {body.title}</DialogTitle><DialogDescription className="break-all">SHA-256 {body.digest}</DialogDescription></DialogHeader>
        <DialogBody><pre className="whitespace-pre-wrap break-words rounded-md bg-muted/40 p-3 text-xs">{body.content}</pre></DialogBody>
      </DialogContent>
    </Dialog>}
  </main>;
}

function ComponentCounts({ manifest }: { readonly manifest: ToolkitPreview['manifest'] }) {
  const { t } = usePluginTranslation();
  return <p className="text-xs text-muted-foreground">{toolkitText(t,'skills','Skills')} {manifest.components.skills.length} · {toolkitText(t,'masks','面具')} {manifest.components.masks.length} · {toolkitText(t,'rules','Rules')} {manifest.components.rules.length} · {toolkitText(t,'hooks','Hooks')} {manifest.components.hooks.length} · {toolkitText(t,'mcp','MCP')} {manifest.components.mcp.length}</p>;
}
function ComponentSection({ title, count, empty, available, children }: { readonly title:string;readonly count:number;readonly empty:string;readonly available:boolean;readonly children:ReactNode }) {
  const { t } = usePluginTranslation();
  return <section className="space-y-2 rounded-xl border p-4"><div className="flex justify-between gap-2"><h3 className="font-semibold">{title}</h3><span className="text-xs text-muted-foreground">{available?`${count} ${toolkitText(t,'items','项')}`:toolkitText(t,'ownerUnavailable','当前构建未包含此组件能力')}</span></div>{!available?<p className="text-sm text-muted-foreground">{toolkitText(t,'stagedComponent','组件会保留在工具包中，但当前构建暂不可用。')}</p>:count===0?<p className="text-sm text-muted-foreground">{empty}</p>:<div className="space-y-2">{children}</div>}</section>;
}
function ComponentRow({ id, kind, label, onRead }: { readonly id:string;readonly kind:string;readonly label?:string;readonly onRead:(trigger:HTMLButtonElement)=>void }) {
  const { t } = usePluginTranslation();
  return <div className="flex items-center justify-between gap-2 rounded-md bg-muted/40 px-3 py-2"><span className="min-w-0 truncate text-sm">{label??id} <span className="text-xs text-muted-foreground">{kind}/{id}</span></span><button type="button" className="min-h-10 shrink-0 rounded-md border px-2 py-1 text-xs" onClick={(event) => onRead(event.currentTarget)}>{toolkitText(t,'readContent','查看内容')}</button></div>;
}
function ExecutableRow({ kind,id,detail,item,inspection,working,available,onConsent,onSaveBindings,slots }: { readonly kind:'hook'|'mcp';readonly id:string;readonly detail:string;readonly item:ToolkitInstallation;readonly inspection:ToolkitInspection;readonly working:boolean;readonly available:boolean;readonly onConsent:(allow:boolean)=>void;readonly onSaveBindings:(bindings:readonly {readonly name:string;readonly reference:string}[])=>void;readonly slots:readonly string[] }) {
  const { t } = usePluginTranslation();
  const executable=inspection.executables.find((entry)=>entry.kind===kind&&entry.local_id===id);
  const consent=inspection.consents.find((entry)=>entry.kind===kind&&entry.local_id===id);
  const recorded=Boolean(consent?.allowed ?? executable?.consent_allowed);
  const current=Boolean(executable?.consent_current&&executable.consent_allowed);
  const componentBindings=inspection.bindings.filter((binding)=>binding.kind===kind&&binding.local_id===id);
  const status=current?toolkitText(t,'consentCurrent','已确认（当前声明）'):recorded?toolkitText(t,'consentStale','旧授权已失效，需重新确认') : toolkitText(t,'consentMissing','未确认');
  return <div className="space-y-3 rounded-md bg-muted/40 p-3"><div><p className="text-sm font-medium">{id} · {status}</p><p className="break-all text-xs text-muted-foreground">{detail}</p></div>
    {executable?.missing_bindings.length ? <p className="text-xs text-amber-700 dark:text-amber-300">{toolkitText(t,'missingBindings','缺少主机环境变量绑定')}: {executable.missing_bindings.join(', ')}</p> : null}
    {kind==='mcp'&&<p className="text-xs">{toolkitText(t,'mcpRuntime','运行状态')}: {mcpRuntimeLabel(t,executable?.runtime_state??'inactive')}{executable?.runtime_initialized&&typeof executable.runtime_tool_count==='number'&&executable.runtime_tool_count>=0?` · ${toolkitText(t,'mcpToolCount','工具数')} ${executable.runtime_tool_count}`:''}</p>}
    {slots.length > 0 && <BindingsEditor key={`${kind}/${id}/${slots.join('|')}/${bindingSignature(componentBindings)}`} kind={kind} id={id} slots={slots} existing={componentBindings} disabled={working||!available} onSave={onSaveBindings} />}
    <div className="flex flex-wrap gap-2">
      <button type="button" className="min-h-10 rounded-md border px-3 py-1.5 text-xs" disabled={working||!available||item.state!=='enabled'} onClick={()=>onConsent(current?false:true)}>{current?toolkitText(t,'revokeConsent','撤销授权'):recorded?toolkitText(t,'reauthorizeConsent','重新确认授权'):toolkitText(t,'confirmConsent','确认授权')}</button>
      {recorded&&!current&&<button type="button" className="min-h-10 rounded-md border px-3 py-1.5 text-xs" disabled={working||!available||item.state!=='enabled'} onClick={()=>onConsent(false)}>{toolkitText(t,'revokeStaleConsent','撤销旧授权')}</button>}
    </div>
    {item.state!=='enabled'&&<p className="text-xs text-muted-foreground">{toolkitText(t,'enableBeforeConsent','工具包停用时不可授权。授权仍需在启用后单独确认。')}</p>}
    {kind==='mcp'&&<p className="text-xs text-muted-foreground">{toolkitText(t,'mcpHealthHint','运行状态只反映此前的连接操作；查看此页不会主动连接服务器。')}</p>}
  </div>;
}
function BindingsEditor({ kind,id,slots,existing,disabled,onSave }: { readonly kind:'hook'|'mcp';readonly id:string;readonly slots:readonly string[];readonly existing:ToolkitInspection['bindings'];readonly disabled:boolean;readonly onSave:(bindings:readonly {readonly name:string;readonly reference:string}[])=>void }) {
  const { t } = usePluginTranslation();
  const [values,setValues] = useState<Record<string,string>>(() => {
    const refs = new Map(existing.map((binding)=>[binding.name,binding.reference]));
    return Object.fromEntries(slots.map((slot)=>[slot,refs.get(slot)??'']));
  });
  return <form className="space-y-2 rounded-md border bg-background p-3" aria-label={`${toolkitText(t,'bindings','环境变量绑定')} · ${kind}/${id}`} onSubmit={(event)=>{event.preventDefault();onSave(slots.map((name)=>({name,reference:(values[name]??'').trim()})).filter((binding)=>binding.reference!==''));}}>
    <div><h4 className="text-sm font-medium">{toolkitText(t,'bindings','环境变量绑定')}</h4><p className="text-xs text-muted-foreground">{toolkitText(t,'bindingHint','只填主机环境变量名，不要输入密钥值。留空会清除此绑定。')}</p></div>
    {slots.map((slot)=>{const inputID=`binding-${kind}-${id}-${slot.replace(/[^a-zA-Z0-9_-]/g,'-')}`;return <label key={slot} htmlFor={inputID} className="block text-xs">{slot}<input id={inputID} className="mt-1 block min-h-10 w-full rounded-md border bg-background px-3 py-2 text-sm" autoComplete="off" spellCheck={false} pattern="[A-Za-z_][A-Za-z0-9_]*" placeholder="HOST_ENV_NAME" value={values[slot]??''} onChange={(event)=>setValues((current)=>({...current,[slot]:event.target.value}))} /></label>;})}
    <button type="submit" className="min-h-10 rounded-md border px-3 py-2 text-xs" disabled={disabled}>{toolkitText(t,'saveBindings','保存绑定')}</button>
  </form>;
}
function bindingSignature(bindings: ToolkitInspection['bindings']):string { return JSON.stringify(bindings.map(({name,reference})=>[name,reference]).sort(([left],[right])=>String(left).localeCompare(String(right)))); }
function catalogKey(catalog: ToolkitPageResult['catalog']):string { return `${catalog.epoch}/${catalog.config_revision}/${catalog.effective_digest}/${catalog.generation_id}`; }
function toolkitText(t: (key:string, values?:Record<string,string|number>)=>string, key:string, fallback:string):string { const fullKey=`plugin.vivy/toolkits-ui.${key}`; const value=t(fullKey); return value===fullKey?fallback:value; }
function mcpRuntimeLabel(t: (key:string, values?:Record<string,string|number>)=>string, state:string):string {
  const labels:Readonly<Record<string,readonly [string,string]>>={
    'not-compiled':['mcpNotCompiled','当前构建未编译'],unconfigured:['mcpUnconfigured','未配置'],inactive:['mcpInactive','未启用'],
    ready:['mcpReady','已连接'],unavailable:['mcpUnavailable','不可用'],deferred:['mcpDeferred','等待授权或绑定'],
  };
  const [key,fallback]=labels[state]??['mcpUnknown','状态未知'];
  return toolkitText(t,key,fallback);
}
function readLastOperationID():string { try{return sessionStorage.getItem(LAST_OPERATION_KEY)??'';}catch{return '';} }
function rememberOperationID(id:string):void { try{sessionStorage.setItem(LAST_OPERATION_KEY,id);}catch{/* recovery hint is optional; backend stays authoritative */} }
function clearLastOperationID():void { try{sessionStorage.removeItem(LAST_OPERATION_KEY);}catch{/* ignore storage policy */} }
