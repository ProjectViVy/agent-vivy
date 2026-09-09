# CH-C4 acceptance — how to confirm it works

1. **Default body is clean**: `git diff -- go.mod go.sum` is empty; no
   `github.com/mymmrac/telego` appears in `go list -m all` → daily
   `vivy.exe` / `just run` has no Telegram dependency.
2. **The candidate body has an ear**: `go run ./sdk pack --with telegram --out dist/`
   produces a candidate EXE; `go run ./sdk inspect-artifact dist/<gen>/`'s
   recipe.plugins lists `telegram` → this generation links telego.
3. **Real-device send/receive** (optional smoke; requires a real bot token):
   - Prepare: `export TELEGRAM_BOT_TOKEN=<token>`; add this to the config:
     ```yaml
     channels:
       telegram:
         enabled: true
         allow_from: ["telegram:<your numeric user ID>"]
         token_env: TELEGRAM_BOT_TOKEN
         settings:
           token_env: TELEGRAM_BOT_TOKEN
     ```
   - Start the candidate EXE, send the bot "hello" in a direct message →
     Vivy replies with text; the Journal Message row has
     `source=channel, channel=telegram`.
   - Send a direct message from an account not in allow_from → the bot does nothing,
     and the gateway log contains `dropping inbound from sender outside allow_from`.
   - Add the bot to a group and speak → no response (this slice accepts direct messages
     only).
   - Edit a sent message → no new turn is triggered (only new messages are accepted).
4. **Secret pinning**: change settings' token_env to a name that does not match the
   envelope's `token_env`, then start → the log reports channel startup failure
   (Secret fail-closed), and the channel does not appear in the started list.
5. **Regression**: `just ci` is all green; a channel with empty allow_from still
   refuses to start (C3 TCK behavior has not regressed).
