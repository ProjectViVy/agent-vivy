# CH-C1 — verification

日期：2026-08-30。工作区：worktree `agent-vivy-channel-c1`，分支 `feat/channel-c1`，基线 82ecf14。

## GOAL 运行方式（子代理分工）

- explore（只读）：扫全仓 `domain.Message{` 字面量（全部键名式，加字段零破坏）、sqlite/pg schema、conformance 入口、`EventTypes` 与 schema 镜像关系。**修正 PLAN §9 一处事实**：sqlite 测试库不是全量 DDL，而是有序迁移链（migration001–015，测试经 `Open` 全链执行）；postgres 才是一次性全量引导。因此 sqlite 走 `migration016`，postgres 走升版 + 原地 ALTER。
- executor（写盘，cwd 锁定 C1 worktree）：按 CH-C1 文件清单实现 12 文件；追加任务补 postgres v14→15 原地升级测试。
- reviewer（只读独立评审）：结论 **PASS**，无 blocker；两条 should-fix（postgres 升级分支无测试覆盖 → 已补 `upgrade_test.go`；log/TODO 工件缺失 → 本目录与 TODO 更新即闭环）。

## 命令与结果（均在 C1 worktree 根执行）

| 命令 | 结果 |
|---|---|
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go test ./...`（executor 实现 后） | 全部包 ok；runtime 60.9s、storage/sqlite 40.3s、storage/postgres 6.9s（DSN 门控用例 SKIP）；`TestBackendConformance/CN-17_message_provenance_round-trip` PASS（sqlite harness） |
| `gofmt -l ./internal ./sdk ./cmd` | 无输出（干净） |
| reviewer 复核 `go test ./internal/... ./sdk/... ./cmd/...` | 全部包 ok（runtime 37.6s 等） |
| `go test ./internal/storage/postgres/... -v`（补测试 后，`env -u VIVY_POSTGRES_TEST_DSN`） | exit 0：`TestMigrateUpgradesV14InPlace` SKIP（DSN 未设）、`TestBackendConformance` SKIP、既有用例 PASS |
| `just ci`（upgrade_test.go 落盘后最终跑） | **exit 0**：`fmt-check`/`vet`/`test`/`headless-compile`/`ui-ci` 全绿；Go 全部包 ok（runtime 43.9s），UI `21 passed (21)` 文件 / `175 passed (175)` 测试，`vite build` 绿 |

## 验收清单（CH-C1.md §7）

- `just ci` 绿 ✅
- `git diff 82ecf14 -- go.mod go.sum` 为空；无 telego/discordgo/lark/botgo ✅
- `channel.inbound` schema 存在；无 token/密钥/原始 body 字段；夹具（含 CN-17 与单测）无密钥 ✅
- 旧 Message 行可 List：迁移列 `DEFAULT ''`，CN-17 空 Source 行读回 `EffectiveSource()=="ui"`；`TestMessageEffectiveSource` 零值兼容 ✅
- UI 路径回归：runtime 全套测试绿，用户行显式 `Source:"ui"`，assistant/tool 投影行为不变 ✅
- 不存在 `internal/channelhost`；`sdk/plugin`、`pluginhost`、`zz_register.go`（仍 `return nil`）零改动；无新增 eino import（Eino 仍在 `internal/runtime`/`internal/provider` 检疫内）✅

## 诚实声明（环境限制）

- 本机无 Docker、无 5432 Postgres：`VIVY_POSTGRES_TEST_DSN` 门控的 `TestBackendConformance`（postgres harness，含 CN-17 pg 侧）与新增 `TestMigrateUpgradesV14InPlace` **测试体未在真实 Postgres 上执行**，仅验证编译、vet 与干净 SKIP。两条依赖标准行为的断言（`information_schema` 的 `column_default` 渲染为 `''`、多语句 fixture exec）需在带 DSN 的 CI 或下一次有 Postgres 的环境跑一轮。仓库既有约定即 DSN 可选、不在 `just ci` 门内，与改动前一致。
- `pnpm build` 弄脏的 `ui/src/routeTree.gen.ts`（纯生成物/行尾噪声）已在提交前 `git checkout --` 还原，未进 commit。

## 范围看守证据

- `git status`：12 改 + 2 新（`channel.inbound.json`、`upgrade_test.go`），全部在 CH-C1 §5 清单内（`postgres/postgres.go` 的 migrate 分支为升版必需管线，已在上文说明）。
- 根树 `main`（脏）与 `feat/channel-super-contract`（文档分支）零写入。
- 未 push（未授权）。
