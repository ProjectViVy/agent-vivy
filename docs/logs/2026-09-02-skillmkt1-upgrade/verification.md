# 验证记录：SKILL-MKT-1

## 聚焦测试（提交前）

- `go test ./internal/runtime/ -run "TestMarketplace" -count=1` → ok
  （12 用例含新增 `TestMarketplaceUpgradeMirrorsSnapshot`（升级镜像快照 +
  hosted 删除 + 根散件保留 + 清单字段 + create 仍 409）、
  `TestMarketplaceUpgradeUpToDateWritesNothing`（清单字节不变）、
  `TestMarketplaceUpgradeGuards`（未知 mode/无清单/来源不符）、
  `TestMarketplaceCheckUpdateStatuses`（四状态 + 非法名拒绝））
- `go test ./internal/rpc/ -run "TestMarketplaceInstallModeAndCheckRoute" -count=1` → ok
  （mode 校验 InvalidParams、upgrade/create 透传、not marketplace-managed →
  CodeConflict、check 路由、未配置 marketplace → MethodNotFound）
- `gofmt -w` + `go build ./...` + `go vet ./internal/{runtime,rpc,tools}/` → 绿
- `cd ui && pnpm typecheck` → 绿

## 产品门禁（just ci + just ui-e2e，后台 tail-check）

- `just ci` → **CI-EXIT:0**（/tmp/ci-skillmkt1.log）
- `just ui-e2e` → **E2E-EXIT:0，17 passed (28.3s)**（/tmp/uie2e-skillmkt1.log）

## 真实路径冒烟（3015 split Vite，2026-09-02）

- `just run`（8787，CI-EXIT 后启动）+ `cd ui; pnpm dev`（3015），双端口 200。
- skills.sh 可达（`curl /api/search?q=commit` → 200）。
- Playwright 驱动（scratch 脚本，跑后即删）：
  1. `/skills` → 「市场」tab → 精选榜单出现 `find-skills`；
  2. 点「安装」→ 真实下载 skills.sh 快照并安装成功，行内按钮变「检查更新」；
  3. 点「检查更新」→ 真实二次下载 + hosted 集合字节级比对 → 「已是最新」徽标
     出现。
  - 输出：`STEP install find-skills` → `STEP check-update button visible` →
    `STEP up-to-date badge visible` → `SMOKE-OK`。
- 冒烟在 dev home 的 `data/skills/` 留下 find-skills 安装（产品自身路径，非
  air-gap 禁区三路径）；升级 `upgrade_available` 人工场景依赖上游发版，由
  httptest 回放覆盖。

## 备注

- 判别性：升级路径的 hosted 集合镜像（含删除）与清单守卫均由 httptest 回放
  断言；上游真实发版的 `upgrade_available` 场景不依赖线上时序。
