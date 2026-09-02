# F0 采纳合同 paperwork（FACE-TUI-1 前置 · docs-only）

## Summary

`VIVY-FACE-PACK.md`（F0 合同）已由 2026-08-31 用户拍板方向采纳（VC 决策 D2，
见 `docs/TODO.md` FACE-0 行），但 PR-1（采纳 paperwork）一直未执行：四份架构
文档的状态行仍停在「提案」。本切片只补 paperwork，不改任何运行时代码：

- `docs/architecture/VIVY-FACE-PACK.md`：状态「提案」→「方向采纳」（2026-08-31，
  VC 决策 D2）；注明 §14 四个开放问题（faces/ 独立 go.mod、默认世代恒 web、
  Journal 同居、headless 审批产品句）在各实施片开工前按推荐值呈报拍板。
- `docs/architecture/VIVY-ASSEMBLY.md`：头注改为已采纳；出厂脸走配方 `face:`
  恰好一张、用户脸走 `plugins/` + `seam: face`。
- `docs/architecture/SELF-EVOLVING-GATEWAY.md`：内核永不插件化名单加入
  FaceHost（与 ChannelHost 并列）。
- `docs/architecture/VIVY-PLUGIN-SPEC.md`：`seam: face` 分流声明改为已采纳
  （原「未扩 seam 之前不得当 tool 插件提交」的临时句改为正式规则引用）。

同时复核 FACE-TUI-1（F3）的实际前置并更新行注：F0 已采纳（本文档切片收口）；
headless 的**功能**已以 `vivy run`（D11）存在但未做成配方器官；F3 的真实剩余
工作 = F1（无网页控制面：进程内 RPC 在无 embed/无 listen 下跑完对话+审批）+
FaceHost/SDK Face 契约 + 出厂 `faces/tui` 器官 + pack `face:` 配方键，按合同
PR 计划分片实施（估 2–3 片）。

## Explicitly not done

- 不写任何运行时代码（FaceHost、SDK Face 契约、`faces/` 目录、pack `face:`
  键均不动）——那是后续独立切片。
- §14 的四个开放问题不在本切片拍板；留给对应实施片。
- 不改 `sdk/plugin` 已落地缝（合同原文约束）。
