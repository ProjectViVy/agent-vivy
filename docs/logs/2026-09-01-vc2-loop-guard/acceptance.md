# VC-2 死循环检测 验收路径（人工）

1. 正常会话：让模型连续多次调用同一个只读工具但参数不同（如 grep 不同
   pattern）→ run 正常完成（窗口内签名互异，不触发）。
2. 合法重复：让模型对同一文件连续读 5 次（参数相同、内容相同）→ 正好在上限
   内，run 正常完成。
3. 触发防护：诱导模型重复同一调用（如对 echo 类工具反复发同一指令）→ 第 6 次
   重复时 run 以失败终止，UI 错误条显示
   "The run was stopped because the same tool call kept repeating without
   making progress. ..."，Journal 终态 run.failed 的
   cause_category = `loop_detected`。
4. 预算/轮次上限共存：触发时优先级表现为"先到先停"——循环检测在 6 次重复
   即停，早于默认 MaxToolTurns 与预算熔断。

说明：真实触发需要一次真实模型会话（无本地 mock provider，TEST-1）；自动化
验证由 `internal/runtime` 的 scripted-model 集成测试覆盖（见 verification.md），
人工路径可在有 key 的环境下按上述步骤走查。
