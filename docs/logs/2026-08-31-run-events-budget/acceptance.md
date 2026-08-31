# Acceptance — 2026-08-31 运行事件预算豁免

人怎么确认修好了：

1. 重启开发实例（`just run`，加载新二进制 + 删除陈旧 `tools.enabled`
   后的 config 默认 26 工具面）。
2. 设置 → 工具：卡片应显示绝大部分工具为激活态，顶部说明为
   "配置默认值：26 个（当前未写覆盖层）"。
3. 新建会话，发中文："你现在有什么工具？能看到工作区里有什么吗？"
   - 修复前：数秒后报"这次对话没有完成…reached a safety budget"。
   - 修复后：模型完整回复；能看到它真实调用 `list_dir`（智能体模式
     下工作区文件列表出现在工具卡片/回复里）。
4. 长回复不再中断：让模型写一段长内容（如"详细介绍一下你自己"），
   流式输出应完整结束，不触发 safety budget。
5. `data/logs/vivy.log.*` 不再新增
   `run budget circuit breaker opened ... kind=events`。

注意：如果模型陷入无意义循环，仍会停在 model_calls(32)/tool_calls(64)
预算上——这是设计内的失控保护，不是本缺陷。
