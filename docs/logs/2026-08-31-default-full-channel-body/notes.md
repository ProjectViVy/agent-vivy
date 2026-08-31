# Notes — 2026-08-31 default-full-channel-body

## 为什么插件模块需要重新 tidy（可复发的耦合）

插件是独立模块，但每块都有 `replace agent-vivy => ../..`。这让
**根模块的整个 require 闭包进入每个插件的 MVS 图**。全量本体把 5 个
插件的依赖并集抬进根图后，共享依赖被抬高（golang.org/x/net → v0.50.0，
连带 golang.org/x/crypto → v0.48.0），discord / qq 自己 go.mod 里的旧
pin 与新解分辨率不一致，`go list`（readonly）即报
"updates to go.mod needed"，`vivy-sdk verify` 的 linkable 检查随之失败。
telegram / dingtalk / feishu 的 pin 恰好与新图一致，无需改动。

结论：**今后根 go.mod 依赖有任何抬升，跑一遍
`for p in plugins/*; do (cd $p && go mod tidy); done`**。这条耦合值得在
VIVY-PLUGIN-SPEC 或 SDK 文档里成段说明（本次未写入契约，见 TODO 提议）。

## pack 幂等化的边界

只跳过「root 已 require 且已 replace」的插件模块对；三方闭包照常合并，
保证 root 依赖较旧时 overlay 仍然自洽。fake-channel（root 未携带）继续
走完整追加路径，原语义不变。

## 已知留白

- `~/.vivy` 数据根：首次冒烟尝试（忘带 VIVY_CONFIG）打开过它。该目录
  2026-08-30 已存在（此前开发会话所建），本次仅幂等迁移/读查询；无数据
  写损迹象。冒烟已改为隔离配置。
- 根 go.mod 现在携带 5 个通道插件的三方闭包，依赖树变大——这是全量
  默认的直接代价，用户已拍板。
- 打包配方命名（vivy-with-channel / vivy-code 这类别名）未实现，见
  summary「明确未做」。
