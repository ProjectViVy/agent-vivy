# Acceptance — F0 paperwork

## 人工验收

1. 打开 `docs/architecture/VIVY-FACE-PACK.md`：头行状态为「方向采纳
   （2026-08-31，VC 决策 D2）」，不再是「提案」。
2. `docs/architecture/VIVY-ASSEMBLY.md` 头注不再有「未采纳前本表不增加
   face: 行」的条件句；改为已采纳 + `face:`/`seam: face` 指向。
3. `docs/architecture/SELF-EVOLVING-GATEWAY.md` 内核永不插件化清单出现
   FaceHost 行。
4. `docs/architecture/VIVY-PLUGIN-SPEC.md` 的 `seam: face` 段落为正式
   已采纳规则（不再有「本文件未扩 seam 之前」的临时句）。
5. `git grep "提案"` 于上述四文件头部不再命中 face 合同的未采纳状态
   （其他合同的提案状态不在本切片范围）。

## 后续

FACE-TUI-1（F3）代码切片待预算/拍板后按合同 PR 计划开工
（F1 控制面 → FaceHost/SDK Face 契约 → faces/tui 器官 + pack `face:` 键）。
