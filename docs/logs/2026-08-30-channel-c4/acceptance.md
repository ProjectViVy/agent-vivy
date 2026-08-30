# CH-C4 acceptance — 人怎么确认它生效

1. **默认身体干净**：`git diff -- go.mod go.sum` 为空；`go list -m all`
   里没有 `github.com/mymmrac/telego` → 日常 `vivy.exe` / `just run`
   没有任何 Telegram 依赖。
2. **候选身体有耳朵**：`go run ./sdk pack --with telegram --out dist/`
   出候选 EXE；`go run ./sdk inspect-artifact dist/<gen>/` 的
   recipe.plugins 列出 `telegram` → 这一代链接了 telego。
3. **真机收发**（可选冒烟，需真 bot token）：
   - 准备：`export TELEGRAM_BOT_TOKEN=<token>`；config 里写：
     ```yaml
     channels:
       telegram:
         enabled: true
         allow_from: ["telegram:<你的数字用户id>"]
         token_env: TELEGRAM_BOT_TOKEN
         settings:
           token_env: TELEGRAM_BOT_TOKEN
     ```
   - 用候选 EXE 启动，私聊机器人发「你好」→ Vivy 回文本；Journal 的
     Message 行 `source=channel, channel=telegram`。
   - 换一个未列入 allow_from 的账号私聊 → 机器人无任何反应，gateway 日志
     出现 `dropping inbound from sender outside allow_from`。
   - 把机器人拉进群里说话 → 无反应（本刀只收私聊）。
   - 编辑已发出的消息 → 不触发新回合（只收新 message）。
4. **Secret 钉定**：把 settings 里的 token_env 改成与信封
   `token_env` 不一致的名字再启动 → 日志报通道启动失败（Secret
   fail-closed），通道不出现在已启动列表。
5. **regression**：`just ci` 全绿；空 allow_from 的通道仍拒绝启动
   （C3 TCK 行为未回归）。
