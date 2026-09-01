# UI-AUDIT-LIFECYCLE-HOME: 物种侧生命周期页改只读 inspect

## Scope

`VIVY-STUDIO.md` 已裁定（NG-23/NG-28、§258）：换代权威在 Studio，物种只保留
只读 `inspect`；物种侧 Generation/EvalRun/Promotion 是"错误的家"，代码冻结保留
但产品权威已迁走。2026-08-31 审查发现日常 Vivy 的 `/lifecycle` 仍提供
create/reject/startEval/record/promote 写表单与按钮，与该裁定不符。

- `LifecycleView.tsx` 重写为只读：删除 Generations 的创建表单与拒绝按钮、Evals
  的启动/记录外部评测表单、Promotions 的提升表单；保留 Species inspect 卡与
  Generations/Evals/Promotions 三个只读列表（徽章/SHA/actor 等照旧展示）。
- 头部下新增权威说明行 `lifecycle.readonlyNote`（en/zh），subtitle 措辞改为只读。
- en/zh 删除仅服务表单的 30 个 lifecycle 键（已确认无其他引用）。
- `api.ts`/`store.ts` 的 generations/evals/promotions 写动作保留不动（NG-28
  "不再扩展产品语义"、§258 "保留代码直到 Studio 账本可替换它们"）——本切片只收
  UI 暴露面，不删后端能力。

## Explicitly not done

- 未删后端 RPC（`generations/*`、`evals/*`、`promotions/promote`）与 store 动作：
  冻结保留属架构明文，删除是另一切片且需与 Studio 账本落地对齐。
- 未动 Settings 卡的「打开生命周期」链接（inspect 入口合法保留）。
- 未动 Evolution 页与 Studio 侧任何内容。
