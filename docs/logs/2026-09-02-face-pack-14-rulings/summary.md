# FACE-PACK §14 四问拍板记录

## What changed

`docs/architecture/VIVY-FACE-PACK.md` §14：四道"采纳前要人拍板"的问题全部收到用户终审（2026-09-02，全取合同推荐值），F2/F3 切片前置解除：

1. **faces/ 独立 `go.mod`**——网关世代编译期不见 TUI deps。
2. **默认永远 `face: web`**——coding 世代是另一条配方，不替换日常双击。
3. **网页与 TUI 第一刀不同居**——一代一张嘴；多客户端同居后切且需先钉死审批 first-writer-wins。
4. **headless 遇审批 = 失败退出**——不挂起等待、不 yolo（run 级持久挂起+可取消由 F1 测试钉死，进程级句子=失败退出）。

`docs/TODO.md`：FACE-TUI-1 行补拍板注记 + §10 记账。

## Explicitly not done

- F2（出厂 `faces/headless` 器官 + pack overlay）与 F3（出厂 `faces/tui` 器官）的实现——本片只解除其决策前置。
