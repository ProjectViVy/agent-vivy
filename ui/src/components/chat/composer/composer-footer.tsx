import { useState } from 'react';
import { Clock, Send, Shrink, Square, Zap } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import type { SessionContext } from '@/lib/api';
import { useVivyStore } from '@/lib/store';
import { WorkspaceSelector } from '../WorkspaceSelector';
import { dateTimeLocale, useTranslation } from '@/i18n';

export type SendTrack = 'send' | 'steer' | 'follow_up' | 'send_now';

const ESTIMATED_CONTEXT_LIMIT_TOKENS = 128000;
const CIRCUMFERENCE = 2 * Math.PI * 10;

interface ComposerFooterProps {
  context: SessionContext | null;
  draftBytes: number;
  workspacePath: string;
  notice: string | null;
  onNotice: (message: string) => void;
  running: boolean;
  disabled: boolean;
  canSend: boolean;
  totalQueued: number;
  permissionLocked: boolean;
  onSend: (track: SendTrack) => Promise<void> | void;
  onClearQueue: () => void;
  onCancel?: () => Promise<void> | void;
}

/**
 * Bottom row: real context usage (server feed + draft estimate), the workspace
 * switch, and the one send action for the current run state. The row wraps
 * rather than truncating, so a long workspace name never hides the send button.
 */
export function ComposerFooter({ context, draftBytes, workspacePath, notice, onNotice, running, disabled, canSend, totalQueued, permissionLocked, onSend, onClearQueue, onCancel }: ComposerFooterProps) {
  const { t } = useTranslation();
  const activeSessionId = useVivyStore((state) => state.activeSessionId);
  const compactSession = useVivyStore((state) => state.compactSession);
  const chooseWorkspace = useVivyStore((state) => state.chooseWorkspace);
  const [compactOpen, setCompactOpen] = useState(false);
  const [compactInstructions, setCompactInstructions] = useState('');
  const [compacting, setCompacting] = useState(false);

  // Real usage: the server's feed occupancy plus the current draft (≈4 bytes/token).
  const usedTokens = (context?.feed_tokens ?? 0) + Math.round(draftBytes / 4);
  const limitTokens = context?.model_limit_tokens ?? ESTIMATED_CONTEXT_LIMIT_TOKENS;
  const ratio = Math.min(1, usedTokens / Math.max(1, limitTokens));
  const percent = Math.round(ratio * 100);
  const wouldCompact = context?.compaction_enabled ? usedTokens > (context.trigger_tokens ?? 0) : false;
  const color = wouldCompact || percent >= 80 ? 'text-danger' : percent >= 60 ? 'text-warn' : 'text-brand';
  const title = () => {
    const base = t('chatInput.contextTitle', { percent, used: usedTokens.toLocaleString(dateTimeLocale()), limit: limitTokens.toLocaleString(dateTimeLocale()) });
    if (!context) return base;
    const bytes = ` · ${context.feed_bytes.toLocaleString(dateTimeLocale())} / ${context.limit_bytes.toLocaleString(dateTimeLocale())} B`;
    const status = wouldCompact
      ? ` · ${t('chatInput.contextWouldCompact')}`
      : context.has_compaction_summary || context.last_compaction
        ? ` · ${t('chatInput.contextCompacted')}`
        : '';
    return base + bytes + status;
  };
  // The meter doubles as the compaction entry when the session allows it.
  const compactAvailable = Boolean(context?.compaction_enabled && activeSessionId);
  const meter = (
    <>
      <div className="relative h-7 w-7" aria-hidden="true">
        <svg viewBox="0 0 24 24" className="h-7 w-7 -rotate-90" aria-hidden="true">
          <circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" strokeWidth="2.5" className="text-line" />
          <circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeDasharray={CIRCUMFERENCE} strokeDashoffset={CIRCUMFERENCE * (1 - ratio)} className={`transition-[stroke-dashoffset] duration-300 ${color}`} />
        </svg>
      </div>
      <span className="min-w-[2.25rem] text-meta font-medium text-fg-muted">{percent}%</span>
    </>
  );

  const runCompact = async () => {
    if (!activeSessionId || compacting) return;
    setCompacting(true);
    try {
      const instructions = compactInstructions.trim();
      const result = await compactSession(activeSessionId, instructions || undefined);
      setCompactOpen(false);
      setCompactInstructions('');
      onNotice(result.skipped
        ? t('chatInput.compactSkipped')
        : t('chatInput.compactDone', { before: result.before_tokens.toLocaleString(dateTimeLocale()), after: result.after_tokens.toLocaleString(dateTimeLocale()) }));
    } catch (error) {
      onNotice(error instanceof Error ? error.message : t('chatInput.compactFailed'));
    } finally {
      setCompacting(false);
    }
  };

  return (
    <div className="flex flex-wrap items-center gap-2 px-3 pb-2.5">
      <div className="flex shrink-0 items-center gap-1.5" title={title()}>
        {compactAvailable ? (
          <Popover open={compactOpen} onOpenChange={setCompactOpen}>
            <PopoverTrigger asChild>
              <button type="button" className="flex shrink-0 cursor-pointer items-center gap-1.5 rounded-lg px-1 py-0.5 transition-colors hover:bg-hover focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-brand">
                <span className="sr-only">{t('chatInput.contextLabel')}</span>
                {meter}
                <Shrink className="h-3.5 w-3.5 shrink-0 text-fg-muted" aria-hidden="true" />
              </button>
            </PopoverTrigger>
            <PopoverContent side="top" align="start" className="w-72 p-3">
              <p className="text-ui font-medium">{t('chatInput.compact')}</p>
              <p className="mt-1 text-meta text-fg-muted">{t('chatInput.compactHint')}</p>
              {running ? <p className="mt-1.5 text-meta text-warn">{t('chatInput.compactBusy')}</p> : null}
              <Input value={compactInstructions} onChange={(event) => setCompactInstructions(event.target.value)} placeholder={t('chatInput.compactInstructionsPlaceholder')} className="mt-2 h-8 text-ui" aria-label={t('chatInput.compactInstructionsPlaceholder')} />
              <Button type="button" size="sm" className="mt-2 w-full" disabled={compacting} onClick={() => void runCompact()}>
                {compacting ? t('chatInput.compacting') : t('chatInput.compactNow')}
              </Button>
            </PopoverContent>
          </Popover>
        ) : (
          <div role="progressbar" aria-label={t('chatInput.contextLabel')} aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent} aria-valuetext={t('chatInput.contextValueText', { used: usedTokens, limit: limitTokens })} className="flex shrink-0 items-center gap-1.5">
            {meter}
          </div>
        )}
      </div>
      <WorkspaceSelector workspacePath={workspacePath} disabled={!activeSessionId || permissionLocked} onSelect={chooseWorkspace} />
      {notice ? <span className="min-w-0 max-w-full truncate text-meta text-fg-muted" aria-live="polite">{notice}</span> : null}
      <div className="ml-auto flex items-center gap-2">
        {running ? (
          <>
            <button type="button" onClick={() => void onSend('send_now')} disabled={disabled || !canSend} className="rounded-full border border-line p-2.5 text-warn transition-colors hover:bg-hover disabled:opacity-40" title={t('chatInput.sendNow')} aria-label={t('chatInput.sendNow')}><Zap className="h-4 w-4" /></button>
            <button type="button" onClick={() => void onSend('follow_up')} disabled={disabled || !canSend} className="rounded-full border border-line p-2.5 text-fg-muted transition-colors hover:bg-hover hover:text-fg disabled:opacity-40" title={t('chatInput.sendAsFollowUp')} aria-label={t('chatInput.sendAsFollowUp')}><Clock className="h-4 w-4" /></button>
            <button type="button" onClick={() => void onSend('steer')} disabled={disabled || !canSend} className="rounded-full bg-brand p-2.5 text-brand-fg transition-opacity hover:opacity-90 disabled:opacity-40" title={t('chatInput.steer')} aria-label={t('chatInput.steer')}><Send className="h-4 w-4" /></button>
            <Button size="icon" variant="destructive" className="rounded-full" onClick={totalQueued ? onClearQueue : () => void onCancel?.()} disabled={disabled} title={totalQueued ? t('chatInput.clearQueue') : t('chatInput.cancelRun')} aria-label={totalQueued ? t('chatInput.clearQueue') : t('chatInput.cancelRun')}><Square className="h-4 w-4" /></Button>
          </>
        ) : (
          <button type="button" onClick={() => void onSend('send')} disabled={disabled || !canSend} className="rounded-full bg-brand p-2.5 text-brand-fg transition-opacity hover:opacity-90 disabled:opacity-40" title={t('chatInput.send')} aria-label={t('chatInput.send')}><Send className="h-4 w-4" /></button>
        )}
      </div>
    </div>
  );
}
