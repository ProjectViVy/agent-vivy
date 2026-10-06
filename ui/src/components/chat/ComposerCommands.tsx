import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react';
import { ListTodo, Settings2, Sparkles, X } from 'lucide-react';
import * as api from '@/lib/api';
import { reconcileRun, useVivyStore } from '@/lib/store';
import { useTranslation } from '@/i18n';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

type CommandName = 'plan' | 'goal' | 'skill';
type CommandDraft = {
  name: CommandName;
  skill?: api.SkillSummary;
  submissionId?: string;
  goalRef?: Pick<api.WorkGoal, 'id' | 'revision'>;
};
const COMMANDS = [
  { name: 'plan', icon: Settings2 },
  { name: 'goal', icon: ListTodo },
  { name: 'skill', icon: Sparkles },
] as const;
const unfinished = (goal?: api.WorkGoal) => goal && goal.phase !== 'completed' && goal.phase !== 'blocked';

/** Unsent command intent only. Effective modes always come from WorkState. */
export function useComposerCommands(value: string, setValue: (value: string) => void) {
  const { t } = useTranslation();
  const sessionId = useVivyStore((state) => state.activeSessionId);
  const work = useVivyStore((state) => state.work);
  const [draft, setDraft] = useState<CommandDraft | null>(null);
  const [rounds, setRounds] = useState('3');
  const [dismissed, setDismissed] = useState(false);
  const [active, setActive] = useState(0);
  const [skills, setSkills] = useState<api.SkillSummary[]>([]);
  const [skillsPhase, setSkillsPhase] = useState<'loading' | 'ready' | 'error'>('loading');
  const [error, setError] = useState<string | null>(null);
  const mounted = useRef(true);
  const scope = useRef(sessionId);
  const menuId = useId();
  const discoveringSkills = draft?.name === 'skill' && !draft.skill;
  const query = /^\/[a-z]*$/.test(value) && !draft ? value.slice(1) : null;
  const options = discoveringSkills
    ? skills.filter((skill) => skill.name.toLowerCase().includes(value.toLowerCase())).map((skill) => ({ id: skill.name, label: skill.name, description: skill.description, icon: Sparkles, skill }))
    : COMMANDS.filter((command) => command.name.startsWith(query ?? '')).map((command) => ({ id: command.name, label: `/${command.name}`, description: t(`composerCommands.${command.name}Description`), icon: command.icon, skill: undefined }));
  const menuOpen = !dismissed && (discoveringSkills || (query !== null && options.length > 0));
  const index = Math.min(active, Math.max(0, options.length - 1));

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);
  useEffect(() => {
    scope.current = sessionId;
    setDraft(null);
    setRounds('3');
    setError(null);
    setDismissed(false);
  }, [sessionId]);
  useEffect(() => {
    if (!discoveringSkills) return;
    let current = true;
    setSkills([]);
    setSkillsPhase('loading');
    api.listSkills().then(({ skills }) => {
      if (!current) return;
      setSkills(skills.filter((skill) => skill.enabled));
      setSkillsPhase('ready');
    }).catch((reason: unknown) => {
      if (!current) return;
      setSkillsPhase('error');
      setError(reason instanceof Error ? reason.message : String(reason));
    });
    return () => { current = false; };
  }, [discoveringSkills, sessionId]);

  const selectCommand = (name: CommandName, args = '') => {
    const current = useVivyStore.getState().work;
    setDraft({ name,
      submissionId: name === 'goal' && current?.plan.review_status === 'pending' ? current.plan.submission_id : undefined,
      goalRef: name === 'goal' && unfinished(current?.goal) ? { id: current!.goal!.id, revision: current!.goal!.revision } : undefined,
    });
    setValue(args);
    setActive(0);
    setError(null);
    setDismissed(false);
  };
  const changeValue = (next: string) => {
    setError(null);
    setActive(0);
    setDismissed(false);
    const command = !draft && /^\/(plan|goal|skill)[ \t]+([\s\S]*)$/.exec(next);
    if (command) selectCommand(command[1] as CommandName, command[2]);
    else setValue(next);
  };
  const choose = (position: number) => {
    const option = options[position];
    if (!option) return;
    if (option.skill) {
      setDraft({ name: 'skill', skill: option.skill });
      setValue('');
      setError(null);
    } else selectCommand(option.id as CommandName);
  };
  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.nativeEvent.isComposing || event.keyCode === 229) return true;
    if (!menuOpen) return false;
    if (event.key === 'Escape') { event.preventDefault(); setDismissed(true); return true; }
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      setActive((index + (event.key === 'ArrowDown' ? 1 : options.length - 1)) % Math.max(1, options.length));
      return true;
    }
    if ((event.key === 'Enter' && !event.shiftKey) || event.key === 'Tab') {
      event.preventDefault(); choose(index); return true;
    }
    return false;
  };
  const clear = () => { setDraft(null); setRounds('3'); setError(null); setDismissed(true); };
  const isCurrent = () => mounted.current && scope.current === sessionId && useVivyStore.getState().activeSessionId === sessionId;

  const prepare = async (text: string, hasContext: boolean): Promise<{ text: string; forceSend?: boolean } | null> => {
    const state = useVivyStore.getState();
    if (!draft) return { text };
    if (!isCurrent()) return null;
    if (draft.name === 'skill') {
      if (!draft.skill) throw new Error(t('composerCommands.selectSkill'));
      const { skills } = await api.listSkills();
      if (!isCurrent()) return null;
      if (!skills.some((skill) => skill.name === draft.skill!.name && skill.enabled)) throw new Error(t('composerCommands.skillUnavailable'));
      return { text: `Use the skill ${JSON.stringify(draft.skill.name)} for this request.${text ? `\n\n${text}` : ''}` };
    }
    if (!state.work || state.work.session_id !== sessionId) throw new Error(t('workControl.unavailable'));
    if (draft.name === 'plan') {
      if (!text && hasContext) throw new Error(t('composerCommands.planNeedsText'));
      if (text === 'off') {
        if (hasContext) throw new Error(t('composerCommands.commandContext'));
        await state.leavePlan();
        return null;
      }
      if (!state.work.plan.active) {
        await state.enterPlan();
        if (!isCurrent()) return null;
        const run = state.currentRun;
        if (run && !['completed', 'failed', 'cancelled'].includes(run.status) && useVivyStore.getState().currentRun?.id === run.id) {
          await reconcileRun(run.id, sessionId!);
        }
      }
      return isCurrent() && text ? { text, forceSend: true } : null;
    }
    if (hasContext) throw new Error(t('composerCommands.commandContext'));
    if (state.work.goal && !unfinished(state.work.goal)) throw new Error(t('composerCommands.clearGoalFirst'));
    if (draft.goalRef) {
      if (text) throw new Error(t('composerCommands.resumeOnly'));
      if (state.work.goal?.id !== draft.goalRef.id || state.work.goal.revision !== draft.goalRef.revision) throw new Error(t('composerCommands.goalChanged'));
      await state.commitWork('goal/resume', { goal_id: draft.goalRef.id, goal_revision: draft.goalRef.revision });
      return null;
    }
    const maxRounds = Number(rounds);
    if (!text || !Number.isInteger(maxRounds) || maxRounds < 1 || maxRounds > 1000) throw new Error(t('composerCommands.goalRequired'));
    if (unfinished(state.work.goal)) throw new Error(t('composerCommands.goalChanged'));
    if (draft.submissionId) {
      if (state.work.plan.submission_id !== draft.submissionId || state.work.plan.review_status !== 'pending') throw new Error(t('workControl.reviewStale'));
      await state.decidePlan('start_goal', '', text, maxRounds, draft.submissionId);
    } else {
      if (state.work.plan.active) throw new Error(t('composerCommands.goalDuringPlan'));
      await state.createGoal(text, maxRounds);
    }
    return null;
  };

  return { draft, rounds, setRounds, error, setError, menuOpen, menuId, options, index, choose, clear, changeValue, handleKeyDown, prepare, isCurrent,
    dismiss: () => setDismissed(true), skillsPhase, work,
  };
}

export function ComposerCommandControls({ commands, disabled, focus }: {
  commands: ReturnType<typeof useComposerCommands>;
  disabled: boolean;
  focus: () => void;
}) {
  const { t } = useTranslation();
  const { draft, work } = commands;
  const busy = useVivyStore((state) => state.workBusy);
  const leavePlan = useVivyStore((state) => state.leavePlan);
  return <>
    {commands.menuOpen ? <div className="absolute bottom-full left-0 right-0 z-30 mb-2 rounded-xl border bg-popover p-1 shadow-lg">
      <div id={commands.menuId} role="listbox" aria-label={t(draft?.name === 'skill' ? 'composerCommands.skills' : 'composerCommands.commands')} className="max-h-56 overflow-y-auto">
        {commands.options.map((option, index) => <button key={option.id} id={`${commands.menuId}-${index}`} type="button" role="option" aria-selected={commands.index === index} disabled={disabled} onMouseDown={(event) => event.preventDefault()} onClick={() => { commands.choose(index); focus(); }} className={cn('flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left text-sm', commands.index === index ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/60')}>
          <option.icon className="h-4 w-4 shrink-0 text-muted-foreground" />
          <span className="min-w-0"><span className="block font-medium">{option.label}</span><span className="block truncate text-xs text-muted-foreground">{option.description}</span></span>
        </button>)}
        {!commands.options.length ? <p className="px-3 py-3 text-xs text-muted-foreground">{t(commands.skillsPhase === 'loading' ? 'composerCommands.loadingSkills' : commands.skillsPhase === 'error' ? 'composerCommands.skillsFailed' : 'composerCommands.noSkills')}</p> : null}
      </div>
    </div> : null}
    {draft || work?.plan.active ? <div className="flex flex-wrap items-center gap-2 px-4 pt-2">
      {draft ? <span data-command-tag className="inline-flex items-center gap-1 rounded-md border border-primary/25 bg-primary/10 px-2 py-1 text-xs text-primary">
        /{draft.name}{draft.skill ? ` · ${draft.skill.name}` : ''}
        <button type="button" aria-label={t('composerCommands.remove')} disabled={disabled} onClick={() => { commands.clear(); focus(); }} className="rounded p-0.5 hover:bg-primary/10"><X className="h-3 w-3" /></button>
      </span> : null}
      {draft?.name === 'goal' && !draft.goalRef ? <label className="flex items-center gap-2 text-xs text-muted-foreground">{t('workControl.maxRounds')}<Input type="number" min={1} max={1000} aria-label={t('workControl.maxRounds')} value={commands.rounds} onChange={(event) => commands.setRounds(event.target.value)} disabled={disabled} className="h-7 w-20" /></label> : null}
      {draft?.name === 'plan' && disabled && !work?.plan.active ? <span role="status" className="text-xs text-muted-foreground">{t('workControl.stoppingForPlan')}</span> : null}
      {work?.plan.active && draft?.name !== 'plan' ? <span data-plan-mode className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground"><Settings2 className="h-3 w-3" />{t('chatInput.planMode')}<button type="button" aria-label={t('workControl.leavePlan')} disabled={disabled || busy} onClick={() => void leavePlan().catch(() => {})} className="rounded p-0.5 hover:bg-accent"><X className="h-3 w-3" /></button></span> : null}
    </div> : null}
    {commands.error ? <p role="alert" className="px-4 pt-2 text-xs text-destructive">{commands.error}</p> : null}
  </>;
}
