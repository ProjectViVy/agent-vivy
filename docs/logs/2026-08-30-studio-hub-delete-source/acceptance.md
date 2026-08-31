# Acceptance — 2026-08-30 Studio 插件中心卸载闭环 + 自动提交

## 用户怎么确认它生效

1. **卸载删除源码目录**：在 Vivy Studio 插件中心（VIVY-STUDIO-PLUGIN-HUB）
   的「已安装」列表对任一 **Vivy 源码安装**的插件点「卸载」：
   - 确认弹窗出现琥珀警示「将同时删除源码目录 studio/<插件名>…」；
   - 确认后任务日志可见 `[vivy-source] deleting source directory
     ...\studio\<插件名>`（克隆来源若带未提交改动，先出现
     `warning: ... has uncommitted changes` 行）；
   - 完成后 `studio/` 下该插件目录**已不存在**（`git -C studio status`
     能看到删除待提交/已提交）。
2. **自动 commit**（默认开启）：卸载完成后 `git -C studio log -1` 是
   `chore(hub): remove plugin <name> source`；安装 / 更新源码插件后同样
   自动出现 `chore(hub): install/update plugin <name> source` 提交，且
   只含该插件路径，`studio/` 里其他未提交改动原样保留。
3. **开关**：插件中心 → 设置 → 更新与源（Updates & Sources）可见
   「源码插件自动提交」，关掉后安装/卸载不再产生提交（删除仍执行，改动
   留待手动 `git -C studio commit`）。
4. **非源码插件不受影响**：系统目录安装（非 vivy-source）的插件照旧，
   无源码目录可删，弹窗无警示。

## 环境

- Vivy Studio（DSH harness web，端口 3090），profile `vivy-studio`，
  `vivySourceInstall` 开启（默认）。
- 卸载目标必须是注册表 `vivy-source-plugins.json` 或 `file:` 指向
  `studio/` 的依赖；否则走原有系统目录卸载路径（不删源码，符合预期）。