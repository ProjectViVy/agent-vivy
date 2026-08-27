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

function placeholderContent(platform: ChannelPlatformInfo, publicIPText: string): string {
  return [
    '## 平台概览',
    '',
    `- **接入方式**: ${platform.accessMethod}`,
    `- **需要公网 IP**: ${publicIPText}`,
    `- **配置难度**: ${difficultyStars(platform.difficulty)}`,
    '',
    '## 前置条件',
    '',
    '- 准备相关平台的开发者账号',
    '- 确保网络环境正常',
    '',
    '## 平台端申请步骤',
    '',
    '请参考相关平台的官方文档完成应用创建和凭证获取。',
    '',
    '## Agent Vivy 配置',
    '',
    '1. 在上方表单中填写凭证信息',
    '2. 点击「下一步」进入完成页',
    '3. 保存后即可启用该通道',
    '',
    '## 验证与测试',
    '',
    '- 启用通道后观察运行日志输出',
    '- 发送测试消息验证连接',
    '',
    '## 常见问题',
    '',
    '如有问题，请查看项目文档或提交 Issue。',
  ].join('\n');
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
  const content = platform ? placeholderContent(platform, publicIPText) : '';

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