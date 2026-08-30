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
    accessMethod: 'Long Polling',
    credentialFields: CHANNEL_CREDENTIAL_FIELDS.telegram,
    quickGuideSteps: [
      '在 Telegram 中搜索 @BotFather 并打开对话',
      '发送 /newbot 命令创建新机器人',
      '按提示设置机器人名称和用户名（以 bot 结尾）',
      '复制 BotFather 返回的 Bot Token',
    ],
  },
  discord: {
    name: 'discord',
    displayName: 'Discord',
    tutorialPath: '/docs/channels/discord.md',
    difficulty: 2,
    requiresPublicIP: false,
    accessMethod: 'WebSocket Gateway',
    credentialFields: CHANNEL_CREDENTIAL_FIELDS.discord,
    quickGuideSteps: [
      '访问 Discord Developer Portal (https://discord.com/developers)',
      '创建新应用并进入 Bot 页面',
      '点击 "Reset Token" 生成机器人 Token',
      '复制并保存 Token（只显示一次）',
    ],
  },
  feishu: {
    name: 'feishu',
    displayName: '飞书',
    tutorialPath: '/docs/channels/feishu.md',
    difficulty: 2,
    requiresPublicIP: false,
    accessMethod: 'WebSocket 长连接',
    credentialFields: CHANNEL_CREDENTIAL_FIELDS.feishu,
    quickGuideSteps: [
      '登录飞书开放平台 (https://open.feishu.cn)',
      '创建企业自建应用',
      '在"凭证与基础信息"获取 App ID 和 App Secret',
      '添加机器人能力并开通权限',
      '选择"使用长连接接收事件"并添加 im.message.receive_v1',
    ],
  },
  dingtalk: {
    name: 'dingtalk',
    displayName: '钉钉',
    tutorialPath: '/docs/channels/dingtalk.md',
    difficulty: 2,
    requiresPublicIP: false,
    accessMethod: 'Stream 模式 (WebSocket)',
    credentialFields: CHANNEL_CREDENTIAL_FIELDS.dingtalk,
    quickGuideSteps: [
      '登录钉钉开放平台 (https://open-dev.dingtalk.com)',
      '创建企业内部应用',
      '在"应用凭证"获取 AppKey 和 AppSecret',
      '开启机器人功能并选择"Stream 模式"',
      '发布应用并设置可见范围',
    ],
  },
  qq: {
    name: 'qq',
    displayName: 'QQ',
    tutorialPath: '/docs/channels/qq.md',
    difficulty: 2,
    requiresPublicIP: false,
    accessMethod: 'QQ 开放平台 API',
    credentialFields: CHANNEL_CREDENTIAL_FIELDS.qq,
    quickGuideSteps: [
      '访问 QQ 开放平台 (https://q.qq.com)',
      '创建机器人应用',
      '在开发设置获取 AppID 和 AppSecret',
      '配置功能权限和沙箱环境',
    ],
  },
};
