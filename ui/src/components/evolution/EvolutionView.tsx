/**
 * 进化页面 —— 参考 agent-diva EvolutionView 的治理结构：
 * Skill 权威（evolution_managed）/ 待审请求（接受 · 拒绝 · stale）/ AutoDream 运行记录。
 * 数据来自本地演示层（demo-api），与技能页共享同一份存储。
 */

import { useState } from 'react';
import type { SkillDocument, SkillDto, SkillHistoryEntry, SkillHistoryDocument, SkillRequest, SkillRequestStatus, CreateSkillRequestPayload, AutoDreamRunRecord, AutoDreamRunEvent, AutoDreamRunState } from '@/lib/types';
import { useEvolution } from '@/hooks/useEvolution';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { MasterDetail } from '@/components/layout/MasterDetail';
import { useTranslation } from '@/i18n';
import { Dna, GitBranch, History, Plus, RefreshCw, ShieldCheck, X } from 'lucide-react';

type BadgeVariant = 'default' | 'secondary' | 'destructive' | 'outline';

const REQUEST_STATUS_VARIANT: Record<SkillRequestStatus, BadgeVariant> = {
  pending: 'default',
  accepted: 'secondary',
  rejected: 'destructive',
  stale: 'outline',
};

const RUN_STATE_VARIANT: Record<AutoDreamRunState, BadgeVariant> = {
  pending: 'default',
  running: 'default',
  completed: 'secondary',
  cancelled: 'destructive',
  failed: 'destructive',
};

function formatDateTime(iso: string | null | undefined, placeholder: string) {
  return iso ? new Date(iso).toLocaleString() : placeholder;
}

function masterListShell(children: React.ReactNode) {
  return (
    <ScrollArea className="h-full min-h-0 rounded-xl border bg-card">
      <div className="space-y-1 p-2">{children}</div>
    </ScrollArea>
  );
}

function selectHintShell(icon: React.ReactNode, text: string) {
  return (
    <div className="flex h-full min-h-64 items-center justify-center rounded-xl border bg-card p-6 text-center text-muted-foreground">
      <div>
        <div className="mx-auto mb-3 h-10 w-10 opacity-50">{icon}</div>
        {text}
      </div>
    </div>
  );
}

// ==================== Skill 权威 ====================

interface SkillDetailPaneProps {
  doc: SkillDocument;
  summary: SkillDto | undefined;
  history: SkillHistoryEntry[];
  historyPreview: SkillHistoryDocument | null;
  isBusy: (key: string) => boolean;
  onSave: (slug: string, markdown: string, baseHash: string) => Promise<boolean>;
  onToggleEnabled: (slug: string, enabled: boolean) => Promise<void>;
  onRemove: (slug: string) => Promise<void>;
  onPreviewHistory: (slug: string, revision: number) => Promise<void>;
}

function SkillDetailPane({ doc, summary, history, historyPreview, isBusy, onSave, onToggleEnabled, onRemove, onPreviewHistory }: SkillDetailPaneProps) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(doc.markdown);
  const [historyOpen, setHistoryOpen] = useState(false);
  const busy = isBusy(`save:${doc.slug}`) || isBusy(`toggle:${doc.slug}`) || isBusy(`delete:${doc.slug}`);

  const beginEdit = () => {
    setDraft(doc.markdown);
    setEditing(true);
  };

  return (
    <article className="h-full overflow-auto rounded-xl border bg-card p-4 sm:p-6">
      <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-xl font-semibold">{summary?.name ?? doc.slug}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{doc.description || doc.slug}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Badge variant="outline">{doc.source === 'builtin' ? t('skills.builtin') : t('skills.user')}</Badge>
          <Badge variant={doc.enabled ? 'default' : 'secondary'}>
            {doc.enabled ? t('common.enabled') : t('common.disabled')}
          </Badge>
          {doc.always && <Badge variant="secondary">{t('skills.alwaysLoaded')}</Badge>}
        </div>
      </div>

      <div className="mb-5 grid gap-3 text-sm sm:grid-cols-2">
        <p className="min-w-0 break-all font-mono text-xs text-muted-foreground">{t('skills.contentHash')}{doc.content_hash}</p>
        <p className="text-xs text-muted-foreground">{t('skills.updatedAt')}{new Date(doc.updated_at).toLocaleString()}</p>
      </div>

      <div className="mb-4 flex flex-wrap gap-2">
        {editing ? (
          <>
            <Button size="sm" disabled={busy} onClick={() => void onSave(doc.slug, draft, doc.content_hash).then((ok) => ok && setEditing(false))}>
              {t('common.save')}
            </Button>
            <Button size="sm" variant="outline" disabled={busy} onClick={() => setEditing(false)}>
              <X className="mr-1 h-4 w-4" />
              {t('common.cancel')}
            </Button>
          </>
        ) : (
          <Button size="sm" variant="outline" disabled={busy} onClick={beginEdit}>
            {t('common.edit')}
          </Button>
        )}
        <Button
          size="sm"
          variant="outline"
          disabled={busy}
          onClick={() => void onToggleEnabled(doc.slug, !doc.enabled)}
        >
          {doc.enabled ? t('evolution.skills.disable') : t('evolution.skills.enable')}
        </Button>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button size="sm" variant="destructive" disabled={busy || !doc.can_hard_delete}>
              {t('evolution.skills.hardDelete')}
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t('evolution.skills.hardDeleteTitle')}</AlertDialogTitle>
              <AlertDialogDescription>{t('evolution.skills.hardDeleteConfirm', { slug: doc.slug })}</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
              <AlertDialogAction onClick={() => void onRemove(doc.slug)}>{t('common.delete')}</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>

      <div className="rounded-lg bg-muted p-4">
        {editing ? (
          <Textarea
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            className="min-h-72 font-mono text-sm"
            aria-label={t('common.edit')}
          />
        ) : (
          <pre className="whitespace-pre-wrap break-words font-sans text-sm leading-6">{doc.markdown}</pre>
        )}
      </div>

      <Button variant="ghost" size="sm" className="mt-4 gap-1" onClick={() => setHistoryOpen((open) => !open)}>
        <History className="h-4 w-4" />
        {t('evolution.skills.history')}
      </Button>
      {historyOpen ? (
        <div className="mt-2 space-y-2">
          {history.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('evolution.skills.historyEmpty')}</p>
          ) : (
            history.map((entry) => (
              <div key={entry.revision} className="rounded-lg border p-3">
                <Button variant="ghost" size="sm" className="h-auto justify-start py-1 font-mono text-xs" onClick={() => void onPreviewHistory(doc.slug, entry.revision)}>
                  #{entry.revision} · {entry.content_hash.slice(0, 12)} · {new Date(entry.updated_at).toLocaleString()}
                </Button>
              </div>
            ))
          )}
          {historyPreview ? (
            <div className="rounded-lg bg-muted p-4">
              <p className="mb-2 text-xs font-medium text-muted-foreground">
                {t('evolution.skills.historyPreview', { revision: historyPreview.revision })}
              </p>
              <pre className="whitespace-pre-wrap break-words font-sans text-sm leading-6">{historyPreview.markdown}</pre>
            </div>
          ) : null}
        </div>
      ) : null}
    </article>
  );
}

// ==================== 新建待审请求 ====================

const CREATE_MARKDOWN_TEMPLATE = '---\nname: \ndescription: \nenabled: true\nalways: false\n---\n\n# Skill\n';

interface CreateRequestPanelProps {
  skills: SkillDto[];
  busy: boolean;
  onSubmit: (payload: CreateSkillRequestPayload) => Promise<SkillRequest | null>;
  onCancel: () => void;
}

function CreateRequestPanel({ skills, busy, onSubmit, onCancel }: CreateRequestPanelProps) {
  const { t } = useTranslation();
  const [slug, setSlug] = useState('');
  const [title, setTitle] = useState('');
  const [reason, setReason] = useState('');
  const [attestation, setAttestation] = useState('');
  const [markdown, setMarkdown] = useState(CREATE_MARKDOWN_TEMPLATE);
  const [validationError, setValidationError] = useState<string | null>(null);

  const submit = async () => {
    if (!slug.trim() || !title.trim() || !reason.trim()) {
      setValidationError(t('evolution.requests.createRequired'));
      return;
    }
    setValidationError(null);
    const existing = skills.find((skill) => skill.slug === slug.trim());
    await onSubmit({
      slug: slug.trim(),
      title: title.trim(),
      reason: reason.trim(),
      attestation: attestation.trim() || null,
      base_hash: existing?.content_hash ?? 'new-skill',
      proposed_markdown: markdown,
    });
  };

  return (
    <Card className="mb-4">
      <CardHeader>
        <CardTitle className="text-base">{t('evolution.requests.create')}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1.5">
            <Label htmlFor="evolution-create-slug">{t('evolution.requests.createSlug')}</Label>
            <Input id="evolution-create-slug" value={slug} onChange={(event) => setSlug(event.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="evolution-create-title">{t('evolution.requests.createTitle')}</Label>
            <Input id="evolution-create-title" value={title} onChange={(event) => setTitle(event.target.value)} />
          </div>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="evolution-create-reason">{t('evolution.requests.createReason')}</Label>
          <Input id="evolution-create-reason" value={reason} onChange={(event) => setReason(event.target.value)} />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="evolution-create-attestation">{t('evolution.requests.createAttestation')}</Label>
          <Input
            id="evolution-create-attestation"
            value={attestation}
            placeholder={t('evolution.requests.createAttestationHint')}
            onChange={(event) => setAttestation(event.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="evolution-create-markdown">{t('evolution.requests.createMarkdown')}</Label>
          <Textarea
            id="evolution-create-markdown"
            value={markdown}
            onChange={(event) => setMarkdown(event.target.value)}
            className="min-h-48 font-mono text-sm"
          />
        </div>
        {validationError ? <p className="text-sm text-destructive">{validationError}</p> : null}
        <div className="flex justify-end gap-2">
          <Button variant="outline" size="sm" disabled={busy} onClick={onCancel}>
            {t('common.cancel')}
          </Button>
          <Button size="sm" disabled={busy} onClick={() => void submit()}>
            {t('evolution.requests.createSubmit')}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

// ==================== 待审请求详情 ====================

interface RequestDetailPaneProps {
  request: SkillRequest;
  isBusy: (key: string) => boolean;
  onAccept: (id: string) => Promise<boolean>;
  onReject: (id: string) => Promise<boolean>;
}

function RequestDetailPane({ request, isBusy, onAccept, onReject }: RequestDetailPaneProps) {
  const { t } = useTranslation();
  const busy = isBusy(`accept:${request.id}`) || isBusy(`reject:${request.id}`);

  return (
    <article className="h-full overflow-auto rounded-xl border bg-card p-4 sm:p-6">
      <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-xl font-semibold">{request.title}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{request.slug} · {request.reason}</p>
        </div>
        <Badge variant={REQUEST_STATUS_VARIANT[request.status]}>
          {t(`skills.status.${request.status}`)}
        </Badge>
      </div>

      <div className="mb-5 flex flex-wrap gap-2">
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="outline" size="sm" disabled={busy || request.status !== 'pending'}>
              {t('common.reject')}
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t('evolution.requests.rejectTitle')}</AlertDialogTitle>
              <AlertDialogDescription>{t('evolution.requests.rejectConfirm', { title: request.title })}</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
              <AlertDialogAction onClick={() => void onReject(request.id)}>{t('common.reject')}</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
        <Button size="sm" disabled={busy || request.status !== 'pending'} onClick={() => void onAccept(request.id)}>
          {t('common.accept')}
        </Button>
      </div>

      {request.status === 'stale' ? (
        <p className="mb-4 rounded-lg border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
          {t('evolution.requests.staleNotice')}
        </p>
      ) : null}
      {request.status === 'accepted' || request.status === 'rejected' ? (
        <p className="mb-4 rounded-lg bg-muted p-3 text-sm text-muted-foreground">{t('evolution.requests.handledNotice')}</p>
      ) : null}

      <div className="mb-5 grid gap-3 text-xs text-muted-foreground sm:grid-cols-2">
        <p className="min-w-0 break-all font-mono">{t('evolution.requests.baseHash', { hash: request.base_hash })}</p>
        <p>{t('evolution.requests.createdAt', { time: new Date(request.created_at).toLocaleString() })}</p>
      </div>

      <div className="rounded-lg bg-muted p-4">
        <pre className="whitespace-pre-wrap break-words font-sans text-sm leading-6">{request.proposed_markdown}</pre>
      </div>

      <Card className="mt-5">
        <CardHeader>
          <CardTitle className="text-sm">{t('evolution.requests.evidence')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {request.attestation ? (
            <p className="text-sm">{t('evolution.requests.attestation', { text: request.attestation })}</p>
          ) : null}
          {request.evidence.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('evolution.requests.evidenceEmpty')}</p>
          ) : (
            request.evidence.map((item, index) => (
              <pre key={index} className="overflow-auto rounded-lg border bg-muted p-3 font-mono text-xs">
                {JSON.stringify(item, null, 2)}
              </pre>
            ))
          )}
        </CardContent>
      </Card>
    </article>
  );
}

// ==================== AutoDream 运行详情 ====================

interface RunDetailPaneProps {
  run: AutoDreamRunRecord;
  events: AutoDreamRunEvent[];
  onOpenRequests: (requestId: string | null) => void;
}

function RunDetailPane({ run, events, onOpenRequests }: RunDetailPaneProps) {
  const { t } = useTranslation();
  const phaseLabel = run.orchestration
    ? t(`evolution.autodream.phases.${run.orchestration.phase}`)
    : t(`evolution.autodream.states.${run.state}`);
  const firstProposalId = run.proposal_ids[0] ?? null;
  const noValue = t('evolution.autodream.noValue');

  return (
    <article className="h-full overflow-auto rounded-xl border bg-card p-4 sm:p-6">
      <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="break-all font-mono text-lg font-semibold">{run.id}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{run.summary ?? 'AutoDream'}</p>
        </div>
        <Badge variant={RUN_STATE_VARIANT[run.state]}>{t(`evolution.autodream.states.${run.state}`)}</Badge>
      </div>

      <div className="mb-5 grid gap-3 text-sm sm:grid-cols-2">
        <p className="text-muted-foreground">{t('evolution.autodream.trigger', { value: run.trigger })}</p>
        <p className="text-muted-foreground">{t('evolution.autodream.phase', { value: phaseLabel })}</p>
        <p className="text-muted-foreground">{t('evolution.autodream.startedAt', { time: formatDateTime(run.started_at, noValue) })}</p>
        <p className="text-muted-foreground">{t('evolution.autodream.completedAt', { time: formatDateTime(run.completed_at, noValue) })}</p>
        <p className="text-muted-foreground">{t('evolution.autodream.attempt', { count: run.orchestration?.attempt ?? 0 })}</p>
        {run.error ? (
          <p className="text-destructive">{t('evolution.autodream.failure', { reason: run.error })}</p>
        ) : null}
      </div>

      {run.input_summary ? (
        <p className="mb-5 text-sm text-muted-foreground">
          {t('evolution.autodream.inputs', { items: run.input_summary.total_items, bytes: run.input_summary.total_bytes })}
        </p>
      ) : null}

      {run.proposal_ids.length > 0 ? (
        <Button variant="outline" size="sm" className="mb-5" onClick={() => onOpenRequests(firstProposalId)}>
          {t('evolution.autodream.openRequests')}
        </Button>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">{t('evolution.autodream.events')}</CardTitle>
        </CardHeader>
        <CardContent>
          {events.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('evolution.autodream.eventsEmpty')}</p>
          ) : (
            <ol className="space-y-2">
              {events.map((event) => (
                <li key={event.id} className="grid gap-1 rounded-lg border p-3 text-xs sm:grid-cols-[10rem_8rem_minmax(0,1fr)] sm:items-baseline sm:gap-3">
                  <time className="text-muted-foreground">{new Date(event.created_at).toLocaleString()}</time>
                  <span className="font-mono font-semibold text-primary">{event.kind}</span>
                  <span className="text-muted-foreground">{event.message}</span>
                </li>
              ))}
            </ol>
          )}
        </CardContent>
      </Card>
    </article>
  );
}

// ==================== 页面主体 ====================

export function EvolutionView() {
  const { t } = useTranslation();
  const {
    skills,
    evolutionSkills,
    requests,
    pendingCount,
    runs,
    selectedSkill,
    history,
    historyPreview,
    selectedRequestId,
    selectedRunId,
    runEvents,
    isLoading,
    error,
    isBusy,
    refresh,
    selectSkill,
    clearSelectedSkill,
    saveSkill,
    toggleSkillEnabled,
    removeSkill,
    previewHistory,
    createRequest,
    acceptRequest,
    rejectRequest,
    selectRequest,
    clearSelectedRequest,
    selectRun,
    clearSelectedRun,
  } = useEvolution();
  const [tab, setTab] = useState('requests');
  const [createOpen, setCreateOpen] = useState(false);

  const selectedRequest = requests.find((request) => request.id === selectedRequestId) ?? null;
  const selectedRun = runs.find((run) => run.id === selectedRunId) ?? null;
  const selectedSkillSummary = skills.find((skill) => skill.slug === selectedSkill?.slug);
  const noData = evolutionSkills.length === 0 && requests.length === 0 && runs.length === 0;

  if (isLoading && noData) {
    return <div className="p-6 text-sm text-muted-foreground">{t('evolution.loading')}</div>;
  }

  if (error && noData) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <Card className="max-w-md">
          <CardHeader>
            <CardTitle>{t('evolution.loadFailed')}</CardTitle>
            <CardDescription>{error}</CardDescription>
          </CardHeader>
          <CardContent>
            <Button onClick={() => void refresh()}>{t('common.retry')}</Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  const openRequestsFromRun = (requestId: string | null) => {
    if (requestId) {
      selectRequest(requestId);
    }
    setTab('requests');
  };

  return (
    <Tabs value={tab} onValueChange={setTab} className="flex h-full min-h-0 flex-col p-4 sm:p-6">
      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <TabsList>
          <TabsTrigger value="skills">{t('evolution.tabs.skills')} ({evolutionSkills.length})</TabsTrigger>
          <TabsTrigger value="requests">{t('evolution.tabs.requests')} ({pendingCount})</TabsTrigger>
          <TabsTrigger value="autodream">{t('evolution.tabs.autodream')} ({runs.length})</TabsTrigger>
        </TabsList>
        <div className="flex gap-2">
          {tab === 'requests' ? (
            <Button variant="outline" size="sm" onClick={() => setCreateOpen((open) => !open)}>
              <Plus className="mr-2 h-4 w-4" />
              {t('evolution.requests.create')}
            </Button>
          ) : null}
          <Button variant="outline" size="sm" disabled={isLoading} onClick={() => void refresh()}>
            <RefreshCw className="mr-2 h-4 w-4" />
            {t('common.refresh')}
          </Button>
        </div>
      </div>

      {error && !noData ? <p className="mb-4 text-sm text-destructive">{error}</p> : null}

      <TabsContent value="skills" className="mt-0 min-h-0 flex-1">
        {evolutionSkills.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center text-muted-foreground">
              <Dna className="mx-auto mb-3 h-10 w-10 opacity-50" />
              {t('evolution.skills.empty')}
            </CardContent>
          </Card>
        ) : (
          <MasterDetail
            selected={selectedSkill !== null}
            onBack={clearSelectedSkill}
            columnsClassName="md:grid-cols-[minmax(16rem,0.8fr)_minmax(0,1.6fr)] md:gap-4"
            master={masterListShell(
              evolutionSkills.map((skill) => (
                <button
                  key={skill.slug}
                  type="button"
                  onClick={() => void selectSkill(skill.slug)}
                  className={`w-full rounded-lg p-3 text-left transition-colors hover:bg-accent ${
                    selectedSkill?.slug === skill.slug ? 'bg-accent' : ''
                  }`}
                >
                  <div className="flex items-start justify-between gap-2">
                    <span className="min-w-0 font-medium">{skill.name}</span>
                    <Badge variant="outline">{skill.source === 'builtin' ? t('skills.builtin') : t('skills.user')}</Badge>
                  </div>
                  <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{skill.description}</p>
                  <p className={`mt-2 text-xs ${skill.enabled ? 'text-muted-foreground' : 'text-muted-foreground line-through'}`}>
                    {skill.enabled ? t('common.enabled') : t('common.disabled')}
                    {skill.always ? ` · ${t('skills.alwaysLoaded')}` : ''}
                  </p>
                </button>
              )),
            )}
            detail={
              selectedSkill ? (
                <SkillDetailPane
                  key={selectedSkill.slug}
                  doc={selectedSkill}
                  summary={selectedSkillSummary}
                  history={history}
                  historyPreview={historyPreview}
                  isBusy={isBusy}
                  onSave={saveSkill}
                  onToggleEnabled={toggleSkillEnabled}
                  onRemove={removeSkill}
                  onPreviewHistory={previewHistory}
                />
              ) : (
                selectHintShell(<ShieldCheck className="h-full w-full" />, t('evolution.skills.selectHint'))
              )
            }
          />
        )}
      </TabsContent>

      <TabsContent value="requests" className="mt-0 min-h-0 flex-1">
        {createOpen ? (
          <CreateRequestPanel
            skills={skills}
            busy={isBusy('create-request')}
            onSubmit={async (payload) => {
              const created = await createRequest(payload);
              if (created) {
                setCreateOpen(false);
                selectRequest(created.id);
                return created;
              }
              return null;
            }}
            onCancel={() => setCreateOpen(false)}
          />
        ) : null}
        {requests.length === 0 && !createOpen ? (
          <Card>
            <CardContent className="py-12 text-center text-muted-foreground">
              <ShieldCheck className="mx-auto mb-3 h-10 w-10 opacity-50" />
              {t('evolution.requests.empty')}
            </CardContent>
          </Card>
        ) : (
          <MasterDetail
            selected={selectedRequest !== null}
            onBack={clearSelectedRequest}
            columnsClassName="md:grid-cols-[minmax(16rem,0.8fr)_minmax(0,1.6fr)] md:gap-4"
            master={masterListShell(
              requests.map((request) => (
                <button
                  key={request.id}
                  type="button"
                  onClick={() => selectRequest(request.id)}
                  className={`w-full rounded-lg p-3 text-left transition-colors hover:bg-accent ${
                    selectedRequest?.id === request.id ? 'bg-accent' : ''
                  }`}
                >
                  <div className="flex items-start justify-between gap-2">
                    <span className="min-w-0 font-medium">{request.title}</span>
                    <Badge variant={REQUEST_STATUS_VARIANT[request.status]}>
                      {t(`skills.status.${request.status}`)}
                    </Badge>
                  </div>
                  <p className="mt-1 truncate text-sm text-muted-foreground">{request.slug} · {request.source}</p>
                  <p className="mt-2 text-xs text-muted-foreground">{new Date(request.updated_at).toLocaleString()}</p>
                </button>
              )),
            )}
            detail={
              selectedRequest ? (
                <RequestDetailPane request={selectedRequest} isBusy={isBusy} onAccept={acceptRequest} onReject={rejectRequest} />
              ) : (
                selectHintShell(<ShieldCheck className="h-full w-full" />, t('evolution.requests.selectHint'))
              )
            }
          />
        )}
      </TabsContent>

      <TabsContent value="autodream" className="mt-0 min-h-0 flex-1">
        {runs.length === 0 ? (
          <Card>
            <CardContent className="py-12 text-center text-muted-foreground">
              <GitBranch className="mx-auto mb-3 h-10 w-10 opacity-50" />
              {t('evolution.autodream.empty')}
            </CardContent>
          </Card>
        ) : (
          <MasterDetail
            selected={selectedRun !== null}
            onBack={clearSelectedRun}
            columnsClassName="md:grid-cols-[minmax(16rem,0.8fr)_minmax(0,1.6fr)] md:gap-4"
            master={masterListShell(
              runs.map((run) => (
                <button
                  key={run.id}
                  type="button"
                  onClick={() => void selectRun(run.id)}
                  className={`w-full rounded-lg p-3 text-left transition-colors hover:bg-accent ${
                    selectedRun?.id === run.id ? 'bg-accent' : ''
                  }`}
                >
                  <div className="flex items-start justify-between gap-2">
                    <span className="min-w-0 break-all font-mono text-sm font-medium">{run.id}</span>
                    <Badge variant={RUN_STATE_VARIANT[run.state]}>{t(`evolution.autodream.states.${run.state}`)}</Badge>
                  </div>
                  <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{run.summary ?? run.trigger}</p>
                  <p className="mt-2 text-xs text-muted-foreground">{new Date(run.started_at).toLocaleString()}</p>
                </button>
              )),
            )}
            detail={
              selectedRun ? (
                <RunDetailPane run={selectedRun} events={runEvents} onOpenRequests={openRequestsFromRun} />
              ) : (
                selectHintShell(<GitBranch className="h-full w-full" />, t('evolution.autodream.selectHint'))
              )
            }
          />
        )}
      </TabsContent>
    </Tabs>
  );
}
