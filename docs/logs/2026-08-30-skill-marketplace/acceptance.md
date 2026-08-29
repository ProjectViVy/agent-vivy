# 验收：用户怎么确认这次改动生效

## 前置

```text
just run            # 或 just dev（后端 + Vite 双开）
# 浏览器打开 http://127.0.0.1:3015/skills
```

页面顶部**不再有**黄色 "Demo / Local simulation" 横幅。

## 1. 已安装技能页签（真数据）

1. 列表来自 `skills/list`（`skills_root` 目录逐个 SKILL.md）。空目录时显示
   空态与提示"在市场中安装一个技能，或把含 SKILL.md 的目录放到
   skills_root 后刷新"。
2. 手工放一个技能目录（`data/skills/demo-skill/SKILL.md`，frontmatter 含
   `name: demo-skill` 与 `description`）后点"刷新"，列表出现该技能。
3. 点开技能：能看到正文（untrusted 数据）、内容哈希、附属文件按钮（点击
   加载该文件内容）；若正文含 `curl ` 之类短语，会显示琥珀色警告条
   （服务端注入扫描结果）。
4. 启停开关：切到"停用"后徽标变为"已停用"；此时让 agent 跑一回合，
   `skill` 工具目录里不再出现该技能；再切回"已启用"即恢复。若期间文件被
   其他进程改过，开关报 409 并自动重拉目录。

## 2. 市场页签（skills.sh 适配）

1. 页签"市场"只在后端广播 `skills.marketplace` capability 时出现。
2. 不输入搜索词时显示内置 featured 排行（含快照日期），条目有安装数
   （如 846.6k）与来源仓库。
3. 搜索框输入至少 2 个字符（防抖 300ms）得到 skills.sh 搜索结果；结果不足
   显示空态。
4. 点"安装"：按钮进入安装中状态；成功后自动刷新已安装列表，新技能出现在
   "已安装技能"页签，磁盘 `skills_root` 出现同名目录；再次安装同一技能时
   按钮禁用（已安装）。
5. 断网或上游故障时市场页显示错误框与"重试"按钮（RPC -32010 → 502
   语义），不影响已安装页签。
6. 安装的技能下一回合即可被 agent 使用（Eino skill 中间件每轮从磁盘重列，
   无需重启）：让 agent 调 `skill` 工具加载刚装的技能验证。

## 3. 变更请求页签（真实暂存修订）

1. 在会话里让 agent 调 `skill_manage`（例如"给 demo-skill 加一节说明"），
   审批流里会出现 diff 预览（暂存未批准状态保持即可，不必批准）。
2. `/skills` 页"变更请求 (N)"页签显示该待审修订：技能名、动作、目标路径、
   运行绑定、预览 diff、警告；状态徽标"待审"。
3. 页签为只读：批准/拒绝仍发生在会话的评审流里。

## 4. 配置

- `config.yaml` 可用 `runtime.skills_marketplace_url` 覆盖市场 base URL
  （默认 `https://skills.sh`；环境变量 `VIVY_SKILLS_MARKETPLACE_URL` 优先）。
  指向非法 URL 时启动被 config 校验拒绝。
- `python scripts/fetch_marketplace_featured.py` 可刷新内置 featured 快照并
  重写 `internal/runtime/marketplace_featured.yaml`（需重新编译生效）。
