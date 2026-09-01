# Acceptance

## 人工验收

1. 打开 `http://127.0.0.1:3015`，进 设置 → 通用 → 上下文压缩卡：无运行
   时「立即压缩」照常可点。
2. 在聊天页发起一个 turn（长回复便于观察），回到设置页：按钮变为禁用，
   按钮行尾出现 amber 提示"有运行进行中，压缩会在运行内自动进行"。
3. 运行结束后（或点「刷新占用」后）按钮恢复可点。
4. 有后台运行（background/list 非终结态）时同样禁用。
5. 英文界面提示为 "A run is in flight; compaction runs inside it.
   Wait for it to finish."，无原始 i18n 键。

## 判定标准

- `just ci` 绿（tsc/eslint/vitest/build 无断裂；compaction e2e spec
  只断言标签，不受按钮禁用影响）。
- 409 路径保留：竞态下点击仍能收到错误消息（feedback 区）。
