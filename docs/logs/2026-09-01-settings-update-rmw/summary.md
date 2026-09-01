# SET-RMW — settings.Update 原子读改写

## What changed

问题：`internal/rpc` 的 8 个 settings 写 handler 采用跨调用 Load→modify→Save；
包级 `fileMu` 只串行化单次 Load/Save 的文件 I/O 窗口，不覆盖「读出来之后、
写回去之前」的窗口。两个 handler 交错时后写者用陈旧快照覆盖，丢一次更新
（last-writer-wins）——SET-FILERACE 修复文件损坏时已识别为遗留项。

- `internal/app/settings`：抽出无锁 `load`/`write` 内部实现（`Load`/`Save`
  保持原语义，锁窗口不变）；新增 `Update(path, fn)`——一次 fileMu 持有内
  完成 load → fn → validate → write。fn 错误原样穿透（调用方携带自己的域
  错误跨过事务）；候选文档校验失败包 `*ValidationError`，新增
  `IsValidationError` 分类。fn 内禁止再调 Load/Save/Update（死锁）。
- `internal/rpc/control.go`：新增 `updateSettingsOrError` 错误映射帮手 +
  `settingsFnError` 包装（域 `*Error` 经 errors.As 穿透）。迁移全部 8 处
  读改写：updateSettings、setActiveTools、upsertProvider、deleteProvider、
  refreshProviderModels、updateChannel、upsertMCP、deleteMCP。域内
  not-found 检查移进 fn（对新鲜文档判定，原来对快照判定）。
- `refreshProviderModels` 两段式：阶段一快照解析目标 + 上游
  `ModelLists.List` 网络调用不持锁（15s 网络不再挡住其他 settings 写）；
  阶段二 Update 内按 id 重解析，catalog 首刷克隆场景兼按
  (bundle, base_url) 匹配，防并发双克隆产生重复行（Validate 会拒）；合并
  逻辑提取为 `unionModels`（语义不变：上游序在前、本地新增保留）。

## Behavior notes

- 单用户 UI 流程的请求/响应形状不变；回显仍用持久化后的文档。
- 错误映射变化：写路径上文档 I/O 失败（读/临时文件/rename）此前被一律映射
  InvalidParams，现经分类后为 internal error；校验失败仍 InvalidParams，
  消息文本不变。

## Explicitly not done

- 跨进程互斥：fileMu 是进程内锁，settings.yaml 的单写者边界仍是「单进程」
  （与既有部署模型一致，未引入文件锁）。
- config.yaml 不在范围（只读装载，无写路径）。
- `Save` 未包 ValidationError（仅 Update 包；Save 的既有返回值语义未动，
  迁移后 control.go 已无 Save 调用方）。
- 无 fsync/durability 变更。
