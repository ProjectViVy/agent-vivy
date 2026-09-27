import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { ReviewCard } from '@/components/approvals/ReviewCard';
import type { ChildHistoryMessage, ChildMailboxMessage } from '@/lib/api';
import * as childApi from '@/lib/child-api';
import * as workflowApi from '@/lib/workflow-api';
import type { WorkflowDescriptor, WorkflowProposal, WorkflowResult } from '@/lib/workflow-api';
import { foldContinuityRows } from '@/lib/run-rows';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';

export function RunInspector() {
  const run = useVivyStore((state) => state.currentRun);
  const events = useVivyStore((state) => state.runEvents);
  const background = useVivyStore((state) => state.backgroundRuns);
  const children = useVivyStore((state) => state.children);
  const selectedChild = useVivyStore((state) => state.selectedChild);
  const childrenError = useVivyStore((state) => state.childrenError);
  const error = useVivyStore((state) => state.backgroundError ?? childrenError);
  const busyId = useVivyStore((state) => state.childBusyId ?? state.backgroundBusyId);
  const attach = useVivyStore((state) => state.attachBackgroundRun);
  const startChild = useVivyStore((state) => state.startChild);
  const openChild = useVivyStore((state) => state.openChild);
  const waitChild = useVivyStore((state) => state.waitChild);
  const cancelChild = useVivyStore((state) => state.cancelChild);
  const reviews = useVivyStore((state) => state.reviews);
  const reviewBusyIds = useVivyStore((state) => state.reviewBusyIds);
  const loadReviews = useVivyStore((state) => state.loadReviews);
  const respondReview = useVivyStore((state) => state.respondReview);
  const openRun = useVivyStore((state) => state.openRun);
  const activeSessionId = useVivyStore((state) => state.activeSessionId);
  const sessionMessages = useVivyStore((state) => state.messages);
  const [childText, setChildText] = useState('');
  const [policy, setPolicy] = useState('');
  const [tools, setTools] = useState('');
  const [continuable, setContinuable] = useState(false);
  const [startOperationID, setStartOperationID] = useState('');
  const [childHistory, setChildHistory] = useState<ChildHistoryMessage[]>([]);
  const [childHistoryPhase, setChildHistoryPhase] = useState<'idle' | 'loading' | 'ready' | 'empty' | 'error'>('idle');
  const [childMailbox, setChildMailbox] = useState<ChildMailboxMessage[]>([]);
  const [childActionError, setChildActionError] = useState('');
  const [childActionBusyId, setChildActionBusyId] = useState<string | null>(null);
  const [followupText, setFollowupText] = useState('');
  const [followupOperationID, setFollowupOperationID] = useState('');
  const [messageText, setMessageText] = useState('');
  const [messageOperationID, setMessageOperationID] = useState('');
  const [messageAcknowledgement, setMessageAcknowledgement] = useState('');
  const [workflowText, setWorkflowText] = useState(() => JSON.stringify({
    schema_version: 1,
    start_nodes: ['draft'],
    nodes: [{ key: 'draft', task: 'Draft a concise result.' }],
    edges: [],
    outputs: ['draft'],
  }, null, 2));
  const [workflowProposal, setWorkflowProposal] = useState<WorkflowProposal | null>(null);
  const [proposalSource, setProposalSource] = useState('');
  const [workflowOperationID, setWorkflowOperationID] = useState('');
  const [workflowItems, setWorkflowItems] = useState<WorkflowResult[]>([]);
  const [workflowState, setWorkflowState] = useState<WorkflowResult | null>(null);
  const [workflowBusy, setWorkflowBusy] = useState(false);
  const [workflowError, setWorkflowError] = useState('');
  const [openSeq, setOpenSeq] = useState<number | null>(null);
  const [tab, setTab] = useState('run');
  const { t } = useTranslation();
  const active = (status: string) => !['completed', 'failed', 'cancelled'].includes(status);
  useEffect(() => { if (tab === 'review') void loadReviews(); }, [tab, loadReviews]);
  useEffect(() => {
    let stale = false;
    setWorkflowItems([]); setWorkflowState(null); setWorkflowProposal(null); setProposalSource('');
    setWorkflowOperationID(''); setWorkflowError('');
    if (!run?.id) return () => { stale = true; };
    void workflowApi.listWorkflows(run.id).then(({ workflows }) => {
      if (stale) return;
      setWorkflowItems(workflows);
      if (workflows.length) setWorkflowState(workflows[workflows.length - 1]);
    }).catch((cause: unknown) => {
      if (!stale) setWorkflowError(cause instanceof Error ? cause.message : String(cause));
    });
    return () => { stale = true; };
  }, [run?.id]);
  useEffect(() => {
    const workflowRunID = workflowState?.id;
    if (!workflowRunID || !active(workflowState.status)) return;
    let stale = false;
    const refresh = async () => {
      try {
        const latest = await workflowApi.getWorkflow(workflowRunID);
        if (!stale) {
          setWorkflowState(latest);
          setWorkflowItems((items) => items.map((item) => item.id === latest.id ? latest : item));
        }
      } catch (cause) {
        if (!stale) setWorkflowError(cause instanceof Error ? cause.message : String(cause));
      }
    };
    void refresh();
    const timer = window.setInterval(() => { void refresh(); }, 1000);
    return () => { stale = true; window.clearInterval(timer); };
  }, [workflowState?.id, workflowState?.status]);
  useEffect(() => {
    let stale = false;
    const authorizerRunID = run?.id;
    if (!selectedChild || selectedChild.child_mode !== 'continuable' || !authorizerRunID) {
      setChildHistory([]); setChildMailbox([]); setChildHistoryPhase('idle');
      return () => { stale = true; };
    }
    setChildActionError('');
    setChildHistoryPhase('loading');
    void Promise.allSettled([
      childApi.getChildHistory(selectedChild.session_id, authorizerRunID),
      childApi.listChildMessages(selectedChild.session_id, authorizerRunID),
    ]).then(([historyResult, mailboxResult]) => {
      if (stale) return;
      if (historyResult.status === 'fulfilled') {
        setChildHistory(historyResult.value.messages);
        setChildHistoryPhase(historyResult.value.messages.length ? 'ready' : 'empty');
      } else {
        setChildHistoryPhase('error');
        setChildActionError(historyResult.reason instanceof Error ? historyResult.reason.message : String(historyResult.reason));
      }
      if (mailboxResult.status === 'fulfilled') {
        setChildMailbox(mailboxResult.value.messages);
      } else {
        setChildActionError(mailboxResult.reason instanceof Error ? mailboxResult.reason.message : String(mailboxResult.reason));
      }
    });
    return () => { stale = true; };
  }, [run?.id, selectedChild, selectedChild?.id, selectedChild?.session_id, selectedChild?.child_mode]);
  const runReviews = run ? reviews.filter((item) => item.run_id === run.id) : [];
  const attachedRefs = foldContinuityRows(events);
  const sessionRuns = (() => {
    const ids: string[] = [];
    const seen = new Set<string>();
    for (const item of background) if (item.session_id === activeSessionId && !seen.has(item.id)) { seen.add(item.id); ids.push(item.id); }
    if (run && !seen.has(run.id)) { seen.add(run.id); ids.push(run.id); }
    for (let index = sessionMessages.length - 1; index >= 0 && ids.length < 50; index -= 1) {
      const id = sessionMessages[index].run_id;
      if (id && !seen.has(id)) { seen.add(id); ids.push(id); }
    }
    return ids;
  })();

  return <Tabs value={tab} onValueChange={setTab} className="flex h-full flex-col">
    {sessionRuns.length ? <div className="mx-4 mt-4">
      <label className="sr-only" htmlFor="run-inspector-run-picker">{t('runInspector.selectRun')}</label>
      <select id="run-inspector-run-picker" className="w-full rounded-md border border-input bg-background px-2 py-1.5 text-xs" value={run?.id ?? ''} onChange={(event) => { const id = event.target.value; if (id && activeSessionId) void openRun(id, activeSessionId); }}>
        {!run ? <option value="" disabled>{t('runInspector.noRun')}</option> : null}
        {sessionRuns.map((id) => <option key={id} value={id}>{id}{id === run?.id ? ` · ${run.status}` : ''}</option>)}
      </select>
    </div> : null}
    <TabsList className="mx-4 mt-4 grid grid-cols-4">
      <TabsTrigger value="run">{t('runInspector.currentRun')}</TabsTrigger>
      <TabsTrigger value="background">{t('runInspector.background', { count: background.length })}</TabsTrigger>
      <TabsTrigger value="children">{t('runInspector.children', { count: children.length })}</TabsTrigger>
      <TabsTrigger value="review">{t('runInspector.review', { count: runReviews.length })}</TabsTrigger>
    </TabsList>
    <div className="flex-1 overflow-auto p-4">
      <TabsContent value="run" className="mt-0 space-y-4">
        {run ? <>
          <div className="rounded-lg border p-3 text-sm">
            <div className="flex items-center justify-between"><code>{run.id}</code><Badge>{run.status}</Badge></div>
            <div className="mt-2 text-muted-foreground">{t('hostLabels.session', { id: run.session_id })}</div>
          </div>
          <div>
            <h3 className="mb-2 text-sm font-medium">{t('runInspector.events')}</h3>
            <div className="space-y-1">{events.slice().reverse().map((event) => <div key={event.seq}>
              <button type="button" className={`flex w-full items-center gap-2 rounded p-2 text-left text-xs hover:bg-muted ${openSeq === event.seq ? 'bg-muted' : ''}`} aria-expanded={openSeq === event.seq} onClick={() => setOpenSeq(openSeq === event.seq ? null : event.seq)}>
                <span className="w-10 text-muted-foreground">#{event.seq}</span><span>{event.type}</span>
              </button>
              {openSeq === event.seq ? <pre className="mb-1 max-h-48 overflow-auto whitespace-pre-wrap break-words rounded bg-muted/60 p-2 font-mono text-[11px] leading-4">{JSON.stringify(event.payload ?? null, null, 2)}</pre> : null}
            </div>)}</div>
          </div>
          {attachedRefs.length > 0 ? <div>
            <h3 className="mb-2 text-sm font-medium">{t('runInspector.references', { count: attachedRefs.length })}</h3>
            <div className="space-y-1">{attachedRefs.map((row) => <div key={row.id} className="rounded border p-2 text-xs"><code>{row.reference.id}</code> <span className="text-muted-foreground">{row.reference.source_session_id} · {row.reference.items.length} · {row.reference.origin}</span></div>)}</div>
          </div> : null}
        </> : <p className="py-10 text-center text-sm text-muted-foreground">{t('runInspector.noRun')}</p>}
      </TabsContent>

      <TabsContent value="background" className="mt-0 space-y-2">
        {background.length ? background.map((item) => <div key={item.id} className="rounded-lg border p-3 text-sm">
          <div className="flex items-center justify-between"><div className="min-w-0"><div className="truncate font-medium">{item.id}</div><div className="text-xs text-muted-foreground">{item.status}</div></div><Button size="sm" variant="outline" disabled={busyId === item.id} onClick={() => void attach(item.id)}>{t('common.open')}</Button></div>
        </div>) : <p className="py-10 text-center text-sm text-muted-foreground">{t('runInspector.noBackground')}</p>}
      </TabsContent>

      <TabsContent value="children" className="mt-0 space-y-4">
        <details className="rounded-lg border p-3">
          <summary className="cursor-pointer text-sm font-medium">{t('runInspector.workflows')}</summary>
          <div className="mt-3 space-y-3">
            <Textarea
              aria-label={t('runInspector.workflowDescriptor')}
              className="min-h-40 font-mono text-xs"
              value={workflowText}
              placeholder={t('runInspector.workflowDescriptorPlaceholder')}
              onChange={(event) => {
                setWorkflowText(event.target.value); setWorkflowProposal(null); setProposalSource('');
                setWorkflowOperationID(''); setWorkflowError('');
              }}
            />
            <div className="flex flex-wrap gap-2">
              <Button size="sm" variant="outline" disabled={!run || run.status !== 'active' || workflowBusy} onClick={async () => {
                if (!run) return;
                setWorkflowBusy(true); setWorkflowError('');
                try {
                  const descriptor = JSON.parse(workflowText) as WorkflowDescriptor;
                  const proposal = await workflowApi.proposeWorkflow({ parent_run_id: run.id, descriptor });
                  setWorkflowProposal(proposal); setProposalSource(workflowText);
                } catch (cause) {
                  setWorkflowError(cause instanceof SyntaxError ? t('runInspector.workflowInvalidJSON') : cause instanceof Error ? cause.message : String(cause));
                } finally { setWorkflowBusy(false); }
              }}>{t('runInspector.workflowValidate')}</Button>
              <Button size="sm" disabled={!run || !workflowProposal || proposalSource !== workflowText || workflowBusy} onClick={async () => {
                if (!run) return;
                setWorkflowBusy(true); setWorkflowError('');
                const operationID = workflowOperationID || crypto.randomUUID();
                setWorkflowOperationID(operationID);
                try {
                  const descriptor = JSON.parse(workflowText) as WorkflowDescriptor;
                  const started = await workflowApi.startWorkflow({ parent_run_id: run.id, operation_id: operationID, descriptor });
                  setWorkflowState(started);
                  setWorkflowItems((items) => [ ...items.filter((item) => item.id !== started.id), started ]);
                } catch (cause) {
                  setWorkflowError(cause instanceof Error ? cause.message : String(cause));
                } finally { setWorkflowBusy(false); }
              }}>{t('runInspector.workflowStart')}</Button>
            </div>
            {workflowProposal ? <p className="break-all text-xs text-muted-foreground">{t('runInspector.workflowDigest', { digest: workflowProposal.digest.slice(0, 16) })}</p> : null}
            {workflowItems.length ? <div className="flex flex-wrap gap-2">
              {workflowItems.map((item) => <Button key={item.id} size="sm" variant={workflowState?.id === item.id ? 'secondary' : 'outline'} onClick={() => {
                setWorkflowState(item); setWorkflowText(JSON.stringify(item.descriptor, null, 2)); setWorkflowProposal(null); setProposalSource('');
              }}>{item.id.slice(0, 16)} · {item.status}</Button>)}
            </div> : <p className="text-xs text-muted-foreground">{t('runInspector.noWorkflows')}</p>}
            {workflowState ? <div className="space-y-2 rounded bg-muted/40 p-2 text-xs">
              <div className="flex items-center justify-between gap-2"><code className="truncate">{workflowState.id}</code><Badge variant="outline">{workflowState.status}</Badge></div>
              <p className="break-all text-muted-foreground">{t('runInspector.workflowDigest', { digest: workflowState.revision_digest.slice(0, 16) })}</p>
              <h4 className="font-medium">{t('runInspector.workflowNodes')}</h4>
              {workflowState.nodes.map((node) => <div key={node.key} className="flex items-start justify-between gap-2 rounded border bg-background p-2">
                <div className="min-w-0"><code>{node.key}</code>{node.message ? <p className="mt-1 text-muted-foreground">{node.message}</p> : null}
                  {node.child_run_id ? <Button className="mt-1 h-auto px-0 py-0" size="sm" variant="link" onClick={async () => { await openChild(node.child_run_id!); setTab('children'); }}>{t('runInspector.workflowOpenChild')}</Button> : null}
                </div><Badge variant="outline">{node.status}</Badge>
              </div>)}
              {workflowState.outputs && Object.keys(workflowState.outputs).length ? <div className="space-y-1 rounded border bg-background p-2">
                <h4 className="font-medium">{t('runInspector.workflowOutputs')}</h4>
                <pre className="whitespace-pre-wrap break-words text-muted-foreground">{JSON.stringify(workflowState.outputs, null, 2)}</pre>
              </div> : null}
              {active(workflowState.status) ? <Button size="sm" variant="destructive" disabled={workflowBusy} onClick={async () => {
                setWorkflowBusy(true); setWorkflowError('');
                try {
                  const cancelled = await workflowApi.cancelWorkflow(workflowState.id);
                  setWorkflowState(cancelled);
                  setWorkflowItems((items) => items.map((item) => item.id === cancelled.id ? cancelled : item));
                } catch (cause) { setWorkflowError(cause instanceof Error ? cause.message : String(cause)); }
                finally { setWorkflowBusy(false); }
              }}>{t('runInspector.workflowCancel')}</Button> : null}
            </div> : null}
          </div>
        </details>
        {run ? <form className="space-y-2 rounded-lg border p-3" onSubmit={async (event) => {
          event.preventDefault();
          if (!childText.trim()) return;
          const mode = continuable ? 'continuable' : 'one-shot';
          const operationID = continuable ? (startOperationID || crypto.randomUUID()) : undefined;
          if (operationID) setStartOperationID(operationID);
          setChildActionError('');
          try {
            const toolNames = tools.split(',').map((value) => value.trim()).filter(Boolean);
            if (continuable && run) {
              setChildActionBusyId('create');
              const child = await childApi.startChild({ parent_run_id: run.id, text: childText.trim(), mode, operation_id: operationID, tool_names: toolNames.length ? toolNames : undefined });
              await useVivyStore.getState().loadChildren(run.id);
              await useVivyStore.getState().openChild(child.id);
              setChildActionBusyId(null);
            } else {
              await startChild(childText.trim(), policy.trim(), toolNames);
            }
            setChildText(''); setStartOperationID('');
          } catch (cause) {
            setChildActionBusyId(null);
            setChildActionError(cause instanceof Error ? cause.message : String(cause));
            /* Keep the operation ID so retry remains idempotent. */
          }
        }}>
          <div className="text-sm font-medium">{t('runInspector.startChild')}</div>
          <Input value={childText} onChange={(event) => setChildText(event.target.value)} placeholder={t('runInspector.delegatePlaceholder')} />
          {!continuable ? <Input value={policy} onChange={(event) => setPolicy(event.target.value)} placeholder={t('runInspector.policyPlaceholder')} /> : null}
          <Input value={tools} onChange={(event) => setTools(event.target.value)} placeholder={t('runInspector.toolsPlaceholder')} />
          <label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={continuable} onChange={(event) => setContinuable(event.target.checked)} />{t('runInspector.childMode')}</label>
          <Button type="submit" size="sm" disabled={!childText.trim() || busyId === 'create' || childActionBusyId === 'create'}>{t('common.start')}</Button>
        </form> : null}

        {children.map((child) => <div key={child.id} className="rounded-lg border p-3 text-sm">
          <button className="flex w-full items-center justify-between text-left" onClick={() => void openChild(child.id)}>
            <span className="truncate">{'—'.repeat(Math.max(0, child.depth - 1))} {child.id}</span><Badge variant="outline">{child.status}</Badge>
          </button>
          {selectedChild?.id === child.id ? <div className="mt-3 border-t pt-3">
            <p className="whitespace-pre-wrap text-xs text-muted-foreground">{selectedChild.result || selectedChild.error || t('runInspector.noResult')}</p>
            {selectedChild.child_mode === 'continuable' ? <>
              <p className="mt-2 break-all text-xs text-muted-foreground">{t('runInspector.childSession', { id: selectedChild.session_id })}</p>
              <div className="mt-3">
                <h4 className="mb-2 text-xs font-medium">{t('runInspector.childHistory')}</h4>
                {childHistoryPhase === 'loading' ? <p className="text-xs text-muted-foreground">{t('common.loading')}</p> : null}
                {childHistory.length ? <div className="max-h-40 space-y-2 overflow-auto rounded bg-muted/50 p-2">
                  {childHistory.map((message) => <div key={message.id} className="text-xs"><span className="font-medium">{message.role}</span><p className="whitespace-pre-wrap">{message.content}</p></div>)}
                </div> : childHistoryPhase !== 'loading' ? <p className="text-xs text-muted-foreground">{t('runInspector.noChildHistory')}</p> : null}
              </div>
              {childMailbox.length ? <div className="mt-3 space-y-2">
                <h4 className="text-xs font-medium">{t('runInspector.pendingMessages')}</h4>
                {childMailbox.map((message) => <p key={message.id} className="rounded bg-muted/50 p-2 text-xs">{message.text}</p>)}
              </div> : null}
              {run?.status === 'active' ? <>
                <form className="mt-3 flex gap-2" onSubmit={async (event) => {
                  event.preventDefault(); if (!followupText.trim()) return;
                  if (!run) return;
                  const operationID = followupOperationID || crypto.randomUUID(); setFollowupOperationID(operationID);
                  setChildActionBusyId(selectedChild.session_id); setChildActionError('');
                  try {
                    const child = await childApi.followupChild({ child_session_id: selectedChild.session_id, parent_run_id: run.id, operation_id: operationID, text: followupText.trim() });
                    await useVivyStore.getState().loadChildren(run.id);
                    await useVivyStore.getState().openChild(child.id);
                    setFollowupText(''); setFollowupOperationID('');
                  } catch (cause) {
                    setChildActionError(cause instanceof Error ? cause.message : String(cause));
                    /* Keep the operation ID so retry remains idempotent. */
                  } finally { setChildActionBusyId(null); }
                }}>
                  <Input value={followupText} onChange={(event) => setFollowupText(event.target.value)} placeholder={t('runInspector.followupPlaceholder')} />
                  <Button type="submit" size="sm" disabled={!followupText.trim() || busyId === selectedChild.session_id || childActionBusyId === selectedChild.session_id}>{t('runInspector.followup')}</Button>
                </form>
                <form className="mt-2 flex gap-2" onSubmit={async (event) => {
                  event.preventDefault(); if (!messageText.trim()) return;
                  if (!run) return;
                  const operationID = messageOperationID || crypto.randomUUID(); setMessageOperationID(operationID);
                  setChildActionBusyId(selectedChild.session_id); setChildActionError('');
                  try {
                    const admitted = await childApi.sendChildMessage({ child_session_id: selectedChild.session_id, parent_run_id: run.id, operation_id: operationID, text: messageText.trim() });
                    const mailbox = await childApi.listChildMessages(selectedChild.session_id, run.id);
                    setChildMailbox(mailbox.messages);
                    setMessageAcknowledgement(t('runInspector.messageAdmitted', { id: admitted.message.id })); setMessageText(''); setMessageOperationID('');
                  } catch (cause) {
                    setChildActionError(cause instanceof Error ? cause.message : String(cause));
                    /* Keep the operation ID so retry remains idempotent. */
                  } finally { setChildActionBusyId(null); }
                }}>
                  <Input value={messageText} onChange={(event) => setMessageText(event.target.value)} placeholder={t('runInspector.messagePlaceholder')} />
                  <Button type="submit" size="sm" disabled={!messageText.trim() || busyId === selectedChild.session_id || childActionBusyId === selectedChild.session_id}>{t('runInspector.sendMessage')}</Button>
                </form>
                {messageAcknowledgement ? <p role="status" className="mt-1 text-xs text-muted-foreground">{messageAcknowledgement}</p> : null}
          {attachedRefs.length > 0 ? <div>
            <h3 className="mb-2 text-sm font-medium">{t('runInspector.references', { count: attachedRefs.length })}</h3>
            <div className="space-y-1">{attachedRefs.map((row) => <div key={row.id} className="rounded border p-2 text-xs"><code>{row.reference.id}</code> <span className="text-muted-foreground">{row.reference.source_session_id} · {row.reference.items.length} · {row.reference.origin}</span></div>)}</div>
          </div> : null}
              </> : null}
          {attachedRefs.length > 0 ? <div>
            <h3 className="mb-2 text-sm font-medium">{t('runInspector.references', { count: attachedRefs.length })}</h3>
            <div className="space-y-1">{attachedRefs.map((row) => <div key={row.id} className="rounded border p-2 text-xs"><code>{row.reference.id}</code> <span className="text-muted-foreground">{row.reference.source_session_id} · {row.reference.items.length} · {row.reference.origin}</span></div>)}</div>
          </div> : null}
            </> : null}
            {active(child.status) ? <div className="mt-2 flex gap-2">
              <Button size="sm" variant="outline" disabled={busyId === child.id} onClick={() => void waitChild(child.id)}>{t('runInspector.waitComplete')}</Button>
              {selectedChild.child_mode === 'continuable'
                ? <Button size="sm" variant="outline" disabled={busyId === child.id || childActionBusyId === child.id} onClick={async () => {
                  setChildActionBusyId(child.id); setChildActionError('');
                  try {
                    const updated = await childApi.interruptChild(child.id);
                    await useVivyStore.getState().loadChildren(run?.id);
                    await useVivyStore.getState().openChild(updated.id);
                  } catch (cause) { setChildActionError(cause instanceof Error ? cause.message : String(cause)); }
                  finally { setChildActionBusyId(null); }
                }}>{t('runInspector.interrupt')}</Button>
                : <Button size="sm" variant="destructive" disabled={busyId === child.id} onClick={() => void cancelChild(child.id)}>{t('common.cancel')}</Button>}
            </div> : null}
          </div> : null}
        </div>)}
        {!children.length ? <p className="py-8 text-center text-sm text-muted-foreground">{t('runInspector.noChildren')}</p> : null}
      </TabsContent>

      <TabsContent value="review" className="mt-0 space-y-3">
        {runReviews.length ? runReviews.map((review) => <ReviewCard key={review.id} review={review} busy={reviewBusyIds.includes(review.id)} onRespond={(response) => respondReview(review.id, response)} />) : <p className="py-10 text-center text-sm text-muted-foreground">{t('runInspector.noReviews')}</p>}
      </TabsContent>
      {error || childActionError || workflowError ? <p className="mt-3 rounded bg-destructive/10 p-2 text-sm text-destructive">{childActionError || workflowError || error}</p> : null}
    </div>
  </Tabs>;
}
