import { useEffect, useRef, useState } from 'react';
import { Clock, GitBranch, Mic, Palette, Paperclip, Plus, Send, ShieldCheck, Sparkles, Square, Zap } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';

interface ChatInputProps {
  onSend: (content: string) => Promise<void> | void;
  onCancel?: () => Promise<void> | void;
  disabled?: boolean;
  running?: boolean;
  placeholder?: string;
  contextBytes?: number;
}

const ESTIMATED_CONTEXT_LIMIT_BYTES = 256 * 1024;
const TEXT_ENCODER = new TextEncoder();

export function ChatInput({ onSend, onCancel, disabled, running, placeholder, contextBytes = 0 }: ChatInputProps) {
  const [value, setValue] = useState('');
  const [notice, setNotice] = useState<string | null>(null);
  const [agentMode, setAgentMode] = useState(true);
  const [recording, setRecording] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const noticeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reviewCenterOpen = useVivyStore((state) => state.reviewCenterOpen);
  const openReviewCenter = useVivyStore((state) => state.setReviewCenterOpen);
  const pendingReviewCount = useVivyStore((state) => state.reviews.filter((review) => review.status === 'pending').length);
  const { t } = useTranslation();
  const draftBytes = TEXT_ENCODER.encode(value).length;
  const usedContextBytes = contextBytes + draftBytes;
  const contextRatio = Math.min(1, usedContextBytes / ESTIMATED_CONTEXT_LIMIT_BYTES);
  const contextPercent = Math.round(contextRatio * 100);
  const contextCircumference = 2 * Math.PI * 10;
  const contextColor = contextPercent >= 80 ? 'text-destructive' : contextPercent >= 60 ? 'text-amber-500' : 'text-primary';

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
    await onSend(content);
    setValue('');
  };

  return <div className="p-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] pt-2 sm:p-4 sm:pb-4"><div className="mx-auto max-w-3xl rounded-2xl border border-border bg-card shadow-sm">
    <div className="flex items-center gap-1 overflow-x-auto px-3 pb-1 pt-2.5 text-muted-foreground">
      <button type="button" aria-pressed={agentMode} onClick={() => { setAgentMode((current) => !current); showNotice(agentMode ? t('chatInput.switchedToNormal') : t('chatInput.switchedToAgent')); }} className="flex shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-xs transition-colors hover:bg-accent"><Zap className="h-3.5 w-3.5"/>{t('chatInput.agentMode')}</button><span className="mx-1 h-4 w-px shrink-0 bg-border"/>
      <button type="button" onClick={() => showNotice(t('chatInput.attachmentUnavailable'))} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title={t('chatInput.attachment')} aria-label={t('chatInput.attachment')}><Paperclip className="h-4 w-4"/></button>
      <button type="button" onClick={() => showNotice(t('chatInput.drawUnavailable'))} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title={t('chatInput.draw')} aria-label={t('chatInput.draw')}><Palette className="h-4 w-4"/></button>
      <button type="button" onClick={() => showNotice(t('chatInput.branchUnavailable'))} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title={t('chatInput.branch')} aria-label={t('chatInput.branch')}><GitBranch className="h-4 w-4"/></button>
      <button type="button" onClick={() => showNotice(t('chatInput.smartStrategy'))} className="flex shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-xs transition-colors hover:bg-accent"><Sparkles className="h-3.5 w-3.5"/>{t('chatInput.smart')}</button>
      <div className="min-w-2 flex-1"/><button type="button" onClick={() => showNotice(t('chatInput.historyHint'))} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title={t('chatInput.history')} aria-label={t('chatInput.history')}><Clock className="h-4 w-4"/></button><button type="button" aria-expanded={reviewCenterOpen} onClick={() => openReviewCenter(true)} className="relative shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title={t('chatInput.reviewCenter')} aria-label={t('chatInput.reviewCenter')}><ShieldCheck className="h-4 w-4"/>{pendingReviewCount ? <span className="absolute right-0.5 top-0.5 h-2 w-2 rounded-full bg-destructive" aria-hidden="true"/> : null}</button>
    </div>
    <Textarea ref={textareaRef} value={value} onChange={(event) => setValue(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); void send(); } }} placeholder={placeholder || t('chatInput.placeholder')} disabled={disabled || running} className="max-h-40 min-h-14 resize-none border-0 bg-transparent px-4 shadow-none focus-visible:ring-0" rows={1}/>
    <div className="flex items-center gap-2 px-3 pb-2.5"><div className="flex shrink-0 items-center gap-1.5" title={t('chatInput.contextTitle', { percent: contextPercent, used: usedContextBytes.toLocaleString(), limit: ESTIMATED_CONTEXT_LIMIT_BYTES.toLocaleString() })}><div role="progressbar" aria-label={t('chatInput.contextLabel')} aria-valuemin={0} aria-valuemax={100} aria-valuenow={contextPercent} aria-valuetext={t('chatInput.contextValueText', { used: usedContextBytes, limit: ESTIMATED_CONTEXT_LIMIT_BYTES })} className="relative h-7 w-7"><svg viewBox="0 0 24 24" className="h-7 w-7 -rotate-90" aria-hidden="true"><circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" strokeWidth="2.5" className="text-muted"/><circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeDasharray={contextCircumference} strokeDashoffset={contextCircumference * (1 - contextRatio)} className={`transition-[stroke-dashoffset] duration-300 ${contextColor}`}/></svg></div><span className="min-w-[2.25rem] text-xs font-medium text-muted-foreground">{contextPercent}%</span></div>{notice ? <span className="min-w-0 truncate text-xs text-muted-foreground" aria-live="polite">{notice}</span> : null}<div className="flex-1"/><button type="button" onClick={() => showNotice(t('chatInput.moreUnavailable'))} className="rounded-full p-2 text-muted-foreground transition-colors hover:bg-accent" title={t('chatInput.more')} aria-label={t('chatInput.more')}><Plus className="h-4 w-4"/></button><button type="button" aria-pressed={recording} onClick={() => { setRecording((current) => !current); showNotice(recording ? t('chatInput.voiceStopped') : t('chatInput.voiceStarted')); }} className="rounded-full p-2 text-muted-foreground transition-colors hover:bg-accent" title={t('chatInput.voice')} aria-label={t('chatInput.voice')}><Mic className="h-4 w-4"/></button>{running ? <Button size="icon" variant="destructive" className="rounded-full" onClick={() => void onCancel?.()} disabled={disabled} title={t('chatInput.cancelRun')} aria-label={t('chatInput.cancelRun')}><Square className="h-4 w-4"/></Button> : <button type="button" onClick={() => void send()} disabled={disabled || !value.trim()} className="rounded-full bg-primary p-2.5 text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-40" title={t('chatInput.send')} aria-label={t('chatInput.send')}><Send className="h-4 w-4"/></button>}</div>
  </div></div>;
}
