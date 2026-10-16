import type { ReactNode } from 'react';
import { AlertTriangle, ExternalLink, Eye } from 'lucide-react';
import type { UITranslator } from '@vivy/ui-sdk';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import type { ToolkitInspection, ToolkitManifest } from './toolkit-client';
import { ToolkitExecutableRow } from './ToolkitExecutableRow';
import {
  COMPONENT_KINDS,
  componentCount,
  componentKindLabel,
  emptyKindLabel,
  ownerKey,
  type ComponentKind,
  type PendingAction,
} from './toolkit-view';

/**
 * What a Toolkit provides, grouped in the five declared component kinds.
 *
 * The summary strip only summarizes and navigates; each section keeps the full
 * heading + count shape the rest of the surface and the tests rely on.
 */
export function ToolkitInventory({
  manifest,
  inspection,
  owners,
  pending,
  mutationsLocked,
  onRead,
  onConsent,
  onSaveBindings,
  t,
}: {
  readonly manifest: ToolkitManifest;
  readonly inspection: ToolkitInspection;
  readonly owners: Readonly<Record<string, boolean>>;
  readonly pending: PendingAction | null;
  readonly mutationsLocked: boolean;
  readonly onRead: (kind: string, id: string, trigger: HTMLButtonElement) => void;
  readonly onConsent: (kind: 'hook' | 'mcp', allow: boolean, componentID: string) => void;
  readonly onSaveBindings: (
    kind: 'hook' | 'mcp',
    componentID: string,
    bindings: readonly { readonly name: string; readonly reference: string }[],
  ) => void;
  readonly t: UITranslator;
}) {
  const enabled = inspection.installation.state === 'enabled';
  const available = (kind: ComponentKind) => owners[ownerKey(kind)] !== false;
  return (
    <Card className="min-w-0">
      <CardHeader className="gap-3 space-y-0 p-4 sm:p-5">
        <CardTitle className="text-base">{t('plugin.vivy/toolkits-ui.inventoryTitle')}</CardTitle>
        <ul className="flex flex-wrap gap-1.5" data-testid="toolkit-summary">
          {COMPONENT_KINDS.map((kind) => (
            <li key={kind}>
              <a
                href={`#toolkit-kind-${kind}`}
                className="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs transition-colors hover:bg-accent/40"
              >
                <span>{componentKindLabel(t, kind)}</span>
                <span className="font-mono text-[10px] text-muted-foreground">{componentCount(manifest, kind)}</span>
                {!available(kind) ? (
                  <AlertTriangle className="h-3 w-3 text-amber-600 dark:text-amber-400" aria-hidden />
                ) : null}
              </a>
            </li>
          ))}
        </ul>
      </CardHeader>
      <CardContent className="space-y-4 p-4 pt-0 sm:p-5 sm:pt-0">
        <Section kind="skills" count={manifest.components.skills.length} available={available('skills')} t={t}>
          <OwnerLink href="/skills" label={t('plugin.vivy/toolkits-ui.openSkills')} />
          {manifest.components.skills.map((entry) => (
            <ComponentRow
              key={entry.id}
              kind="skill"
              id={entry.id}
              onRead={(trigger) => onRead('skill', entry.id, trigger)}
              t={t}
            />
          ))}
        </Section>

        <Section kind="masks" count={manifest.components.masks.length} available={available('masks')} t={t}>
          <OwnerLink href="/masks" label={t('plugin.vivy/toolkits-ui.openMasks')} />
          {manifest.components.masks.map((entry) => (
            <div key={entry.id} className="space-y-1">
              <ComponentRow
                kind="mask"
                id={entry.id}
                label={entry.name}
                onRead={(trigger) => onRead('mask', entry.id, trigger)}
                t={t}
              />
              {entry.description ? (
                <p className="pl-1 text-xs text-muted-foreground">{entry.description}</p>
              ) : null}
              {entry.recommendedSkills.length > 0 ? (
                <p className="pl-1 text-xs text-muted-foreground">
                  {t('plugin.vivy/toolkits-ui.recommendedSkillsLine', { skills: entry.recommendedSkills.join(', ') })}
                </p>
              ) : null}
            </div>
          ))}
        </Section>

        <Section kind="rules" count={manifest.components.rules.length} available={available('rules')} t={t}>
          {manifest.components.rules.map((entry) => (
            <div key={entry.id} className="space-y-1">
              <ComponentRow
                kind="rule"
                id={entry.id}
                onRead={(trigger) => onRead('rule', entry.id, trigger)}
                t={t}
              />
              {entry.faces.length > 0 ? (
                <p className="pl-1 text-xs text-muted-foreground">
                  {t('plugin.vivy/toolkits-ui.ruleFacesLine', { faces: entry.faces.join(', ') })}
                </p>
              ) : null}
            </div>
          ))}
        </Section>

        <Section kind="hooks" count={manifest.components.hooks.length} available={available('hooks')} t={t}>
          {manifest.components.hooks.map((entry) => (
            <ToolkitExecutableRow
              key={entry.id}
              kind="hook"
              id={entry.id}
              detail={entry.event}
              slots={Object.keys(entry.envFrom ?? {}).map((target) => `envFrom:${target}`)}
              inspection={inspection}
              ownerAvailable={available('hooks')}
              enabled={enabled}
              pending={pending}
              mutationsLocked={mutationsLocked}
              onConsent={(allow) => onConsent('hook', allow, entry.id)}
              onSaveBindings={(bindings) => onSaveBindings('hook', entry.id, bindings)}
              t={t}
            />
          ))}
        </Section>

        <Section kind="mcp" count={manifest.components.mcp.length} available={available('mcp')} t={t}>
          {manifest.components.mcp.map((entry) => (
            <ToolkitExecutableRow
              key={entry.id}
              kind="mcp"
              id={entry.id}
              detail={entry.endpoint || entry.command || ''}
              slots={[
                ...(entry.authEnv ? ['authEnv'] : []),
                ...Object.keys(entry.envFrom ?? {}).map((target) => `envFrom:${target}`),
              ]}
              inspection={inspection}
              ownerAvailable={available('mcp')}
              enabled={enabled}
              pending={pending}
              mutationsLocked={mutationsLocked}
              onConsent={(allow) => onConsent('mcp', allow, entry.id)}
              onSaveBindings={(bindings) => onSaveBindings('mcp', entry.id, bindings)}
              t={t}
            />
          ))}
        </Section>
      </CardContent>
    </Card>
  );
}

function Section({
  kind,
  count,
  available,
  children,
  t,
}: {
  readonly kind: ComponentKind;
  readonly count: number;
  readonly available: boolean;
  readonly children: ReactNode;
  readonly t: UITranslator;
}) {
  return (
    <section
      id={`toolkit-kind-${kind}`}
      data-testid={`toolkit-kind-${kind}`}
      className="scroll-mt-4 space-y-2"
      aria-label={componentKindLabel(t, kind)}
    >
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-sm font-semibold">{componentKindLabel(t, kind)}</h3>
        <span className="text-xs text-muted-foreground">
          {available ? t('plugin.vivy/toolkits-ui.componentCountLine', { count }) : t('plugin.vivy/toolkits-ui.ownerUnavailable')}
        </span>
      </div>
      {!available ? (
        <p className="text-sm text-muted-foreground">{t('plugin.vivy/toolkits-ui.stagedComponent')}</p>
      ) : count === 0 ? (
        <p className="text-sm text-muted-foreground">{emptyKindLabel(t, kind)}</p>
      ) : (
        <div className="space-y-2">{children}</div>
      )}
    </section>
  );
}

function OwnerLink({ href, label }: { readonly href: string; readonly label: string }) {
  return (
    <p className="text-xs">
      <a className="inline-flex items-center gap-1 text-primary underline-offset-4 hover:underline" href={href}>
        {label}
        <ExternalLink className="h-3 w-3" aria-hidden />
      </a>
    </p>
  );
}

function ComponentRow({
  kind,
  id,
  label,
  onRead,
  t,
}: {
  readonly kind: string;
  readonly id: string;
  readonly label?: string;
  readonly onRead: (trigger: HTMLButtonElement) => void;
  readonly t: UITranslator;
}) {
  return (
    <div className="flex min-w-0 items-center justify-between gap-2 rounded-xl border bg-muted/30 px-3 py-2">
      <span className="min-w-0 truncate text-sm">
        {label ?? id} <span className="font-mono text-xs text-muted-foreground">{kind}/{id}</span>
      </span>
      <Button
        type="button"
        size="sm"
        variant="outline"
        className="shrink-0"
        data-action="read-content"
        data-component={`${kind}/${id}`}
        onClick={(event) => onRead(event.currentTarget)}
      >
        <Eye className="h-3.5 w-3.5" aria-hidden />
        {t('plugin.vivy/toolkits-ui.readContent')}
      </Button>
    </div>
  );
}
