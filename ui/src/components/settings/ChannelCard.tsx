import type { ComponentType, SVGProps } from 'react';
import { MessageSquare, Pencil, Power, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useTranslation } from '@/i18n';
import { PLATFORM_DISPLAY_NAMES, PLATFORM_ICONS } from './channel-icons';
import type { ChannelStatus } from '../../lib/api';

/**
 * 单通道卡片：识别（名称/图标）+ 判断（进程状态徽标、跳过/失败原因）+
 * 操作（启停、编辑、关耳朵）。数据全部来自 channel/inspect（进程真值）
 * 与 channel/get（文档真值）的对比。
 */

export type ChannelCardModel = {
  status: ChannelStatus;
  /** 文档真值与进程真值出现差异：写入已保存，重启进程后生效。 */
  pendingRestart: boolean;
};

type IconComponent = ComponentType<SVGProps<SVGSVGElement>>;

function ChannelCard({
  channel,
  busy,
  onToggle,
  onEdit,
  onDisable,
}: {
  channel: ChannelCardModel;
  busy?: boolean;
  onToggle: () => void;
  onEdit: () => void;
  onDisable: () => void;
}) {
  const { t } = useTranslation();
  const { status, pendingRestart } = channel;
  const Icon: IconComponent = PLATFORM_ICONS[status.name] ?? MessageSquare;
  const displayName = PLATFORM_DISPLAY_NAMES[status.name] ?? status.name;

  const badgeLabel = status.started
    ? t('channels.activated')
    : status.enabled
      ? t('channels.needsConfig')
      : t('channels.disabled');
  const stateLine = status.started
    ? t('channels.started')
    : status.enabled
      ? t('channels.enabled')
      : t('channels.disabled');

  return (
    <div
      className={`flex flex-col gap-4 rounded-lg border bg-card p-5 transition-all hover:-translate-y-0.5 hover:shadow-md ${
        status.started ? 'border-l-4 border-l-emerald-500' : status.enabled ? 'border-l-4 border-l-amber-500' : 'opacity-60'
      }`}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex h-12 w-12 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Icon className="h-6 w-6" />
        </div>
        <div className="flex flex-col items-end gap-1">
          <Badge variant={status.started ? 'default' : 'secondary'}>{badgeLabel}</Badge>
          {pendingRestart ? <Badge variant="outline">{t('channels.pendingRestart')}</Badge> : null}
        </div>
      </div>

      <div className="space-y-1.5">
        <h3 className="text-base font-semibold">{displayName}</h3>
        <p className="text-sm text-muted-foreground">{stateLine}</p>
        {status.note ? (
          <p className="text-xs text-amber-600">{status.note}</p>
        ) : null}
      </div>

      <div className="mt-auto flex justify-end gap-1">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          disabled={busy}
          onClick={onToggle}
          title={status.enabled ? t('channels.deactivate') : t('channels.activate')}
          aria-label={status.enabled ? t('channels.deactivate') : t('channels.activate')}
        >
          <Power className="h-4 w-4" />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          disabled={busy}
          onClick={onEdit}
          title={t('channels.editConfig')}
          aria-label={t('channels.editConfig')}
        >
          <Pencil className="h-4 w-4" />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          disabled={busy || !status.enabled}
          className="text-muted-foreground hover:bg-destructive hover:text-white"
          onClick={onDisable}
          title={t('channels.disable')}
          aria-label={t('channels.disable')}
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}

export default ChannelCard;
