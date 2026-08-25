# Vivy UI 皮肤（主题）功能

> **2026-08-25 后续**：`love`（恋粉）主题经用户反馈已移除，现行 4 套。
> 见 `docs/logs/2026-08-25-remove-love-theme/`。下文为交付时点的原始记录。

## 变更内容

Vivy UI 从"只有默认浅色主题"升级为 5 套可切换皮肤，主题色值统一走
shadcn 语义 Token（新规范），选择立即生效并持久化到当前浏览器。

- `ui/src/hooks/use-theme.ts`（新增）：主题单一入口——注册表
  （id / label / description / appearance / 预览色板字面量）、
  `applyThemeToDOM`（设 `data-theme`，深色外观同步挂 `.dark` 类驱动
  Tailwind `dark:` 变体）、`vivy.theme` localStorage 持久化、
  `useSyncExternalStore` 模块级 store 的 `useTheme()`。
- `ui/src/styles.css`：
  - `.dark` 令牌块迁移为 `[data-theme="dark"]`（数值不变，深蓝夜色主题）；
    `.dark` 类从此只承担 `dark:` 变体开关，不再承载令牌值。
  - 新增三个完整令牌块：`[data-theme="love"]`（恋粉，浅色）、
    `[data-theme="pink"]`（简约粉白，浅色）、`[data-theme="miku"]`
    （Miku 青，GitHub Dark 底 + 应援青，深色）。色值从 Agent-Diva
    `love / default / miku` 主题的变量换算映射到语义 Token
    （`--background/--primary/--sidebar-*` 等）。
- `ui/index.html`：反闪烁内联脚本改为读取 `vivy.theme` 并应用
  `data-theme` + `.dark`；删除旧的 iframe 父窗口 light/dark 推送桥
  （演示遗留，无消费方）。内联底色按主题给出近似值。
- `ui/src/components/settings/ThemePicker.tsx`（新增）：设置 → 通用 tab
  的真实主题选择卡（预览渐变 + 名称 + 描述 + 选中态），位于"应用信息"
  与迁移预览之间。
- `SettingsView.tsx`：通用 tab 接入 `ThemePicker`；`DemoNote` 移到
  只覆盖迁移预览的位置，避免把真实主题卡误标为演示内容。
- 删除伪操作：`DivaSettingsPreview` 通用预览里"选择不会改变 Vivy 全局
  主题"的假主题卡、`diva-preview-data.ts` 的 `DIVA_THEME_PREVIEWS` 与
  `'theme'` section id（真实功能已取代它）。
- 测试：新增 `ui/src/hooks/use-theme.test.ts`（8 例：注册表完整性、
  id 校验、存储回退、DOM 应用与 `.dark` 切换、非法值忽略）；
  `diva-preview-data.test.ts` 同步移除主题预览断言。

## 主题清单

| id | 名称 | 外观 | 来源 |
|---|---|---|---|
| `default` | Vivy 蓝 | 浅色 | 原 `:root` 默认主题（不变） |
| `love` | 恋粉 | 浅色 | 移植 Agent-Diva `love` |
| `pink` | 简约粉白 | 浅色 | 移植 Agent-Diva `default`（vivy 的 default 已被占用，按其"简约粉白"含义改名） |
| `dark` | 深蓝夜色 | 深色 | 原 `.dark` 令牌块（与 Agent-Diva `dark` 同为深底蓝强调，取已有数值零回归） |
| `miku` | Miku 青 | 深色 | 移植 Agent-Diva `miku` |

## 明确不做

- 不移植 Diva 的渐变 App 背景、玻璃拟态、樱花/爱心浮动装饰：新规范
  下皮肤只表达为语义 Token 的取值差异；大面积渐变背景不在 token 体系内。
- 不按主题改 `--radius`（12px 统一），避免嵌套圆角回归面。
- 主题不进后端 Settings RPC：皮肤是浏览器本地偏好（与活动会话 key
  同级），不写运行配置。
- 不改 `index.html` 的 `<title>`（仍为旧演示标题，已记 §0.1 UI-TITLE）。
- 不加侧边栏快捷切换入口；本轮只做设置页选择卡。

## 验证

见 `verification.md`；人工验收路径见 `acceptance.md`。
