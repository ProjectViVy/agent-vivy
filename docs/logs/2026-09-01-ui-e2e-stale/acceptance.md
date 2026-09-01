# 验收：人视角怎么确认

## 套件层面

- 在仓库根跑 `just ui-e2e`，结尾应看到
  `1 skipped / 9 passed`（无真实供应商环境跳过 runtime 无供应商分支之外的 provider 用例），退出码 0。
- `ui/test-results/` 不再出现 `welcome-wizard` 的 raw i18n key（如 `welcome.provider`）截图；
  向导「配置模型」步骤显示的是中文文案，不再是 `welcome.provider` 这样的键名。

## 向导文案（首次使用路径）

- 清空 `data/dev-home/settings.yaml` 后打开 `http://127.0.0.1:3015`：
  向导第二步标题为「配置模型」，正文说明 Provider 运行束（openai / anthropic）、
  Base URL 与默认模型，并明示「API Key 由运行环境注入，向导不收集」。
- 中英文切换后同一表单没有缺失键渲染（原缺陷：`welcome.provider` 直接显示为字面文本）。

## 设置 → 模型：新增模型不再间歇报 internal error

- 自定义供应商行 → 新增 → 输入模型 id → 回车/点击新增：
  注册表写入完成后才应用该模型，成功后新模型出现在列表并成为当前模型。
- 重复快速操作（连点/回车）不再出现红色错误条「internal error」，
  也不再出现注册表缺少刚新增模型的行。

## 内核：settings 文档并发损坏（与 UI 无关的独立缺陷）

- 并发保存场景（测试 `TestSaveConcurrentWritersKeepDocumentValid`，含 `-race -count=3`）通过；
  Windows 上「rename 覆盖被并发读句柄」的 access denied 不再出现。
- 回归方法：`go test ./internal/app/settings/ -race -count=3` 退出码 0；
  修复前可复现 `settings: commit: rename ... Access is denied`。
