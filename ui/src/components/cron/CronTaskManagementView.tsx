import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
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
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { Skeleton } from '@/components/ui/skeleton';
import { CalendarClock, ExternalLink, LoaderCircle, Pencil, Play, Plus, Trash2 } from 'lucide-react';
import { useNavigate } from '@tanstack/react-router';
import { cn } from '@/lib/utils';
import {
  createCronJob, deleteCronJob, listCronJobs, triggerCronJob, updateCronJob,
  type CronJobDto, type CronJobInput, type ScheduleKind,
} from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { MasterDetail } from '@/components/layout/MasterDetail';
import { useTranslation, dateTimeLocale, t } from '@/i18n';

const HOUR_MS = 60 * 60 * 1000;
/** 面板打开期间的轻轮询，刷新 nextRun/isRunning/lastStatus。 */
const REFRESH_INTERVAL_MS = 5000;
const DEFAULT_TZ = 'Asia/Shanghai';
const emptyForm = {
  name: '', enabled: true, scheduleKind: 'cron' as Exclude<ScheduleKind, 'at'>,
  cronExpr: '0 9 * * *', everyHours: 24, message: '',
};
function cronStatusLabel(status: string): string {
  // 后端终态是 ok|error（diva 语义）；ok 展示为“已完成”。
  if (status === 'ok') return t('cron.status.completed');
  const keys: Record<string, string> = {
    running: 'cron.status.running', scheduled: 'cron.status.scheduled', paused: 'cron.status.paused',
    completed: 'cron.status.completed', failed: 'cron.status.failed',
  };
  return keys[status] ? t(keys[status]) : status;
}

function formatSchedule(job: CronJobDto) {
  if (job.schedule.kind === 'cron') return `${job.schedule.expr || t('cron.scheduleFormat.notSet')} · ${job.schedule.tz || t('cron.scheduleFormat.localTz')}`;
  if (job.schedule.kind === 'every') {
    const interval = job.schedule.everyMs || 0;
    if (interval >= 24 * HOUR_MS && interval % (24 * HOUR_MS) === 0) return t('cron.scheduleFormat.everyDays', { count: interval / (24 * HOUR_MS) });
    if (interval >= HOUR_MS && interval % HOUR_MS === 0) return t('cron.scheduleFormat.everyHours', { count: interval / HOUR_MS });
    return t('cron.scheduleFormat.everyMinutes', { count: Math.max(1, Math.round(interval / 60000)) });
  }
  return t('cron.scheduleFormat.once');
}

function formatTime(value?: number | null) {
  if (!value) return '—';
  return new Date(value).toLocaleString(dateTimeLocale(), {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  });
}

function statusClass(status: string) {
  if (status === 'running') return 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-300';
  if (status === 'scheduled') return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300';
  if (status === 'failed') return 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300';
  return 'border-border bg-muted text-muted-foreground';
}

function PageShell({ children }: { children: ReactNode }) {
  return (
    <div className="h-full overflow-y-auto p-4 sm:p-6">
      <div className="mx-auto max-w-5xl space-y-5">{children}</div>
    </div>
  );
}

function CronPageSkeleton() {
  return (
    <PageShell>
      <header className="flex items-center justify-between gap-4">
        <div className="space-y-2"><Skeleton className="h-7 w-28" /><Skeleton className="h-4 w-72 max-w-[70vw]" /></div>
        <Skeleton className="h-9 w-24" />
      </header>
      <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
        <Card><CardHeader className="border-b px-4 py-3"><Skeleton className="h-5 w-20" /></CardHeader><CardContent className="space-y-2 p-2">
          {[1, 2, 3].map((item) => <div key={item} className="rounded-lg px-3 py-3"><Skeleton className="h-4 w-40" /><Skeleton className="mt-2 h-3 w-56" /></div>)}
        </CardContent></Card>
        <Card><CardHeader className="space-y-3 border-b px-5 py-4"><Skeleton className="h-3 w-16" /><Skeleton className="h-6 w-32" /><Skeleton className="h-9 w-full" /></CardHeader><CardContent className="space-y-3 p-5"><Skeleton className="h-20 w-full" /><Skeleton className="h-16 w-full" /><Skeleton className="h-9 w-full" /></CardContent></Card>
      </div>
    </PageShell>
  );
}

export function CronTaskManagementView() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const selectSession = useVivyStore((state) => state.selectSession);
  const attachBackgroundRun = useVivyStore((state) => state.attachBackgroundRun);
  const [jobs, setJobs] = useState<CronJobDto[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [formError, setFormError] = useState('');
  const [busyId, setBusyId] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [showDelete, setShowDelete] = useState(false);
  const [editingJob, setEditingJob] = useState<CronJobDto | null>(null);
  const [formData, setFormData] = useState(emptyForm);
  const refreshingRef = useRef(false);

  const selectedJob = useMemo(
    () => jobs.find((job) => job.id === selectedId) || null,
    [jobs, selectedId],
  );

  const refreshJobs = useCallback(async () => {
    // 静默轮询：不打断初次加载，也不覆盖显式加载的错误呈现。
    if (refreshingRef.current) return;
    refreshingRef.current = true;
    try {
      const data = await listCronJobs();
      setJobs(data.jobs);
      setSelectedId((current) => (data.jobs.some((job) => job.id === current) ? current : null));
    } catch {
      // 轮询失败保持现有内容，下一轮再试。
    } finally {
      refreshingRef.current = false;
    }
  }, []);

  const loadJobs = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const data = await listCronJobs();
      setJobs(data.jobs);
      setSelectedId((current) => (data.jobs.some((job) => job.id === current) ? current : null));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('cron.errors.loadFailed'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { void loadJobs(); }, [loadJobs]);
  useEffect(() => {
    const timer = window.setInterval(() => { void refreshJobs(); }, REFRESH_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [refreshJobs]);

  const openCreate = () => {
    setEditingJob(null); setFormData(emptyForm); setFormError(''); setShowForm(true);
  };

  const openEdit = (job: CronJobDto) => {
    setEditingJob(job);
    setFormData({
      name: job.name,
      enabled: job.enabled,
      scheduleKind: job.schedule.kind === 'every' ? 'every' : 'cron',
      cronExpr: job.schedule.expr || '0 9 * * *',
      everyHours: Math.max(0.25, (job.schedule.everyMs || 24 * HOUR_MS) / HOUR_MS),
      message: job.payload.message,
    });
    setFormError(''); setShowForm(true);
  };

  const handleSubmit = async () => {
    const name = formData.name.trim();
    const cronExpr = formData.cronExpr.trim();
    const message = formData.message.trim();
    if (!name) { setFormError(t('cron.errors.nameRequired')); return; }
    if (formData.scheduleKind === 'cron' && !cronExpr) { setFormError(t('cron.errors.cronRequired')); return; }
    if (formData.scheduleKind === 'every' && (!Number.isFinite(formData.everyHours) || formData.everyHours <= 0)) {
      setFormError(t('cron.errors.intervalPositive')); return;
    }
    if (!message) { setFormError(t('cron.errors.messageRequired')); return; }
    const schedule = formData.scheduleKind === 'cron'
      ? { kind: 'cron' as const, expr: cronExpr, tz: DEFAULT_TZ }
      : { kind: 'every' as const, everyMs: Math.round(formData.everyHours * HOUR_MS) };
    const payload = { kind: 'agent_turn', message, deliver: false };
    const input: CronJobInput = { name, enabled: formData.enabled, schedule, payload, delete_after_run: false };

    setBusyId('save'); setFormError('');
    try {
      const saved = editingJob
        ? (await updateCronJob(editingJob.id, input)).job
        : (await createCronJob(input)).job;
      setJobs((current) => editingJob
        ? current.map((job) => (job.id === saved.id ? saved : job))
        : [...current, saved]);
      setSelectedId(saved.id); setShowForm(false); setEditingJob(null);
    } catch (cause) {
      setFormError(cause instanceof Error ? cause.message : t('cron.errors.saveFailed'));
    } finally {
      setBusyId('');
    }
  };

  const handleToggle = async (job: CronJobDto) => {
    setBusyId(`toggle:${job.id}`); setError('');
    const input: CronJobInput = {
      name: job.name, enabled: !job.enabled, schedule: job.schedule,
      payload: job.payload, delete_after_run: job.deleteAfterRun,
    };
    try {
      const updated = (await updateCronJob(job.id, input)).job;
      setJobs((current) => current.map((item) => (item.id === updated.id ? updated : item)));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('cron.errors.toggleFailed'));
    } finally { setBusyId(''); }
  };

  const handleTrigger = async (job: CronJobDto) => {
    setBusyId(`trigger:${job.id}`); setError('');
    try {
      const result = (await triggerCronJob(job.id)).job;
      setJobs((current) => current.map((item) => (item.id === result.id ? result : item)));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('cron.errors.triggerFailed'));
    } finally { setBusyId(''); }
  };

  const handleDelete = async () => {
    if (!selectedJob) return;
    setBusyId(`delete:${selectedJob.id}`); setError('');
    try {
      await deleteCronJob(selectedJob.id);
      const remaining = jobs.filter((job) => job.id !== selectedJob.id);
      setJobs(remaining); setSelectedId(null); setShowDelete(false);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t('cron.errors.deleteFailed'));
    } finally { setBusyId(''); }
  };

  const openJobSession = async (job: CronJobDto) => {
    if (job.isRunning && job.activeRun) {
      await attachBackgroundRun(job.activeRun.run_id);
    } else if (job.sessionId) {
      await selectSession(job.sessionId);
    } else {
      return;
    }
    await navigate({ to: '/' });
  };

  if (loading) return <CronPageSkeleton />;
  if (error && jobs.length === 0) {
    return (
      <PageShell>
        <header>
          <h1 className="text-2xl font-semibold tracking-tight">{t('cron.title')}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t('cron.subtitle')}</p>
        </header>
        <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          <p>{error}</p>
          <Button variant="outline" size="sm" className="mt-2" onClick={() => void loadJobs()}>{t('common.retry')}</Button>
        </div>
      </PageShell>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col p-4 sm:p-6">
      <div className="mx-auto flex min-h-0 w-full max-w-5xl flex-1 flex-col gap-5">
        <header className="flex shrink-0 flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">{t('cron.title')}</h1>
            <p className="mt-1 text-sm text-muted-foreground">{t('cron.subtitle')}</p>
          </div>
          <Button onClick={openCreate} className="self-start sm:self-auto"><Plus className="mr-2 h-4 w-4" />{t('cron.create')}</Button>
        </header>

        {error ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">{error}</div> : null}

        <MasterDetail
          className="min-h-0 flex-1"
          columnsClassName="md:grid-cols-[minmax(0,1fr)_20rem] md:gap-4"
          selected={selectedJob !== null}
          onBack={() => setSelectedId(null)}
          master={
          <Card className="flex h-full min-h-0 min-w-0 flex-col">
            <CardHeader className="flex-row items-center justify-between space-y-0 border-b px-4 py-3">
              <CardTitle className="text-base">{t('cron.taskList')}</CardTitle><span className="text-xs text-muted-foreground">{t('cron.taskCount', { count: jobs.length })}</span>
            </CardHeader>
            <CardContent className="min-h-0 flex-1 overflow-auto p-2">
              {jobs.length === 0 ? (
                <div className="flex min-h-64 flex-col items-center justify-center gap-3 px-6 text-center">
                  <div className="rounded-full bg-muted p-3 text-muted-foreground"><CalendarClock className="h-5 w-5" /></div>
                  <div><p className="font-medium">{t('cron.emptyTitle')}</p><p className="mt-1 text-sm text-muted-foreground">{t('cron.emptyHint')}</p></div>
                  <Button variant="outline" size="sm" onClick={openCreate}>{t('cron.createTask')}</Button>
                </div>
              ) : (
                <div className="space-y-1">
                  {jobs.map((job) => (
                    <button
                      key={job.id} type="button" onClick={() => setSelectedId(job.id)}
                      className={cn(
                        'flex w-full min-w-0 items-center gap-3 rounded-lg border border-transparent px-3 py-3 text-left transition-colors hover:bg-muted/60',
                        selectedId === job.id && 'border-primary/20 bg-primary/5',
                      )}
                    >
                      <span className="min-w-0 flex-1">
                        <span className="flex items-center gap-2">
                          <span className="truncate font-medium">{job.name}</span>
                          <Badge variant="outline" className={cn('shrink-0 font-normal', statusClass(job.computedStatus))}>{cronStatusLabel(job.computedStatus)}</Badge>
                        </span>
                        <span className="mt-1 flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
                          <CalendarClock className="h-3.5 w-3.5 shrink-0" />
                          <span className="truncate">{formatSchedule(job)}</span>
                        </span>
                      </span>
                      <span className="w-20 shrink-0 text-right text-xs">
                        <span className="block text-muted-foreground">{t('cron.nextRun')}</span>
                        <span className="mt-0.5 block truncate">{formatTime(job.state.nextRunAtMs)}</span>
                      </span>
                    </button>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
          }
          detail={
          <Card className="flex h-full min-h-0 flex-col">
            {selectedJob ? (
              <>
                <CardHeader className="space-y-3 border-b px-5 py-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0"><p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{t('cron.taskDetail')}</p><CardTitle className="mt-1 truncate text-lg">{selectedJob.name}</CardTitle></div>
                    <Badge variant="outline" className={cn('shrink-0 font-normal', statusClass(selectedJob.computedStatus))}>{cronStatusLabel(selectedJob.computedStatus)}</Badge>
                  </div>
                  <div className="flex items-center justify-between rounded-lg bg-muted/60 px-3 py-2">
                    <Label htmlFor="cron-enabled" className="text-sm">{t('cron.enableTask')}</Label>
                    <Switch id="cron-enabled" checked={selectedJob.enabled} disabled={busyId === `toggle:${selectedJob.id}`} onCheckedChange={() => void handleToggle(selectedJob)} />
                  </div>
                </CardHeader>
                <CardContent className="space-y-5 p-5">
                  <dl className="grid grid-cols-[88px_minmax(0,1fr)] gap-x-3 gap-y-3 text-sm">
                    <dt className="text-muted-foreground">{t('cron.schedule')}</dt><dd className="break-words text-right">{formatSchedule(selectedJob)}</dd>
                    <dt className="text-muted-foreground">{t('cron.nextRun')}</dt><dd className="text-right">{formatTime(selectedJob.state.nextRunAtMs)}</dd>
                    <dt className="text-muted-foreground">{t('cron.lastRun')}</dt><dd className="text-right">{formatTime(selectedJob.state.lastRunAtMs)}</dd>
                    <dt className="text-muted-foreground">{t('cron.lastStatus')}</dt><dd className="text-right">{selectedJob.state.lastStatus ? cronStatusLabel(selectedJob.state.lastStatus) : '—'}</dd>
                  </dl>
                  {selectedJob.state.lastError ? (
                    <div className="space-y-1 border-t pt-4"><p className="text-xs font-medium text-destructive">{t('cron.errorLabel')}</p><p className="whitespace-pre-wrap break-words text-sm leading-6 text-destructive">{selectedJob.state.lastError}</p></div>
                  ) : null}
                  <div className="space-y-1.5 border-t pt-4"><p className="text-xs font-medium text-muted-foreground">{t('cron.messageLabel')}</p><p className="whitespace-pre-wrap break-words text-sm leading-6">{selectedJob.payload.message || t('cron.noDescription')}</p></div>
                  <div className="grid grid-cols-2 gap-2 border-t pt-4">
                    <Button className="col-span-2" disabled={Boolean(busyId) || selectedJob.computedStatus === 'running'} onClick={() => void handleTrigger(selectedJob)}>
                      {busyId === `trigger:${selectedJob.id}` ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" /> : <Play className="mr-2 h-4 w-4" />}
                      {selectedJob.computedStatus === 'running' ? t('cron.running') : t('cron.runNow')}
                    </Button>
                    <Button
                      variant="outline" disabled={Boolean(busyId) || (!selectedJob.isRunning && !selectedJob.sessionId)}
                      onClick={() => void openJobSession(selectedJob)}
                    >
                      <ExternalLink className="mr-2 h-4 w-4" />
                      {selectedJob.isRunning ? t('cron.attachRun') : t('cron.viewSession')}
                    </Button>
                    <Button variant="outline" disabled={Boolean(busyId)} onClick={() => openEdit(selectedJob)}><Pencil className="mr-2 h-4 w-4" />{t('common.edit')}</Button>
                    <Button variant="outline" disabled={Boolean(busyId)} onClick={() => setShowDelete(true)} className="text-destructive hover:text-destructive"><Trash2 className="mr-2 h-4 w-4" />{t('common.delete')}</Button>
                  </div>
                </CardContent>
              </>
            ) : (
              <CardContent className="flex min-h-64 flex-col items-center justify-center px-6 text-center">
                <CalendarClock className="mb-3 h-6 w-6 text-muted-foreground" /><p className="font-medium">{t('cron.selectTaskTitle')}</p><p className="mt-1 text-sm text-muted-foreground">{t('cron.selectTaskHint')}</p>
              </CardContent>
            )}
          </Card>
          }
        />
      </div>
      <Dialog open={showForm} onOpenChange={(open) => { if (busyId !== 'save') setShowForm(open); }}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader><DialogTitle>{editingJob ? t('cron.formTitleEdit') : t('cron.formTitleCreate')}</DialogTitle><DialogDescription>{t('cron.formDescription')}</DialogDescription></DialogHeader>
          <DialogBody className="space-y-4 py-2 pr-1">
            <div className="space-y-2"><Label htmlFor="cron-name">{t('cron.nameLabel')}</Label><Input id="cron-name" value={formData.name} onChange={(event) => setFormData((current) => ({ ...current, name: event.target.value }))} placeholder={t('cron.namePlaceholder')} autoFocus /></div>
            <div className="flex items-center justify-between rounded-lg border px-3 py-2.5">
              <div><Label htmlFor="cron-form-enabled">{t('cron.enableAfterCreate')}</Label><p className="mt-0.5 text-xs text-muted-foreground">{t('cron.enableAfterCreateHint')}</p></div>
              <Switch id="cron-form-enabled" checked={formData.enabled} onCheckedChange={(enabled) => setFormData((current) => ({ ...current, enabled }))} />
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2"><Label>{t('cron.scheduleKind')}</Label><Select value={formData.scheduleKind} onValueChange={(scheduleKind) => setFormData((current) => ({ ...current, scheduleKind: scheduleKind as Exclude<ScheduleKind, 'at'> }))}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="cron">{t('cron.cronOption')}</SelectItem><SelectItem value="every">{t('cron.everyOption')}</SelectItem></SelectContent></Select></div>
              {formData.scheduleKind === 'cron' ? (
                <div className="space-y-2"><Label htmlFor="cron-expression">{t('cron.cronExprLabel')}</Label><Input id="cron-expression" value={formData.cronExpr} onChange={(event) => setFormData((current) => ({ ...current, cronExpr: event.target.value }))} placeholder="0 9 * * *" /></div>
              ) : (
                <div className="space-y-2"><Label htmlFor="cron-hours">{t('cron.intervalHours')}</Label><Input id="cron-hours" type="number" min="0.25" step="0.25" value={formData.everyHours} onChange={(event) => setFormData((current) => ({ ...current, everyHours: Number(event.target.value) }))} /></div>
              )}
            </div>
            <div className="space-y-2"><Label htmlFor="cron-message">{t('cron.messageLabel')}</Label><Textarea id="cron-message" value={formData.message} onChange={(event) => setFormData((current) => ({ ...current, message: event.target.value }))} placeholder={t('cron.messagePlaceholder')} rows={4} /></div>
            {formError ? <p role="alert" className="text-sm text-destructive">{formError}</p> : null}
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" disabled={busyId === 'save'} onClick={() => setShowForm(false)}>{t('common.cancel')}</Button>
            <Button disabled={busyId === 'save'} onClick={() => void handleSubmit()}>{busyId === 'save' ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" /> : null}{editingJob ? t('cron.saveChanges') : t('cron.createTask')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={showDelete} onOpenChange={(open) => { if (!busyId) setShowDelete(open); }}>
        <AlertDialogContent>
          <AlertDialogHeader><AlertDialogTitle>{t('cron.deleteConfirm', { name: selectedJob?.name ?? '' })}</AlertDialogTitle><AlertDialogDescription>{t('cron.deleteDescription')}</AlertDialogDescription></AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={Boolean(busyId)}>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction disabled={Boolean(busyId)} className="bg-destructive text-destructive-foreground hover:bg-destructive/90" onClick={(event) => { event.preventDefault(); void handleDelete(); }}>
              {busyId.startsWith('delete:') ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" /> : null}{t('cron.deleteAction')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
