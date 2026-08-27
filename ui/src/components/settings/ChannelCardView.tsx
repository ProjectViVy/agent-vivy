import { MessageSquarePlus, Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useTranslation } from '@/i18n';
import { isRetiredChannel } from './channel-platforms';
import type { ChannelStatusSummary, StoredChannels } from './channel-store';
import ChannelCard, { type ChannelCardModel } from './ChannelCard';

/**
 * 卡片视图（移植自 Agent-Diva ChannelCardView.vue）。
 * 空态引导「添加通道」；网格展示每个激活通道的就绪/缺失摘要。
 */

function ChannelCardView({
  channels,
  statuses,
  onAdd,
  onEdit,
  onDelete,
  onToggle,
}: {
  channels: StoredChannels;
  statuses: ChannelStatusSummary[];
  onAdd: () => void;
  onEdit: (name: string) => void;
  onDelete: (name: string) => void;
  onToggle: (name: string) => void;
}) {
  const { t } = useTranslation();
  const statusMap = new Map(statuses.map((s) => [s.name, s]));

  const channelList: Array<{ name: string; channel: ChannelCardModel; status?: ChannelStatusSummary }> =
    Object.entries(channels)
      .filter(([name]) => !isRetiredChannel(name))
      .map(([name, raw]) => ({
        name,
        channel: { name, enabled: Boolean(raw?.enabled), config: raw } satisfies ChannelCardModel,
        status: statusMap.get(name),
      }));

  if (channelList.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center px-8 py-16 text-center">
        <MessageSquarePlus className="mb-6 h-20 w-20 text-muted-foreground/30" />
        <h3 className="mb-1 text-xl font-semibold">{t('channels.noChannels')}</h3>
        <p className="mb-6 text-sm text-muted-foreground">{t('channels.noChannelsHint')}</p>
        <Button type="button" onClick={onAdd}>
          <Plus className="mr-1.5 h-4 w-4" />
          {t('channels.addChannel')}
        </Button>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(17.5rem,1fr))] gap-4 p-4 sm:p-6">
      {channelList.map(({ name, channel, status }) => (
        <ChannelCard
          key={name}
          channel={channel}
          status={status}
          onToggle={() => onToggle(name)}
          onEdit={() => onEdit(name)}
          onDelete={() => onDelete(name)}
        />
      ))}
    </div>
  );
}

export default ChannelCardView;