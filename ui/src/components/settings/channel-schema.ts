import { t } from '@/i18n';

// 通道配置表单字段 schema（移植自 Agent-Diva agent-diva-gui
// src/components/settings/channel-wizard-fields.ts，纯 TS，无 UI 依赖）。
// 只保留本代可编译进内核的通道（telegram/discord/feishu/dingtalk/qq），
// 以 plugin.Name() 为键；email / neuro-link 已下架（不可编译进本代）。
// 每通道凭据字段是后端通道插件的元数据（C6/C7 复用）；kernel envelope
// 只携带 enabled / allow_from / token_env 三个旋钮，密钥永远是环境变量
// 名（D-010），本 schema 不产生任何密钥值输入。

export type WizardFieldGroup = 'basic' | 'advanced';

export interface WizardFormField {
  key: string;
  label: string;
  type?: 'text' | 'password' | 'number' | 'select' | 'textarea' | 'boolean' | 'string-list';
  secret?: boolean;
  /** 字面占位示例（URL、端口等非翻译文本）。 */
  placeholder?: string;
  /** 翻译占位 / 提示的 i18n key（如 channels.allowFromPlaceholder）。 */
  placeholderKey?: string;
  hint?: string;
  hintKey?: string;
  required?: boolean;
  default?: unknown;
  group?: WizardFieldGroup;
  options?: Array<{ label: string; value: string }>;
}

/**
 * 各平台（通道类型）的凭据字段配置。key 与后端配置一致。
 */
export const CHANNEL_CREDENTIAL_FIELDS: Record<string, WizardFormField[]> = {
  telegram: [
    {
      key: 'token',
      get label() { return t('channelFields.botToken'); },
      type: 'password',
      secret: true,
      required: true,
      get placeholder() { return t('channelFields.telegramToken'); },
    },
    {
      key: 'allow_from',
      get label() { return t('channelFields.allowedUsers'); },
      type: 'string-list',
      placeholderKey: 'channels.allowFromPlaceholder',
      hintKey: 'channels.allowFromHint',
      group: 'advanced',
    },
    {
      key: 'proxy',
      get label() { return t('channelFields.proxyUrl'); },
      type: 'text',
      placeholder: 'http://127.0.0.1:7890',
      group: 'advanced',
    },
  ],
  discord: [
    {
      key: 'token',
      get label() { return t('channelFields.botToken'); },
      type: 'password',
      secret: true,
      required: true,
      get placeholder() { return t('channelFields.discordToken'); },
    },
    {
      key: 'allow_from',
      get label() { return t('channelFields.allowedUsers'); },
      type: 'string-list',
      placeholderKey: 'channels.allowFromPlaceholder',
      hintKey: 'channels.allowFromHint',
      group: 'advanced',
    },
    {
      key: 'guild_id',
      get label() { return t('channelFields.guildId'); },
      type: 'text',
      get placeholder() { return t('channelFields.guildHint'); },
      group: 'advanced',
    },
    {
      key: 'mention_only',
      get label() { return t('channelFields.mentionOnly'); },
      type: 'boolean',
      default: false,
      group: 'advanced',
    },
    {
      key: 'listen_to_bots',
      get label() { return t('channelFields.listenToBots'); },
      type: 'boolean',
      default: false,
      group: 'advanced',
    },
    {
      key: 'group_reply_allowed_sender_ids',
      get label() { return t('channelFields.mentionExempt'); },
      type: 'string-list',
      get placeholder() { return t('channelFields.oneIdPerLine'); },
      get hint() { return t('channelFields.mentionExemptHint'); },
      group: 'advanced',
    },
    {
      key: 'gateway_url',
      get label() { return t('channelFields.gatewayUrl'); },
      type: 'text',
      default: 'wss://gateway.discord.gg/?v=10&encoding=json',
      get hint() { return t('channelFields.gatewayHint'); },
      group: 'advanced',
    },
    {
      key: 'intents',
      get label() { return t('channelFields.gatewayIntents'); },
      type: 'number',
      default: 37377,
      group: 'advanced',
    },
  ],
  feishu: [
    {
      key: 'app_id',
      get label() { return t('channelFields.appId'); },
      type: 'text',
      required: true,
      get placeholder() { return t('channelFields.feishuAppId'); },
    },
    {
      key: 'app_secret',
      get label() { return t('channelFields.appSecret'); },
      type: 'password',
      secret: true,
      required: true,
      get placeholder() { return t('channelFields.feishuAppSecret'); },
    },
    {
      key: 'verification_token',
      get label() { return t('channelFields.verificationToken'); },
      type: 'password',
      secret: true,
      get placeholder() { return t('channelFields.feishuToken'); },
      group: 'advanced',
    },
    {
      key: 'encrypt_key',
      get label() { return t('channelFields.encryptKey'); },
      type: 'password',
      secret: true,
      get placeholder() { return t('channelFields.feishuEncryptKey'); },
      group: 'advanced',
    },
    {
      key: 'allow_from',
      get label() { return t('channelFields.allowedUsers'); },
      type: 'string-list',
      placeholderKey: 'channels.allowFromPlaceholder',
      hintKey: 'channels.allowFromHint',
      group: 'advanced',
    },
    {
      key: 'port',
      get label() { return t('channelFields.webhookPort'); },
      type: 'number',
      get hint() { return t('channelFields.webhookHint'); },
      group: 'advanced',
    },
  ],
  dingtalk: [
    {
      key: 'client_id',
      get label() { return t('channelFields.clientId'); },
      type: 'text',
      required: true,
      get placeholder() { return t('channelFields.dingtalkClientId'); },
    },
    {
      key: 'client_secret',
      get label() { return t('channelFields.clientSecret'); },
      type: 'password',
      secret: true,
      required: true,
      get placeholder() { return t('channelFields.dingtalkClientSecret'); },
    },
    {
      key: 'robot_code',
      get label() { return t('channelFields.robotCode'); },
      type: 'text',
      get placeholder() { return t('channelFields.robotCodeHint'); },
      group: 'advanced',
    },
    {
      key: 'dm_policy',
      get label() { return t('channelFields.dmPolicy'); },
      type: 'select',
      default: 'open',
      options: [
        { get label() { return t('channelFields.open'); }, value: 'open' },
        { get label() { return t('channelFields.allowlist'); }, value: 'allowlist' },
      ],
      group: 'advanced',
    },
    {
      key: 'group_policy',
      get label() { return t('channelFields.groupPolicy'); },
      type: 'select',
      default: 'open',
      options: [
        { get label() { return t('channelFields.open'); }, value: 'open' },
        { get label() { return t('channelFields.allowlist'); }, value: 'allowlist' },
      ],
      group: 'advanced',
    },
    {
      key: 'allow_from',
      get label() { return t('channelFields.allowedUsersGroups'); },
      type: 'string-list',
      placeholderKey: 'channels.allowFromPlaceholder',
      hintKey: 'channels.allowFromHint',
      group: 'advanced',
    },
  ],
  qq: [
    {
      key: 'app_id',
      get label() { return t('channelFields.appId'); },
      type: 'text',
      required: true,
      get placeholder() { return t('channelFields.qqAppId'); },
    },
    {
      key: 'secret',
      get label() { return t('channelFields.botSecret'); },
      type: 'password',
      secret: true,
      required: true,
      get placeholder() { return t('channelFields.qqSecret'); },
    },
    {
      key: 'allow_from',
      get label() { return t('channelFields.allowedUsers'); },
      type: 'string-list',
      placeholderKey: 'channels.allowFromPlaceholder',
      hintKey: 'channels.allowFromHint',
      group: 'advanced',
    },
  ],
};

/** 是否已知通道类型（在凭据 schema 中有字段定义）。 */
export function isKnownChannel(platform: string): boolean {
  return Object.prototype.hasOwnProperty.call(CHANNEL_CREDENTIAL_FIELDS, platform);
}

export function fieldDefaults(platform: string): Record<string, unknown> {
  const defaults: Record<string, unknown> = {};
  for (const field of CHANNEL_CREDENTIAL_FIELDS[platform] || []) {
    if (field.default !== undefined) {
      defaults[field.key] = field.default;
    } else if (field.type === 'string-list') {
      defaults[field.key] = [];
    } else if (field.type === 'boolean') {
      defaults[field.key] = false;
    }
  }
  return defaults;
}

export function fieldsByGroup(platform: string, group: WizardFieldGroup): WizardFormField[] {
  return (CHANNEL_CREDENTIAL_FIELDS[platform] || []).filter(
    (field) => (field.group ?? 'basic') === group,
  );
}

export function splitIdList(text: string): string[] {
  return text
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export function joinIdList(value: unknown): string {
  if (Array.isArray(value)) {
    return value.map((item) => String(item)).join('\n');
  }
  if (typeof value === 'string') {
    return value;
  }
  return '';
}

export function coerceChannelFieldValue(field: WizardFormField, value: unknown): unknown {
  if (field.type === 'boolean') {
    if (typeof value === 'boolean') return value;
    if (value === 'true') return true;
    if (value === 'false') return false;
    return field.default ?? false;
  }
  if (field.type === 'number') {
    if (typeof value === 'number' && Number.isFinite(value)) return value;
    if (typeof value === 'string' && value.trim() !== '') {
      const parsed = Number(value);
      if (Number.isFinite(parsed)) return parsed;
    }
    return field.default ?? null;
  }
  if (field.type === 'string-list') {
    if (Array.isArray(value)) {
      return value.map((item) => String(item)).filter((item) => item.trim() !== '');
    }
    if (typeof value === 'string') return splitIdList(value);
    return [];
  }
  if (value === undefined || value === null) {
    return field.default ?? '';
  }
  return value;
}

export function normalizeChannelConfig(
  platform: string,
  config: Record<string, unknown>,
): Record<string, unknown> {
  const next: Record<string, unknown> = { ...config };
  for (const field of CHANNEL_CREDENTIAL_FIELDS[platform] || []) {
    if (!(field.key in next)) {
      continue;
    }
    next[field.key] = coerceChannelFieldValue(field, next[field.key]);
  }
  return next;
}

/** 获取平台的必填字段列表。 */
export function getRequiredFields(platform: string): string[] {
  const fields = CHANNEL_CREDENTIAL_FIELDS[platform] || [];
  return fields.filter((f) => f.required).map((f) => f.key);
}

/** 验证配置是否完整（必填字段非空）。 */
export function validateConfig(
  platform: string,
  config: Record<string, unknown>,
): {
  valid: boolean;
  missing: string[];
} {
  const required = getRequiredFields(platform);
  const missing = required.filter((key) => {
    const value = config[key];
    return value === undefined || value === null || value === '';
  });

  return {
    valid: missing.length === 0,
    missing,
  };
}
