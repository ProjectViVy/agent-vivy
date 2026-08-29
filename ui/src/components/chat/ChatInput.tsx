import { useEffect, useRef, useState } from 'react';
import {
  Brain, Cat, Check, CheckCircle, ChevronDown, Clock, GitBranch, Lightbulb, LightbulbOff,
  Mic, Paperclip, Plus, Send, Settings2, Shield, ShieldCheck, Sparkles, Square, Zap,
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import type { PermissionPreset } from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/i18n';

interface ChatInputProps {
  onSend: (content: string) => Promise<void> | void;
  onCancel?: () => Promise<void> | void;
  disabled?: boolean;
  running?: boolean;
  placeholder?: string;
  contextBytes?: number;
}

type ExecMode = 'agent' | 'plan' | 'ask';
type ThinkingMode = 'auto' | 'on' | 'off';
type PermissionMode = 'cautious' | 'smart' | 'trusted';

const ESTIMATED_CONTEXT_LIMIT_BYTES = 256 * 1024;
const TEXT_ENCODER = new TextEncoder();

// 执行模式选项（对照 Agent-DIVA ChatView.modeOptions）
const MODES: { value: ExecMode; icon: LucideIcon; label: string; desc: string }[] = [
  { value: 'agent', icon: Zap, label: 'chatInput.agentMode', desc: 'chatInput.agentModeDesc' },
  { value: 'plan', icon: Settings2, label: 'chatInput.planMode', desc: 'chatInput.planModeDesc' },
  { value: 'ask', icon: Brain, label: 'chatInput.askMode', desc: 'chatInput.askModeDesc' },
];

// 思考模式选项（对照 Agent-DIVA ThinkingToggle）
const THINKING_MODES: { value: ThinkingMode; icon: LucideIcon; label: string; filled?: boolean }[] = [
  { value: 'auto', icon: Lightbulb, label: 'chatInput.thinkingModeAuto' },
  { value: 'on', icon: Lightbulb, label: 'chatInput.thinkingModeOn', filled: true },
  { value: 'off', icon: LightbulbOff, label: 'chatInput.thinkingModeOff' },
];

// 权限模式选项（对照 Agent-DIVA ChatView.permissionOptions）
const PERMISSION_MODES: { value: PermissionMode; icon: LucideIcon; label: string; desc: string }[] = [
  { value: 'cautious', icon: Shield, label: 'chatInput.permissionCautious', desc: 'chatInput.permissionCautiousDesc' },
  { value: 'smart', icon: Sparkles, label: 'chatInput.permissionSmart', desc: 'chatInput.permissionSmartDesc' },
  { value: 'trusted', icon: CheckCircle, label: 'chatInput.permissionTrusted', desc: 'chatInput.permissionTrustedDesc' },
];

export function ChatInput({ onSend, onCancel, disabled, running, placeholder, contextBytes = 0 }: ChatInputProps) {
  const [value, setValue] = useState('');
  const [notice, setNotice] = useState<string | null>(null);
  const [execMode, setExecMode] = useState<ExecMode>('agent');
  const [thinkingMode, setThinkingMode] = useState<ThinkingMode>('auto');
  const [recording, setRecording] = useState(false);
  const [confirmTrusted, setConfirmTrusted] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const noticeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reviewCenterOpen = useVivyStore((state) => state.reviewCenterOpen);
  const openReviewCenter = useVivyStore((state) => state.setReviewCenterOpen);
  const openSessionDrawer = useVivyStore((state) => state.setSessionDrawerOpen);
  const pendingReviewCount = useVivyStore((state) => state.reviews.filter((review) => review.status === 'pending').length);
  const activeSessionId = useVivyStore((state) => state.activeSessionId);
  const sessions = useVivyStore((state) => state.sessions);
  const sessionBusyId = useVivyStore((state) => state.sessionBusyId);
  const setSessionPermission = useVivyStore((state) => state.setSessionPermission);
  const { t } = useTranslation();
  const activeSession = sessions.find((session) => session.id === activeSessionId);
  const permissionPreset: PermissionPreset = activeSession?.permission_preset ?? 'smart';
  const permissionBusy = sessionBusyId === activeSessionId;
  const draftBytes = TEXT_ENCODER.encode(value).length;
  const usedContextBytes = contextBytes + draftBytes;
  const contextRatio = Math.min(1, usedContextBytes / ESTIMATED_CONTEXT_LIMIT_BYTES);
  const contextPercent = Math.round(contextRatio * 100);
  const contextCircumference = 2 * Math.PI * 10;
  const contextColor = contextPercent >= 80 ? 'text-destructive' : contextPercent >= 60 ? 'text-amber-500' : 'text-primary';
  const execModeOption = MODES.find((mode) => mode.value === execMode)!;
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

  const showNotice = (message: string) => {
    setNotice(message);
    if (noticeTimer.current) clearTimeout(noticeTimer.current);
    noticeTimer.current = setTimeout(() => setNotice(null), 1800);
  };

  const send = async () => {
    const content = value.trim();
    if (!content || disabled || running) return;
    try {
      await onSend(content);
      setValue('');
    } catch {
      /* keep the draft; ChatView / store already expose the failure */
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

  return <div className="p-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] pt-2 sm:p-4 sm:pb-4"><div className="mx-auto max-w-3xl rounded-2xl border border-border bg-card shadow-sm">
    {/* 顶部功能栏（内容与交互对照 Agent-DIVA chat-input-toolbar） */}
    <div className="flex items-center gap-1 overflow-x-auto px-3 pb-1 pt-2.5 text-muted-foreground">
      {/* 执行模式选择 */}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button type="button" className="flex shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-xs transition-colors hover:bg-accent">
            <execModeOption.icon className="h-3.5 w-3.5" />
            <span>{t(execModeOption.label)}</span>
            <ChevronDown className="h-3 w-3" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent side="top" align="start" className="w-64 p-1.5">
          {MODES.map((mode) => (
            <DropdownMenuItem
              key={mode.value}
              onSelect={() => setExecMode(mode.value)}
              className={cn('gap-2.5 py-2', execMode === mode.value && 'bg-accent text-accent-foreground')}
            >
              <mode.icon className="size-4 shrink-0" />
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-semibold">{t(mode.label)}</span>
                <span className="block text-xs text-muted-foreground">{t(mode.desc)}</span>
              </span>
              {execMode === mode.value ? <Check className="size-4 shrink-0 text-primary" /> : null}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      {/* 附件 */}
      <button type="button" onClick={() => showNotice(t('chatInput.attachmentUnavailable'))} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title={t('chatInput.attachment')} aria-label={t('chatInput.attachment')}><Paperclip className="h-4 w-4" /></button>

      {/* 思考模式选择 */}
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
              <span className="flex-1">{t(mode.label)}</span>
              {thinkingMode === mode.value ? <Check className="size-4 shrink-0 text-primary" /> : null}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      {/* AutoDream 触发 */}
      <button type="button" onClick={() => showNotice(t('chatInput.autodreamUnavailable'))} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title={t('chatInput.autodreamTrigger')} aria-label={t('chatInput.autodreamTrigger')}><GitBranch className="h-4 w-4" /></button>

      {/* 桌面伙伴 */}
      <button type="button" onClick={() => showNotice(t('chatInput.mateUnavailable'))} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title={t('chatInput.openMate')} aria-label={t('chatInput.openMate')}><Cat className="h-4 w-4" /></button>

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

      {/* 右侧：历史 + 审批中心（对照 Agent-DIVA chat-corner-actions） */}
      <div className="ml-auto flex shrink-0 items-center gap-1">
        <button type="button" onClick={() => openSessionDrawer(true)} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title={t('chatInput.history')} aria-label={t('chatInput.history')}><Clock className="h-4 w-4" /></button>
        <button type="button" aria-expanded={reviewCenterOpen} onClick={() => openReviewCenter(true)} className="relative shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title={t('chatInput.reviewCenter')} aria-label={t('chatInput.reviewCenter')}><ShieldCheck className="h-4 w-4" />{pendingReviewCount ? <span className="absolute -right-0.5 -top-0.5 grid h-[18px] min-w-[18px] place-items-center rounded-full bg-destructive px-1 text-[10px] font-bold leading-none text-white" aria-hidden="true">{pendingReviewCount}</span> : null}</button>
      </div>
    </div>
    <Textarea ref={textareaRef} value={value} onChange={(event) => setValue(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); void send(); } }} placeholder={placeholder || t('chatInput.placeholder')} disabled={disabled || running} className="max-h-40 min-h-14 resize-none border-0 bg-transparent px-4 shadow-none focus-visible:ring-0" rows={1} />
    <div className="flex items-center gap-2 px-3 pb-2.5"><div className="flex shrink-0 items-center gap-1.5" title={t('chatInput.contextTitle', { percent: contextPercent, used: usedContextBytes.toLocaleString(), limit: ESTIMATED_CONTEXT_LIMIT_BYTES.toLocaleString() })}><div role="progressbar" aria-label={t('chatInput.contextLabel')} aria-valuemin={0} aria-valuemax={100} aria-valuenow={contextPercent} aria-valuetext={t('chatInput.contextValueText', { used: usedContextBytes, limit: ESTIMATED_CONTEXT_LIMIT_BYTES })} className="relative h-7 w-7"><svg viewBox="0 0 24 24" className="h-7 w-7 -rotate-90" aria-hidden="true"><circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" strokeWidth="2.5" className="text-muted" /><circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeDasharray={contextCircumference} strokeDashoffset={contextCircumference * (1 - contextRatio)} className={`transition-[stroke-dashoffset] duration-300 ${contextColor}`} /></svg></div><span className="min-w-[2.25rem] text-xs font-medium text-muted-foreground">{contextPercent}%</span></div>{notice ? <span className="min-w-0 truncate text-xs text-muted-foreground" aria-live="polite">{notice}</span> : null}<div className="flex-1" /><button type="button" onClick={() => showNotice(t('chatInput.moreUnavailable'))} className="rounded-full p-2 text-muted-foreground transition-colors hover:bg-accent" title={t('chatInput.more')} aria-label={t('chatInput.more')}><Plus className="h-4 w-4" /></button><button type="button" aria-pressed={recording} onClick={() => { setRecording((current) => !current); showNotice(recording ? t('chatInput.voiceStopped') : t('chatInput.voiceStarted')); }} className="rounded-full p-2 text-muted-foreground transition-colors hover:bg-accent" title={t('chatInput.voice')} aria-label={t('chatInput.voice')}><Mic className="h-4 w-4" /></button>{running ? <Button size="icon" variant="destructive" className="rounded-full" onClick={() => void onCancel?.()} disabled={disabled} title={t('chatInput.cancelRun')} aria-label={t('chatInput.cancelRun')}><Square className="h-4 w-4" /></Button> : <button type="button" onClick={() => void send()} disabled={disabled || !value.trim()} className="rounded-full bg-primary p-2.5 text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-40" title={t('chatInput.send')} aria-label={t('chatInput.send')}><Send className="h-4 w-4" /></button>}</div>
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
  </div>;
}