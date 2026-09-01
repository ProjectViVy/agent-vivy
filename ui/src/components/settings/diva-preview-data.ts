// 'language' 已升级为真实设置分区（LanguagePicker + src/i18n），不再是迁移预览。
// 'compaction' 已并入通用分区（DivaSettingsPreview 的 GeneralPreview），不再是独立设置分区。
// 'network' 已升级为真实设置分区（NetworkToolsCard + settings/get|update），不再是迁移预览。
// 'channels' 已升级为真实设置分区（ChannelsSettings + channel-store），不再是迁移预览。
// 'sandbox' 已升级为真实设置分区（SandboxSettingsCard + settings/get|update），不再是迁移预览。
export const DIVA_PREVIEW_SECTIONS = [
  'general',
  'self-evolution',
] as const;

export type DivaPreviewSection = (typeof DIVA_PREVIEW_SECTIONS)[number];

export type DivaAdditionalSection = Exclude<DivaPreviewSection, 'general'>;

export const DIVA_ADDITIONAL_SECTIONS = DIVA_PREVIEW_SECTIONS.filter(
  (section): section is DivaAdditionalSection => section !== 'general',
);

// 动作名只留 id；显示文案在 i18n（divaPreview.actions.<id>）。
export const DIVA_EVOLUTION_ACTIONS = [
  { id: 'identity' },
  { id: 'relationship' },
  { id: 'commitment' },
  { id: 'sop' },
  { id: 'deprecation' },
] as const;

export type DivaEvolutionAction = (typeof DIVA_EVOLUTION_ACTIONS)[number]['id'];