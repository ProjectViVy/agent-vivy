# Acceptance

人工判定方式（无需读代码）：

1. `config.yaml` 的 `runtime.compaction` 下加 `summary_model: <便宜模型id>`
   （同一 provider），重启 vivy。
2. 让一个会话超过压缩阈值（长对话/大工具结果）：日志/事件里的摘要生成走
   便宜模型；上下文照常压缩，回答不变形。
3. 把 `summary_model` 改成一个不存在的 id：压缩照常完成——摘要调用失败自动
   回退主模型一次（唯一可见差异是 failover 事件/日志），run 不挂。
4. 删掉 `summary_model`：行为与升级前逐字节一致（主模型摘要、无 failover）。
5. Settings → 通用 → 上下文压缩 的开关/阈值行为不变（本字段不进设置覆盖层）。
