# Acceptance

## 人工验收

1. 打开 `http://127.0.0.1:3015`，发起/打开一个有事件的 run，展开聊天侧
   Run Inspector → "当前运行"：事件列表每行（#seq + 类型）可点击。
2. 点击任意事件行：行下展开格式化 JSON 详情（缩进、可换行、超出高度
   内部滚动）；再点收起；同一时间只有一行展开。
3. 仅用键盘 Tab 到事件行、按 Enter/Space 同样能展开/收起（原生 button）；
   读屏可从 `aria-expanded` 获知状态。
4. 事件行不再有悬停 tooltip（title 已移除）；空 payload 事件展开显示
   `null`。

## 判定标准

- `just ci` 绿（tsc/eslint/vitest/build）。
- `just ui-e2e` 全套绿（真实控制面 spec 覆盖 Inspector 所在聊天页）。
