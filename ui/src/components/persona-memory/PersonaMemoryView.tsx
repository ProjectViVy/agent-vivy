/**
 * 角色记忆视图组件 - Persona Memory
 * 管理 7 份 Persona Markdown 文档，支持当前文档 / 待审变更 / 历史 三个视图
 */

import { useState, useMemo, useEffect } from 'react';
import type { PersonaKind, PersonaDocument, PersonaHistoryEntry, PersonaChangeRequest } from '@/lib/types';
import {
  getPersonaDocument,
  savePersonaDocument,
  listPersonaHistory,
  getPersonaHistoryRevision,
  listPersonaRequests,
  acceptPersonaRequest,
  rejectPersonaRequest,
} from '@/lib/demo-api';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Save, Eye, Code2, History, Loader2, Check, X, GitPullRequest } from 'lucide-react';

const PERSONA_KINDS: PersonaKind[] = ['identity', 'relationship', 'redline', 'user', 'world', 'dream', 'dark'];

const KIND_LABELS: Record<PersonaKind, string> = {
  identity: 'IDENTITY.MD',
  relationship: 'RELATIONSHIP.MD',
  redline: 'REDLINE.MD',
  user: 'USER.MD',
  world: 'WORLD.MD',
  dream: 'DREAM.MD',
  dark: 'DARK.MD',
};

export function PersonaMemoryView() {
  const [selectedKind, setSelectedKind] = useState<PersonaKind>('identity');
  const [tab, setTab] = useState<'current' | 'pending' | 'history'>('current');
  const [mode, setMode] = useState<'source' | 'preview'>('source');
  const [document, setDocument] = useState<PersonaDocument | null>(null);
  const [draft, setDraft] = useState('');
  const [history, setHistory] = useState<PersonaHistoryEntry[]>([]);
  const [requests, setRequests] = useState<PersonaChangeRequest[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const dirty = document !== null && draft !== document.content;

  // 简单的 Markdown 预览渲染
  const renderedPreview = useMemo(() => {
    if (!draft) return '';
    return draft
      .split('\n')
      .map((line) => {
        if (line.startsWith('# ')) return `<h1 class="text-2xl font-bold mt-4 mb-2">${line.slice(2)}</h1>`;
        if (line.startsWith('## ')) return `<h2 class="text-xl font-semibold mt-3 mb-2">${line.slice(3)}</h2>`;
        if (line.startsWith('- ')) return `<li class="ml-4">${line.slice(2)}</li>`;
        if (line.trim() === '') return '<br/>';
        return `<p>${line}</p>`;
      })
      .join('');
  }, [draft]);

  const loadCurrent = async (kind: PersonaKind) => {
    setLoading(true);
    setError('');
    try {
      const doc = await getPersonaDocument(kind);
      setDocument(doc);
      setDraft(doc.content);
      const hist = await listPersonaHistory(kind);
      setHistory(hist);
      const reqs = await listPersonaRequests(kind);
      setRequests(reqs);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败');
    } finally {
      setLoading(false);
    }
  };

  const handleSelectKind = async (kind: PersonaKind) => {
    if (kind === selectedKind) return;
    if (dirty && !confirm('当前有未保存的更改，是否放弃？')) return;
    setSelectedKind(kind);
    setTab('current');
    setMode('source');
    await loadCurrent(kind);
  };

  const handleSave = async () => {
    if (!document || !dirty) return;
    setSaving(true);
    setError('');
    try {
      const result = await savePersonaDocument(selectedKind, draft, document.revision);
      setDocument(result.document);
      setDraft(result.document.content);
      const hist = await listPersonaHistory(selectedKind);
      setHistory(hist);
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const handleViewHistory = async (revision: number) => {
    try {
      const rev = await getPersonaHistoryRevision(selectedKind, revision);
      setDraft(rev.content);
      setMode('preview');
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载历史版本失败');
    }
  };

  const handleAcceptRequest = async (requestId: string) => {
    try {
      await acceptPersonaRequest(requestId);
      const reqs = await listPersonaRequests(selectedKind);
      setRequests(reqs);
    } catch (err) {
      setError(err instanceof Error ? err.message : '接受失败');
    }
  };

  const handleRejectRequest = async (requestId: string) => {
    try {
      await rejectPersonaRequest(requestId);
      const reqs = await listPersonaRequests(selectedKind);
      setRequests(reqs);
    } catch (err) {
      setError(err instanceof Error ? err.message : '拒绝失败');
    }
  };

  // 初始加载
  useEffect(() => {
    loadCurrent('identity');
  }, []);

  if (loading && !document) {
    return (
      <div className="flex items-center justify-center h-full">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col">
      {/* 顶部文档标签栏 */}
      <div className="border-b p-2 overflow-x-auto">
        <div className="flex gap-1 min-w-max">
          {PERSONA_KINDS.map((kind) => (
            <Button
              key={kind}
              variant={selectedKind === kind ? 'default' : 'ghost'}
              size="sm"
              onClick={() => handleSelectKind(kind)}
              className="shrink-0 gap-1.5"
            >
              {KIND_LABELS[kind]}
              {document?.kind === kind && (
                <span className="text-[10px] opacity-70">r{document.revision}</span>
              )}
            </Button>
          ))}
        </div>
      </div>

      {/* 内容区 */}
      <div className="flex-1 flex flex-col overflow-hidden">
        <Tabs value={tab} onValueChange={(v) => setTab(v as typeof tab)} className="flex-1 flex flex-col">
          <div className="px-4 pt-4 flex items-center justify-between">
            <TabsList>
              <TabsTrigger value="current">当前文档</TabsTrigger>
              <TabsTrigger value="pending" className="gap-1.5">
                待审变更
                {requests.filter((r) => r.state === 'pending').length > 0 && (
                  <Badge variant="secondary" className="h-4 px-1 text-[10px]">
                    {requests.filter((r) => r.state === 'pending').length}
                  </Badge>
                )}
              </TabsTrigger>
              <TabsTrigger value="history">历史</TabsTrigger>
            </TabsList>
            {tab === 'current' && (
              <div className="flex gap-2">
                <Button
                  variant={mode === 'source' ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setMode('source')}
                >
                  <Code2 className="h-4 w-4 mr-1" />
                  源码
                </Button>
                <Button
                  variant={mode === 'preview' ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setMode('preview')}
                >
                  <Eye className="h-4 w-4 mr-1" />
                  预览
                </Button>
                {dirty && (
                  <Button onClick={handleSave} disabled={saving} size="sm">
                    {saving ? <Loader2 className="h-4 w-4 mr-1 animate-spin" /> : <Save className="h-4 w-4 mr-1" />}
                    保存
                  </Button>
                )}
              </div>
            )}
          </div>

          <TabsContent value="current" className="flex-1 m-0 p-4 overflow-hidden">
            {error && (
              <div className="mb-2 p-2 bg-destructive/10 text-destructive rounded text-sm">{error}</div>
            )}
            {mode === 'source' ? (
              <Textarea
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                className="flex-1 font-mono text-sm resize-none h-full"
                placeholder="编辑 Persona 文档内容..."
              />
            ) : (
              <ScrollArea className="flex-1 border rounded-lg p-4 h-full">
                <div
                  className="prose prose-sm dark:prose-invert max-w-none"
                  dangerouslySetInnerHTML={{ __html: renderedPreview }}
                />
              </ScrollArea>
            )}
          </TabsContent>

          <TabsContent value="pending" className="flex-1 m-0 p-4 overflow-hidden">
            <ScrollArea className="h-full">
              {requests.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
                  <GitPullRequest className="h-10 w-10 mb-3 opacity-40" />
                  <p className="text-sm">暂无待审变更</p>
                </div>
              ) : (
                <div className="space-y-3">
                  {requests.map((req) => (
                    <Card key={req.id}>
                      <CardContent className="p-4">
                        <div className="flex items-center justify-between mb-2">
                          <div className="flex items-center gap-2">
                            <Badge
                              variant={
                                req.state === 'pending'
                                  ? 'secondary'
                                  : req.state === 'accepted'
                                    ? 'default'
                                    : 'destructive'
                              }
                            >
                              {req.state === 'pending' ? '待审' : req.state === 'accepted' ? '已接受' : '已拒绝'}
                            </Badge>
                            <span className="text-xs text-muted-foreground">
                              {new Date(req.created_at).toLocaleString()}
                            </span>
                          </div>
                          {req.state === 'pending' && (
                            <div className="flex gap-2">
                              <Button size="sm" variant="outline" onClick={() => handleAcceptRequest(req.id)}>
                                <Check className="h-4 w-4 mr-1" />
                                接受
                              </Button>
                              <Button size="sm" variant="outline" onClick={() => handleRejectRequest(req.id)}>
                                <X className="h-4 w-4 mr-1" />
                                拒绝
                              </Button>
                            </div>
                          )}
                        </div>
                        {req.reason && (
                          <p className="text-sm text-muted-foreground mb-2">原因：{req.reason}</p>
                        )}
                        <pre className="text-xs bg-muted p-3 rounded-lg whitespace-pre-wrap overflow-x-auto">
                          {req.proposed_content}
                        </pre>
                      </CardContent>
                    </Card>
                  ))}
                </div>
              )}
            </ScrollArea>
          </TabsContent>

          <TabsContent value="history" className="flex-1 m-0 p-4 overflow-hidden">
            <ScrollArea className="h-full">
              <div className="space-y-2">
                {history.length === 0 ? (
                  <p className="text-muted-foreground text-center py-8">暂无历史记录</p>
                ) : (
                  history.map((entry) => (
                    <Card key={entry.revision}>
                      <CardContent className="p-3 flex items-center justify-between">
                        <div>
                          <p className="font-medium">版本 r{entry.revision}</p>
                          <p className="text-xs text-muted-foreground">
                            {new Date(entry.updated_at).toLocaleString()}
                          </p>
                        </div>
                        <Button variant="outline" size="sm" onClick={() => handleViewHistory(entry.revision)}>
                          查看
                        </Button>
                      </CardContent>
                    </Card>
                  ))
                )}
              </div>
            </ScrollArea>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
