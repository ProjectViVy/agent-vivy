import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react';
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Dialog, DialogBody, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useTranslation } from '@/i18n';
import { Download, FileUp, Globe, LoaderCircle, PackageOpen, Pencil, Plus, RefreshCw, Search, Terminal, Trash2 } from 'lucide-react';
import * as api from '@/lib/api';
import { exportMcpConfig, parseMcpImport } from './mcp-import';

type McpFilter = 'all' | 'enabled' | 'disabled';
type McpForm = {
  name: string;
  transport: api.McpTransport;
  endpoint: string;
  command: string;
  args: string[];
  cwd: string;
  auth_env: string;
};
type EnvRow = { child: string; host: string };

const emptyForm: McpForm = { name: '', transport: 'http', endpoint: '', command: '', args: [], cwd: '', auth_env: '' };

function serverInput(server: api.McpServer, enabled = server.enabled): api.McpServerInput {
  return {
    name: server.name,
    transport: server.transport,
    endpoint: server.endpoint,
    command: server.command,
    args: server.args ? [...server.args] : undefined,
    env_from: server.env_from,
    cwd: server.cwd,
    auth_env: server.auth_env,
    enabled,
  };
}

function PageShell({ children }: { children: ReactNode }) {
  return (
    <div className="h-full overflow-y-auto p-4 sm:p-6">
      <div className="mx-auto max-w-4xl space-y-5">{children}</div>
    </div>
  );
}

function McpPageSkeleton() {
  return (
    <PageShell>
      <header className="flex items-center justify-between gap-4">
        <div className="space-y-2"><Skeleton className="h-7 w-32" /><Skeleton className="h-4 w-72 max-w-[70vw]" /></div>
        <Skeleton className="h-9 w-28" />
      </header>
      <Card>
        <CardHeader className="border-b px-4 py-3"><Skeleton className="h-5 w-24" /></CardHeader>
        <CardContent className="space-y-2 p-2">
          {[1, 2].map((item) => <div key={item} className="rounded-lg px-3 py-3"><Skeleton className="h-4 w-44" /><Skeleton className="mt-2 h-3 w-64" /></div>)}
        </CardContent>
      </Card>
    </PageShell>
  );
}

function mergeServer(list: api.McpServer[], next: api.McpServer) {
  const others = list.filter((server) => server.name.toLowerCase() !== next.name.toLowerCase());
  return [...others, next].sort((left, right) => left.name.localeCompare(right.name));
}

export function McpView() {
  const { t } = useTranslation();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [servers, setServers] = useState<api.McpServer[]>([]);
  const [readOnly, setReadOnly] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [importWarning, setImportWarning] = useState('');
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<McpFilter>('all');
  const [busyId, setBusyId] = useState('');
  const [importing, setImporting] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [editingName, setEditingName] = useState<string | null>(null);
  const [formData, setFormData] = useState(emptyForm);
  const [envRows, setEnvRows] = useState<EnvRow[]>([]);
  const [formError, setFormError] = useState('');
  const [deleting, setDeleting] = useState<api.McpServer | null>(null);

  const loadServers = useCallback(async () => {
    setLoading(true);
    setError(''); setImportWarning('');
    try {
      const view = await api.listMcpServers();
      setServers(view.servers);
      setReadOnly(view.read_only);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('mcp.errors.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { void loadServers(); }, [loadServers]);

  const visibleServers = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return servers
      .filter((server) => filter === 'all' || server.enabled === (filter === 'enabled'))
      .filter((server) => !needle || `${server.name} ${server.endpoint ?? ''} ${server.command ?? ''} ${server.auth_env ?? ''} ${Object.entries(server.env_from ?? {}).flat().join(' ')}`.toLowerCase().includes(needle))
      .sort((left, right) => left.name.localeCompare(right.name));
  }, [servers, filter, query]);

  const enabledCount = useMemo(() => servers.filter((server) => server.enabled).length, [servers]);
  const locked = readOnly || Boolean(busyId);

  const openCreate = () => {
    setEditingName(null); setFormData({ ...emptyForm }); setEnvRows([]); setFormError(''); setFormOpen(true);
  };

  const openEdit = (server: api.McpServer) => {
    setEditingName(server.name);
    setFormData({
      name: server.name,
      transport: server.transport,
      endpoint: server.endpoint ?? '',
      command: server.command ?? '',
      args: server.args ? [...server.args] : [],
      cwd: server.cwd ?? '',
      auth_env: server.auth_env ?? '',
    });
    setEnvRows(Object.entries(server.env_from ?? {}).map(([child, host]) => ({ child, host })));
    setFormError(''); setFormOpen(true);
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setBusyId('save'); setFormError('');
    try {
      const nextName = formData.name.trim();
      const args = [...formData.args];
      const env_from = Object.fromEntries(envRows
        .map((row) => [row.child.trim(), row.host.trim()] as const)
        .filter(([child, host]) => child || host));
      if (Object.entries(env_from).some(([child, host]) => !child || !host)) {
        throw new Error(t('mcp.errors.envPairRequired'));
      }
      const saved = await api.upsertMcpServer({
        name: nextName,
        transport: formData.transport,
        endpoint: formData.transport === 'http' ? formData.endpoint.trim() : undefined,
        command: formData.transport === 'stdio' ? formData.command.trim() : undefined,
        args: formData.transport === 'stdio' ? args : undefined,
        env_from: formData.transport === 'stdio' && Object.keys(env_from).length ? env_from : undefined,
        cwd: formData.transport === 'stdio' ? formData.cwd.trim() || undefined : undefined,
        auth_env: formData.transport === 'http' ? formData.auth_env.trim() || undefined : undefined,
        enabled: editingName ? servers.find((server) => server.name === editingName)?.enabled : true,
      });
      if (editingName && editingName.toLowerCase() !== saved.name.toLowerCase()) {
        await api.deleteMcpServer(editingName);
      }
      setServers((current) => {
        const withoutOld = editingName ? current.filter((server) => server.name !== editingName) : current;
        return mergeServer(withoutOld, saved);
      });
      setFormOpen(false); setEditingName(null); setEnvRows([]);
    } catch (cause) {
      setFormError(cause instanceof Error ? cause.message : t('mcp.errors.saveFailed'));
    } finally {
      setBusyId('');
    }
  };

  const handleToggle = async (server: api.McpServer) => {
    setBusyId(`toggle:${server.name}`); setError(''); setImportWarning('');
    try {
      const saved = await api.upsertMcpServer(serverInput(server, !server.enabled));
      setServers((current) => mergeServer(current, saved));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('mcp.errors.toggleFailed'));
    } finally {
      setBusyId('');
    }
  };

  const handleProbe = async (server: api.McpServer) => {
    setBusyId(`probe:${server.name}`); setError(''); setImportWarning('');
    try {
      const probed = await api.probeMcpServer(server.name);
      setServers((current) => mergeServer(current, probed));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('mcp.errors.probeFailed'));
    } finally {
      setBusyId('');
    }
  };

  const handleDelete = async () => {
    if (!deleting) return;
    setBusyId(`delete:${deleting.name}`); setError(''); setImportWarning('');
    try {
      await api.deleteMcpServer(deleting.name);
      setServers((current) => current.filter((server) => server.name !== deleting.name));
      setDeleting(null);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('mcp.errors.deleteFailed'));
    } finally {
      setBusyId('');
    }
  };

  const handleImport = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;
    setImporting(true); setError(''); setImportWarning('');
    try {
      const parsed = parseMcpImport(JSON.parse(await file.text()) as unknown);
      if (!parsed.servers.length) {
        throw new Error(t('mcp.errors.emptyImport'));
      }
      let next = servers;
      for (const input of parsed.servers) {
        const saved = await api.upsertMcpServer(input);
        next = mergeServer(next, saved);
      }
      setServers(next);
      setImportWarning(parsed.warnings.length ? t('mcp.importEnvWarning', { count: parsed.warnings.length }) : '');
      setQuery(''); setFilter('all');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('mcp.errors.importFailed'));
    } finally {
      setImporting(false);
    }
  };

  const handleExport = () => {
    setError(''); setImportWarning('');
    try {
      const blob = new Blob([JSON.stringify(exportMcpConfig(servers), null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = 'vivy-mcp-servers.json';
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      URL.revokeObjectURL(url);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('mcp.errors.exportFailed'));
    }
  };

  const resetFilters = () => { setQuery(''); setFilter('all'); };

  if (loading) return <McpPageSkeleton />;
  if (error && servers.length === 0) {
    return (
      <PageShell>
        <header>
          <h1 className="text-2xl font-semibold tracking-tight">{t('mcp.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t('mcp.subtitle')}</p>
        </header>
        <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          <p>{error}</p>
          <Button className="mt-3" size="sm" variant="outline" onClick={() => void loadServers()}>{t('common.retry')}</Button>
        </div>
      </PageShell>
    );
  }

  return (
    <PageShell>
      <header className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">{t('mcp.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t('mcp.subtitle')}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <input ref={fileInputRef} className="hidden" type="file" accept="application/json,.json" onChange={(event) => void handleImport(event)} />
          <Button variant="outline" disabled={importing || readOnly} title={t('mcp.importTitle')} onClick={() => fileInputRef.current?.click()}>
            {importing ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" /> : <FileUp className="mr-2 h-4 w-4" />}
            {t('mcp.importJson')}
          </Button>
          <Button variant="outline" disabled={servers.length === 0} onClick={handleExport}>
            <Download className="mr-2 h-4 w-4" />{t('mcp.exportJson')}
          </Button>
          <Button onClick={openCreate} disabled={readOnly} className="self-start sm:self-auto"><Plus className="mr-2 h-4 w-4" />{t('mcp.addService')}</Button>
        </div>
      </header>

      {readOnly ? <p className="rounded-lg bg-amber-500/10 px-4 py-3 text-sm text-amber-700">{t('mcp.readOnly')}</p> : null}
      {importWarning ? <p role="status" className="rounded-lg bg-amber-500/10 px-4 py-3 text-sm text-amber-700">{importWarning}</p> : null}
      {error ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">{error}</div> : null}

      <Card>
        <CardHeader className="flex-row items-center justify-between space-y-0 border-b px-4 py-3">
          <CardTitle className="text-base">{t('mcp.listTitle')}</CardTitle>
          <span className="text-xs text-muted-foreground">{t('mcp.countLabel', { total: servers.length, enabled: enabledCount })}</span>
        </CardHeader>
        {servers.length > 0 ? (
          <div className="flex flex-col gap-2 border-b px-4 py-3 sm:flex-row">
            <div className="relative min-w-0 flex-1">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input value={query} onChange={(event) => setQuery(event.target.value)} className="pl-9" placeholder={t('mcp.searchPlaceholder')} aria-label={t('mcp.searchAria')} />
            </div>
            <Select value={filter} onValueChange={(value) => setFilter(value as McpFilter)}>
              <SelectTrigger className="sm:w-36" aria-label={t('mcp.filterAria')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t('mcp.filterAll')}</SelectItem>
                <SelectItem value="enabled">{t('mcp.filterEnabled')}</SelectItem>
                <SelectItem value="disabled">{t('mcp.filterDisabled')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        ) : null}
        <CardContent className="p-2">
          {servers.length === 0 ? (
            <div className="flex min-h-64 flex-col items-center justify-center gap-3 px-6 text-center">
              <div className="rounded-full bg-muted p-3 text-muted-foreground"><PackageOpen className="h-5 w-5" /></div>
              <div><p className="font-medium">{t('mcp.emptyTitle')}</p><p className="mt-1 text-sm text-muted-foreground">{t('mcp.emptyHint')}</p></div>
              {readOnly ? null : (
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" onClick={() => fileInputRef.current?.click()}>{t('mcp.importJson')}</Button>
                  <Button size="sm" onClick={openCreate}>{t('mcp.addService')}</Button>
                </div>
              )}
            </div>
          ) : visibleServers.length === 0 ? (
            <div className="flex min-h-48 flex-col items-center justify-center gap-2 px-6 text-center">
              <p className="font-medium">{t('mcp.noMatchTitle')}</p>
              <Button variant="ghost" size="sm" onClick={resetFilters}>{t('mcp.clearFilters')}</Button>
            </div>
          ) : (
            <div className="space-y-1">
              {visibleServers.map((server) => (
                <div key={server.name} className="flex items-center gap-3 rounded-lg border border-transparent px-3 py-3 transition-colors hover:bg-muted/60">
                  <Switch
                    checked={server.enabled}
                    disabled={locked || busyId === `toggle:${server.name}`}
                    aria-label={t('mcp.toggleAria', { name: server.name })}
                    onCheckedChange={() => void handleToggle(server)}
                  />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-medium">{server.name}</span>
                      <Badge variant="outline" className="shrink-0 font-normal">{server.transport === 'stdio' ? 'STDIO' : 'HTTP'}</Badge>
                      {server.status === 'ok' ? <Badge variant="outline" className="shrink-0 font-normal">{t('mcp.statusOk')}</Badge> : null}
                      {server.status === 'error' ? <Badge variant="destructive" className="shrink-0 font-normal">{t('mcp.statusError')}</Badge> : null}
                    </div>
                    <span className="mt-1 flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
                      {server.transport === 'stdio' ? <Terminal className="h-3.5 w-3.5 shrink-0" /> : <Globe className="h-3.5 w-3.5 shrink-0" />}
                      <span className="truncate">{server.transport === 'stdio' ? server.command : server.endpoint}</span>
                      {server.status === 'ok' ? <span className="shrink-0">· {t('mcp.toolCount', { count: server.tool_count })}</span> : null}
                    </span>
                    {server.env_missing?.length ? <p className="mt-1 truncate text-xs text-destructive">{t('mcp.envMissing', { names: server.env_missing.join(', ') })}</p> : null}
                    {server.status === 'error' && server.error ? <p className="mt-1 truncate text-xs text-destructive">{server.error}</p> : null}
                  </div>
                  <Button variant="ghost" size="icon" aria-label={t('mcp.probeAria', { name: server.name })} title={t('mcp.probeTitle')} disabled={Boolean(busyId) || !server.enabled} onClick={() => void handleProbe(server)}>
                    {busyId === `probe:${server.name}` ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                  </Button>
                  <Button variant="ghost" size="icon" aria-label={t('mcp.editAria', { name: server.name })} title={t('mcp.editTitle')} disabled={locked} onClick={() => openEdit(server)}>
                    <Pencil className="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" aria-label={t('mcp.deleteAria', { name: server.name })} title={t('mcp.deleteTitle')} className="text-destructive hover:text-destructive" disabled={locked} onClick={() => setDeleting(server)}>
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={formOpen} onOpenChange={(open) => { if (busyId !== 'save') { setFormOpen(open); if (!open) { setEditingName(null); setEnvRows([]); } } }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editingName ? t('mcp.formTitleEdit') : t('mcp.formTitleAdd')}</DialogTitle>
            <DialogDescription>{t('mcp.formDescription')}</DialogDescription>
          </DialogHeader>
          <form onSubmit={(event) => void handleSubmit(event)} className="contents">
            <DialogBody className="space-y-4 py-2">
              <div className="space-y-2">
                <Label htmlFor="mcp-name">{t('mcp.nameLabel')}</Label>
                <Input id="mcp-name" value={formData.name} onChange={(event) => setFormData((current) => ({ ...current, name: event.target.value }))} placeholder={t('mcp.namePlaceholder')} autoFocus />
              </div>
              <div className="space-y-2">
                <Label htmlFor="mcp-transport">{t('mcp.transportLabel')}</Label>
                <Select value={formData.transport} onValueChange={(value) => setFormData((current) => ({ ...current, transport: value as api.McpTransport }))}>
                  <SelectTrigger id="mcp-transport"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="http">HTTP</SelectItem>
                    <SelectItem value="stdio">STDIO</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {formData.transport === 'http' ? (
                <>
                  <div className="space-y-2">
                    <Label htmlFor="mcp-url">{t('mcp.urlLabel')}</Label>
                    <Input id="mcp-url" value={formData.endpoint} onChange={(event) => setFormData((current) => ({ ...current, endpoint: event.target.value }))} placeholder="https://example.com/mcp" />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="mcp-auth">{t('mcp.authEnvLabel')}</Label>
                    <Input id="mcp-auth" value={formData.auth_env} onChange={(event) => setFormData((current) => ({ ...current, auth_env: event.target.value }))} placeholder="MCP_DOCS_TOKEN" />
                    <p className="text-xs text-muted-foreground">{t('mcp.authEnvHint')}</p>
                  </div>
                </>
              ) : (
                <>
                  <div className="space-y-2">
                    <Label htmlFor="mcp-command">{t('mcp.commandLabel')}</Label>
                    <Input id="mcp-command" value={formData.command} onChange={(event) => setFormData((current) => ({ ...current, command: event.target.value }))} placeholder="npx" />
                    <p className="text-xs text-muted-foreground">{t('mcp.commandHint')}</p>
                  </div>
                  <div className="space-y-2">
                    <div className="flex items-center justify-between gap-2">
                      <Label>{t('mcp.argsLabel')}</Label>
                      <Button type="button" variant="ghost" size="sm" onClick={() => setFormData((current) => ({ ...current, args: [...current.args, ''] }))}><Plus className="mr-1 h-3.5 w-3.5" />{t('mcp.addArg')}</Button>
                    </div>
                    {formData.args.length === 0 ? <p className="text-xs text-muted-foreground">{t('mcp.argsHint')}</p> : null}
                    <div className="space-y-2">
                      {formData.args.map((arg, index) => (
                        <div key={index} className="flex items-center gap-2">
                          <Input aria-label={t('mcp.argLabel', { index: index + 1 })} value={arg} onChange={(event) => setFormData((current) => ({ ...current, args: current.args.map((value, argIndex) => argIndex === index ? event.target.value : value) }))} placeholder={t('mcp.argsPlaceholder')} />
                          <Button type="button" variant="ghost" size="icon" aria-label={t('mcp.removeArg')} onClick={() => setFormData((current) => ({ ...current, args: current.args.filter((_, argIndex) => argIndex !== index) }))}><Trash2 className="h-4 w-4" /></Button>
                        </div>
                      ))}
                    </div>
                    <p className="text-xs text-muted-foreground">{t('mcp.argsHint')}</p>
                  </div>
                  <div className="space-y-2">
                    <div className="flex items-center justify-between gap-2">
                      <Label>{t('mcp.envFromLabel')}</Label>
                      <Button type="button" variant="ghost" size="sm" onClick={() => setEnvRows((rows) => [...rows, { child: '', host: '' }])}><Plus className="mr-1 h-3.5 w-3.5" />{t('mcp.addEnv')}</Button>
                    </div>
                    {envRows.length === 0 ? <p className="text-xs text-muted-foreground">{t('mcp.envFromHint')}</p> : null}
                    <div className="space-y-2">
                      {envRows.map((row, index) => (
                        <div key={index} className="flex items-center gap-2">
                          <Input aria-label={t('mcp.envChildLabel')} value={row.child} onChange={(event) => setEnvRows((rows) => rows.map((current, rowIndex) => rowIndex === index ? { ...current, child: event.target.value } : current))} placeholder="CHILD_VAR" />
                          <span className="text-muted-foreground">←</span>
                          <Input aria-label={t('mcp.envHostLabel')} value={row.host} onChange={(event) => setEnvRows((rows) => rows.map((current, rowIndex) => rowIndex === index ? { ...current, host: event.target.value } : current))} placeholder="HOST_VAR" />
                          <Button type="button" variant="ghost" size="icon" aria-label={t('mcp.removeEnv')} onClick={() => setEnvRows((rows) => rows.filter((_, rowIndex) => rowIndex !== index))}><Trash2 className="h-4 w-4" /></Button>
                        </div>
                      ))}
                    </div>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="mcp-cwd">{t('mcp.cwdLabel')}</Label>
                    <Input id="mcp-cwd" value={formData.cwd} onChange={(event) => setFormData((current) => ({ ...current, cwd: event.target.value }))} placeholder="." />
                    <p className="text-xs text-muted-foreground">{t('mcp.cwdHint')}</p>
                  </div>
                  <p className="rounded-md bg-amber-500/10 px-3 py-2 text-xs text-amber-700">{t('mcp.stdioWarning')}</p>
                </>
              )}
              {formError ? <p role="alert" className="text-sm text-destructive">{formError}</p> : null}
            </DialogBody>
            <DialogFooter>
              <Button type="button" variant="outline" disabled={busyId === 'save'} onClick={() => { setFormOpen(false); setEditingName(null); setEnvRows([]); }}>{t('common.cancel')}</Button>
              <Button type="submit" disabled={busyId === 'save' || readOnly}>
                {busyId === 'save' ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" /> : null}
                {editingName ? t('mcp.saveChanges') : t('mcp.addService')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleting !== null} onOpenChange={(open) => { if (!busyId && !open) setDeleting(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('mcp.deleteConfirm', { name: deleting?.name ?? '' })}</AlertDialogTitle>
            <AlertDialogDescription>{t('mcp.deleteDescription')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={Boolean(busyId)}>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              disabled={Boolean(busyId)}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={(event) => { event.preventDefault(); void handleDelete(); }}
            >
              {busyId.startsWith('delete:') ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" /> : null}{t('mcp.deleteAction')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PageShell>
  );
}
