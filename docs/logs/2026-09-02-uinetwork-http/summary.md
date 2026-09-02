# UI-NETWORK-HTTP：http_request 工具面进设置文档 + RPC 段 + 设置卡

## 交付

`http_request`（只读网页抓取）的域名白名单与请求超时从纯 `config.yaml` 字段升级为
「配置默认值 + 用户工作区 settings.yaml 覆盖层」三层结构，并在设置页网络工具卡
提供编辑入口。

- **config**：`runtime.http_timeout_seconds`（默认 10；runtime 后端钳制 1–120），
  与既有 `runtime.http_allowed_hosts` 并列。
- **settings.yaml**：`http` 覆盖层（`HTTPSettings{allowed_hosts *[]string;
  timeout_seconds int}`）。nil 指针 = 沿用配置；块内 nil hosts = 沿用配置白名单；
  超时 0 = 沿用配置；显式空列表 = 拒绝全部的只读面。空覆盖层在 load 时归一化为
  nil（文档回写稳定）。
- **runtime**：`EinoHTTPBackend` 增加 `SetConfig(allowedHosts, timeoutSeconds)`
  实时生效缝隙（RWMutex 保护 allowlist/client 读取），构造函数增加 timeoutSeconds
  参数与 `httpTimeout` 钳制（≤0 → 10s，>120 → 120s）。
- **RPC**：`settings/get` 增加 `http` 段（有效值 + `config_*` 回退 +
  `overlay_set`）；`settings/update` 接受 `http{allowed_hosts, timeout_seconds}`
  （sandbox 同款替换语义：块内省略 hosts = 保留现值，显式空数组 = 拒绝全部）。
- **live-apply**：`applyLiveHTTPSettings` 在启动时与 OnSettingsChanged 时把
  settings 覆盖层合并到运行中的后端（照 `applyLiveSandboxSettings` 模式）。
- **UI**：`NetworkToolsCard` 新增 http_request 工具面区块——域名白名单文本域
  （换行/逗号分隔，`*.域名` 通配语义提示）、超时数字输入（0 = 沿用配置默认，
  >120 禁用保存）、覆盖徽标与独立保存回执；`settingsUpdateFrom` 随整文档携带
  http 段防其他分区保存时误清。

启停（tools_enabled）不在本片范围——已有工具开关覆盖层覆盖，卡片文案只提示
只读语义。

## 明确不做

- http_request 启用/停用开关（tools_enabled 既有能力，不重复建设）。
- 请求头、User-Agent 等更细粒度的抓取策略（无需求输入）。

## 相关

- TODO：UI-NETWORK-HTTP → DONE（§0.1 翻行 + §10 记录）。
- 模式先例：sandbox/compaction 覆盖层（指针字段区分「未设」与「设空」）、
  `applyLiveSandboxSettings`、`NetworkToolsCard`。
