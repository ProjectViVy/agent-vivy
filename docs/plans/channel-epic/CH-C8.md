# CH-C8 — 同二进制子进程（DEFERRED 备忘，不是开工令）

## 1. 身份

| | |
|---|---|
| ID | CH-C8 |
| 状态 | **DEFERRED** — 不进本期关门 |
| 依赖 | 至少 M-CH2（Host ABI 已按进程边界写） |
| 合同 | §6 第三层卸载、C8 |

未另开能力提案前 **不要领取本文件实现**。

## 2. 目标（将来）

`vivy channel --name telegram` 子进程跑适配器。Host 监督。崩溃 = `channel_lost`，不是物种死亡。杀一只耳朵不断 Journal。

## 3. 现状

C3 起契约按进程边界写（Inbound 只走 Env），但第一刀同进程调用。worker 子 run 的 argv 模式可抄。

## 4. 目标结构

同二进制、不同进程。不是第二种身体（NG-11）。不是 `.dll`。Env.PublishInbound 走 IPC（具体协议后定）。

## 5–8. 不做

不要在 C4–C7 提前发明自定义 IPC。不要外挂 `telegram.exe`。

## 9. 风险

过早拆进程会让 ABI 样板失真。文本闭环稳定后再切。

## 10. 交接

需要独立能力提案后再写真正的十节开工 PLAN。
