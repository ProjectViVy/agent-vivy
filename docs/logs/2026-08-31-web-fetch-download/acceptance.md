# Acceptance — web_fetch + download(2026-08-31)

## 人类如何确认它生效

前提:`config.yaml` 不需要任何新配置(两工具默认启用)。

1. **问 Vivy 抓一个网页**(会话里):「用 web_fetch 抓 https://example.com」
   → 回复应包含 "Example Domain" 的正文(markdown 格式),而不是报
   "host not allowlisted"。这与 `http_request` 的分界:
   `http_request` 只能抓 `http_allowed_hosts` 白名单内主机,`web_fetch`
   可以抓任意公网页面。
2. **格式控制**:「用 web_fetch 抓 xxx 页面,format=text」→ 返回折叠空白
   的纯文本;`format=html` → 返回 body 内 HTML。
3. **安全边界肉眼可见**:
   - 「web_fetch 抓 http://localhost:8787/」→ 拒绝(private or local),
     与 `http_request`(白名单内允许 localhost)行为不同。
   - 带 `?token=...` 的 URL → 拒绝。
   - 二进制直链(如 zip)→ 提示改用 download。
4. **download 走审批**:「下载 https://example.com 存为 example.html」
   → 出现熟悉的审批卡片(含 URL、落盘路径、"writes remote content into
   the workspace" 风险提示);批准后文件出现在该 run 的 workspace;
   再下同一目标时卡片多一条 "overwrites existing file"。拒绝则不落盘。
5. **tool_search 可发现**:新会话问「我有哪些网络工具」→ tool_search
   索引里能看到 web_fetch / download。

## 与旧行为的差异(用户可感知)

- 以前:抓任意公网网页做不到(`http_request` 被白名单挡住),
  「读网页进上下文」只能靠 network_search 的摘要。
- 现在:一句话抓任意公网页面进上下文(markdown 优先),文件可审批落盘。
- `http_request` 原有能力与安全语义零变化,已有依赖不受影响。

## 不验收什么

- agentic_fetch / sourcegraph 不在本期(见 TODO §0.1 WEB-1)。
- 模型侧自动选择工具的准确率(取决于模型,工具描述已含
  "For reading content into the conversation use web_fetch" 互斥提示)。
