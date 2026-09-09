import { BookOpen } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { isKnownChannel } from './channel-schema';
import { CHANNEL_PLATFORMS, type ChannelPlatformInfo } from './channel-platforms';
import { useTranslation } from '@/i18n';

/**
 * 通道配置教程弹窗（移植自 Agent-Diva TutorialModal.vue）。
 * Diva 侧从 public/docs/channels/*.md 拉取正文、缺失时回退占位内容；
 * 本移植直接以内置指南占位渲染（等价回退分支），避免引入不存在的外部文档。
 */

function difficultyStars(difficulty: ChannelPlatformInfo['difficulty']): string {
  return '★'.repeat(difficulty) + '☆'.repeat(3 - difficulty);
}

function placeholderContent(platform: ChannelPlatformInfo, publicIPText: string, t: ReturnType<typeof useTranslation>['t']): string {
  return t('channels.tutorialBody', {
    accessMethod: platform.accessMethod,
    publicIP: publicIPText,
    difficulty: difficultyStars(platform.difficulty),
  });
}

function ChannelTutorialModal({
  open,
  platformName,
  onOpenChange,
  onStartConfig,
}: {
  open: boolean;
  platformName: string | null;
  onOpenChange: (open: boolean) => void;
  onStartConfig: () => void;
}) {
  const { t } = useTranslation();
  const platform = platformName && isKnownChannel(platformName) ? CHANNEL_PLATFORMS[platformName] : null;
  const publicIPText = platform?.requiresPublicIP ? t('channels.tutorialPublicIPYes') : t('channels.tutorialPublicIPNo');
  const content = platform ? placeholderContent(platform, publicIPText, t) : '';

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85dvh] max-w-2xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <BookOpen className="h-5 w-5" />
            {platform?.displayName ?? ''} {t('channels.tutorialTitle')}
          </DialogTitle>
          <DialogDescription>{t('channels.tutorialSubtitle')}</DialogDescription>
        </DialogHeader>

        {platform ? (
          <div className="grid gap-4 rounded-lg border p-4 sm:grid-cols-3">
            <div>
              <p className="text-xs text-muted-foreground">{t('channels.accessMethod')}</p>
              <p className="mt-0.5 text-sm font-semibold">{platform.accessMethod}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">{t('channels.tutorialPublicIP')}</p>
              <p className="mt-0.5 text-sm font-semibold">{publicIPText}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">{t('channels.tutorialDifficulty')}</p>
              <p className="mt-0.5 text-sm font-semibold">{difficultyStars(platform.difficulty)}</p>
            </div>
          </div>
        ) : null}

        <div className="prose prose-sm max-w-none dark:prose-invert">
          <ReactMarkdown>{content}</ReactMarkdown>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.close')}
          </Button>
          <Button
            type="button"
            onClick={() => {
              onOpenChange(false);
              onStartConfig();
            }}
          >
            {t('channels.tutorialStartConfig')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default ChannelTutorialModal;
