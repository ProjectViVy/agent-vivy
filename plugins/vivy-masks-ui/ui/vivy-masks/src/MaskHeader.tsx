import type { ChatHeaderContext, UITranslator } from '@vivy/ui-sdk';
import { usePluginHost, usePluginTranslation } from '@vivy/ui-sdk';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { MaskSelector } from './MaskSelector';
import { MaskClient, type MaskMetadata, type MaskSelection } from './mask-client';

/**
 * Small session-bound contribution for the host-owned chat.header slot.
 * Catalog and selection are read from the mask action owner; the host context
 * only supplies the admitted session identity and running state.
 */
export function MaskHeader({ context }: { readonly context: ChatHeaderContext }) {
  const host = usePluginHost();
  const { t } = usePluginTranslation();
  const client = useMemo(() => (host ? MaskClient.fromRPC(host.rpc) : null), [host]);
  const [catalog, setCatalog] = useState<readonly MaskMetadata[]>([]);
  const [selection, setSelection] = useState<MaskSelection | null>(null);
  const [loading, setLoading] = useState(false);
  const [epoch, setEpoch] = useState(0);

  useEffect(() => {
    if (!client) return;
    let cancelled = false;
    void client.list({ limit: 100 }).then((page) => {
      if (!cancelled) setCatalog(page.items);
    }).catch(() => {
      if (!cancelled) setCatalog([]);
    });
    return () => { cancelled = true; };
  }, [client]);

  useEffect(() => {
    const sessionId = context.sessionId;
    const requestEpoch = epoch + 1;
    setEpoch(requestEpoch);
    setSelection(null);
    if (!client || !sessionId) return;
    let cancelled = false;
    setLoading(true);
    void client.getSelection(sessionId).then((next) => {
      if (!cancelled) setSelection(next);
    }).catch(() => {
      if (!cancelled) setSelection(null);
    }).finally(() => {
      if (!cancelled) setLoading(false);
    });
    return () => { cancelled = true; };
  }, [client, context.sessionId]);

  const selectMask = useCallback(async (maskId: string) => {
    if (!client || !context.sessionId || !selection) return;
    const requestEpoch = epoch;
    setLoading(true);
    try {
      const next = await client.setSelection({
        session_id: context.sessionId,
        mask_id: maskId,
        expected_revision: selection.revision,
      });
      if (requestEpoch === epoch) setSelection(next);
    } finally {
      if (requestEpoch === epoch) setLoading(false);
    }
  }, [client, context.sessionId, epoch, selection]);

  if (!host) return null;
  return (
    <div className="min-w-52 max-w-sm">
      <MaskSelector
        catalog={catalog}
        selectedMaskId={selection?.mask_id ?? ''}
        activeSessionId={context.sessionId}
        disabled={!selection?.available}
        running={context.running}
        loading={loading}
        onSelect={(maskId) => void selectMask(maskId)}
        t={t as UITranslator}
      />
    </div>
  );
}
