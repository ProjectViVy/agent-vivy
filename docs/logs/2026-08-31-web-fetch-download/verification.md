# Verification — web_fetch + download(2026-08-31)

环境:worktree `../agent-vivy-web-fetch`,分支 `feat/web-fetch-download`
(根树有并行 lane 的未提交改动,按 parallel-worktree-isolation 隔离)。

## Commands and results

| Command | Result |
|---|---|
| `go get html-to-markdown@v1.6.0 goquery@v1.12.0` + 插件模块 `go mod tidy` | OK(go.mod/go.sum 落盘;discord/qq 随根依赖升级 tidy) |
| `go build ./internal/...` | OK |
| `go vet ./...` | OK(0 findings) |
| `go test ./internal/runtime/ ./internal/tools/ ./internal/config/` | OK(新增用例全过) |
| `go test ./...` | OK(全部包;首次失败为插件 go.mod 未 tidy,tidy 后通过) |
| `just ci` | **EXIT=0**(fmt-check / vet / test / headless-compile / ui-ci 全绿;首次 ui 冒烟 503 是新 worktree 无 ui/dist 所致,`pnpm build` 后复跑通过) |

## New tests(确定性,httptest 本地,零外网)

- `internal/runtime/web_fetch_test.go` — 三格式转换与剔噪、JSON 美化、
  非 2xx 有界结果、截断标记、私网目标默认拒(无测试缝)、凭证 query 拒、
  二进制拒、带凭证的重定向拒、非法 URL 拒、timeout clamp 表驱动。
- `internal/runtime/download_test.go` — 落盘 + SHA256 + 建父目录、覆盖 +
  precondition 防陈旧、超上限拒且不落目标、凭证 query 拒、无测试缝时私网拒、
  非 2xx 报错、提案字段/覆盖警告;**受限沙箱(workspace-write)+ 新嵌套目录
  回归测试**(钉住「校验须在 MkdirAll 之后」的顺序)。
- `internal/tools/tools_test.go` — web_fetch/download Spec 只读性约定、
  args 校验(*ArgError)、download `ProposalProvider` 通用提案回退。
- `internal/config/config_test.go` — 默认 enabled 清单含 `web_fetch`/`download`。

## Real-path smoke

1. **真实公网 fetch**(生产构造器,无测试缝,真实 DNS/拨号):
   `web_fetch https://example.com` → status=200、format=markdown、
   内容 `# Example Domain` + `[Learn more](https://iana.org/domains/example)`;
   同一后端抓 `http://127.0.0.1:8787/` 被拒
   (`public fetch: target address is private or local`)。
2. **真实公网 download**(生产默认 workspace-write 沙箱模式):
   `download https://example.com → smoke/example.html` → 559 字节落盘、
   SHA256、content_type 透传;同目标二次提案正确警告
   `overwrites existing file`。
3. **真实启动路径**:worktree 后端 `VIVY_ADDR=127.0.0.1:8799 go run ./cmd/vivy`
   启动成功、`/rpc/bootstrap` 200——证明新 `Tools.Enabled` 名称
   (`web_fetch`/`download`)通过 `Registry.Resolve`,配置→注册→装配全链真实可用。
   (8787 上是并行 lane 的常驻后端,未触碰。)
4. 浏览器 UI:`http://127.0.0.1:3015` 由既有 dev pair 正常服务(本分支
   **零 UI 改动**,tools 为模型面,无新 UI 表面)。

## Known limitations(如实记录)

- **模型驱动的端到端会话冒烟未做**:本机 shell 无 provider API key
  (config 只存 env_key 名,密钥不入库),无法让模型在会话里真实调用两工具;
  已用「真实公网 + 生产沙箱 + 真实启动」的后端级冒烟替代,执行路径
  (dialer→HTTP→转换/落盘)已覆盖,唯一未覆盖的是模型侧工具选择。
- smoke 用的临时 harness 与临时 config 已删除,未入库。
