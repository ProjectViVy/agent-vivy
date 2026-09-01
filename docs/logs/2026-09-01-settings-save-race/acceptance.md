# 验收：人视角怎么确认

- 日常使用中「设置 → 模型」的各种写操作（新增模型、切换当前模型、
  修改自定义供应商）不再间歇出现红色「internal error」条。
- 并发压力下文档不损坏：`go test ./internal/app/settings/ -race -count=3`
  通过；测试断言并发 Save 后文档仍可解析、无残留 `.tmp` 文件。
- Windows 特有失败模式（rename 覆盖被读句柄）不再出现；
  修复前可用上述测试首轮复现 `Access is denied`。
- 无可感知行为变化：设置文件格式、RPC 契约、错误文案均保持原样
  （internal error 仍不泄露内部细节）。
