import { t } from '@/i18n';

// 通道平台信息定义（移植自 Agent-Diva agent-diva-gui
// src/components/settings/channel-platforms.ts）。
// 只保留本代可编译进内核的平台（以 plugin.Name() 为键）；向导的
// 可选集合不再来自这份固定表，而是 channel/inspect 返回的编译内
// 通道中"尚未配置"的子集——本表只提供展示元数据（图标、指引、难度）。

import { CHANNEL_CREDENTIAL_FIELDS, type WizardFormField } from './channel-schema';

export type { WizardFormField };

export interface ChannelPlatformInfo {
  name: string;
  displayName: string;
  /** 教程文档路径（Diva 侧为 public/docs/channels/*.md；本移植以内置指南占位代替） */
  tutorialPath: string;
  /** 配置难度 1-3 星 */
  difficulty: 1 | 2 | 3;
  requiresPublicIP: boolean;
  /** 接入方式 */
  accessMethod: string;
  credentialFields: WizardFormField[];
  /** 快速获取凭证的步骤数组 */
  quickGuideSteps: string[];
}

/** 各平台展示元数据（与凭据 schema 一一对应；键 = plugin.Name()）。 */
export const CHANNEL_PLATFORMS: Record<string, ChannelPlatformInfo> = {
  telegram: {
    name: 'telegram',
    displayName: 'Telegram',
    tutorialPath: '/docs/channels/telegram.md',
    difficulty: 1,
    requiresPublicIP: false,
    get accessMethod() { return t('channelGuide.telegram.accessMethod'); },
    credentialFields: CHANNEL_CREDENTIAL_FIELDS.telegram,
    get quickGuideSteps() { return [t('channelGuide.telegram.steps.0'), t('channelGuide.telegram.steps.1'), t('channelGuide.telegram.steps.2'), t('channelGuide.telegram.steps.3')]; },
  },
  discord: {
    name: 'discord',
    displayName: 'Discord',
    tutorialPath: '/docs/channels/discord.md',
    difficulty: 2,
    requiresPublicIP: false,
    get accessMethod() { return t('channelGuide.discord.accessMethod'); },
    credentialFields: CHANNEL_CREDENTIAL_FIELDS.discord,
    get quickGuideSteps() { return [t('channelGuide.discord.steps.0'), t('channelGuide.discord.steps.1'), t('channelGuide.discord.steps.2'), t('channelGuide.discord.steps.3')]; },
  },
  feishu: {
    name: 'feishu',
    displayName: '飞书',
    tutorialPath: '/docs/channels/feishu.md',
    difficulty: 2,
    requiresPublicIP: false,
    get accessMethod() { return t('channelGuide.feishu.accessMethod'); },
    credentialFields: CHANNEL_CREDENTIAL_FIELDS.feishu,
    get quickGuideSteps() { return [t('channelGuide.feishu.steps.0'), t('channelGuide.feishu.steps.1'), t('channelGuide.feishu.steps.2'), t('channelGuide.feishu.steps.3'), t('channelGuide.feishu.steps.4')]; },
  },
  dingtalk: {
    name: 'dingtalk',
    displayName: '钉钉',
    tutorialPath: '/docs/channels/dingtalk.md',
    difficulty: 2,
    requiresPublicIP: false,
    get accessMethod() { return t('channelGuide.dingtalk.accessMethod'); },
    credentialFields: CHANNEL_CREDENTIAL_FIELDS.dingtalk,
    get quickGuideSteps() { return [t('channelGuide.dingtalk.steps.0'), t('channelGuide.dingtalk.steps.1'), t('channelGuide.dingtalk.steps.2'), t('channelGuide.dingtalk.steps.3'), t('channelGuide.dingtalk.steps.4')]; },
  },
  qq: {
    name: 'qq',
    displayName: 'QQ',
    tutorialPath: '/docs/channels/qq.md',
    difficulty: 2,
    requiresPublicIP: false,
    get accessMethod() { return t('channelGuide.qq.accessMethod'); },
    credentialFields: CHANNEL_CREDENTIAL_FIELDS.qq,
    get quickGuideSteps() { return [t('channelGuide.qq.steps.0'), t('channelGuide.qq.steps.1'), t('channelGuide.qq.steps.2'), t('channelGuide.qq.steps.3')]; },
  },
};
