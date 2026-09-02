# SKILL-MKT-1 — 市场技能版本比对与原地升级

## 交付

市场技能此前是 create-only：重装一个已存在的技能名直接 409（"already exists"），
只能删除后重装；没有"装的是哪个版本 / 有没有新版"的概念。本次把版本关系做成
本地内容真相（hosted 文件集字节级比对），并补上原地升级：

- **来源清单（provenance manifest）**：安装/升级时在技能根目录写
  `.vivy-skill.json`（`marketplace_id` / `snapshot_hash` / `installed_at` /
  `upgraded_at`）。技能加载器只读 SKILL.md 与四个 hosted 目录，该清单对
  内容、警告、列表完全不可见。
- **`skills/marketplace/install` 增 `mode` 参数**：`create`（默认，行为不变，
  已存在即 409）与 `upgrade`（原地升级：要求技能带清单且
  `marketplace_id` 一致，否则 409 并提示删除重装）。返回增 `outcome`：
  `created` / `upgraded` / `up_to_date`（hosted 集合字节级一致时不写盘）。
- **升级语义**：镜像快照的 hosted 集合（SKILL.md + references/templates/
  scripts/assets）——覆盖新增、删除快照不再包含的 hosted 文件；hosted 范围
  之外的根目录散件不动。全部快照校验（路径穿越/大小/二进制/frontmatter 名）
  先于任何落盘；升级用内存备份回滚（不产生技能根之外的临时目录，
  `ListSkills` 任何时刻都看不到非法名目录；写入全走 atomicWrite）。
- **`skills/marketplace/check`（新 RPC）**：按已装技能名查其清单指向的市场
  快照，返回 `not_installed` / `unmanaged` / `up_to_date` /
  `upgrade_available`（带 `marketplace_id` 与 `snapshot_hash`）。
- **UI（MarketplaceTab）**：已安装行从死按钮"已安装"改为「检查更新」→
  结果呈现：`upgrade_available` →「升级」按钮（mode=upgrade）；`up_to_date`
  → "已是最新"徽标；`unmanaged` → "本地手置技能"提示；升级完成显示 outcome
  通知条。删去不再使用的 `installedLabel` 键。

## 明确不做

- 不做自动批量检查（N 技能 = N 次上游下载，由用户逐技能点击）。
- 不迁移无清单的既有手置技能（`unmanaged` 状态如实呈现，删除重装路径保留）。
- 上游 `hash` 字段只作展示回显，不参与判定（算法不可知，字节级内容比对是
  唯一真相）。

## 相关文件

`internal/tools/skills.go`（接口/类型）、`internal/runtime/marketplace.go`
（清单/升级/检查）、`internal/rpc/control.go`（mode 校验/check 路由/错误码）、
`ui/src/lib/api.ts`、`ui/src/components/skills/MarketplaceTab.tsx`、
`ui/src/i18n/{en,zh}.ts`。
