import { useEffect, useRef, useState } from 'react';
import { Clock, GitBranch, Mic, Palette, Paperclip, Plus, Send, ShieldCheck, Sparkles, Square, Zap } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { useVivyStore } from '@/lib/store';

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
      <button type="button" aria-pressed={agentMode} onClick={() => { setAgentMode((current) => !current); showNotice(agentMode ? '已切换到普通模式' : '已切换到智能体模式'); }} className="flex shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-xs transition-colors hover:bg-accent"><Zap className="h-3.5 w-3.5"/>智能体模式</button><span className="mx-1 h-4 w-px shrink-0 bg-border"/>
      <button type="button" onClick={() => showNotice('附件功能暂未接入')} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title="附件" aria-label="附件"><Paperclip className="h-4 w-4"/></button>
      <button type="button" onClick={() => showNotice('画图功能暂未接入')} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title="画图" aria-label="画图"><Palette className="h-4 w-4"/></button>
      <button type="button" onClick={() => showNotice('分支功能暂未接入')} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title="分支" aria-label="分支"><GitBranch className="h-4 w-4"/></button>
      <button type="button" onClick={() => showNotice('智能策略：自动')} className="flex shrink-0 items-center gap-1 rounded-lg px-2 py-1 text-xs transition-colors hover:bg-accent"><Sparkles className="h-3.5 w-3.5"/>智能</button>
      <div className="min-w-2 flex-1"/><button type="button" onClick={() => showNotice('会话历史请从右上角打开')} className="shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title="历史" aria-label="历史"><Clock className="h-4 w-4"/></button><button type="button" aria-expanded={reviewCenterOpen} onClick={() => openReviewCenter(true)} className="relative shrink-0 rounded-lg p-1.5 transition-colors hover:bg-accent" title="审批中心" aria-label="审批中心"><ShieldCheck className="h-4 w-4"/>{pendingReviewCount ? <span className="absolute right-0.5 top-0.5 h-2 w-2 rounded-full bg-destructive" aria-hidden="true"/> : null}</button>
    </div>
    <Textarea ref={textareaRef} value={value} onChange={(event) => setValue(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); void send(); } }} placeholder={placeholder || '输入消息... (Enter 发送)'} disabled={disabled || running} className="max-h-40 min-h-14 resize-none border-0 bg-transparent px-4 shadow-none focus-visible:ring-0" rows={1}/>
    <div className="flex items-center gap-2 px-3 pb-2.5"><div className="flex shrink-0 items-center gap-1.5" title={`上下文占用估算：${contextPercent}% · ${usedContextBytes.toLocaleString()} / ${ESTIMATED_CONTEXT_LIMIT_BYTES.toLocaleString()} 字节`}><div role="progressbar" aria-label="上下文占用" aria-valuemin={0} aria-valuemax={100} aria-valuenow={contextPercent} aria-valuetext={`估算已使用 ${usedContextBytes} 字节，共 ${ESTIMATED_CONTEXT_LIMIT_BYTES} 字节`} className="relative h-7 w-7"><svg viewBox="0 0 24 24" className="h-7 w-7 -rotate-90" aria-hidden="true"><circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" strokeWidth="2.5" className="text-muted"/><circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeDasharray={contextCircumference} strokeDashoffset={contextCircumference * (1 - contextRatio)} className={`transition-[stroke-dashoffset] duration-300 ${contextColor}`}/></svg></div><span className="min-w-[2.25rem] text-xs font-medium text-muted-foreground">{contextPercent}%</span></div>{notice ? <span className="min-w-0 truncate text-xs text-muted-foreground" aria-live="polite">{notice}</span> : null}<div className="flex-1"/><button type="button" onClick={() => showNotice('更多工具暂未接入')} className="rounded-full p-2 text-muted-foreground transition-colors hover:bg-accent" title="更多" aria-label="更多"><Plus className="h-4 w-4"/></button><button type="button" aria-pressed={recording} onClick={() => { setRecording((current) => !current); showNotice(recording ? '语音输入已停止' : '语音输入演示已开启'); }} className="rounded-full p-2 text-muted-foreground transition-colors hover:bg-accent" title="语音" aria-label="语音"><Mic className="h-4 w-4"/></button>{running ? <Button size="icon" variant="destructive" className="rounded-full" onClick={() => void onCancel?.()} disabled={disabled} title="取消运行" aria-label="取消运行"><Square className="h-4 w-4"/></Button> : <button type="button" onClick={() => void send()} disabled={disabled || !value.trim()} className="rounded-full bg-primary p-2.5 text-primary-foreground transition-opacity hover:opacity-90 disabled:opacity-40" title="发送" aria-label="发送"><Send className="h-4 w-4"/></button>}</div>
  </div></div>;
}
