import { useEffect, useId, useState } from 'react';
import type { UITranslator } from '@vivy/ui-sdk';
import { Download, Loader2, ScanSearch } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import type { CatalogIdentity, ToolkitPreview } from './toolkit-client';
import { COMPONENT_KINDS, componentCount, componentKindLabel, sourceKindLabel, type SourceKind } from './toolkit-view';
import type { ToolkitManifest } from './toolkit-client';

/**
 * The single install surface.
 *
 * It owns only the source form and the preview it validated; the page owns the
 * actions. Nothing here is shared with the update/rebind fields of a selected
 * Toolkit, so typing a path to install can never retarget an update.
 */
export function ToolkitInstallDialog({
  open,
  catalog,
  mutationsLocked,
  onClose,
  onPreview,
  onInstall,
  t,
}: {
  readonly open: boolean;
  readonly catalog: CatalogIdentity | null;
  readonly mutationsLocked: boolean;
  readonly onClose: () => void;
  readonly onPreview: (input: { readonly kind: SourceKind; readonly location: string; readonly ref: string }) => Promise<ToolkitPreview | null>;
  readonly onInstall: (preview: ToolkitPreview) => Promise<'ok' | 'failed' | 'unknown'>;
  readonly t: UITranslator;
}) {
  const locationId = useId();
  const refId = useId();
  const kindLabelId = useId();
  const [kind, setKind] = useState<SourceKind>('local');
  const [location, setLocation] = useState('');
  const [ref, setRef] = useState('main');
  const [preview, setPreview] = useState<ToolkitPreview | null>(null);
  const [busy, setBusy] = useState<'preview' | 'install' | null>(null);
  const [error, setError] = useState('');

  // A preview is only valid for the catalog it was validated against and the
  // source on screen. Closing the dialog discards it instead of letting a later
  // open install a stale release.
  useEffect(() => {
    if (!open) {
      setPreview(null);
      setError('');
      setBusy(null);
    }
  }, [open]);

  const discardPreview = () => {
    setPreview(null);
    setError('');
  };

  const startPreview = async () => {
    if (!location.trim() || busy) return;
    setBusy('preview');
    setError('');
    const result = await onPreview({ kind, location: location.trim(), ref: ref.trim() });
    if (result) setPreview(result);
    else {
      setPreview(null);
      // The page refuses to preview while another change waits, so name the real
      // reason instead of blaming the source.
      setError(
        mutationsLocked
          ? t('plugin.vivy/toolkits-ui.candidateMutationLock')
          : t('plugin.vivy/toolkits-ui.previewFailed'),
      );
    }
    setBusy(null);
  };

  const startInstall = async () => {
    if (!preview || busy) return;
    setBusy('install');
    setError('');
    const outcome = await onInstall(preview);
    setBusy(null);
    if (outcome === 'ok') return;
    if (outcome === 'failed') setError(t('plugin.vivy/toolkits-ui.installFailed'));
    // `unknown` closes the dialog: the page shows the operation receipt and the
    // user must not be told the install failed or succeeded.
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && busy) return;
        if (!next) onClose();
      }}
    >
      <DialogContent data-testid="toolkit-install-dialog" className="max-w-xl">
        <DialogHeader>
          <DialogTitle>{t('plugin.vivy/toolkits-ui.installTitle')}</DialogTitle>
          <DialogDescription>{t('plugin.vivy/toolkits-ui.installDescription')}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <div className="space-y-2">
            <span id={kindLabelId} className="text-sm font-medium">
              {t('plugin.vivy/toolkits-ui.sourceKind')}
            </span>
            <ToggleGroup
              type="single"
              value={kind}
              aria-labelledby={kindLabelId}
              className="justify-start"
              onValueChange={(value) => {
                if (value !== 'local' && value !== 'git') return;
                setKind(value);
                discardPreview();
              }}
            >
              <ToggleGroupItem value="local" aria-label={t('plugin.vivy/toolkits-ui.localSource')}>
                {t('plugin.vivy/toolkits-ui.localSource')}
              </ToggleGroupItem>
              <ToggleGroupItem value="git" aria-label={t('plugin.vivy/toolkits-ui.gitSource')}>
                {t('plugin.vivy/toolkits-ui.gitSource')}
              </ToggleGroupItem>
            </ToggleGroup>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor={locationId}>
              {kind === 'local'
                ? t('plugin.vivy/toolkits-ui.localPath')
                : t('plugin.vivy/toolkits-ui.gitAddress')}
            </Label>
            <Input
              id={locationId}
              value={location}
              autoComplete="off"
              spellCheck={false}
              onChange={(event) => {
                setLocation(event.target.value);
                discardPreview();
              }}
            />
          </div>

          {kind === 'git' ? (
            <div className="space-y-1.5">
              <Label htmlFor={refId}>{t('plugin.vivy/toolkits-ui.gitRef')}</Label>
              <Input
                id={refId}
                value={ref}
                autoComplete="off"
                spellCheck={false}
                onChange={(event) => {
                  setRef(event.target.value);
                  discardPreview();
                }}
              />
            </div>
          ) : null}

          <p className="text-xs text-muted-foreground">{t('plugin.vivy/toolkits-ui.hostPathHint')}</p>

          {error ? (
            <p role="alert" className="rounded-lg border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
              {error}
            </p>
          ) : null}

          {preview ? (
            <div className="space-y-2 rounded-xl border bg-muted/40 p-3" data-testid="toolkit-preview">
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <h3 className="min-w-0 break-words text-sm font-semibold">
                  {preview.manifest.name} · {preview.manifest.version}
                </h3>
                <Badge variant="outline">{sourceKindLabel(t, preview.source.kind)}</Badge>
              </div>
              <p className="break-all text-xs text-muted-foreground">{preview.source.origin}</p>
              <p className="break-all text-xs">
                {t('plugin.vivy/toolkits-ui.releaseDigest')} <span className="font-mono">{preview.release_digest}</span>
              </p>
              <ComponentCounts manifest={preview.manifest} t={t} />
            </div>
          ) : null}

          <Button
            type="button"
            variant="outline"
            data-action="preview-source"
            disabled={busy !== null || !location.trim() || !catalog}
            onClick={() => {
              void startPreview();
            }}
          >
            {busy === 'preview' ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden /> : <ScanSearch className="h-4 w-4" aria-hidden />}
            {t('plugin.vivy/toolkits-ui.previewSource')}
          </Button>
        </DialogBody>
        <DialogFooter>
          <Button type="button" variant="outline" data-action="install-cancel" disabled={busy !== null} onClick={onClose}>
            {t('plugin.vivy/toolkits-ui.installCancel')}
          </Button>
          <Button
            type="button"
            data-action="install-disabled"
            disabled={!preview || busy !== null || mutationsLocked}
            onClick={() => {
              void startInstall();
            }}
          >
            {busy === 'install' ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden /> : <Download className="h-4 w-4" aria-hidden />}
            {t('plugin.vivy/toolkits-ui.installDisabled')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** Counts of what the previewed release provides. */
function ComponentCounts({
  manifest,
  t,
}: {
  readonly manifest: ToolkitManifest;
  readonly t: UITranslator;
}) {
  return (
    <ul className="flex flex-wrap gap-1.5">
      {COMPONENT_KINDS.map((kind) => (
        <li key={kind}>
          <Badge variant="secondary" className="text-[10px] font-normal">
            {componentKindLabel(t, kind)} · {componentCount(manifest, kind)}
          </Badge>
        </li>
      ))}
    </ul>
  );
}
