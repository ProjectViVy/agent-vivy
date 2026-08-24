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
import { DemoLoadError } from '@/components/demo/DemoBanner';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { Download, FileUp, Globe, LoaderCircle, PackageOpen, Pencil, Plus, Search, Terminal, Trash2 } from 'lucide-react';
import {
  addDemoMcpServer,
  exportDemoMcpConfig,
  getDemoMcpServers,
  importDemoMcpConfig,
  removeDemoMcpServer,
  toggleDemoMcpServer,
  updateDemoMcpServer,
} from '@/lib/demo-api';
import type { DemoMcpServer } from '@/lib/types';

type McpFilter = 'all' | 'enabled' | 'disabled';

const emptyForm = {
  name: '',
  transport: 'stdio' as DemoMcpServer['transport'],
  command: '',
  url: '',
};

function serverTarget(server: DemoMcpServer) {
  return server.transport === 'http' ? server.url ?? '未设置服务地址' : server.command ?? '未设置启动命令';
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

export function McpDemoView() {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [servers, setServers] = useState<DemoMcpServer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<McpFilter>('all');
  const [busyId, setBusyId] = useState('');
  const [importing, setImporting] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formData, setFormData] = useState(emptyForm);
  const [formError, setFormError] = useState('');
  const [deleting, setDeleting] = useState<DemoMcpServer | null>(null);

  const loadServers = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setServers(await getDemoMcpServers());
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '无法加载 MCP 服务');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void loadServers(); }, [loadServers]);

  const visibleServers = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return servers
      .filter((server) => filter === 'all' || server.enabled === (filter === 'enabled'))
      .filter((server) => !needle || `${server.name} ${serverTarget(server)}`.toLowerCase().includes(needle))
      .sort((left, right) => left.name.localeCompare(right.name));
  }, [servers, filter, query]);

  const enabledCount = useMemo(() => servers.filter((server) => server.enabled).length, [servers]);

  const openCreate = () => {
    setEditingId(null); setFormData(emptyForm); setFormError(''); setFormOpen(true);
  };

  const openEdit = (server: DemoMcpServer) => {
    setEditingId(server.id);
    setFormData({
      name: server.name,
      transport: server.transport,
      command: server.command ?? '',
      url: server.url ?? '',
    });
    setFormError(''); setFormOpen(true);
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setBusyId('save'); setFormError('');
    try {
      const input = { name: formData.name, transport: formData.transport, command: formData.command, url: formData.url };
      setServers(editingId ? await updateDemoMcpServer(editingId, input) : await addDemoMcpServer(input));
      setFormOpen(false); setEditingId(null);
    } catch (cause) {
      setFormError(cause instanceof Error ? cause.message : '无法保存 MCP 服务');
    } finally {
      setBusyId('');
    }
  };

  const handleToggle = async (server: DemoMcpServer) => {
    setBusyId(`toggle:${server.id}`); setError('');
    try {
      setServers(await toggleDemoMcpServer(server.id));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '无法更新启用状态');
    } finally {
      setBusyId('');
    }
  };

  const handleDelete = async () => {
    if (!deleting) return;
    setBusyId(`delete:${deleting.id}`); setError('');
    try {
      setServers(await removeDemoMcpServer(deleting.id));
      setDeleting(null);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '无法删除 MCP 服务');
    } finally {
      setBusyId('');
    }
  };

  const handleImport = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;
    setImporting(true); setError('');
    try {
      const config = JSON.parse(await file.text()) as unknown;
      setServers(await importDemoMcpConfig(config));
      setQuery(''); setFilter('all');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'JSON 文件无法导入');
    } finally {
      setImporting(false);
    }
  };

  const handleExport = async () => {
    setError('');
    try {
      const config = await exportDemoMcpConfig();
      const blob = new Blob([JSON.stringify(config, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = 'vivy-mcp-servers.json';
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      URL.revokeObjectURL(url);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '无法导出 MCP 配置');
    }
  };

  const resetFilters = () => { setQuery(''); setFilter('all'); };

  if (loading) return <McpPageSkeleton />;
  if (error && servers.length === 0) {
    return (
      <PageShell>
        <header>
          <h1 className="text-2xl font-semibold tracking-tight">MCP 服务</h1>
          <p className="mt-1 text-sm text-muted-foreground">配置 MCP 服务器，决定哪些服务参与工具调用。</p>
        </header>
        <DemoLoadError message={error} onRetry={() => void loadServers()} />
      </PageShell>
    );
  }

  return (
    <PageShell>
      <header className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">MCP 服务</h1>
          <p className="mt-1 text-sm text-muted-foreground">配置 MCP 服务器，决定哪些服务参与工具调用。</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <input ref={fileInputRef} className="hidden" type="file" accept="application/json,.json" onChange={(event) => void handleImport(event)} />
          <Button
            variant="outline"
            disabled={importing}
            title="支持单个 MCP 对象、服务字典或包含 tools.mcpServers 的完整配置"
            onClick={() => fileInputRef.current?.click()}
          >
            {importing ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" /> : <FileUp className="mr-2 h-4 w-4" />}
            导入 JSON
          </Button>
          <Button variant="outline" disabled={servers.length === 0} onClick={() => void handleExport()}>
            <Download className="mr-2 h-4 w-4" />导出 JSON
          </Button>
          <Button onClick={openCreate} className="self-start sm:self-auto"><Plus className="mr-2 h-4 w-4" />添加服务</Button>
        </div>
      </header>

      {error ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">{error}</div> : null}

      <Card>
        <CardHeader className="flex-row items-center justify-between space-y-0 border-b px-4 py-3">
          <CardTitle className="text-base">服务列表</CardTitle>
          <span className="text-xs text-muted-foreground">{servers.length} 个服务 · {enabledCount} 个启用</span>
        </CardHeader>
        {servers.length > 0 ? (
          <div className="flex flex-col gap-2 border-b px-4 py-3 sm:flex-row">
            <div className="relative min-w-0 flex-1">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input value={query} onChange={(event) => setQuery(event.target.value)} className="pl-9" placeholder="搜索名称或连接目标" aria-label="搜索 MCP 服务" />
            </div>
            <Select value={filter} onValueChange={(value) => setFilter(value as McpFilter)}>
              <SelectTrigger className="sm:w-36" aria-label="按启用状态筛选">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部状态</SelectItem>
                <SelectItem value="enabled">已启用</SelectItem>
                <SelectItem value="disabled">已停用</SelectItem>
              </SelectContent>
            </Select>
          </div>
        ) : null}
        <CardContent className="p-2">
          {servers.length === 0 ? (
            <div className="flex min-h-64 flex-col items-center justify-center gap-3 px-6 text-center">
              <div className="rounded-full bg-muted p-3 text-muted-foreground"><PackageOpen className="h-5 w-5" /></div>
              <div><p className="font-medium">尚未配置 MCP 服务</p><p className="mt-1 text-sm text-muted-foreground">添加一个服务，或从 JSON 配置导入已有服务。</p></div>
              <div className="flex gap-2">
                <Button variant="outline" size="sm" onClick={() => fileInputRef.current?.click()}>导入 JSON</Button>
                <Button size="sm" onClick={openCreate}>添加服务</Button>
              </div>
            </div>
          ) : visibleServers.length === 0 ? (
            <div className="flex min-h-48 flex-col items-center justify-center gap-2 px-6 text-center">
              <p className="font-medium">没有符合条件的 MCP 服务</p>
              <Button variant="ghost" size="sm" onClick={resetFilters}>清除筛选</Button>
            </div>
          ) : (
            <div className="space-y-1">
              {visibleServers.map((server) => {
                const TargetIcon = server.transport === 'http' ? Globe : Terminal;
                return (
                  <div key={server.id} className="flex items-center gap-3 rounded-lg border border-transparent px-3 py-3 transition-colors hover:bg-muted/60">
                    <Switch
                      checked={server.enabled}
                      disabled={busyId === `toggle:${server.id}`}
                      aria-label={`${server.name} 启用状态`}
                      onCheckedChange={() => void handleToggle(server)}
                    />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-medium">{server.name}</span>
                        <Badge variant="outline" className="shrink-0 font-normal">{server.transport === 'http' ? 'HTTP' : 'STDIO'}</Badge>
                      </div>
                      <span className="mt-1 flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
                        <TargetIcon className="h-3.5 w-3.5 shrink-0" />
                        <span className="truncate">{serverTarget(server)}</span>
                        <span className="shrink-0">· {server.toolCount} 个工具</span>
                      </span>
                    </div>
                    <Button variant="ghost" size="icon" aria-label={`编辑 ${server.name}`} title="编辑" disabled={Boolean(busyId)} onClick={() => openEdit(server)}>
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" aria-label={`删除 ${server.name}`} title="删除" className="text-destructive hover:text-destructive" disabled={Boolean(busyId)} onClick={() => setDeleting(server)}>
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={formOpen} onOpenChange={(open) => { if (busyId !== 'save') { setFormOpen(open); if (!open) setEditingId(null); } }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editingId ? '编辑 MCP 服务' : '添加 MCP 服务'}</DialogTitle>
            <DialogDescription>配置服务名称、传输方式与连接目标。</DialogDescription>
          </DialogHeader>
          <form onSubmit={(event) => void handleSubmit(event)} className="contents">
            <DialogBody className="space-y-4 py-2">
              <div className="space-y-2">
                <Label htmlFor="mcp-name">服务名称</Label>
                <Input id="mcp-name" value={formData.name} onChange={(event) => setFormData((current) => ({ ...current, name: event.target.value }))} placeholder="例如：Workspace Files" autoFocus />
              </div>
              <div className="space-y-2">
                <Label>传输方式</Label>
                <Select value={formData.transport} onValueChange={(transport) => setFormData((current) => ({ ...current, transport: transport as DemoMcpServer['transport'] }))}>
                  <SelectTrigger aria-label="传输方式"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="stdio">STDIO · 本地进程</SelectItem>
                    <SelectItem value="http">HTTP · 远程服务</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {formData.transport === 'stdio' ? (
                <div className="space-y-2">
                  <Label htmlFor="mcp-command">启动命令</Label>
                  <Input id="mcp-command" value={formData.command} onChange={(event) => setFormData((current) => ({ ...current, command: event.target.value }))} placeholder="npx -y @modelcontextprotocol/server-filesystem ." />
                </div>
              ) : (
                <div className="space-y-2">
                  <Label htmlFor="mcp-url">服务地址</Label>
                  <Input id="mcp-url" value={formData.url} onChange={(event) => setFormData((current) => ({ ...current, url: event.target.value }))} placeholder="https://example.com/mcp" />
                </div>
              )}
              {formError ? <p role="alert" className="text-sm text-destructive">{formError}</p> : null}
            </DialogBody>
            <DialogFooter>
              <Button type="button" variant="outline" disabled={busyId === 'save'} onClick={() => { setFormOpen(false); setEditingId(null); }}>取消</Button>
              <Button type="submit" disabled={busyId === 'save'}>
                {busyId === 'save' ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" /> : null}
                {editingId ? '保存修改' : '添加服务'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleting !== null} onOpenChange={(open) => { if (!busyId && !open) setDeleting(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除“{deleting?.name}”？</AlertDialogTitle>
            <AlertDialogDescription>该服务配置会从列表中移除，此操作无法撤销。</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={Boolean(busyId)}>取消</AlertDialogCancel>
            <AlertDialogAction
              disabled={Boolean(busyId)}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={(event) => { event.preventDefault(); void handleDelete(); }}
            >
              {busyId.startsWith('delete:') ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" /> : null}删除服务
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PageShell>
  );
}
