import { useMemo, useState } from 'react';
import { LayoutGrid, List, LoaderCircle, MessageSquare, Plus, RefreshCw } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { useTranslation } from '@/i18n';
import { isRetiredChannel } from './channel-platforms';
import {
  channelStatusFor,
  getChannels,
  removeChannel,
  saveChannel,
  toggleChannel,
  useChannels,
  type ChannelConfig,
  type ChannelStatusSummary,
} from './channel-store';
import ChannelCardView from './ChannelCardView';
import { ChannelEditorForm } from './ChannelEditorForm';
import ChannelWizardModal from './ChannelWizardModal';
import { normalizeChannelConfig } from './channel-schema';
import { PLATFORM_DISPLAY_NAMES, PLATFORM_ICONS } from './channel-icons';

/**
 * 通道配置主视图（移植自 Agent-Diva ChannelsSettings.vue）。
 * 双模式：卡片视图（概览 + 快捷操作）与列表视图（左侧通道列表 + 右侧内联编辑）。
 * 数据层为纯前端本地存储（vivy.ui.channels），就绪状态按 schema 必填字段
 * 近似计算——后端接入后替换 channel-store 读写层即可。
 */

function cloneValue<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

export function ChannelsSettings() {
  const { t } = useTranslation();
  const channels = useChannels();

  const [viewMode, setViewMode] = useState<'card' | 'list'>('card');
  const [wizardOpen, setWizardOpen] = useState(false);
  const [editingName, setEditingName] = useState<string | null>(null);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [drafts, setDrafts] = useState<Record<string, ChannelConfig>>(() => cloneValue(getChannels()));
  const [savedSnapshots, setSavedSnapshots] = useState<Record<string, ChannelConfig>>(() => cloneValue(getChannels()));
  const [isSaving, setIsSaving] = useState(false);

  const visibleNames = useMemo(
    () => Object.keys(channels).filter((name) => !isRetiredChannel(name)).sort(),
    [channels],
  );

  const statusMap = useMemo(() => {
    const map = new Map<string, ChannelStatusSummary>();
    for (const [name, config] of Object.entries(channels)) {
      map.set(name, channelStatusFor(name, config));
    }
    return map;
  }, [channels]);

  const selected = selectedName && !isRetiredChannel(selectedName) ? selectedName : null;
  const selectedDraft = selected ? drafts[selected] ?? null : null;
  const isDirty = selected && selectedDraft
    ? JSON.stringify(selectedDraft) !== JSON.stringify(savedSnapshots[selected] ?? null)
    : false;

  /** 外部已保存状态变化时刷新草稿（回到基线）。 */
  const refreshDrafts = () => {
    const saved = cloneValue(getChannels());
    setDrafts(saved);
    setSavedSnapshots(saved);
    if (selected && !saved[selected]) {
      setSelectedName(visibleNames[0] ?? null);
    }
  };

  const persistSelected = () => {
    if (!selected || !selectedDraft || isSaving || !isDirty) return;
    setIsSaving(true);
    try {
      saveChannel(selected, cloneValue(selectedDraft));
      setSavedSnapshots((current) => ({ ...current, [selected]: cloneValue(selectedDraft) }));
    } finally {
      setIsSaving(false);
    }
  };

  const handleWizardComplete = (data: { platform: string; credentials: Record<string, unknown> }) => {
    const existing = channels[data.platform] ?? {};
    const credentials = normalizeChannelConfig(data.platform, data.credentials);
    delete credentials.enabled;
    const enabled = editingName ? Boolean(existing.enabled) : true;
    saveChannel(data.platform, {
      ...cloneValue(existing),
      ...credentials,
      enabled,
    });
    setEditingName(null);
    setSelectedName(data.platform);
    refreshDrafts();
  };

  const handleCardEdit = (name: string) => {
    setEditingName(name);
    setWizardOpen(true);
  };

  const handleDelete = (name: string) => {
    if (!window.confirm(t('channels.deleteConfirm', { name }))) return;
    removeChannel(name);
    if (selected === name) setSelectedName(null);
    refreshDrafts();
  };

  const handleToggle = (name: string) => {
    toggleChannel(name);
    refreshDrafts();
  };

  const openNewWizard = () => {
    setEditingName(null);
    setWizardOpen(true);
  };

  return (
    <div className="flex h-[min(44rem,calc(100dvh-16rem))] min-h-0 overflow-hidden rounded-lg border">
      {viewMode === 'list' ? (
        <div className="w-56 shrink-0 overflow-y-auto border-r bg-muted/30">
          {visibleNames.map((name) => {
            const config = channels[name];
            const status = statusMap.get(name);
            return (
              <button
                key={name}
                type="button"
                className={`flex w-full cursor-pointer items-center gap-2 border-b px-3 py-2.5 text-left transition-colors ${
                  selected === name ? 'bg-background' : 'hover:bg-background/60'
                }`}
                onClick={() => setSelectedName(name)}
              >
                <MessageSquare className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium capitalize">{name}</span>
                  <span className={`block text-xs ${config?.enabled ? 'text-emerald-600' : 'text-muted-foreground'}`}>
                    {config?.enabled
                      ? status?.ready
                        ? t('channels.ready')
                        : t('channels.needsSetup')
                      : t('channels.disabled')}
                  </span>
                </span>
              </button>
            );
          })}
        </div>
      ) : null}

      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center justify-between gap-2 border-b bg-muted/20 px-4 py-2">
          <div className="flex items-center gap-2">
            <Button type="button" variant="outline" size="icon" onClick={refreshDrafts} title={t('common.refresh')} aria-label={t('common.refresh')}>
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
          </div>
          <Button type="button" onClick={openNewWizard}>
            <Plus className="mr-1.5 h-4 w-4" />
            {t('channels.addChannel')}
          </Button>
        </div>

        {viewMode === 'card' ? (
          <div className="min-h-0 flex-1 overflow-y-auto">
            <ChannelCardView
              channels={channels}
              statuses={[...statusMap.values()]}
              onAdd={openNewWizard}
              onEdit={handleCardEdit}
              onDelete={handleDelete}
              onToggle={handleToggle}
            />
          </div>
        ) : (
          <div className="min-h-0 flex-1 overflow-y-auto">
            {selected && selectedDraft ? (
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
                        <Badge variant={selectedDraft.enabled ? 'default' : 'secondary'}>
                          {selectedDraft.enabled ? t('channels.enabled') : t('channels.disabled')}
                        </Badge>
                        <Switch
                          checked={Boolean(selectedDraft.enabled)}
                          onCheckedChange={(checked) => {
                            setDrafts((current) => ({
                              ...current,
                              [selected]: { ...current[selected], enabled: checked },
                            }));
                          }}
                          aria-label={selectedDraft.enabled ? t('channels.enabled') : t('channels.disabled')}
                        />
                      </div>
                    </div>
                  </div>
                  <Button
                    type="button"
                    disabled={!isDirty || isSaving}
                    onClick={persistSelected}
                  >
                    {isSaving ? <LoaderCircle className="mr-1.5 h-4 w-4 animate-spin" /> : null}
                    {isSaving ? t('channels.saving') : t('channels.saveConfig')}
                  </Button>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-lg border p-3">
                    <p className="text-xs text-muted-foreground">{t('channels.readiness')}</p>
                    <p className={`mt-1 text-sm font-semibold ${statusMap.get(selected)?.ready ? 'text-emerald-600' : 'text-amber-600'}`}>
                      {statusMap.get(selected)?.ready ? t('channels.ready') : t('channels.needsSetup')}
                    </p>
                  </div>
                  <div className="rounded-lg border p-3">
                    <p className="text-xs text-muted-foreground">{t('channels.missingFields')}</p>
                    <p className="mt-1 text-sm">
                      {statusMap.get(selected) && (statusMap.get(selected)?.missing_fields.length ?? 0) > 0
                        ? statusMap.get(selected)?.missing_fields.join(', ')
                        : t('channels.none')}
                    </p>
                  </div>
                </div>

                <div className="rounded-lg border bg-muted/20 p-4">
                  <ChannelEditorForm
                    platform={selected}
                    config={selectedDraft}
                    onFieldChange={(field, value) => {
                      setDrafts((current) => ({
                        ...current,
                        [selected]: { ...current[selected], [field.key]: value },
                      }));
                    }}
                    onExtraKeyChange={(key, value) => {
                      setDrafts((current) => ({
                        ...current,
                        [selected]: { ...current[selected], [key]: value },
                      }));
                    }}
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
      </div>

      <ChannelWizardModal
        open={wizardOpen}
        initialPlatform={editingName ?? undefined}
        initialCredentials={editingName ? cloneValue(channels[editingName] ?? {}) : undefined}
        onOpenChange={setWizardOpen}
        onComplete={handleWizardComplete}
      />
    </div>
  );
}