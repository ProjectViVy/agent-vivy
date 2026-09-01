# Acceptance（人如何确认）

- 行为对单用户不可见——这是并发正确性修复。可观察证据：
  - `go test ./internal/app/settings/ -run TestUpdateConcurrentUpserts -race`
    绿：8 个并发 writer 各写入一条 provider 条目，8/8 存活。把实现换回
    Load→modify→Save 同一测试会丢条目（可在本地 stash 后复跑验证）。
- 所有 Settings UI 流程照常（回归面）：模型/Provider 注册表增删改与「刷新
  模型列表」、MCP 增删、工具开关、通道旋钮、沙箱/压缩/网络偏好保存——
  `just ui-e2e` 全绿覆盖。
- e2e 里 model-refresh spec（真实 UI 路径触发两段式刷新）通过，确认
  两段式重构未改变用户可见的刷新行为。
