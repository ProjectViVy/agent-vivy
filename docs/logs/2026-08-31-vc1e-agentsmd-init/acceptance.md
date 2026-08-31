# Acceptance — VC-1e

如何向人证明这个交付有效。

## 1. `vivy init`（人的视角）

在一个已有项目的目录里运行：

```text
$ vivy init
created C:\path\to\project\AGENTS.md
record only what is non-obvious: an agent can read the code, but it cannot guess intent.
```

- 生成的 `AGENTS.md` 有四个空节（overview / build / conventions / pitfalls），每节带 HTML 注释引导。
- 目录里已有 `.cursorrules` 或 `.github/copilot-instructions.md` 时，输出会点名这些文件，生成的 AGENTS.md 末尾有"保持同步或引用"一节。
- 空目录（连 `main.go` 都没有）→ 报错退出，不生成。
- 已有 AGENTS.md → 报错退出，原文件一字不动。

## 2. AGENTS.md 注入（run 的视角）

1. 把 AGENTS.md 放进某个 run 的 workspace（例如 `vivy init` 后通过 UI/工具放入，或让 run 自己写入）。
2. 在该 session 发一条消息。
3. **可观察效果**：模型回答遵循 AGENTS.md 里的指令（如"回答以 VIVY 开头"这类可验证规则）；AGENTS.md 不存在时行为与从前完全一致。
4. **逐 run 隔离**：另一个 run 的 workspace 没有该文件，就不受影响。

## 3. 瞬态性（D6 的核心承诺）

- 注入内容只出现在模型调用现场：Journal 事件 replay 与消息存储里**永远搜不到** AGENTS.md 的正文（测试 #4/#5 以 marker 断言）。
- 上下文压缩（compaction）永远不需要处理它——注入发生在压缩层之后，天然不进摘要。

## 4. 边界

- 无 AGENTS.md 的 run：零注入、零行为差异（测试 #2）。
- 审批挂起→恢复后：注入内容仍恰好出现一次，不因 checkpoint 往返而重复（测试 #5）。
