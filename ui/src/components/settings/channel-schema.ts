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
      label: 'Bot Token',
      type: 'password',
      secret: true,
      required: true,
      placeholder: '输入 Telegram 机器人令牌',
    },
    {
      key: 'allow_from',
      label: '允许的用户 ID',
      type: 'string-list',
      placeholderKey: 'channels.allowFromPlaceholder',
      hintKey: 'channels.allowFromHint',
      group: 'advanced',
    },
    {
      key: 'proxy',
      label: '代理 URL',
      type: 'text',
      placeholder: 'http://127.0.0.1:7890',
      group: 'advanced',
    },
  ],
  discord: [
    {
      key: 'token',
      label: 'Bot Token',
      type: 'password',
      secret: true,
      required: true,
      placeholder: '输入 Discord 机器人令牌',
    },
    {
      key: 'allow_from',
      label: '允许的用户 ID',
      type: 'string-list',
      placeholderKey: 'channels.allowFromPlaceholder',
      hintKey: 'channels.allowFromHint',
      group: 'advanced',
    },
    {
      key: 'guild_id',
      label: '服务器 ID（可选）',
      type: 'text',
      placeholder: '仅处理该服务器内群消息；留空表示不限制',
      group: 'advanced',
    },
    {
      key: 'mention_only',
      label: '仅在服务器内 @ 机器人时响应',
      type: 'boolean',
      default: false,
      group: 'advanced',
    },
    {
      key: 'listen_to_bots',
      label: '接收其他机器人消息',
      type: 'boolean',
      default: false,
      group: 'advanced',
    },
    {
      key: 'group_reply_allowed_sender_ids',
      label: '免 @ 触发的用户 ID',
      type: 'string-list',
      placeholder: '每行一个用户 ID',
      hint: '在开启「仅 @ 响应」时，这些用户仍可在群内直接发消息触发。',
      group: 'advanced',
    },
    {
      key: 'gateway_url',
      label: 'Gateway URL',
      type: 'text',
      default: 'wss://gateway.discord.gg/?v=10&encoding=json',
      hint: 'Discord WebSocket 网关地址',
      group: 'advanced',
    },
    {
      key: 'intents',
      label: 'Gateway Intents',
      type: 'number',
      default: 37377,
      group: 'advanced',
    },
  ],
  feishu: [
    {
      key: 'app_id',
      label: 'App ID',
      type: 'text',
      required: true,
      placeholder: '飞书应用 App ID',
    },
    {
      key: 'app_secret',
      label: 'App Secret',
      type: 'password',
      secret: true,
      required: true,
      placeholder: '飞书应用 App Secret',
    },
    {
      key: 'verification_token',
      label: 'Verification Token',
      type: 'password',
      secret: true,
      placeholder: '飞书验证 Token',
      group: 'advanced',
    },
    {
      key: 'encrypt_key',
      label: 'Encrypt Key',
      type: 'password',
      secret: true,
      placeholder: '飞书事件加密 Key',
      group: 'advanced',
    },
    {
      key: 'allow_from',
      label: '允许的用户 ID',
      type: 'string-list',
      placeholderKey: 'channels.allowFromPlaceholder',
      hintKey: 'channels.allowFromHint',
      group: 'advanced',
    },
    {
      key: 'port',
      label: 'Webhook 端口（可选）',
      type: 'number',
      hint: '仅 webhook 模式需要；WebSocket 长连接可留空。',
      group: 'advanced',
    },
  ],
  dingtalk: [
    {
      key: 'client_id',
      label: 'Client ID',
      type: 'text',
      required: true,
      placeholder: '钉钉应用 Client ID',
    },
    {
      key: 'client_secret',
      label: 'Client Secret',
      type: 'password',
      secret: true,
      required: true,
      placeholder: '钉钉应用 Client Secret',
    },
    {
      key: 'robot_code',
      label: '机器人代码',
      type: 'text',
      placeholder: '可选：机器人代码',
      group: 'advanced',
    },
    {
      key: 'dm_policy',
      label: '私聊策略',
      type: 'select',
      default: 'open',
      options: [
        { label: '开放', value: 'open' },
        { label: '白名单', value: 'allowlist' },
      ],
      group: 'advanced',
    },
    {
      key: 'group_policy',
      label: '群聊策略',
      type: 'select',
      default: 'open',
      options: [
        { label: '开放', value: 'open' },
        { label: '白名单', value: 'allowlist' },
      ],
      group: 'advanced',
    },
    {
      key: 'allow_from',
      label: '允许的用户 / 群 ID',
      type: 'string-list',
      placeholderKey: 'channels.allowFromPlaceholder',
      hintKey: 'channels.allowFromHint',
      group: 'advanced',
    },
  ],
  qq: [
    {
      key: 'app_id',
      label: 'App ID',
      type: 'text',
      required: true,
      placeholder: 'QQ 机器人 App ID',
    },
    {
      key: 'secret',
      label: '机器人 Secret',
      type: 'password',
      secret: true,
      required: true,
      placeholder: 'QQ 机器人 client secret',
    },
    {
      key: 'allow_from',
      label: '允许的用户 ID',
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
