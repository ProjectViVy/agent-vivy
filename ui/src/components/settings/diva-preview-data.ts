export const DIVA_PREVIEW_SECTIONS = [
  'general',
  'channels',
  'network',
  'language',
  'compaction',
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

export const DIVA_AUDIT_EVENTS = {
  structured: [
    { at: '10:42:18', level: 'info', source: 'settings', message: '加载设置预览数据' },
    { at: '10:41:52', level: 'info', source: 'session', message: '会话 s-demo 已打开' },
    { at: '10:40:07', level: 'warn', source: 'provider', message: '示例 Provider 尚未配置密钥' },
  ],
  gateway: [
    { at: '10:39:31', level: 'info', source: 'gateway', message: '控制面连接状态：connected' },
    { at: '10:38:44', level: 'info', source: 'rpc', message: 'settings/get 请求完成' },
  ],
  gui: [
    { at: '10:42:20', level: 'info', source: 'ui', message: '切换到审计预览' },
    { at: '10:41:03', level: 'info', source: 'ui', message: '打开模型设置' },
  ],
} as const;

export type DivaAuditTab = keyof typeof DIVA_AUDIT_EVENTS;
