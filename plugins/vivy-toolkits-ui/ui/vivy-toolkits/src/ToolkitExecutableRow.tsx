import { useEffect, useState } from 'react';
import type { UITranslator } from '@vivy/ui-sdk';
import { Loader2, ShieldCheck } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import type { ToolkitInspection } from './toolkit-client';
import { isPending, mcpRuntimeLabel, type PendingAction } from './toolkit-view';
import { Input } from '@/components/ui/input';

/**
 * One hook or MCP declaration, its runtime facts, and the two things the user
 * controls: host environment bindings (names only) and executable consent
 * (bound to the inspected declaration digest).
 */
export function ToolkitExecutableRow({
  kind,
  id,
  detail,
  slots,
  inspection,
  ownerAvailable,
  enabled,
  pending,
  mutationsLocked,
  onConsent,
  onSaveBindings,
  t,
}: {
  readonly kind: 'hook' | 'mcp';
  readonly id: string;
  readonly detail: string;
  readonly slots: readonly string[];
  readonly inspection: ToolkitInspection;
  readonly ownerAvailable: boolean;
  readonly enabled: boolean;
  readonly pending: PendingAction | null;
  readonly mutationsLocked: boolean;
  readonly onConsent: (allow: boolean) => void;
  readonly onSaveBindings: (bindings: readonly { readonly name: string; readonly reference: string }[]) => void;
  readonly t: UITranslator;
}) {
  const executable = inspection.executables.find((entry) => entry.kind === kind && entry.local_id === id);
  const consent = inspection.consents.find((entry) => entry.kind === kind && entry.local_id === id);
  const recorded = Boolean(consent?.allowed ?? executable?.consent_allowed);
  const current = Boolean(executable?.consent_current && executable.consent_allowed);
  const componentBindings = inspection.bindings.filter((binding) => binding.kind === kind && binding.local_id === id);
  const consentBusy = isPending(pending, 'consent', `${kind}/${id}`);
  const bindingsBusy = isPending(pending, 'bindings', `${kind}/${id}`);
  const consentDisabled = !ownerAvailable || !enabled || mutationsLocked || consentBusy;

  return (
    <div
      data-testid="toolkit-executable"
      data-component={`${kind}/${id}`}
      className="space-y-3 rounded-xl border bg-muted/30 p-3"
    >
      <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <p className="break-all font-mono text-sm">{id}</p>
          {detail ? <p className="break-all text-xs text-muted-foreground">{detail}</p> : null}
        </div>
        <Badge
          variant={current ? 'default' : recorded ? 'destructive' : 'outline'}
          className="shrink-0 gap-1 text-[10px] font-normal"
        >
          <ShieldCheck className="h-3 w-3" aria-hidden />
          {current
            ? t('plugin.vivy/toolkits-ui.consentCurrent')
            : recorded
              ? t('plugin.vivy/toolkits-ui.consentStale')
              : t('plugin.vivy/toolkits-ui.consentMissing')}
        </Badge>
      </div>

      {executable?.missing_bindings.length ? (
        <p className="text-xs text-amber-700 dark:text-amber-300">
          {t('plugin.vivy/toolkits-ui.missingBindings')}: {executable.missing_bindings.join(', ')}
        </p>
      ) : null}

      {kind === 'mcp' ? (
        <>
          <p className="text-xs text-muted-foreground">
            {t('plugin.vivy/toolkits-ui.mcpRuntime')}: {mcpRuntimeLabel(t, executable?.runtime_state ?? 'inactive')}
            {typeof executable?.runtime_tool_count === 'number'
              ? ` · ${t('plugin.vivy/toolkits-ui.mcpToolsLine', { count: executable.runtime_tool_count })}`
              : ''}
          </p>
          <p className="text-xs text-muted-foreground">{t('plugin.vivy/toolkits-ui.mcpHealthHint')}</p>
        </>
      ) : null}

      {slots.length > 0 ? (
        <BindingsEditor
          kind={kind}
          id={id}
          slots={slots}
          existing={componentBindings}
          disabled={!ownerAvailable || mutationsLocked || bindingsBusy}
          saving={bindingsBusy}
          onSave={onSaveBindings}
          t={t}
        />
      ) : null}

      <div className="flex flex-wrap gap-2">
        <Button
          type="button"
          size="sm"
          variant={current ? 'outline' : 'default'}
          data-action={current ? 'consent-revoke' : 'consent-grant'}
          data-consent={current ? 'current' : recorded ? 'stale' : 'missing'}
          disabled={consentDisabled}
          onClick={() => onConsent(!current)}
        >
          {consentBusy ? <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden /> : null}
          {current
            ? t('plugin.vivy/toolkits-ui.revokeConsent')
            : recorded
              ? t('plugin.vivy/toolkits-ui.reauthorizeConsent')
              : t('plugin.vivy/toolkits-ui.confirmConsent')}
        </Button>
        {recorded && !current ? (
          <Button
            type="button"
            size="sm"
            variant="outline"
            data-action="consent-revoke"
            data-consent="stale"
            disabled={consentDisabled}
            onClick={() => onConsent(false)}
          >
            {t('plugin.vivy/toolkits-ui.revokeStaleConsent')}
          </Button>
        ) : null}
      </div>

      {!enabled ? (
        <p className="text-xs text-muted-foreground">{t('plugin.vivy/toolkits-ui.enableBeforeConsent')}</p>
      ) : null}
    </div>
  );
}

/**
 * Host environment bindings: variable names, never values.
 *
 * New inspection data refreshes only the slots the user has not edited, so a
 * conflicting save never silently replaces what was typed.
 */
function BindingsEditor({
  kind,
  id,
  slots,
  existing,
  disabled,
  saving,
  onSave,
  t,
}: {
  readonly kind: 'hook' | 'mcp';
  readonly id: string;
  readonly slots: readonly string[];
  readonly existing: ToolkitInspection['bindings'];
  readonly disabled: boolean;
  readonly saving: boolean;
  readonly onSave: (bindings: readonly { readonly name: string; readonly reference: string }[]) => void;
  readonly t: UITranslator;
}) {
  const [values, setValues] = useState<Record<string, string>>(() => initialValues(slots, existing));
  const [touched, setTouched] = useState<Readonly<Record<string, boolean>>>({});

  useEffect(() => {
    const refs = new Map(existing.map((binding) => [binding.name, binding.reference]));
    setValues((current) => {
      let changed = false;
      const next = { ...current };
      for (const slot of slots) {
        if (touched[slot]) continue;
        const value = refs.get(slot) ?? '';
        if (next[slot] !== value) {
          next[slot] = value;
          changed = true;
        }
      }
      return changed ? next : current;
    });
  }, [existing, slots, touched]);

  return (
    <form
      className="space-y-2 rounded-xl border bg-background p-3"
      aria-label={`${t('plugin.vivy/toolkits-ui.bindings')} · ${kind}/${id}`}
      onSubmit={(event) => {
        event.preventDefault();
        onSave(
          slots
            .map((name) => ({ name, reference: (values[name] ?? '').trim() }))
            .filter((binding) => binding.reference !== ''),
        );
      }}
    >
      <div>
        <h4 className="text-sm font-medium">{t('plugin.vivy/toolkits-ui.bindings')}</h4>
        <p className="text-xs text-muted-foreground">{t('plugin.vivy/toolkits-ui.bindingHint')}</p>
      </div>
      {slots.map((slot) => {
        const inputID = `binding-${kind}-${id}-${slot.replace(/[^a-zA-Z0-9_-]/g, '-')}`;
        return (
          <div key={slot} className="space-y-1">
            <label htmlFor={inputID} className="block text-xs font-medium">
              {slot}
            </label>
            <Input
              id={inputID}
              className="h-9 font-mono text-xs"
              autoComplete="off"
              spellCheck={false}
              pattern="[A-Za-z_][A-Za-z0-9_]*"
              placeholder="HOST_ENV_NAME"
              value={values[slot] ?? ''}
              onChange={(event) => {
                const value = event.target.value;
                setValues((current) => ({ ...current, [slot]: value }));
                setTouched((current) => ({ ...current, [slot]: true }));
              }}
            />
          </div>
        );
      })}
      <Button type="submit" size="sm" variant="outline" data-action="save-bindings" disabled={disabled}>
        {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden /> : null}
        {t('plugin.vivy/toolkits-ui.saveBindings')}
      </Button>
    </form>
  );
}

function initialValues(
  slots: readonly string[],
  existing: ToolkitInspection['bindings'],
): Record<string, string> {
  const refs = new Map(existing.map((binding) => [binding.name, binding.reference]));
  return Object.fromEntries(slots.map((slot) => [slot, refs.get(slot) ?? '']));
}
