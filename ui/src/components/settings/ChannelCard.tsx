import type { ComponentType, SVGProps } from 'react';
import { MessageSquare, Pencil, Power, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useTranslation } from '@/i18n';
import { PLATFORM_DISPLAY_NAMES, PLATFORM_ICONS } from './channel-icons';
import type { ChannelStatusSummary } from './channel-store';

export type ChannelCardModel = {
  name: string;
  enabled: boolean;
  config?: Record<string, unknown>;
};

type IconComponent = ComponentType<SVGProps<SVGSVGElement>>;

function ChannelCard({
  channel,
  status,
  onToggle,
  onEdit,
  onDelete,
}: {
  channel: ChannelCardModel;
  status?: ChannelStatusSummary;
  onToggle: () => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const { t } = useTranslation();
  const platform = channel.name.toLowerCase();
  const Icon: IconComponent = PLATFORM_ICONS[platform] ?? MessageSquare;
  const displayName = PLATFORM_DISPLAY_NAMES[platform] ?? channel.name;
  const isReady = status?.ready ?? false;
  const isEnabled = channel.enabled;
  const missingFields = status?.missing_fields ?? [];

  return (
    <div
      className={`flex flex-col gap-4 rounded-lg border bg-card p-5 transition-all hover:-translate-y-0.5 hover:shadow-md ${
        isEnabled ? (isReady ? 'border-l-4 border-l-emerald-500' : 'border-l-4 border-l-amber-500') : 'opacity-60'
      }`}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex h-12 w-12 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Icon className="h-6 w-6" />
        </div>
        <Badge variant={isReady ? 'default' : 'secondary'}>
          {isReady ? t('channels.activated') : t('channels.needsConfig')}
        </Badge>
      </div>

      <div className="space-y-1.5">
        <h3 className="text-base font-semibold">{displayName}</h3>
        <p className="text-sm text-muted-foreground">
          {isEnabled ? t('channels.enabled') : t('channels.disabled')}
        </p>
        {!isReady && missingFields.length > 0 ? (
          <p className="text-xs text-muted-foreground">
            <span className="font-medium">{t('channels.missing')}:</span>{' '}
            <span className="text-destructive">
              {missingFields.slice(0, 2).join(', ')}
              {missingFields.length > 2 ? '...' : ''}
            </span>
          </p>
        ) : null}
      </div>

      <div className="mt-auto flex justify-end gap-1">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          onClick={onToggle}
          title={isEnabled ? t('channels.deactivate') : t('channels.activate')}
          aria-label={isEnabled ? t('channels.deactivate') : t('channels.activate')}
        >
          <Power className="h-4 w-4" />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          onClick={onEdit}
          title={t('channels.editViaWizard')}
          aria-label={t('channels.editViaWizard')}
        >
          <Pencil className="h-4 w-4" />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="text-muted-foreground hover:bg-destructive hover:text-white"
          onClick={onDelete}
          title={t('common.delete')}
          aria-label={t('common.delete')}
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}

export default ChannelCard;