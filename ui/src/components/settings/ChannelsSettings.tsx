import { useMemo, useState } from 'react';
import { LayoutGrid, List, LoaderCircle, MessageSquare, Plus, RefreshCw } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { useTranslation } from '@/i18n';
import { disableChannel, channelPendingRestart, refreshChannels, saveChannel, toggleChannel, useChannelsState } from './channel-store';
import { joinIdList, splitIdList } from './channel-schema';
import type { ChannelEnvelope } from '../../lib/api';
import ChannelCardView from './ChannelCardView';
import { ChannelEditorForm } from './ChannelEditorForm';
import ChannelWizardModal, { type ChannelWizardData } from './ChannelWizardModal';
import { PLATFORM_DISPLAY_NAMES, PLATFORM_ICONS } from './channel-icons';

/**
 * 通道配置主视图。列表 = channel/inspect 结果（编译内通道，唯一来源）；
 * 编辑 = channel/get 文档真值上的草稿，保存走 channel/update（settings
 * overlay），重启进程后生效（卡片上的"待重启"徽标来自文档/进程真值差）。
 * 这一代没有编译进任何通道时显示空态，不提供添加入口。
 */

type ChannelDraft = {
  enabled: boolean;
  allowFromText: string;
};

export function ChannelsSettings() {
  const { t } = useTranslation();
  const { statuses, envelopes, loaded, error } = useChannelsState();

  const [viewMode, setViewMode] = useState<'card' | 'list'>('card');
  const [wizardOpen, setWizardOpen] = useState(false);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [drafts, setDrafts] = useState<Record<string, ChannelDraft>>({});
  const [busyName, setBusyName] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const statusByName = useMemo(() => new Map(statuses.map((s) => [s.name, s])), [statuses]);
  const addablePlatforms = useMemo(
    () => statuses.filter((s) => !s.configured).map((s) => s.name),
    [statuses],
  );

  const selected = selectedName && statusByName.has(selectedName) ? selectedName : null;
  const selectedStatus = selected ? statusByName.get(selected) : undefined;
  const selectedEnvelope: ChannelEnvelope | undefined = selected ? envelopes[selected] : undefined;
  const selectedBaseline: ChannelDraft = {
    enabled: selectedEnvelope?.enabled ?? false,
    allowFromText: joinIdList(selectedEnvelope?.allow_from ?? []),
  };

  /** 草稿懒派生：未编辑过的通道直接以文档真值为草稿（信封异步到达也不会产生假 dirty）。 */
  const draftFor = (name: string): ChannelDraft =>
    drafts[name] ?? {
      enabled: envelopes[name]?.enabled ?? false,
      allowFromText: joinIdList(envelopes[name]?.allow_from ?? []),
    };
  const selectedDraft = selected ? draftFor(selected) : null;
  const isDirty = Boolean(
    selected &&
      selectedDraft &&
      (selectedDraft.enabled !== selectedBaseline.enabled ||
        selectedDraft.allowFromText !== selectedBaseline.allowFromText),
  );

  const setDraft = (name: string, patch: Partial<ChannelDraft>) => {
    setDrafts((current) => ({ ...current, [name]: { ...draftFor(name), ...patch } }));
  };

  const runAction = async (name: string, action: () => Promise<unknown>) => {
    if (busyName) return;
    setBusyName(name);
    setActionError(null);
    try {
      await action();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusyName(null);
    }
  };

  const persistSelected = () => {
    if (!selected || !selectedDraft || !isDirty) return;
    const patch = {
      enabled: selectedDraft.enabled,
      allow_from: splitIdList(selectedDraft.allowFromText),
    };
    void runAction(selected, async () => {
      const envelope = await saveChannel(selected, patch);
      setDrafts((current) => ({
        ...current,
        [selected]: { enabled: envelope.enabled, allowFromText: joinIdList(envelope.allow_from) },
      }));
    });
  };

  const handleToggle = (name: string, enabled: boolean) => {
    void runAction(name, async () => {
      const envelope = await toggleChannel(name, enabled);
      setDrafts((current) => ({
        ...current,
        [name]: { enabled: envelope.enabled, allowFromText: joinIdList(envelope.allow_from) },
      }));
    });
  };

  const handleDisable = (name: string) => {
    if (!window.confirm(t('channels.deleteConfirm', { name }))) return;
    void runAction(name, () => disableChannel(name));
  };

  const handleWizardComplete = ({ platform, allowFromText }: ChannelWizardData) => {
    void runAction(platform, async () => {
      const envelope = await saveChannel(platform, {
        enabled: true,
        allow_from: splitIdList(allowFromText),
      });
      setDrafts((current) => ({
        ...current,
        [platform]: { enabled: envelope.enabled, allowFromText: joinIdList(envelope.allow_from) },
      }));
      setSelectedName(platform);
    });
  };

  /** 卡片"编辑" = 切到列表视图并选中该通道（编辑器在列表视图）。 */
  const handleCardEdit = (name: string) => {
    setSelectedName(name);
    setViewMode('list');
  };

  const pendingRestart = (name: string): boolean =>
    channelPendingRestart(statusByName.get(name), envelopes[name]);

  const emptyGeneration = loaded && !error && statuses.length === 0;

  return (
    <div className="flex h-[min(44rem,calc(100dvh-16rem))] min-h-0 overflow-hidden rounded-lg border">
      {viewMode === 'list' && !emptyGeneration ? (
        <div className="w-56 shrink-0 overflow-y-auto border-r bg-muted/30">
          {statuses.map((status) => (
            <button
              key={status.name}
              type="button"
              className={`flex w-full cursor-pointer items-center gap-2 border-b px-3 py-2.5 text-left transition-colors ${
                selected === status.name ? 'bg-background' : 'hover:bg-background/60'
              }`}
              onClick={() => setSelectedName(status.name)}
            >
              <MessageSquare className="h-4 w-4 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium capitalize">{status.name}</span>
                <span
                  className={`block text-xs ${
                    status.started ? 'text-emerald-600' : 'text-muted-foreground'
                  }`}
                >
                  {status.started
                    ? t('channels.started')
                    : status.enabled
                      ? t('channels.enabled')
                      : t('channels.disabled')}
                  {pendingRestart(status.name) ? ` · ${t('channels.pendingRestart')}` : ''}
                </span>
              </span>
            </button>
          ))}
        </div>
      ) : null}

      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center justify-between gap-2 border-b bg-muted/20 px-4 py-2">
          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="icon"
              disabled={busyName !== null}
              onClick={() => void refreshChannels()}
              title={t('common.refresh')}
              aria-label={t('common.refresh')}
            >
              <RefreshCw className="h-4 w-4" />
            </Button>
            <div className="h-6 w-px bg-border" />
            <Button
              type="button"
              variant={viewMode === 'card' ? 'secondary' : 'ghost'}
              size="icon"
              onClick={() => setViewMode('card')}
              title={t('channels.cardView')}
              aria-label={t('channels.cardView')}
            >
              <LayoutGrid className="h-4 w-4" />
            </Button>
            <Button
              type="button"
              variant={viewMode === 'list' ? 'secondary' : 'ghost'}
              size="icon"
              onClick={() => setViewMode('list')}
              title={t('channels.listView')}
              aria-label={t('channels.listView')}
            >
              <List className="h-4 w-4" />
            </Button>
            {error ? <span className="text-xs text-destructive">{error}</span> : null}
          </div>
          {addablePlatforms.length > 0 ? (
            <Button type="button" onClick={() => setWizardOpen(true)}>
              <Plus className="mr-1.5 h-4 w-4" />
              {t('channels.addChannel')}
            </Button>
          ) : null}
        </div>

        {!loaded && !error ? (
          <div className="flex h-full items-center justify-center text-muted-foreground">
            <LoaderCircle className="mr-2 h-5 w-5 animate-spin" />
            <span className="text-sm">{t('channels.loading')}</span>
          </div>
        ) : emptyGeneration ? (
          <div className="min-h-0 flex-1 overflow-y-auto">
            <ChannelCardView
              statuses={statuses}
              envelopes={envelopes}
              busyName={busyName}
              onEdit={handleCardEdit}
              onToggle={handleToggle}
              onDisable={handleDisable}
            />
          </div>
        ) : (
          <>
            {actionError ? (
              <div className="border-b bg-destructive/10 px-4 py-2 text-xs text-destructive">
                {actionError}
              </div>
            ) : null}
            {viewMode === 'card' ? (
              <div className="min-h-0 flex-1 overflow-y-auto">
                <ChannelCardView
                  statuses={statuses}
                  envelopes={envelopes}
                  busyName={busyName}
                  onEdit={handleCardEdit}
                  onToggle={handleToggle}
                  onDisable={handleDisable}
                />
              </div>
            ) : (
              <div className="min-h-0 flex-1 overflow-y-auto">
                {selected && selectedStatus && selectedDraft ? (
                  <div className="space-y-4 p-4">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="flex min-w-0 items-center gap-3">
                        {(() => {
                          const Icon = PLATFORM_ICONS[selected] ?? MessageSquare;
                          return (
                            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                              <Icon className="h-5 w-5" />
                            </div>
                          );
                        })()}
                        <div className="min-w-0">
                          <h3 className="truncate text-base font-semibold capitalize">
                            {PLATFORM_DISPLAY_NAMES[selected] ?? selected}
                          </h3>
                          <div className="mt-0.5 flex flex-wrap items-center gap-2 text-sm">
                            <span className="text-muted-foreground">{t('channels.status')}:</span>
                            <Badge variant={selectedStatus.started ? 'default' : 'secondary'}>
                              {selectedStatus.started
                                ? t('channels.started')
                                : selectedStatus.enabled
                                  ? t('channels.enabled')
                                  : t('channels.disabled')}
                            </Badge>
                            {pendingRestart(selected) ? (
                              <Badge variant="outline">{t('channels.pendingRestart')}</Badge>
                            ) : null}
                            <Switch
                              checked={selectedDraft.enabled}
                              disabled={busyName !== null}
                              onCheckedChange={(checked) => setDraft(selected, { enabled: checked })}
                              aria-label={
                                selectedDraft.enabled ? t('channels.enabled') : t('channels.disabled')
                              }
                            />
                          </div>
                          {selectedStatus.note ? (
                            <p className="mt-1 text-xs text-amber-600">{selectedStatus.note}</p>
                          ) : null}
                        </div>
                      </div>
                      <Button
                        type="button"
                        disabled={!isDirty || busyName !== null}
                        onClick={persistSelected}
                      >
                        {busyName === selected ? (
                          <LoaderCircle className="mr-1.5 h-4 w-4 animate-spin" />
                        ) : null}
                        {busyName === selected ? t('channels.saving') : t('channels.saveConfig')}
                      </Button>
                    </div>

                    <div className="rounded-lg border bg-muted/20 p-4">
                      <ChannelEditorForm
                        platform={selected}
                        enabled={selectedDraft.enabled}
                        allowFromText={selectedDraft.allowFromText}
                        tokenEnv={selectedEnvelope?.token_env}
                        tokenEnvSet={selectedStatus.token_env_set}
                        onEnabledChange={(enabled) => setDraft(selected, { enabled })}
                        onAllowFromTextChange={(text) => setDraft(selected, { allowFromText: text })}
                      />
                    </div>
                  </div>
                ) : (
                  <div className="flex h-full flex-col items-center justify-center gap-2 text-muted-foreground">
                    <MessageSquare className="h-12 w-12 opacity-20" />
                    <p className="text-sm">{t('channels.selectChannel')}</p>
                  </div>
                )}
              </div>
            )}
          </>
        )}
      </div>

      <ChannelWizardModal
        open={wizardOpen}
        addablePlatforms={addablePlatforms}
        onOpenChange={setWizardOpen}
        onComplete={handleWizardComplete}
      />
    </div>
  );
}
