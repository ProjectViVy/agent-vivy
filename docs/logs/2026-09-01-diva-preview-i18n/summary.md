# UI-DIVA-PREVIEW-I18N — DivaSettingsPreview + SettingsView 全面接入 i18n

## Scope

- `ui/src/components/settings/DivaSettingsPreview.tsx`：整组件去硬编码，改
  `useTranslation()`。GeneralPreview（预览通知 / 聊天显示 / 缓存与运行状态 /
  关于 Vivy / 「压缩配置已毕业」迁移说明）与 SelfEvolutionPreview（自进化分区的
  描述、频率选项、确认策略 toast、五个动作行）全部走 `t()`；动作名用动态键
  `t(`divaPreview.actions.${action.id}`)`，确认策略提示用插值
  `t('divaPreview.confirmUpdated', { label })`。MIT / projectViVY 等专有名词保持字面量。
- `ui/src/components/settings/diva-preview-data.ts`：`DIVA_EVOLUTION_ACTIONS`
  只留 id，显示文案移入 i18n（`divaPreview.actions.<id>`），避免「数据文件带中文
  label、组件再翻译」双轨。`diva-preview-data.test.ts` 无 label 断言，不需要改。
- `ui/src/components/settings/SettingsView.tsx`：同文件同类欠账一并消化——
  删除 `DIVA_TAB_LABELS` 常量，全部 tab 触发器改
  `t('settings.tabs.*')`（DIVA 附加分区动态 `t(`settings.tabs.${…}`)` +
  `t('divaPreview.previewBadge')` 预览徽标）；执行超时卡、应用信息卡、工具卡、
  生命周期卡、Run Inspector 卡、只读提示、错误提示、保存按钮全部改 `t()`，
  优先复用既有键（title/subtitle/appInfo/tools/lifecycle/runInspector/
  readOnlyNotice/saving），新增 `settings.executeTimeout*` 与
  `settings.saveGeneral`。
- `ui/src/i18n/zh.ts` / `en.ts`：新增顶层 `divaPreview` 段（约 50 键 + 嵌套
  `actions`），zh/en 键位对等；`tabs.network` 值「网络」→「网络工具」以匹配
  既有可见文案（`network-tools-setting.spec.ts` 断言 tab 名为「网络工具」）；
  `settings.executeTimeout*`、`settings.saveGeneral` 新键。

## zh 文案政策

`settings.*` 新键的 zh 文案从原硬编码逐字搬运（执行超时卡、错误提示、保存
按钮），英文界面原本就漏中文的预览区则新写英文文案；zh 文案保持逐字不变，
用户可见中文零回归。

## Not done

- `routeTree.gen.ts` 为生成文件，churn 不入本提交（一贯政策）。
- 预览区假数据行为不变（DIVA 迁移预览仍只展示、不落配置）；预览区是否整体
  毕业为真实分区不在本条范围（TODO 其他行）。
- `Welcome` / `ChannelsSettings` / `SandboxSettingsCard` 等其他组件的 i18n
  覆盖不在本条（`language-setting.spec.ts` 已覆盖语言切换主链路）。
