# UI-E2E-STALE — e2e 断言与现 UI 重新对齐（含两处连带缺陷修复）

## What changed

2026-09-01 的 e2e 重跑发现 4 个 spec 共 6 处断言停留在旧版 UI。本次全部按当前
真实 UI 重新对齐，并顺手修复了重跑暴露的两个真实缺陷。

### 1. 四个 spec 的陈旧断言重同步

- `ui/e2e/runtime.spec.ts` — 附件入口从 button 变为 `label[aria-label="附件"]`
  （内嵌 file input），两处改用 `[aria-label="附件"]` 定位；设置 → 模型 tab 的
  断言从已不存在的「密钥只由运行环境管理」改为现卡片标题「已选模型」。
- `ui/e2e/welcome-wizard.spec.ts` — 模型步骤密钥提示换成现行 `welcome.secretNote`
  文案；完成卡片 deep-link 后的模型 tab 断言改为「已选模型」标题 + 「OpenAI 当前」
  供应商行（模型 tab 已重构为供应商注册表 UI，无 Provider 输入框）。
- `ui/e2e/language-setting.spec.ts` — zh/EN 语言选项收窄到 LanguagePicker 的
  `role=group`（中文名「语言」，切换后为英文组名「Language」），不再与顶栏
  模型切换器 aria-label 里的 "en" 子串撞 strict mode。
- `ui/e2e/model-refresh.spec.ts` — gpt-4o / gpt-4o-mini 断言从 `getByText` 改为
  按按钮名整名匹配（模型行是 button；顶栏当前模型 span 会让 `getByText` 的
  gpt-4o-mini 撞 strict mode）；重载后重新点选供应商行再断言。

### 2. 连带缺陷修复一：模型 tab 「新增模型」并发双写（UI）

`ModelSettingsCard.confirmAddModel` 原先以 `void saveProvider(...)` 不等待就调用
`applyModelNow`（内部又有 `settings.save`），两个 RPC 并发对同一 settings 文档
做读改写。改为先 `await` 注册表写再应用模型。

### 3. 连带缺陷修复二：向导缺失 i18n key 与文案/表单不符（UI）

向导模型步骤引用的 `welcome.provider` / `welcome.providerPlaceholder` 在
zh/en 词典中不存在，界面渲染出裸 key；同时 `welcome.modelTitle/modelBody/introBody`
描述「显示名 + API Key」注册表单，而表单实际收集 Provider 运行束 + Base URL +
默认模型（D-010：向导不收集密钥）。修正：补齐两个 key，标题回到「配置模型」，
文案与表单一致；删除 5 个无引用的死 key（displayName/apiKey/fieldsRequired 等）。

### 4. 连带缺陷修复三（内核）：settings 文档并发读写

详见 `docs/logs/2026-09-01-settings-save-race/`（独立提交）：`settings.Save` 曾用
固定 `path+".tmp"` 且无锁，e2e 全量跑时「新增模型」的并发双写造成 tmp 互相覆盖、
以及 Windows 上 rename 撞上并发读的 Access denied，后续 `Load` 失败表现为
providersError "internal error"。修复为 CreateTemp 唯一临时文件 + `fileMu` 串行化
文件 I/O 窗口，配套并发回归测试。

## Explicitly not done

- settings 读改写（Load→修改→Save）在 handler 层面仍未整体串行，跨请求的
  lost-update 语义保持 last-writer-wins；如需事务化需要 settings.Update(path, fn)
  之类的 API 演进（TODO §0.1 开行跟踪）。
- runtime.spec 的 no-provider 分支断言（`无法连接！请检查供应商配置！`）本身
  没有过期——此前失败纯粹是 model-refresh 污染全局设置所致，spec 未改动该断言。
- 文件版本 history 仍等 O1..O6 用户裁决（RB-1），不在本切片。
