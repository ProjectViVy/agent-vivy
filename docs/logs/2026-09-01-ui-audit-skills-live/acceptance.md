# Acceptance

## 人工验收

1. 打开 `http://127.0.0.1:3015`，进入 Skills 页：技能目录来自真实后端
   `skills/list`（安装的 SKILL.md 目录），开/关开关写真实 frontmatter
   （hash CAS），行为与删除前完全一致——本切片对用户零行为变化。
2. 代码层面：`ui/src/hooks/` 下不再有 `useSkills.ts`；
   `grep -rn "useSkills" ui/src` 零命中。
3. Evolution 页（`/evolution`）不受影响——其 demo 数据路径
   （`vivy.demo.skills` 等）原样保留，归 UI-EVO 行后续处理。

## 判定标准

- `just ci` 绿（UI tsc/eslint/vitest/build 无断裂）。
- `/skills` 页在浏览器中照常列出/查看/开关技能（与删除前一致）。
