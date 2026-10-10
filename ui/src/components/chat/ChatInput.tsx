import { useEffect, useRef, useState } from 'react';
import {
  Check, CheckCircle, ChevronDown, Clock, History, Lightbulb, LightbulbOff,
  Paperclip, Send, Shield, ShieldCheck, Shrink, Sparkles, Square, X,
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import type { AttachmentInput, PermissionPreset, QueuedTurn, RunMode, SessionContext, ThinkingMode } from '@/lib/api';
import { retainReturnedTurn, restoreTurnContext, useVivyStore } from '@/lib/store';
import { cn } from '@/lib/utils';
import { dateTimeLocale, useTranslation } from '@/i18n';
import { WorkspaceSelector } from './WorkspaceSelector';
import { HistoryPicker } from './HistoryPicker';
import { ContextReferenceChip } from './ContextReferenceChip';
import { ComposerCommandControls, useComposerCommands } from './ComposerCommands';

interface ChatInputProps {
  onSend: (content: string, mode: RunMode, attachments?: AttachmentInput[], thinking?: ThinkingMode, restored?: QueuedTurn) => Promise<void> | void;
  /** 运行期间发送走排队（对照 Crush）：跳过 UI 预检，服务端门禁仍然生效。 */
  onQueue?: (content: string, mode: RunMode, attachments?: AttachmentInput[], thinking?: ThinkingMode, restored?: QueuedTurn) => Promise<void> | void;
  /** pi 双轨：运行中 Enter=steer（turn/steer），Shift+Alt+Enter=follow-up
   *  （turn/follow_up）；缺省时回退 onQueue。 */
  onSteer?: (content: string, mode: RunMode, attachments?: AttachmentInput[], thinking?: ThinkingMode, restored?: QueuedTurn) => Promise<void> | void;
  onFollowUp?: (content: string, mode: RunMode, attachments?: AttachmentInput[], thinking?: ThinkingMode, restored?: QueuedTurn) => Promise<void> | void;
  /** pi Alt+Up：弹出最新 pending follow-up 文本及完整捕获选项回编辑框。 */
  onDequeue?: () => Promise<QueuedTurn | string | null>;
  onCancel?: () => Promise<void> | void;
  disabled?: boolean;
  running?: boolean;
  placeholder?: string;
  /** 服务端 session/context 真实占用；null 时环显示 0。 */
  context?: SessionContext | null;
  /** 回退预填：seq 变化时把文本写入草稿并聚焦输入框。 */
  draftPreset?: { text: string; seq: number; turn?: QueuedTurn } | null;
}

type PermissionMode = 'cautious' | 'smart' | 'trusted';

const ESTIMATED_CONTEXT_LIMIT_TOKENS = 128000;
const TEXT_ENCODER = new TextEncoder();

// 图片附件门禁（与服务端 turn/start 校验一致；服务端仍是权威门禁）。
const ATTACHMENT_MIMES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp']);
const MAX_ATTACHMENT_BYTES = 5 * 1024 * 1024;
const MAX_ATTACHMENTS = 4;

const fileToAttachment = (file: File): Promise<AttachmentInput> => new Promise((resolve, reject) => {
  const reader = new FileReader();
  reader.onload = () => {
    const result = String(reader.result ?? '');
    const comma = result.indexOf(',');
    resolve({ name: file.name, mime_type: file.type, data: comma >= 0 ? result.slice(comma + 1) : result });
  };
  reader.onerror = () => reject(reader.error ?? new Error('file read failed'));
  reader.readAsDataURL(file);
});

// 思考模式选项：七级 effort 面 + auto/on/off 别名（VCP F1）；level 项直接显示原值。
const THINKING_MODES: { value: ThinkingMode; icon: LucideIcon; label: string; filled?: boolean }[] = [
  { value: 'auto', icon: Lightbulb, label: 'chatInput.thinkingModeAuto' },
  { value: 'minimal', icon: Lightbulb, label: 'minimal' },
  { value: 'low', icon: Lightbulb, label: 'low' },
  { value: 'medium', icon: Lightbulb, label: 'medium' },
  { value: 'high', icon: Lightbulb, label: 'high' },
  { value: 'xhigh', icon: Lightbulb, label: 'xhigh' },
  { value: 'max', icon: Lightbulb, label: 'max', filled: true },
  { value: 'on', icon: Lightbulb, label: 'chatInput.thinkingModeOn', filled: true },
  { value: 'off', icon: LightbulbOff, label: 'chatInput.thinkingModeOff' },
];

// 权限模式选项（对照 Agent-DIVA ChatView.permissionOptions）
const PERMISSION_MODES: { value: PermissionMode; icon: LucideIcon; label: string; desc: string }[] = [
  { value: 'cautious', icon: Shield, label: 'chatInput.permissionCautious', desc: 'chatInput.permissionCautiousDesc' },
  { value: 'smart', icon: Sparkles, label: 'chatInput.permissionSmart', desc: 'chatInput.permissionSmartDesc' },
  { value: 'trusted', icon: CheckCircle, label: 'chatInput.permissionTrusted', desc: 'chatInput.permissionTrustedDesc' },
];

export function ChatInput({ onSend, onQueue, onSteer, onFollowUp, onDequeue, onCancel, disabled: inputDisabled, running, placeholder, context = null, draftPreset = null }: ChatInputProps) {
  const [value, setValue] = useState('');
  const [pending, setPending] = useState<AttachmentInput[]>([]);
  const [restoredTurn, setRestoredTurn] = useState<QueuedTurn | undefined>();
  const [notice, setNotice] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const sendLock = useRef(false);
  const commands = useComposerCommands(value, setValue);
  const disabled = Boolean(inputDisabled || sending);
  const canSend = Boolean(value.trim() || commands.draft?.name === 'plan' || commands.draft?.skill || commands.draft?.goalRef);
  const [thinkingMode, setThinkingMode] = useState<ThinkingMode>('auto');
  const [confirmTrusted, setConfirmTrusted] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const noticeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reviewCenterOpen = useVivyStore((state) => state.reviewCenterOpen);
  const openReviewCenter = useVivyStore((state) => state.setReviewCenterOpen);
  const kernelQueue = useVivyStore((state) => state.kernelQueue);
  const removeKernelQueued = useVivyStore((state) => state.removeKernelQueued);
  const clearQueue = useVivyStore((state) => state.clearQueue);
  const pendingReviewCount = useVivyStore((state) => state.reviews.filter((review) => review.status === 'pending').length);
  const activeSessionId = useVivyStore((state) => state.activeSessionId);
  const currentDraft = useRef({ value, pending, sessionId: activeSessionId });
  currentDraft.current = { value, pending, sessionId: activeSessionId };
  const sessions = useVivyStore((state) => state.sessions);
  const sessionBusyId = useVivyStore((state) => state.sessionBusyId);
  const setSessionPermission = useVivyStore((state) => state.setSessionPermission);
	const chooseWorkspace = useVivyStore((state) => state.chooseWorkspace);
  const compactSession = useVivyStore((state) => state.compactSession);
  const draftReferences = useVivyStore((state) => state.draftReferences);
  const addDraftReference = useVivyStore((state) => state.addDraftReference);
  const removeDraftReference = useVivyStore((state) => state.removeDraftReference);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [compactOpen, setCompactOpen] = useState(false);
  const [compactInstructions, setCompactInstructions] = useState('');
  const [compacting, setCompacting] = useState(false);
  const { t } = useTranslation();
  const activeSession = sessions.find((session) => session.id === activeSessionId);
  const permissionPreset: PermissionPreset = activeSession?.permission_preset ?? 'smart';
  const permissionBusy = sessionBusyId === activeSessionId;
  const draftBytes = TEXT_ENCODER.encode(value).length;
  // 真实上下文：服务端 session/context 的 feed 占用 + 当前草稿（1 token ≈ 4 字节估算）。
  const usedTokens = (context?.feed_tokens ?? 0) + Math.round(draftBytes / 4);
  const limitTokens = context?.model_limit_tokens ?? ESTIMATED_CONTEXT_LIMIT_TOKENS;
  const contextRatio = Math.min(1, usedTokens / Math.max(1, limitTokens));
  const contextPercent = Math.round(contextRatio * 100);
  const wouldCompact = context?.compaction_enabled ? usedTokens > (context.trigger_tokens ?? 0) : false;
  const contextCircumference = 2 * Math.PI * 10;
  const contextColor = wouldCompact || contextPercent >= 80 ? 'text-destructive' : contextPercent >= 60 ? 'text-amber-500' : 'text-primary';
  const contextTitleText = () => {
    const base = t('chatInput.contextTitle', { percent: contextPercent, used: usedTokens.toLocaleString(dateTimeLocale()), limit: limitTokens.toLocaleString(dateTimeLocale()) });
    if (!context) return base;
    const bytes = ` · ${context.feed_bytes.toLocaleString(dateTimeLocale())} / ${context.limit_bytes.toLocaleString(dateTimeLocale())} B`;
    const status = wouldCompact
      ? ` · ${t('chatInput.contextWouldCompact')}`
      : context.has_compaction_summary || context.last_compaction
        ? ` · ${t('chatInput.contextCompacted')}`
        : '';
    return base + bytes + status;
  };
  const thinkingModeOption = THINKING_MODES.find((mode) => mode.value === thinkingMode)!;
  const permissionModeOption = PERMISSION_MODES.find((mode) => mode.value === permissionPreset) ?? PERMISSION_MODES[1];
  const permissionLocked = Boolean(running) || permissionBusy || !activeSessionId;

  useEffect(() => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    textarea.style.height = 'auto';
    textarea.style.height = `${Math.min(textarea.scrollHeight, 160)}px`;
  }, [value]);
  useEffect(() => () => { if (noticeTimer.current) clearTimeout(noticeTimer.current); }, []);
  // 切换会话时丢弃待发送附件（队列在 store 内清空，本地草稿附件保持同生命周期）。
  const previousSession = useRef(activeSessionId);
  useEffect(() => {
    if (previousSession.current !== activeSessionId) { setPending([]); setRestoredTurn(undefined); }
    previousSession.current = activeSessionId;
  }, [activeSessionId]);
  // 回退预填：同一 seq 只应用一次（ref 防重复），写入草稿后聚焦。
  const appliedPresetSeq = useRef(0);
  useEffect(() => {
    if (!draftPreset || draftPreset.seq === appliedPresetSeq.current) return;
    appliedPresetSeq.current = draftPreset.seq;
    setValue(draftPreset.text);
    setRestoredTurn(draftPreset.turn);
    if (draftPreset.turn) restoreTurnContext(draftPreset.turn);
    setPending(draftPreset.turn?.attachments?.map((item) => ({ ...item })) ?? []);
    if (draftPreset.turn) setThinkingMode((draftPreset.turn.thinking ?? 'auto') as ThinkingMode);
    textareaRef.current?.focus();
  }, [draftPreset]);

  const showNotice = (message: string) => {
    setNotice(message);
    if (noticeTimer.current) clearTimeout(noticeTimer.current);
    noticeTimer.current = setTimeout(() => setNotice(null), 1800);
  };

  const addFiles = async (files: Iterable<File>) => {
    if (disabled) return;
    let count = pending.length;
    for (const file of Array.from(files)) {
      if (count >= MAX_ATTACHMENTS) { showNotice(t('chatInput.attachmentMax', { count: MAX_ATTACHMENTS })); return; }
      if (!ATTACHMENT_MIMES.has(file.type)) { showNotice(t('chatInput.attachmentUnsupported', { name: file.name })); continue; }
      if (file.size > MAX_ATTACHMENT_BYTES) { showNotice(t('chatInput.attachmentTooLarge', { name: file.name })); continue; }
      try {
        const attachment = await fileToAttachment(file);
        if (count >= MAX_ATTACHMENTS) return;
        count += 1;
        setPending((current) => current.length >= MAX_ATTACHMENTS ? current : [...current, attachment]);
      } catch {
        showNotice(t('chatInput.attachmentReadFailed', { name: file.name }));
      }
    }
  };

  // 内核双轨队列条目（steer 轨在 enqueue 即 steer，通常瞬态；follow_up
  //  轨待 settle 准入）；所有提交统一由内核持有。
  const kernelItems = [
    ...(kernelQueue?.steering ?? []).map((item) => ({ item, lane: 'steer' as const })),
    ...(kernelQueue?.follow_up ?? []).map((item) => ({ item, lane: 'follow_up' as const })),
  ];
  const recoveryCount = useVivyStore((state) => (state.queueRecoveryTurns[activeSessionId ?? ''] ?? []).length);
  const totalQueued = kernelItems.length;

  const send = async (track: 'send' | 'steer' | 'follow_up' = 'send') => {
    if (disabled || sendLock.current || !activeSessionId) return;
    if (commands.menuOpen) { commands.choose(commands.index); textareaRef.current?.focus(); return; }
    const content = value.trim();
    if (!content && !commands.draft) return;
    if (commands.draft && useVivyStore.getState().workBusy) return;
    sendLock.current = true;
    setSending(true);
    try {
      const prepared = await commands.prepare(content, Boolean(pending.length || draftReferences.length));
      if (!commands.isCurrent()) return;
      if (prepared) {
        const outgoing = pending.length ? pending : undefined;
        const mode = (restoredTurn?.mode ?? 'normal') as RunMode;
        const args: [string, RunMode, AttachmentInput[] | undefined, ThinkingMode, QueuedTurn?] = [prepared.text, mode, outgoing, thinkingMode];
        if (restoredTurn) args.push(restoredTurn);
        if (running && !prepared.forceSend) {
          const handler = track === 'follow_up' ? onFollowUp : onSteer;
          if (!handler && !onQueue) return;
          await (handler ?? onQueue)!(...args);
        } else await onSend(...args);
      }
      if (!commands.isCurrent()) return;
      setValue('');
      setPending([]);
      setRestoredTurn(undefined);
      commands.clear();
    } catch (error) {
      if (commands.isCurrent()) commands.setError(error instanceof Error ? error.message : String(error));
    } finally {
      sendLock.current = false;
      if (commands.isCurrent()) setSending(false);
    }
  };

  // pi Alt+Up：pop 最新 pending follow-up 回编辑器。
  const dequeue = async () => {
    if (!onDequeue) return;
    if (value.trim() || pending.length || draftReferences.length) return;
    const recalled = await onDequeue();
    if (recalled) {
      const turn = typeof recalled === 'string' ? undefined : recalled;
      if (useVivyStore.getState().activeSessionId !== activeSessionId || currentDraft.current.sessionId !== activeSessionId || currentDraft.current.value.trim() || currentDraft.current.pending.length) {
        if (turn) retainReturnedTurn(turn);
        return;
      }
      if (turn) restoreTurnContext(turn);
      setValue(typeof recalled === 'string' ? recalled : recalled.text);
      setRestoredTurn(turn);
      setPending(turn?.attachments?.map((item) => ({ ...item })) ?? []);
      if (turn) setThinkingMode((turn.thinking ?? 'auto') as ThinkingMode);
      textareaRef.current?.focus();
    }
  };

  // D3: chat-level compact — popover anchored at the context meter; the
  // optional instructions steer the summary focus (context/compact RPC).
  const runCompact = async () => {
    if (!activeSessionId || compacting) return;
    setCompacting(true);
    try {
      const instructions = compactInstructions.trim();
      const result = await compactSession(activeSessionId, instructions || undefined);
      setCompactOpen(false);
      setCompactInstructions('');
      showNotice(result.skipped
        ? t('chatInput.compactSkipped')
        : t('chatInput.compactDone', { before: result.before_tokens.toLocaleString(dateTimeLocale()), after: result.after_tokens.toLocaleString(dateTimeLocale()) }));
    } catch (error) {
      showNotice(error instanceof Error ? error.message : t('chatInput.compactFailed'));
    } finally {
      setCompacting(false);
    }
  };

  const applyPermission = async (preset: PermissionMode) => {
    if (!activeSessionId || permissionLocked || preset === permissionPreset) return;
    try {
      await setSessionPermission(activeSessionId, preset);
    } catch (error) {
      showNotice(error instanceof Error ? error.message : t('chatInput.permissionSwitchFailed'));
    }
  };

  const choosePermission = (preset: PermissionMode) => {
    if (permissionLocked || preset === permissionPreset) return;
    if (preset === 'trusted') {
      setConfirmTrusted(true);
      return;
    }
    void applyPermission(preset);
  };

  return <div className="p-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] pt-2 sm:p-4 sm:pb-4"><div className="mx-auto relative max-w-3xl rounded-2xl border border-border bg-card shadow-sm">
    {/* 顶部功能栏（内容与交互对照 Agent-DIVA chat-input-toolbar） */}
    <div className="flex items-center gap-1 overflow-x-auto px-3 pb-1 pt-2.5 text-muted-foreground">
      {/* 历史引用（SC-D4 §13.1）：打开选择器只列会话元数据，不预载转写。 */}
      {activeSessionId ? (
        <button type="button" onClick={() => setPickerOpen(true)} disabled={disabled} title={t('chatInput.attachHistory')} aria-label={t('chatInput.attachHistory')} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent disabled:pointer-events-none disabled:opacity-50">
          <History className="h-4 w-4" />
        </button>
      ) : null}

      {/* 附件（图片）：与服务端同款门禁（png/jpeg/gif/webp、5MB、每条最多 4 张） */}
      <label className={cn('shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent', disabled ? 'pointer-events-none opacity-50' : 'cursor-pointer')} title={t('chatInput.attachment')} aria-label={t('chatInput.attachment')}>
        <Paperclip className="h-4 w-4" />
        <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" multiple className="hidden" disabled={disabled} onChange={(event) => { void addFiles(event.target.files ?? []); event.target.value = ''; }} />
      </label>

      {restoredTurn ? <select aria-label={t('chatInput.executionMode')} disabled={disabled} value={restoredTurn.mode ?? 'normal'} onChange={(event) => setRestoredTurn((turn) => turn && ({ ...turn, mode: event.target.value }))} className="rounded-lg bg-transparent p-1.5 text-xs">
        <option value="normal">{t('chatInput.agentMode')}</option>
        <option value="plan">{t('chatInput.planMode')}</option>
      </select> : null}

      {/* 思考模式选择：D9 门控 — 仅当会话上下文报告活动模型支持思考时出现；
          发送链始终携带当前偏好（默认 auto），由服务端最终裁决。 */}
      {context?.thinking_supported || restoredTurn ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button type="button" className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title={t('chatInput.thinkingMode')} aria-label={t('chatInput.thinkingMode')}>
              {thinkingModeOption.filled
                ? <thinkingModeOption.icon className="h-4 w-4" fill="currentColor" />
                : <thinkingModeOption.icon className="h-4 w-4" />}
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="bottom" align="start" className="min-w-36 p-1.5">
            {THINKING_MODES.map((mode) => (
              <DropdownMenuItem
                key={mode.value}
                onSelect={() => setThinkingMode(mode.value)}
                className={cn('gap-2', thinkingMode === mode.value && 'bg-accent text-accent-foreground')}
              >
                {mode.filled
                  ? <mode.icon className="size-4 shrink-0" fill="currentColor" />
                  : <mode.icon className="size-4 shrink-0" />}
                <span className="flex-1">{mode.label.startsWith('chatInput.') ? t(mode.label) : mode.label}</span>
                {thinkingMode === mode.value ? <Check className="size-4 shrink-0 text-primary" /> : null}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}

      {/* 权限模式选择 */}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button type="button" disabled={permissionLocked} title={running ? t('chatInput.permissionLocked') : t(permissionModeOption.desc)} className="flex shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-xs transition-colors hover:bg-accent disabled:opacity-50">
            <permissionModeOption.icon className="h-3.5 w-3.5" />
            <span>{permissionPreset === 'custom' ? t('chatInput.permissionCustom') : t(permissionModeOption.label)}</span>
            <ChevronDown className="h-3 w-3" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent side="top" align="start" className="w-64 p-1.5">
          {PERMISSION_MODES.map((mode) => (
            <DropdownMenuItem
              key={mode.value}
              disabled={permissionLocked}
              onSelect={() => choosePermission(mode.value)}
              className={cn('gap-2.5 py-2', permissionPreset === mode.value && 'bg-accent text-accent-foreground')}
            >
              <mode.icon className="size-4 shrink-0" />
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-semibold">{t(mode.label)}</span>
                <span className="block text-xs text-muted-foreground">{t(mode.desc)}</span>
              </span>
              {permissionPreset === mode.value ? <Check className="size-4 shrink-0 text-primary" /> : null}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      {/* 右侧：审批中心 */}
      <div className="ml-auto flex shrink-0 items-center gap-1">
        <button type="button" aria-expanded={reviewCenterOpen} onClick={() => openReviewCenter(true)} className="relative shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title={t('chatInput.reviewCenter')} aria-label={t('chatInput.reviewCenter')}><ShieldCheck className="h-4 w-4" />{pendingReviewCount ? <span className="absolute -right-0.5 -top-0.5 grid h-[18px] min-w-[18px] place-items-center rounded-full bg-destructive px-1 text-[10px] font-bold leading-none text-white" aria-hidden="true">{pendingReviewCount}</span> : null}</button>
      </div>
    </div>
    {/* 队列 pill（对照 Crush + pi 双轨）：内核 steer/follow_up 与本地
        条目，可逐条移除或整体清空 */}
    {restoredTurn?.file_contexts?.map((file, index) => <div key={`${file.path}-${index}`} className="flex items-center gap-2 px-3 text-xs text-muted-foreground">
      <span>{t('chatInput.capturedContext', { name: file.name || file.path })}</span>
      <button type="button" aria-label={t('chatInput.removeCapturedContext', { name: file.name || file.path })} onClick={() => setRestoredTurn((turn) => turn && ({ ...turn, file_contexts: turn.file_contexts?.filter((_, i) => i !== index), context_paths: undefined }))}><X className="h-3 w-3" /></button>
    </div>)}
    {recoveryCount ? <button type="button" onClick={() => void dequeue()} className="text-xs text-muted-foreground">{t('chatInput.recallReturned', { count: recoveryCount })}</button> : null}
    {totalQueued ? (
      <div className="flex items-center gap-2 border-t border-border/60 bg-muted/30 px-3 py-1.5 text-xs text-muted-foreground">
        <Clock className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
        <span className="shrink-0 font-medium">{t('chatInput.queuedCount', { count: totalQueued })}</span>
        <div className="flex min-w-0 flex-1 gap-1.5 overflow-x-auto" aria-live="polite">
          {kernelItems.map(({ item, lane }) => (
            <span key={item.id} className="flex shrink-0 items-center gap-1 rounded-full bg-muted px-2 py-0.5">
              <span className={cn('rounded-full px-1 text-[10px] font-semibold', lane === 'steer' ? 'bg-primary/15 text-primary' : 'bg-accent text-muted-foreground')}>{t(lane === 'steer' ? 'chatInput.queueLaneSteer' : 'chatInput.queueLaneFollowUp')}</span>
              <span className="max-w-40 truncate">{item.text}</span>
              <button type="button" onClick={() => void removeKernelQueued(item.id)} title={t('chatInput.removeQueued')} aria-label={`${t('chatInput.removeQueued')}: ${item.text}`} className="rounded-full p-0.5 transition-colors hover:bg-accent hover:text-foreground"><X className="h-3 w-3" aria-hidden="true" /></button>
            </span>
          ))}

        </div>
        <button type="button" onClick={clearQueue} className="shrink-0 rounded-lg px-2 py-0.5 transition-colors hover:bg-accent hover:text-foreground" title={t('chatInput.clearQueue')} aria-label={t('chatInput.clearQueue')}>{t('chatInput.clearQueue')}</button>
      </div>
    ) : null}
    {/* 已附历史引用 chip：来源会话 + 条数 + 捕获时间，可查看/移除 */}
    {draftReferences.length ? (
      <div className="flex flex-wrap gap-1.5 border-t border-border/60 px-3 py-2" aria-live="polite">
        {draftReferences.map((draft) => (
          <ContextReferenceChip key={draft.id} preview={draft.preview} onRemove={() => removeDraftReference(draft.id)} />
        ))}
      </div>
    ) : null}
    {/* 待发送附件缩略图（贴图 / 选择文件共用） */}
    {pending.length ? (
      <div className="flex flex-wrap gap-2 border-t border-border/60 px-3 py-2">
        {pending.map((item, index) => (
          <span key={`${item.name ?? 'image'}-${index}`} className="relative">
            <img src={`data:${item.mime_type};base64,${item.data}`} alt={item.name || t('chatInput.attachment')} className="h-14 w-14 rounded-lg border border-border object-cover" />
            <button type="button" onClick={() => setPending((current) => current.filter((_, position) => position !== index))} title={t('chatInput.removeAttachment')} aria-label={`${t('chatInput.removeAttachment')}: ${item.name || item.mime_type}`} className="absolute -right-1.5 -top-1.5 rounded-full bg-destructive p-0.5 text-white shadow-sm transition-opacity hover:opacity-90"><X className="h-3 w-3" aria-hidden="true" /></button>
          </span>
        ))}
      </div>
    ) : null}
    <ComposerCommandControls commands={commands} disabled={disabled} focus={() => textareaRef.current?.focus()} />
    <Textarea ref={textareaRef} role="combobox" aria-label={t('chatInput.message')} aria-autocomplete="list" aria-expanded={commands.menuOpen} aria-controls={commands.menuOpen ? commands.menuId : undefined} aria-activedescendant={commands.menuOpen && commands.options.length ? `${commands.menuId}-${commands.index}` : undefined} value={value} onChange={(event) => commands.changeValue(event.target.value)} onPaste={(event) => {
      // 剪贴板贴图（对照 Crush）：有图片时接管粘贴，文本粘贴不受影响。
      const images = Array.from(event.clipboardData.files).filter((file) => file.type.startsWith('image/'));
      if (!images.length) return;
      event.preventDefault();
      void addFiles(images);
    }} onBlur={(event) => { if (!event.currentTarget.parentElement?.contains(event.relatedTarget as Node | null)) commands.dismiss(); }} onKeyDown={(event) => {
      if (commands.handleKeyDown(event)) return;
      // pi 键位：运行中 Enter=steer、Shift+Alt+Enter=follow-up（浏览器安全）、
      // Alt+↑=撤回最新排队项回编辑框；Shift+Enter 仍是换行。
      if (event.key === 'Enter' && event.shiftKey && event.altKey && running) { event.preventDefault(); void send('follow_up'); return; }
      if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); void send('steer'); return; }
      if (event.key === 'ArrowUp' && event.altKey) { event.preventDefault(); void dequeue(); return; }
      // esc 两段式（对照 Crush）：第一次清空队列，再一次取消运行
      if (event.key === 'Escape' && running) {
        event.preventDefault();
        if (totalQueued) clearQueue();
        else void onCancel?.();
      }
    }} placeholder={placeholder || t(commands.draft?.name === 'goal' ? (commands.draft.goalRef ? 'composerCommands.resumePlaceholder' : 'composerCommands.goalPlaceholder') : commands.draft?.name === 'plan' ? 'composerCommands.planPlaceholder' : 'chatInput.placeholder')} disabled={disabled} className="max-h-40 min-h-14 resize-none border-0 bg-transparent px-4 shadow-none focus-visible:ring-0" rows={1} />
    <div className="flex items-center gap-2 px-3 pb-2.5"><div className="flex shrink-0 items-center gap-1.5" title={contextTitleText()}>
      <div role="progressbar" aria-label={t('chatInput.contextLabel')} aria-valuemin={0} aria-valuemax={100} aria-valuenow={contextPercent} aria-valuetext={t('chatInput.contextValueText', { used: usedTokens, limit: limitTokens })} className="relative h-7 w-7">
        <svg viewBox="0 0 24 24" className="h-7 w-7 -rotate-90" aria-hidden="true"><circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" strokeWidth="2.5" className="text-muted" /><circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeDasharray={contextCircumference} strokeDashoffset={contextCircumference * (1 - contextRatio)} className={`transition-[stroke-dashoffset] duration-300 ${contextColor}`} /></svg>
      </div>
      <span className="min-w-[2.25rem] text-xs font-medium text-muted-foreground">{contextPercent}%</span>
      {context?.compaction_enabled ? (
        <Popover open={compactOpen} onOpenChange={setCompactOpen}>
          <PopoverTrigger asChild>
            <button type="button" disabled={disabled || !activeSessionId || compacting} title={running ? t('chatInput.compactBusy') : t('chatInput.compact')} aria-label={t('chatInput.compact')} className="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:pointer-events-none disabled:opacity-50">
              <Shrink className="h-4 w-4" />
            </button>
          </PopoverTrigger>
          <PopoverContent side="top" align="start" className="w-72 p-3">
            <p className="text-sm font-medium">{t('chatInput.compact')}</p>
            <p className="mt-1 text-xs text-muted-foreground">{t('chatInput.compactHint')}</p>
            <Input value={compactInstructions} onChange={(event) => setCompactInstructions(event.target.value)} placeholder={t('chatInput.compactInstructionsPlaceholder')} className="mt-2 h-8 text-sm" aria-label={t('chatInput.compactInstructionsPlaceholder')} />
            <Button type="button" size="sm" className="mt-2 w-full" disabled={compacting} onClick={() => void runCompact()}>
              {compacting ? t('chatInput.compacting') : t('chatInput.compactNow')}
            </Button>
          </PopoverContent>
        </Popover>
      ) : null}
    </div><WorkspaceSelector workspacePath={activeSession?.workspace_path ?? ''} disabled={!activeSessionId || permissionLocked} onSelect={chooseWorkspace} />{notice ? <span className="min-w-0 truncate text-xs text-muted-foreground" aria-live="polite">{notice}</span> : null}<div className="flex-1" />{running ? (
  <>
    <button type="button" onClick={() => void send('follow_up')} disabled={disabled || !canSend} className="rounded-full border border-border p-2.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:opacity-40" title={t('chatInput.sendAsFollowUp')} aria-label={t('chatInput.sendAsFollowUp')}><Clock className="h-4 w-4" /></button>
    <button type="button" onClick={() => void send('steer')} disabled={disabled || !canSend} className="rounded-full bg-primary p-2.5 text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-40" title={t('chatInput.steer')} aria-label={t('chatInput.steer')}><Send className="h-4 w-4" /></button>
    <Button size="icon" variant="destructive" className="rounded-full" onClick={totalQueued ? () => clearQueue() : () => void onCancel?.()} disabled={disabled} title={totalQueued ? t('chatInput.clearQueue') : t('chatInput.cancelRun')} aria-label={totalQueued ? t('chatInput.clearQueue') : t('chatInput.cancelRun')}><Square className="h-4 w-4" /></Button>
  </>
) : <button type="button" onClick={() => void send('send')} disabled={disabled || !canSend} className="rounded-full bg-primary p-2.5 text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-40" title={t('chatInput.send')} aria-label={t('chatInput.send')}><Send className="h-4 w-4" /></button>}</div>
  </div>
    <AlertDialog open={confirmTrusted} onOpenChange={setConfirmTrusted}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('chatInput.trustedConfirmTitle')}</AlertDialogTitle>
          <AlertDialogDescription>{t('chatInput.trustedConfirmDescription')}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
          <AlertDialogAction onClick={() => { setConfirmTrusted(false); void applyPermission('trusted'); }}>{t('chatInput.trustedConfirm')}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
    {activeSessionId ? (
      <HistoryPicker
        destinationSessionId={activeSessionId}
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        onAttach={addDraftReference}
      />
    ) : null}
  </div>;
}
