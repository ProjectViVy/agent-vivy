import { MessageSquarePlus } from 'lucide-react';
import { useTranslation } from '@/i18n';
import { channelPendingRestart } from './channel-store';
import type { ChannelEnvelope, ChannelStatus } from '../../lib/api';
import ChannelCard, { type ChannelCardModel } from './ChannelCard';

/**
 * 卡片视图：每个编译进当前代的通道一张卡（channel/inspect 是唯一来源）。
 * 空态 = 这一代没有编译进任何通道（没有可配置的耳朵），不提供添加入口。
 */

function ChannelCardView({
  statuses,
  envelopes,
  busyName,
  onEdit,
  onToggle,
  onDisable,
}: {
  statuses: ChannelStatus[];
  envelopes: Record<string, ChannelEnvelope>;
  busyName: string | null;
  onEdit: (name: string) => void;
  onToggle: (name: string, enabled: boolean) => void;
  onDisable: (name: string) => void;
}) {
  const { t } = useTranslation();

  if (statuses.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center px-8 py-16 text-center">
        <MessageSquarePlus className="mb-6 h-20 w-20 text-muted-foreground/30" />
        <h3 className="mb-1 text-xl font-semibold">{t('channels.noChannels')}</h3>
        <p className="mb-6 max-w-md text-sm text-muted-foreground">{t('channels.noChannelsHint')}</p>
      </div>
    );
  }

  const cards: ChannelCardModel[] = statuses.map((status) => ({
    status,
    pendingRestart: channelPendingRestart(status, envelopes[status.name]),
  }));

  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(17.5rem,1fr))] gap-4 p-4 sm:p-6">
      {cards.map((channel) => (
        <ChannelCard
          key={channel.status.name}
          channel={channel}
          busy={busyName === channel.status.name}
          onToggle={() => onToggle(channel.status.name, !channel.status.enabled)}
          onEdit={() => onEdit(channel.status.name)}
          onDisable={() => onDisable(channel.status.name)}
        />
      ))}
    </div>
  );
}

export default ChannelCardView;
