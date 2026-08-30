# 超级通道 EPIC（C1–C7c）综合审查报告（summary）

日期：2026-08-31。对象：`feat/channel-c7c` @ eb0cba0（9 个实现 commit，151 文件 +16.5k/−1.3k，基线 82ecf14）。
方法：机械门禁清扫（协调人亲跑）+ 六条独立审查 lane（全新子代理，与实现上下文隔离）+ 修复轮 + 复门禁 + 只读合入预演。

## 判词：可合入（修复轮后）

六条 lane 全部 **PASS、零 blocker**。修复轮处理 12 项（1 代码防御 + 2 verify 规则补强 + 2 pack 卫生 + 2 插件生命周期 + 3 UI 呈现 + 2 文档对账），复门禁 `just ci` exit 0。分支链与 `main` 的合入预演**零冲突**。

## 机械门禁清扫（全部实测，非引用旧记录）

- `just ci` exit 0（fmt-check / vet / test / headless-compile / ui-ci 172 测试）。
- 五插件逐 module（现有门禁盲区）：gofmt 0 脏、vet 0、test 全绿、**`-race` 全绿**。
- verify 矩阵：5 真插件全 ok；8 个 bad-* 夹具全部 exit 1。
- pack 矩阵：5 插件各产候选、EXE 均链接各自 SDK；默认身体 `go list -deps` 对 telego/dingtalk/lark/botgo/discordgo/pion **全零**；`go.mod`/`go.sum` 对 82ecf14 字节差为空；`zz_register.go` 仍 `return nil`。
- 存储：sqlite 17.9s 绿 + postgres（DSN 门控 SKIP，CH-C1-N5 维持 OPEN——本机无 Docker/5432）。
- UI：typecheck/test/build 绿；`just ui-e2e` **6 过 2 败——失败集与基线 82ecf14 完全一致**（runtime 全流程 + welcome-wizard，`git worktree` 基线复跑实证），属 e2e 基线腐烂的既有问题，非本 EPIC 回归；已立 §0.1 TEST-3。
- L5 浏览器冒烟（协调人亲跑，3015）：默认身体空态（无 email/neuro-link、无添加按钮）+ pack telegram 候选（卡片/失败原因逐字/编辑器 fail-closed 文案/token_env 只显名）两场景全过，与 C5 落地时一致。

## 六 lane 结论与代表发现

| Lane | 判词 | 代表发现（详见 findings.md） |
|---|---|---|
| L1 合同符合性 | PASS | 3 条未上板的登记缺口（§8 错误分类槽、verify picoclaw 行未实现、per-seam inspect 未上板）；§12 payload 建议回写合同 |
| L2 内核+安全 | PASS | `deliverCompleted` nil 通道 panic 形（装配序不可达）→ 已修；allow_from 无绕过；密钥零泄漏 |
| L3 适配器横切 | PASS | **dingtalk 网络级静默断线后耳朵失聪**（SDK 语义，注释已纠正 + §0.1 CH-C6-N3）；dingtalk/feishu 重启锁存未复位 → 已修；5 项 SDK 主张源码全证实 |
| L4 SDK/pack | PASS | Listen 封禁可被方法调用绕过 → 封禁已加宽；pack 静默丢 replace/exclude → 改显式报错；双独立 module pack 无测试 → 已补；picoclaw import 封禁缺失 → 已补 |
| L5 UI | PASS | inspect 失败误显空态 → 已修；向导文案超承诺 → 已修；教程文案过期 → 已修；禁语「不限制」残留 → 已修 |
| L6 文档看板 | PASS | 幽灵分支名 c7a（commit 实落 c6 线）→ 文档已纠正；`Settings()` 未回写合同/spec → 已补；UI-CHANNELS-BE 陈旧行 → 已闭 |

## 修复轮（12 项全落，协调查验证）

代码：dispatch nil 通道守卫（+测试）、Listen 封禁加宽（任意接收者 ListenAndServe*/ListenPacket + tls.Listen，+夹具 bad-channel-listen2）、picoclaw/.workspace import 封禁（+夹具）、pack 对非 agent-vivy replace/exclude 显式报错（+测试）、TestPackTwoStandaloneModules + 重复 --with 去重、dingtalk/feishu Start 复位 stopped 锁存（+TestStartAfterStopStartsFresh，feishu 潜在挂起一并消除）、UI 错误态/向导文案/教程文案/禁语四修。
文档：TODO §0.1 新增 CH-R-1/4/5、CH-C6-N3，修 CH-C1-N2/N3、UI-CHANNELS-BE→DONE；§0.2.7 与 CH-C7a/C7b 分支表述纠正；V0 文档 CN-17 漏网处补齐；CHANNEL-PACK §9.3 与 PLUGIN-SPEC §4 补 `Settings()`（C4 新增的符号级对齐）。

## 合入预演（只读）

`git merge-tree`（merge-base HEAD↔main）冲突块数 **0**。合入清单：

- 分支链干净（eb0cba0 处 `git status` 干净）；25 个未 push commit（含合同分支先行的 16 个）。
- 两条路径：**A. 整链 merge `feat/channel-c7c` 进 main**——保留 9 个切片 commit + 合同文档 commit，历史最完整，推荐；B. squash——单一 commit，丢切片粒度，不利于回滚到单刀。不建议 B。
- 根树 main 当前有与本 EPIC 无关的脏区（service_test.go、studio、failure.ts 等）——合入前须先由其归属 lane 处理或 stash，勿混入。

## 明确没做（不做声明）

- 真实厂商冒烟 ×5（无凭据；各插件 acceptance 含人工脚本）；真实 Postgres 路径（无 Docker，CH-C1-N5 开）；C8/C9（需点名/需提案）。
- e2e 两条基线腐烂用例的修复（非本 EPIC 范围，已立 §0.1 TEST-3）。
- note 级发现（约 30 条）未逐条修复，全量见 findings.md。
