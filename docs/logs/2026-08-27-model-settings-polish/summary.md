# 模型设置页视觉梳理 + 生成参数卡主题化

## 问题（用户反馈）

1. 「设置 → 模型」页 UI 混乱：供应商行选中态同时叠加 4px 左边框、accent 背景、
   加粗三重强调；「更多供应商」「新增自定义供应商」两行用 `border-dashed + border-l-4
   border-l-transparent` 伪装成列表行；右栏头部三个同尺寸图标按钮（编辑/从官方目录
   同步/新增模型）含义不同却并排，其中「从官方目录同步」只是静态快照重载的伪操作
   （点了只会弹一条“已重新载入”提示，不产生真实结果）；右栏头部/API Key/模型列表
   三块连续用 `border-b` 分隔，分组不明。
2. 「生成参数」卡与主题脱节：卡片里塞进整条琥珀色 `DemoNote` 横幅（与描述文字重复
   告知“示例/不传给 Provider”）；标签是英文 Temperature / Max Tokens；原始数字输入
   没有范围/取值提示；按钮与已保存反馈排版随意。

## 改动

- `ModelSettingsCard.tsx`（真实模型配置卡）：
  - 供应商行选中态去掉 4px 左边框，收敛为 `bg-accent + text-accent-foreground +
    font-medium`（沿用 MaskAndModelSwitcher 当前配置行的 accent 语言）；hover 统一
    `hover:bg-accent/60`。
  - 「更多供应商」「新增自定义供应商」两行删除 `border-dashed / border-l-4` 伪装，
    改普通幽灵行（muted 文字 + hover accent），与列表行同高同 padding。
  - 左栏列表容器 `bg-muted/30` 统一为 `bg-card`，与右栏面板同一表面语言。
  - 右栏：删除「从官方目录同步」伪操作按钮（`handleRefresh`/`refreshNote` 随之删除，
    i18n `refreshModels`/`refreshedModels` 从 zh/en 词典移除）；「新增模型」按钮移入
    模型列表分区头部（标签用既有的 `settingsModel.modelsTitle`「{{provider}} 模型」），
    头部只保留供应商名/地址/运行束徽标 + 编辑铅笔。
  - API Key 分区把「已配置 API Key」提示提为行尾小字，减少正文第二段说明的堆叠；
    只读提示改用 `settings.readOnlyNotice` i18n 与项目琥珀色约定
    （amber-600 / dark:amber-300，对齐 MaskAndModelSwitcher 的 readOnlyHint）。
- `SettingsView.tsx`（设置页）：
  - 模型 Tab 两张卡都加上项目通用的图标卡头（`h-9 w-9 rounded-lg bg-primary/10
    text-primary`，模型卡 `Cpu`、参数卡 `SlidersHorizontal`），标题/描述改走既有
    `settings.*` i18n 词条（此前词条已存在但页面一直写死中文，属重复权威来源）。
  - 「生成参数」卡重做：删除卡内 `DemoNote` 整条横幅，改为标题旁紧凑「演示」徽标；
    温度改为主题化滑杆（`Slider`，0–2、步进 0.1，跟随 primary 令牌）+ 右侧等宽数值
    实时显示；Max Tokens 保留数值输入；保存按钮下方增加主题化成功反馈
    （primary 对勾 + 「已保存到本地」）。
- `i18n/zh.ts` / `en.ts`：新增 `settings.demoBadge`；`settings.temperature` zh 值改
  中文「温度」（原为英文 Temperature）；`settingsModel.noModels` 文案更新为指向列表
  头部新增按钮；删除失效的 `refreshModels` / `refreshedModels` 词条。zh/en 结构经过
  `i18n/index.test.ts` 深度校验保持一致。

## 未做（显式边界）

- 设置页「通用 / 人格 / 工具 / Vivy 功能」等 Tab 仍为既有写死中文文案，未一并接入
  `settings.*` 词条（页面原本就是 i18n 与写死混用；本次只收口用户点名的模型 Tab）。
- 真实在线目录同步（文档注释中的 UI-PROV-RPC）仍为 OPEN，本次删除的是其界面上的
  伪操作占位，不改变后端 RPC 计划。
- 生成参数仍是演示面（vivy.demo.*，不传真实 Provider）——演示身份保留为「演示」徽标
  + 描述，未提升为真实配置。