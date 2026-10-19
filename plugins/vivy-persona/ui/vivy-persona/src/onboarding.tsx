import { useCallback, useState } from 'react';
import { AlertTriangle, ArrowLeft, ArrowRight, Check, Loader2, SkipForward } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { CompanionAvatarPicker } from '@/components/layout/CompanionAvatar';
import { usePluginTranslation } from '@vivy/ui-sdk';
import { cn } from '@/lib/utils';
import { KIND_LABELS } from './kinds';
import type { PersonaClient, PersonaInitialization } from './persona-client';

/**
 * 人格首次引导：两个挂载点（首抵门禁与 /persona 页面）共用这一份实现，
 * 因此编辑、校验、提交只有一处。写入仍由后端权威完成：这里只负责把用户
 * 一步一步问出来，并在失败时保留全部已填内容。
 */
const DOCUMENT_STEPS = ['identity', 'relationship', 'user', 'world', 'redline'] as const;
type DocumentStep = (typeof DOCUMENT_STEPS)[number];

const STEP_ORDER = ['philosophy', 'face', ...DOCUMENT_STEPS, 'done'] as const;
type StepId = (typeof STEP_ORDER)[number];

const EMPTY_ANSWERS: Record<DocumentStep, string> = {
  identity: '',
  relationship: '',
  user: '',
  world: '',
  redline: '',
};

export interface PersonaOnboardingProps {
  readonly client: PersonaClient;
  readonly sessionId: string;
  /** Label of the closing step's primary action (each mount names its own). */
  readonly finishLabelKey: string;
  readonly onFinish: () => void;
  readonly secondaryLabelKey?: string;
  readonly onSecondary?: () => void;
  readonly onSkip?: () => void;
  readonly className?: string;
}

export function PersonaOnboarding({
  client,
  sessionId,
  finishLabelKey,
  onFinish,
  secondaryLabelKey,
  onSecondary,
  onSkip,
  className,
}: PersonaOnboardingProps) {
  const { t } = usePluginTranslation();
  const [step, setStep] = useState<StepId>('philosophy');
  const [answers, setAnswers] = useState<Record<DocumentStep, string>>(EMPTY_ANSWERS);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const stepIndex = STEP_ORDER.indexOf(step);
  const documentStep = DOCUMENT_STEPS.find((kind) => kind === step);
  const isLastDocumentStep = documentStep === DOCUMENT_STEPS[DOCUMENT_STEPS.length - 1];

  const goTo = useCallback((next: StepId) => {
    setError(null);
    setStep(next);
  }, []);

  const goBack = useCallback(() => {
    setError(null);
    setStep((current) => STEP_ORDER[Math.max(0, STEP_ORDER.indexOf(current) - 1)]);
  }, []);

  const submit = useCallback(async () => {
    if (submitting) return;
    const empty = DOCUMENT_STEPS.find((kind) => !answers[kind].trim());
    if (empty) {
      setError(t('plugin.vivy/persona.onboarding.required'));
      setStep(empty);
      return;
    }
    const initialization: PersonaInitialization = {
      identity: answers.identity,
      relationship: answers.relationship,
      redline: answers.redline,
      user: answers.user,
      world: answers.world,
    };
    setSubmitting(true);
    setError(null);
    try {
      const outcome = await client.initialize(sessionId, initialization, 'Initialize persona from first-run onboarding');
      if (outcome.status === 'ok' || outcome.error?.code === 'persona_already_initialized') {
        setStep('done');
        return;
      }
      const message = outcome.error?.message || outcome.error?.code || t('plugin.vivy/persona.errors.initializeFailed');
      setError(message);
      // The authority names the offending document, so a rejected write sends
      // the user back to the page that needs fixing instead of a bare banner.
      const rejected = DOCUMENT_STEPS.find((kind) => message.includes(KIND_LABELS[kind]));
      if (rejected) setStep(rejected);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setSubmitting(false);
    }
  }, [answers, client, sessionId, submitting, t]);

  const stepLabelKey = (id: StepId) => `plugin.vivy/persona.onboarding.step.${id}.label`;

  return (
    <Card className={cn('flex min-h-0 flex-col overflow-hidden', className)}>
      <div className="relative shrink-0 overflow-hidden border-b bg-muted/30 px-5 py-4 sm:px-7">
        <div aria-hidden="true" className="pointer-events-none absolute inset-0 opacity-70">
          <span className="welcome-float absolute -left-4 -top-6 h-20 w-20 rounded-full bg-primary/20 blur-2xl" />
          <span className="welcome-float absolute right-6 -top-8 h-24 w-24 rounded-full bg-sky-400/20 blur-2xl" style={{ animationDelay: '-2s' }} />
        </div>
        <div className="relative flex flex-wrap items-center gap-x-3 gap-y-2">
          <p className="text-xs font-medium text-muted-foreground">
            {t('plugin.vivy/persona.onboarding.progress', { current: stepIndex + 1, total: STEP_ORDER.length })}
          </p>
          <div className="flex flex-wrap gap-1">
            {STEP_ORDER.map((id, index) => {
              const filled = DOCUMENT_STEPS.includes(id as DocumentStep) && answers[id as DocumentStep].trim().length > 0;
              return (
                <button
                  key={id}
                  type="button"
                  disabled={id === 'done' || step === 'done'}
                  aria-current={id === step ? 'step' : undefined}
                  onClick={() => goTo(id)}
                  className={cn(
                    'flex cursor-pointer items-center gap-1 rounded-full px-2.5 py-0.5 text-[11px] transition-colors duration-150 motion-reduce:transition-none',
                    id === step ? 'bg-primary/15 text-foreground ring-1 ring-primary/40' : 'text-muted-foreground hover:bg-accent',
                    id === 'done' ? 'cursor-default' : '',
                  )}
                >
                  {filled && id !== step ? <Check className="h-3 w-3" aria-hidden="true" /> : <span className="tabular-nums opacity-60">{index + 1}</span>}
                  {t(stepLabelKey(id))}
                </button>
              );
            })}
          </div>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-6 sm:px-7">
        <div key={step} className="animate-in fade-in-0 slide-in-from-right-2 duration-200 motion-reduce:animate-none">
          {step === 'philosophy' ? (
            <div className="mx-auto max-w-xl space-y-4 text-center">
              <h2 className="animate-in fade-in-0 duration-300 fill-mode-backwards delay-100 text-lg font-semibold">
                {t('plugin.vivy/persona.onboarding.philosophy.title')}
              </h2>
              <p className="animate-in fade-in-0 duration-300 fill-mode-backwards delay-200 text-sm leading-relaxed text-muted-foreground">
                {t('plugin.vivy/persona.onboarding.philosophy.line1')}
              </p>
              <p className="animate-in fade-in-0 duration-300 fill-mode-backwards delay-300 text-sm leading-relaxed">
                {t('plugin.vivy/persona.onboarding.philosophy.line2')}
              </p>
              <p className="animate-in fade-in-0 duration-300 fill-mode-backwards delay-500 text-sm leading-relaxed text-muted-foreground">
                {t('plugin.vivy/persona.onboarding.philosophy.line3')}
              </p>
              <p className="text-xs text-muted-foreground">{t('plugin.vivy/persona.onboarding.philosophy.aside')}</p>
            </div>
          ) : step === 'face' ? (
            <div className="mx-auto max-w-xl space-y-4">
              <div className="space-y-1.5 text-center">
                <h2 className="text-lg font-semibold">{t('plugin.vivy/persona.onboarding.face.title')}</h2>
                <p className="text-sm leading-relaxed text-muted-foreground">{t('plugin.vivy/persona.onboarding.face.hint')}</p>
              </div>
              <div className="rounded-xl border bg-muted/20 p-4">
                <CompanionAvatarPicker />
              </div>
            </div>
          ) : documentStep ? (
            <div className="mx-auto max-w-xl space-y-3">
              <h2 className="text-lg font-semibold">{t(`plugin.vivy/persona.onboarding.kind.${documentStep}.title`)}</h2>
              <p className="text-sm leading-relaxed text-muted-foreground">{t(`plugin.vivy/persona.onboarding.kind.${documentStep}.hint`)}</p>
              <textarea
                name={documentStep}
                autoFocus
                value={answers[documentStep]}
                disabled={submitting}
                onChange={(event) => setAnswers((current) => ({ ...current, [documentStep]: event.target.value }))}
                placeholder={t(`plugin.vivy/persona.onboarding.kind.${documentStep}.example`)}
                aria-label={KIND_LABELS[documentStep]}
                className="min-h-40 w-full resize-y rounded-xl border bg-background p-4 text-sm leading-relaxed outline-none ring-offset-background placeholder:text-muted-foreground/70 focus-visible:ring-2 focus-visible:ring-ring"
              />
              <p className="text-xs text-muted-foreground">{KIND_LABELS[documentStep]}</p>
            </div>
          ) : (
            <div className="mx-auto max-w-xl space-y-4 text-center">
              <h2 className="text-lg font-semibold">{t('plugin.vivy/persona.onboarding.done.title')}</h2>
              <p className="text-sm leading-relaxed">{t('plugin.vivy/persona.onboarding.done.line1')}</p>
              <p className="text-sm leading-relaxed text-muted-foreground">{t('plugin.vivy/persona.onboarding.done.line2')}</p>
            </div>
          )}
        </div>
      </div>

      <div className="shrink-0 border-t bg-muted/20 px-5 py-4 sm:px-7">
        {error ? (
          <div className="mb-3 flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive" role="alert">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <span>{error}</span>
          </div>
        ) : null}
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            {step !== 'philosophy' && step !== 'done' ? (
              <Button type="button" variant="secondary" size="sm" className="cursor-pointer" disabled={submitting} onClick={goBack}>
                <ArrowLeft className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
                {t('plugin.vivy/persona.onboarding.back')}
              </Button>
            ) : null}
            {onSkip && step !== 'done' ? (
              <Button type="button" variant="ghost" size="sm" className="cursor-pointer text-muted-foreground" disabled={submitting} onClick={onSkip}>
                <SkipForward className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
                {t('plugin.vivy/persona.onboarding.skip')}
              </Button>
            ) : null}
          </div>
          <div className="flex items-center gap-2">
            {step === 'done' ? (
              <>
                {secondaryLabelKey && onSecondary ? (
                  <Button type="button" variant="secondary" size="sm" className="cursor-pointer" onClick={onSecondary}>
                    {t(secondaryLabelKey)}
                  </Button>
                ) : null}
                <Button type="button" size="sm" className="cursor-pointer" onClick={onFinish}>
                  {t(finishLabelKey)}
                </Button>
              </>
            ) : (
              <Button
                type="button"
                size="sm"
                className="cursor-pointer"
                disabled={submitting}
                onClick={() => {
                  if (isLastDocumentStep) void submit();
                  else goTo(STEP_ORDER[Math.min(STEP_ORDER.length - 1, stepIndex + 1)]);
                }}
              >
                {submitting ? <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" aria-hidden="true" /> : null}
                {submitting
                  ? t('plugin.vivy/persona.onboarding.submitting')
                  : isLastDocumentStep
                    ? t('plugin.vivy/persona.onboarding.commit')
                    : t('plugin.vivy/persona.onboarding.next')}
                {!submitting && !isLastDocumentStep ? <ArrowRight className="ml-1 h-3.5 w-3.5" aria-hidden="true" /> : null}
              </Button>
            )}
          </div>
        </div>
        {onSkip && step !== 'done' ? (
          <p className="mt-2 text-xs text-muted-foreground">{t('plugin.vivy/persona.onboarding.dismissHint')}</p>
        ) : null}
      </div>
    </Card>
  );
}
