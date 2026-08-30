# 验证记录

日期：2026-08-30。全部命令在 worktree
`C:\Users\Administrator\Desktop\morediva\diva-go\agent-vivy-skill-market`
（分支 `feat/skill-marketplace`）内执行。

## 门禁

| 命令 | 结果 |
|---|---|
| `go test ./internal/runtime/ ./internal/rpc/ ./internal/config/ ./internal/tools/ ./internal/app/ -count=1` | ok（runtime 42.5s / rpc 17.8s / config 1.6s / tools 0.4s / app 3.9s） |
| `just ci`（fmt-check、vet、go test ./...、headless-compile、ui-ci） | 首跑 fmt-check 失败（app.go/control.go 两处 gofmt 对齐），`gofmt -w` 后复跑 **EXIT=0 全绿** |
| `pnpm typecheck` / `pnpm test` / `pnpm build`（ui-ci 内含） | 21 个测试文件 175 用例全过；构建产物正常 |

新增测试覆盖（kernel）：

- `internal/runtime/marketplace_test.go`：search 映射/limit 钳制/短查询拒绝、
  安装落盘 + ListSkills 可见 + 重装冲突、name/slug 不匹配、缺根 SKILL.md、
  二进制、路径穿越、Vivy 不可承载的 id（先于下载拒绝）、上游错误
  （`MarketplaceUpstreamError` 携带 upstream detail）、非法 base URL、
  `VIVY_SKILLS_MARKETPLACE_URL` 覆盖、内置 featured 快照可解析。
- `internal/runtime/skills_backend_test.go`（追加）：`enabled: false` 从 Eino
  List/Get 隐藏但控制面可见可看；`SetSkillEnabled` 启停往返、CAS 过期哈希
  拒绝、重渲染保留正文与 description。
- `internal/config/config_test.go`：`skills_marketplace_url` 默认值、覆盖
  解析、非法 URL 校验拒绝。

## 真路径 smoke（split dev，非嵌入 UI）

环境：worktree 后端 `go run ./cmd/vivy`（127.0.0.1:18787，runtime.mock: true，
`skills_root: data/skills`，`skills_marketplace_url: http://127.0.0.1:8899`）；
Vite `pnpm exec vite --port 3016`（`VIVY_BACKEND_ADDR` 指向 18787）；本地
Python mock skills.sh（`data/smoke_marketplace.py`，占位 8899 —— 本沙箱
TLS 出站受限，真实 skills.sh 不可达，market URL 语义与 HTTPS 路径同构）。
浏览器实测 `http://127.0.0.1:3016/skills`：

1. **Demo 横幅已消失**；页签为 已安装技能 (0) / 市场 / 变更请求 (0)。
2. **capability 门控**：市场页签出现（`initialize` 广播 `skills.marketplace`）。
3. **featured**：未搜索时展示内置精选排行（find-skills 846.6k 等 100 条，
   快照日期 2026/8/21），安装数 k/m 格式化正常。
4. **搜索**：输入 `demo`（防抖后）命中本地 mock 的 `demo-market-skill`
   （12.3k），显示"共 1 个结果"。
5. **安装**：点击安装 → 已安装页签变 (1)，按钮翻转为"已安装"态；磁盘
   `data/skills/demo-market-skill/` 落盘 SKILL.md + references/guide.md；
   快照中的 README.md 被按预期跳过（skipped）。
6. **详情**：显示描述、内容哈希（SHA-256）、附属文件数、可点击的
   references/guide.md、SKILL.md 正文。
7. **启停**：拨动开关 → 列表与详情徽标翻转为"已停用"（2 处），磁盘
   SKILL.md frontmatter 变为 `enabled: false`（规范化重渲染，正文保留）。
8. **变更请求**：空态"暂无待审的 Skill 修订。"（`skills/revisions/list`）。
9. 首启向导正常弹出并可跳过（与本迭代无关，仅 smoke 路径记录）。

smoke 后已停止后端/Vite/mock 三个进程（端口 18787/3016/8899 释放）。

## 覆盖留白（如实记录）

- **待审修订的非空渲染未在浏览器 smoke**：`skill_manage` 暂存修订依赖真实
  run 中的 HITL 提案；mock 场景不驱动 skill_manage。该链路由单元测试覆盖
  （PrepareSkillProposal → ListPendingSkillRevisions → RPC 映射），UI 卡片
  渲染逻辑与空态共用同一路径，未做浏览器级非空验证。
- **真实 skills.sh 网络路径**：沙箱 TLS 出站受限，search/install 走本地
  mock；适配层与上游契约（`/api/search`、`/api/download/{owner}/{repo}/{slug}`
  的 JSON 形状）按 DIVA 实现与 wiremock 单测对齐，公网联通性需在开发机上
  复核（`just run` 默认 `https://skills.sh`）。
- e2e（`just ui-e2e`）未跑：`UI-E2E-STALE` 两条既有过期规格仍失败（见
  `docs/TODO.md` §0.1，非本迭代引入）。
