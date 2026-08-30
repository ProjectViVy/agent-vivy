import { describe, expect, it } from 'vitest';
import {
  CHANNEL_CREDENTIAL_FIELDS,
  coerceChannelFieldValue,
  fieldDefaults,
  fieldsByGroup,
  getRequiredFields,
  isKnownChannel,
  joinIdList,
  normalizeChannelConfig,
  splitIdList,
  validateConfig,
} from './channel-schema';

describe('channel schema 平台覆盖', () => {
  it('只覆盖本代可编译的平台（无 email / neuro-link）', () => {
    expect(Object.keys(CHANNEL_CREDENTIAL_FIELDS).sort()).toEqual([
      'dingtalk',
      'discord',
      'feishu',
      'qq',
      'telegram',
    ]);
    expect(isKnownChannel('email')).toBe(false);
    expect(isKnownChannel('neuro-link')).toBe(false);
  });

  it('每个平台都有 schema（编辑表单与向导渲染依赖）', () => {
    for (const platform of Object.keys(CHANNEL_CREDENTIAL_FIELDS)) {
      expect(CHANNEL_CREDENTIAL_FIELDS[platform].length, platform).toBeGreaterThan(0);
    }
  });

  it('allow_from 一律是 fail-closed 的 i18n 文案键（拒绝启动，不是"不限制"）', () => {
    for (const [platform, fields] of Object.entries(CHANNEL_CREDENTIAL_FIELDS)) {
      const allowFrom = fields.find((field) => field.key === 'allow_from');
      expect(allowFrom, platform).toBeDefined();
      expect(allowFrom?.placeholderKey).toBe('channels.allowFromPlaceholder');
      expect(allowFrom?.hintKey).toBe('channels.allowFromHint');
    }
  });

  it('isKnownChannel 只认 schema 内平台', () => {
    expect(isKnownChannel('telegram')).toBe(true);
    expect(isKnownChannel('slack')).toBe(false);
    expect(isKnownChannel('unknown')).toBe(false);
  });
});

describe('fieldDefaults', () => {
  it('discord 默认补齐 gateway/intents/布尔/列表', () => {
    const defaults = fieldDefaults('discord');
    expect(defaults.gateway_url).toBe('wss://gateway.discord.gg/?v=10&encoding=json');
    expect(defaults.intents).toBe(37377);
    expect(defaults.mention_only).toBe(false);
    expect(defaults.listen_to_bots).toBe(false);
    expect(defaults.allow_from).toEqual([]);
    expect(defaults.group_reply_allowed_sender_ids).toEqual([]);
  });

  it('未知平台返回空对象', () => {
    expect(fieldDefaults('nope')).toEqual({});
  });
});

describe('coerceChannelFieldValue 按类型收敛', () => {
  const boolField = CHANNEL_CREDENTIAL_FIELDS.discord.find((f) => f.key === 'mention_only')!;
  const intField = CHANNEL_CREDENTIAL_FIELDS.discord.find((f) => f.key === 'intents')!;
  const listField = CHANNEL_CREDENTIAL_FIELDS.discord.find((f) => f.key === 'allow_from')!;
  const textField = CHANNEL_CREDENTIAL_FIELDS.telegram.find((f) => f.key === 'token')!;

  it('boolean 字符串与默认回退', () => {
    expect(coerceChannelFieldValue(boolField, 'true')).toBe(true);
    expect(coerceChannelFieldValue(boolField, 'false')).toBe(false);
    expect(coerceChannelFieldValue(boolField, 'nonsense')).toBe(false);
    expect(coerceChannelFieldValue(boolField, undefined)).toBe(false);
  });

  it('number 字符串解析与非法回退', () => {
    expect(coerceChannelFieldValue(intField, '37377')).toBe(37377);
    expect(coerceChannelFieldValue(intField, 'abc')).toBe(37377);
    expect(coerceChannelFieldValue(intField, '')).toBe(37377);
    expect(coerceChannelFieldValue(intField, undefined)).toBe(37377);
  });

  it('string-list 数组/文本归一化', () => {
    // 数组输入按 Diva 语义原样保留（仅去空），不带 trim；UI 写入一律经 splitIdList
    expect(coerceChannelFieldValue(listField, [' a ', '', 'b'])).toEqual([' a ', 'b']);
    expect(coerceChannelFieldValue(listField, ' 1\n2, 3 ')).toEqual(['1', '2', '3']);
    expect(coerceChannelFieldValue(listField, undefined)).toEqual([]);
  });

  it('text 缺省回退默认或空串', () => {
    expect(coerceChannelFieldValue(textField, undefined)).toBe('');
  });
});

describe('splitIdList / joinIdList', () => {
  it('换行与逗号分隔并去空', () => {
    expect(splitIdList(' 1 \n2,3,,4,')).toEqual(['1', '2', '3', '4']);
    expect(splitIdList('')).toEqual([]);
  });

  it('joinIdList 数组按行输出，非数组回退文本/空串', () => {
    expect(joinIdList(['1', '2'])).toBe('1\n2');
    expect(joinIdList('raw')).toBe('raw');
    expect(joinIdList(undefined)).toBe('');
    expect(joinIdList(42)).toBe('');
  });
});

describe('normalizeChannelConfig', () => {
  it('已知字段按类型收敛，未知字段与 enabled 原样保留', () => {
    const next = normalizeChannelConfig('telegram', {
      enabled: true,
      token: 'abc',
      allow_from: '1\n2',
      proxy: null,
      extra_field: 'keep-me',
    });
    expect(next.enabled).toBe(true);
    expect(next.token).toBe('abc');
    expect(next.allow_from).toEqual(['1', '2']);
    expect(next.proxy).toBe('');
    expect(next.extra_field).toBe('keep-me');
  });
});

describe('getRequiredFields / validateConfig', () => {
  it('telegram 必填 token', () => {
    expect(getRequiredFields('telegram')).toEqual(['token']);
  });

  it('feishu 必填 app_id 与 app_secret', () => {
    expect(getRequiredFields('feishu')).toEqual(['app_id', 'app_secret']);
  });

  it('dingtalk 必填 client_id 与 client_secret', () => {
    expect(getRequiredFields('dingtalk')).toEqual(['client_id', 'client_secret']);
  });

  it('validateConfig 缺失列出、齐全通过', () => {
    expect(validateConfig('telegram', { enabled: true })).toEqual({
      valid: false,
      missing: ['token'],
    });
    expect(validateConfig('telegram', { token: 'x' })).toEqual({ valid: true, missing: [] });
  });

  it('未知平台无必填（视为已配置）', () => {
    expect(validateConfig('unknown', { a: 1 })).toEqual({ valid: true, missing: [] });
  });
});

describe('fieldsByGroup', () => {
  it('telegram 基础字段 = token，高级 = allow_from/proxy', () => {
    expect(fieldsByGroup('telegram', 'basic').map((f) => f.key)).toEqual(['token']);
    expect(fieldsByGroup('telegram', 'advanced').map((f) => f.key)).toEqual(['allow_from', 'proxy']);
  });
});
