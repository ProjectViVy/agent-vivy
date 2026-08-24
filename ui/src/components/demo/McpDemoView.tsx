import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import { useNavigate } from '@tanstack/react-router';
import {
  AlertTriangle,
  ArrowLeft,
  Bot,
  CheckCircle2,
  CirclePause,
  Download,
  FileUp,
  Globe2,
  Heart,
  LoaderCircle,
  PackageOpen,
  Plus,
  RefreshCw,
  Search,
  Terminal,
  Wrench,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import {
  addDemoMcpServer,
  exportDemoMcpConfig,
  getDemoMcpServers,
  importDemoMcpConfig,
  toggleDemoMcpServer,
} from '@/lib/demo-api';
import type { DemoMcpServer } from '@/lib/types';
import { DemoLoadError } from './DemoBanner';
import './mcp.css';

type McpFilter = 'all' | DemoMcpServer['status'];
type McpSort = 'name' | 'tools' | 'status';

const STATUS_LABELS: Record<DemoMcpServer['status'], string> = {
  connected: '在线',
  degraded: '降级',
  disabled: '禁用',
};

const STATUS_ORDER: Record<DemoMcpServer['status'], number> = {
  connected: 0,
  degraded: 1,
  disabled: 2,
};

const TRANSPORT_LABELS: Record<DemoMcpServer['transport'], string> = {
  stdio: 'STDIO',
  http: 'HTTP',
};

function getStatusIcon(status: DemoMcpServer['status']) {
  if (status === 'connected') return CheckCircle2;
  if (status === 'degraded') return AlertTriangle;
  return CirclePause;
}

function getStatusClass(status: DemoMcpServer['status']) {
  return `mcp-status-chip--${status}`;
}

export function McpDemoView() {
  const navigate = useNavigate();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [servers, setServers] = useState<DemoMcpServer[]>([]);
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState<McpFilter>('all');
  const [sort, setSort] = useState<McpSort>('name');
  const [busyId, setBusyId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [importing, setImporting] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [formBusy, setFormBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [transport, setTransport] = useState<DemoMcpServer['transport']>('stdio');

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      setServers(await getDemoMcpServers());
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const summary = useMemo(() => ({
    total: servers.length,
    connected: servers.filter((server) => server.status === 'connected').length,
    degraded: servers.filter((server) => server.status === 'degraded').length,
    disabled: servers.filter((server) => server.status === 'disabled').length,
  }), [servers]);

  const visibleServers = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase();
    return servers
      .filter((server) => filter === 'all' || server.status === filter)
      .filter((server) => !needle || `${server.name} ${server.transport}`.toLocaleLowerCase().includes(needle))
      .sort((left, right) => {
        if (sort === 'tools') return right.toolCount - left.toolCount || left.name.localeCompare(right.name);
        if (sort === 'status') return STATUS_ORDER[left.status] - STATUS_ORDER[right.status] || left.name.localeCompare(right.name);
        return left.name.localeCompare(right.name);
      });
  }, [filter, query, servers, sort]);

  const handleToggle = async (id: string) => {
    setBusyId(id);
    setError(null);
    try {
      setServers(await toggleDemoMcpServer(id));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusyId(null);
    }
  };

  const handleImport = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;

    setImporting(true);
    setError(null);
    try {
      const config = JSON.parse(await file.text()) as unknown;
      setServers(await importDemoMcpConfig(config));
      setQuery('');
      setFilter('all');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'JSON 文件无法导入');
    } finally {
      setImporting(false);
    }
  };

  const handleExport = async () => {
    setExporting(true);
    setError(null);
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
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setExporting(false);
    }
  };

  const openAddDialog = () => {
    setName('');
    setTransport('stdio');
    setFormError(null);
    setDialogOpen(true);
  };

  const handleAdd = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormBusy(true);
    setFormError(null);
    try {
      setServers(await addDemoMcpServer({ name, transport }));
      setDialogOpen(false);
    } catch (cause) {
      setFormError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setFormBusy(false);
    }
  };

  const resetFilters = () => {
    setQuery('');
    setFilter('all');
  };

  return (
    <div className="mcp-page min-h-full">
      <Heart aria-hidden="true" className="mcp-heart mcp-heart--one" />
      <Heart aria-hidden="true" className="mcp-heart mcp-heart--two" />
      <Heart aria-hidden="true" className="mcp-heart mcp-heart--three" />

      <div className="mcp-page__content px-4 py-5 sm:px-6 sm:py-7 lg:px-8 lg:py-8">
        <Button
          variant="ghost"
          size="sm"
          className="mcp-breadcrumb -ml-2 mb-7 gap-2 rounded-lg px-2"
          onClick={() => void navigate({ to: '/' })}
        >
          <ArrowLeft className="h-4 w-4" />
          <span>MCP</span>
        </Button>

        <header className="flex flex-col gap-5">
          <div className="flex items-start gap-4">
            <div className="mcp-hero-icon flex h-14 w-14 shrink-0 items-center justify-center rounded-xl">
              <Bot className="h-7 w-7" strokeWidth={1.8} />
            </div>
            <div>
              <h1 className="mcp-hero-title text-2xl font-bold tracking-tight sm:text-[28px]">MCP</h1>
              <p className="mcp-muted mt-1.5 text-sm sm:text-base">管理 MCP 服务、连接状态与当前运行集合</p>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4 lg:gap-4">
            <div className="mcp-stat-card">
              <span className="mcp-stat-label">总数</span>
              <span className="mcp-stat-value">{summary.total}</span>
            </div>
            <div className="mcp-stat-card mcp-stat-card--online">
              <span className="mcp-stat-label">在线</span>
              <span className="mcp-stat-value">{summary.connected}</span>
            </div>
            <div className="mcp-stat-card mcp-stat-card--degraded">
              <span className="mcp-stat-label">降级</span>
              <span className="mcp-stat-value">{summary.degraded}</span>
            </div>
            <div className="mcp-stat-card mcp-stat-card--disabled">
              <span className="mcp-stat-label">禁用</span>
              <span className="mcp-stat-value">{summary.disabled}</span>
            </div>
          </div>
        </header>

        {error ? <div className="mt-5"><DemoLoadError message={error} onRetry={() => void load()} /></div> : null}

        <section className="mcp-management-panel mt-6 overflow-hidden sm:mt-7" aria-labelledby="mcp-management-title">
          <div className="flex flex-col gap-4 p-5 sm:p-6 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <h2 id="mcp-management-title" className="mcp-management-title text-lg font-semibold">MCP 管理</h2>
              <p className="mcp-muted mt-1 text-sm">管理 MCP 服务器、探测连接状态、控制运行时激活</p>
            </div>

            <div className="flex flex-wrap gap-2 lg:justify-end">
              <Button
                variant="outline"
                className="mcp-action-button"
                onClick={() => void load()}
                disabled={loading}
                aria-label="刷新 MCP 列表"
              >
                <RefreshCw className={loading ? 'animate-spin' : ''} />
                <span>刷新</span>
              </Button>
              <input ref={fileInputRef} className="hidden" type="file" accept="application/json,.json" onChange={(event) => void handleImport(event)} />
              <Button
                variant="outline"
                className="mcp-action-button"
                onClick={() => fileInputRef.current?.click()}
                disabled={importing}
                aria-label="导入 MCP JSON"
              >
                {importing ? <LoaderCircle className="animate-spin" /> : <FileUp />}
                <span>{importing ? '导入中…' : '导入 JSON'}</span>
              </Button>
              <Button
                variant="outline"
                className="mcp-action-button"
                onClick={() => void handleExport()}
                disabled={!servers.length || exporting}
                aria-label="导出 MCP JSON"
              >
                {exporting ? <LoaderCircle className="animate-spin" /> : <Download />}
                <span>导出 JSON</span>
              </Button>
              <Button variant="default" className="mcp-primary-button" onClick={openAddDialog}>
                <Plus />
                <span>添加 MCP</span>
              </Button>
            </div>
          </div>

          <div className="mcp-toolbar flex flex-col gap-3 px-5 py-4 sm:px-6 lg:flex-row">
            <div className="relative min-w-0 flex-1">
              <Search className="mcp-muted pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2" />
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                className="mcp-search-input pl-9"
                placeholder="搜索 MCP 服务器..."
                aria-label="搜索 MCP 服务器"
              />
            </div>
            <div className="grid grid-cols-2 gap-3 sm:flex">
              <Select value={filter} onValueChange={(value) => setFilter(value as McpFilter)}>
                <SelectTrigger className="mcp-select-trigger" aria-label="筛选 MCP 状态">
                  <SelectValue placeholder="全部" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部</SelectItem>
                  <SelectItem value="connected">在线</SelectItem>
                  <SelectItem value="degraded">降级</SelectItem>
                  <SelectItem value="disabled">禁用</SelectItem>
                </SelectContent>
              </Select>
              <Select value={sort} onValueChange={(value) => setSort(value as McpSort)}>
                <SelectTrigger className="mcp-select-trigger" aria-label="MCP 排序方式">
                  <SelectValue placeholder="按名称" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="name">按名称</SelectItem>
                  <SelectItem value="status">按状态</SelectItem>
                  <SelectItem value="tools">按工具数</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="px-5 pt-4 sm:px-6">
            <p className="mcp-import-hint px-4 py-3 text-xs leading-5 sm:text-sm">导入单个 MCP 对象、MCP 字典或包含 <code className="rounded bg-white/70 px-1 py-0.5 font-mono text-[0.9em]">tools.mcpServers</code> 的完整配置。</p>
          </div>

          <div className="p-5 sm:p-6">
            {loading ? (
              <div className="grid gap-3 lg:grid-cols-2">
                {Array.from({ length: 2 }, (_, index) => <div key={index} className="h-36 animate-pulse rounded-[15px] bg-white/60" />)}
              </div>
            ) : visibleServers.length ? (
              <div className="grid gap-3 lg:grid-cols-2">
                {visibleServers.map((server) => {
                  const StatusIcon = getStatusIcon(server.status);
                  const TransportIcon = server.transport === 'stdio' ? Terminal : Globe2;
                  return (
                    <article key={server.id} className="mcp-server-card p-4 sm:p-5">
                      <div className="flex items-start justify-between gap-4">
                        <div className="flex min-w-0 items-center gap-3">
                          <div className="mcp-server-icon flex h-10 w-10 shrink-0 items-center justify-center rounded-xl">
                            <Bot className="h-5 w-5" strokeWidth={1.8} />
                          </div>
                          <div className="min-w-0">
                            <h3 className="mcp-server-name truncate text-sm font-semibold sm:text-base">{server.name}</h3>
                            <p className="mcp-server-meta mt-1 flex items-center gap-1.5 text-xs">
                              <TransportIcon className="h-3.5 w-3.5" />
                              {TRANSPORT_LABELS[server.transport]} transport
                            </p>
                          </div>
                        </div>
                        <Switch
                          checked={server.enabled}
                          disabled={busyId === server.id}
                          className="mcp-server-switch"
                          aria-label={`${server.name} 启用状态`}
                          onCheckedChange={() => void handleToggle(server.id)}
                        />
                      </div>

                      <div className="mt-5 flex flex-wrap items-center gap-x-4 gap-y-2">
                        <span className={`mcp-status-chip ${getStatusClass(server.status)}`}>
                          <StatusIcon className="h-3.5 w-3.5" />
                          <span>{STATUS_LABELS[server.status]}</span>
                          <span className="mcp-status-code font-mono text-[10px]">{server.status}</span>
                        </span>
                        <span className="mcp-server-meta flex items-center gap-1.5 text-xs">
                          <Wrench className="h-3.5 w-3.5" />
                          {server.toolCount} 个工具
                        </span>
                      </div>
                    </article>
                  );
                })}
              </div>
            ) : servers.length ? (
              <div className="mcp-empty-state mcp-empty-state--filtered py-16">
                <div className="text-center">
                  <Search className="mcp-empty-icon mx-auto h-9 w-9" />
                  <p className="mcp-management-title mt-3 text-sm font-medium">没有符合条件的 MCP 服务器</p>
                  <Button variant="ghost" size="sm" className="mcp-filter-reset mt-3" onClick={resetFilters}>清除筛选</Button>
                </div>
              </div>
            ) : (
              <div className="mcp-empty-state">
                <div className="text-center">
                  <PackageOpen className="mcp-empty-icon mx-auto h-10 w-10" strokeWidth={1.7} />
                  <p className="mcp-management-title mt-3 text-sm font-medium">尚未配置 MCP 服务器。</p>
                </div>
              </div>
            )}
          </div>
        </section>
      </div>

      <Dialog open={dialogOpen} onOpenChange={(open) => { setDialogOpen(open); if (!open) setFormError(null); }}>
        <DialogContent className="mcp-dialog max-w-[480px]">
          <DialogHeader className="text-left">
            <DialogTitle>添加 MCP</DialogTitle>
            <DialogDescription className="mcp-muted">演示页仅保存服务名称与传输方式。</DialogDescription>
          </DialogHeader>
          <form onSubmit={(event) => void handleAdd(event)} className="contents">
            <DialogBody className="mt-5 space-y-5">
              <div className="space-y-2">
                <Label htmlFor="mcp-name">服务名称</Label>
                <Input id="mcp-name" value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：Workspace Files" className="mcp-dialog-input" autoFocus />
              </div>
              <div className="space-y-2">
                <Label htmlFor="mcp-transport">传输方式</Label>
                <Select value={transport} onValueChange={(value) => setTransport(value as DemoMcpServer['transport'])}>
                  <SelectTrigger id="mcp-transport" className="mcp-dialog-select"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="stdio">STDIO · 本地进程</SelectItem>
                    <SelectItem value="http">HTTP · 远程服务</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {formError ? <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">{formError}</p> : null}
            </DialogBody>
            <DialogFooter className="mt-6 gap-2 sm:space-x-0">
              <Button type="button" variant="outline" className="mcp-action-button" onClick={() => setDialogOpen(false)}>取消</Button>
              <Button type="submit" variant="default" className="mcp-primary-button" disabled={formBusy}>
                {formBusy ? <LoaderCircle className="animate-spin" /> : <Plus />}
                {formBusy ? '添加中…' : '添加 MCP'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}
