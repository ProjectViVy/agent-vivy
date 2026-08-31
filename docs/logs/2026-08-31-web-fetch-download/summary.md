# 2026-08-31 — web_fetch + download 工具(仿 CRUSH 对齐)

## What changed

Vivy 内核新增两个网络工具,补齐与 CRUSH `fetch` 之间的能力缝,同时保留 Vivy 的
安全模型(CRUSH 只有审批,没有 SSRF 防护):

- **`web_fetch`(只读、免配置)** — `internal/tools/web_fetch.go` +
  `internal/runtime/web_fetch.go`(`EinoWebFetchBackend`)。GET 抓取任意公网
  URL,输出 `markdown | text | html` 三种格式(默认 markdown,CRUSH 对齐)。
  Content-Type 路由:text/html → goquery 剔噪(script/style/nav/header/footer/
  aside/noscript/iframe/svg/template)→ html-to-markdown 转换;JSON → 缩进美化;
  text/* 透传;二进制拒收并提示改用 download。非 2xx 按 DSH 惯例返回有界结果
  而非报错(错误页常含可读信息)。响应上限复用 `runtime.http_max_response_bytes`
  (默认 1MB),超限截断并加 `[Content truncated to N bytes]` 标记(CRUSH 式,
  不硬报错)。
- **`download`(effectful、审批)** — `internal/tools/download.go` +
  `internal/runtime/download.go`(`EinoDownloadBackend`)。把 URL 流式落盘到
  当前 run workspace(CRUSH 对齐:二进制安全、可覆盖、自动建父目录);走既有
  D-012 审批闸(`ProposalProvider.PrepareProposal` → HITL interrupt →
  precondition hash 防陈旧),硬上限 100MB,SHA256 返回。
- **安全闸(两工具共享,CRUSH 没有的部分全部保留)**:
  - `publicOnlyDialContext`:DNS 解析后逐 IP 拒 loopback/私网/link-local,
    **无 http_request 的 localhost 豁免**——没有白名单兜底,私网目标一律
    dial 时拒绝(防 DNS rebinding)。
  - D-021 `sandbox.CheckNetwork` 接入不变。
  - 绝对 HTTP(S)、URL 无内嵌凭证、凭证形 query 参数拒绝;重定向逐跳复检,
    上限 5 跳。
  - 结果恒 `Untrusted: true`,走既有 `[UNTRUSTED TOOL OUTPUT]` normalizer。
- **注册**:`internal/tools/tools.go` 新增 `BuiltinWithWeb`(薄壳链新环节,
  `BuiltinWithCommands` 语义不变),`baseToolsForSearch` 同步索引;`Default()`
  的 `Tools.Enabled` 追加两工具;`config.example.yaml` 注明安全边界。
  **零新增配置键**(免配置目标)。
- **依赖**:新增 `JohannesKaufmann/html-to-markdown v1.6.0`(MIT)、
  `PuerkitoBio/goquery v1.12.0`(BSD-3)及间接 `x/net v0.52.0`/
  `cascadia v1.3.3`;版本对齐 CRUSH 的 go.mod。插件模块(discord/qq)因根
  go.mod 升级做了一次 `go mod tidy`。

## License 红线

CRUSH 是 FSL-1.1-MIT:本迭代只做**行为对齐**(参数面、格式枚举、截断标记、
超时 clamp、download 语义),全部代码自写,未拷贝任何 CRUSH 源码。

## What was explicitly not done

- `agentic_fetch` / `sourcegraph`(CRUSH 对齐余项,已记 TODO §0.1 **WEB-1**,
  研究文档归 VC-4 可选)。
- CRUSH 的 `web_search` 子代理变体(Vivy 已有 `network_search`,不重复)。
- `http_request` 语义零变更——白名单工具原样保留,两者分工不变。
- 新配置键与 UI 设置面(免配置即目标;download 审批走既有通用 HITL UI)。

## Findings filed during the iteration

- **WEB-2**(TODO §0.1,OPEN):既有 `WriteFile` 的 sandbox 校验先于
  `MkdirAll`,受限模式写全新嵌套目录会误拒;本迭代 download 已按
  「resolve → MkdirAll → Validate」自修并带回归测试,WriteFile 留待修复。
