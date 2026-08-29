# Acceptance · Skill 真正可用

人在 split UI（`http://127.0.0.1:3015`，不是 `:8787` 内嵌 UI）打开 `/skills` 可以确认：

1. **没有演示条、没有变更请求 Tab。** 页面只有已安装技能列表和刷新。没有「待审 / 已接受」假请求。

2. **空根。** `skills_root` 没有 `SKILL.md` 时，文案说明把技能目录放到配置的根后再刷新。刷新会重新打 `skills/list`，不会从 `vivy.demo.skills` 长出种子。

3. **有技能。** 列表显示名称 + 描述。点一项加载真实 `SKILL.md` 正文。有警告时正文上方列出。有 `references/` / `templates/` / `scripts/` / `assets/` 文件时可点开，内容来自 `skills/get`。

4. **聊天里可用。** 用户不必说 “skill”。Eino 中间件把目录放进只读 `skill` 工具描述；模型调用后加载正文。改技能仍走对话里的 `skill_manage` + Review Center，不在本页点保存。

5. **进化页仍是演示。** `/evolution` 继续用本地 `vivy.demo.*`，可以和真实 skills 根不一致。
