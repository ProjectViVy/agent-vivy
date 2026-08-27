// 'language' 已升级为真实设置分区（LanguagePicker + src/i18n），不再是迁移预览。
// 'compaction' 已并入通用分区（DivaSettingsPreview 的 GeneralPreview），不再是独立设置分区。
export const DIVA_PREVIEW_SECTIONS = [
  'general',
  'channels',
  'network',
  'self-evolution',
  'sandbox',
] as const;

export type DivaPreviewSection = (typeof DIVA_PREVIEW_SECTIONS)[number];

export type DivaAdditionalSection = Exclude<DivaPreviewSection, 'general'>;

export const DIVA_ADDITIONAL_SECTIONS = DIVA_PREVIEW_SECTIONS.filter(
  (section): section is DivaAdditionalSection => section !== 'general',
);

export type DivaChannelPreview = {
  id: string;
  name: string;
  transport: string;
  enabled: boolean;
  ready: boolean;
};

export const DIVA_CHANNELS: DivaChannelPreview[] = [
  { id: 'telegram', name: 'Telegram', transport: 'Long Polling', enabled: true, ready: true },
  { id: 'discord', name: 'Discord', transport: 'WebSocket Gateway', enabled: true, ready: false },
  { id: 'feishu', name: '飞书', transport: 'WebSocket 长连接', enabled: false, ready: false },
];

export const DIVA_EVOLUTION_ACTIONS = [
  { id: 'identity', label: '身份文档' },
  { id: 'relationship', label: '关系文档' },
  { id: 'commitment', label: '承诺记录' },
  { id: 'sop', label: '操作规范' },
  { id: 'deprecation', label: '弃用建议' },
] as const;

export type DivaEvolutionAction = (typeof DIVA_EVOLUTION_ACTIONS)[number]['id'];
