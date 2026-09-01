# 验收：人视角怎么确认

- 打开 `http://127.0.0.1:3015` → 设置 → 通用：「上下文压缩」卡片全部文案
  正常显示（标题、说明、启用开关、三个表单标签与提示、按钮），不再出现
  `settings.compaction.title` 之类的原始键。
- 右侧语言分区切到 English 并刷新：同一卡片显示英文
  （Context compaction / Max tokens / Compaction threshold (%) /
  Keep recent messages / Compact now / Saving… 等），不再夹杂中文。
- 切回简体中文：文案与修复前逐字一致（中文用户无感知变化）。
- 保存配置/立即压缩/刷新占用的成功与失败反馈同样双语
  （`压缩配置已保存…` / `Compaction config saved…`、`压缩完成：X → Y tokens。`）。
- 回归线：`just ui-e2e` 的 `compaction-setting.spec.ts` 在 zh/en 双语言下
  断言标签齐全且全页无原始键，断言失败即回归。
