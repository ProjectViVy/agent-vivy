# Standing Orders — All Super-Channel Slices

Sub-AGENTs must read this before starting work. Violation means stop; do not rely on review to correct it afterward.

## Authority

Contract `VIVY-CHANNEL-PACK.md` > Evolution `VIVY-CHANNEL-EVOLUTION.md` > this slice's PLAN > `docs/TODO.md` §0.2 calendar.

The calendar does not change the contract. A PLAN must not invent a second loop. If you find a contract gap, write it to `docs/TODO.md` §0.1; do not widen the seam on your own.

## Lanes

- If the root tree is dirty or an existing write lane is present: `git worktree add ../agent-vivy-<id> -b feat/channel-<id>`.
- The documentation package is on `feat/channel-super-contract`. **Implementation slices must not pile code onto that branch.**
- Parallel C4∥C6 requires two worktrees. Open the third real SDK only after C4 merges (ABI template).

## Eino (D-007)

- Only `internal/runtime` and `internal/provider` may import `github.com/cloudwego/eino*`.
- `internal/channelhost`, `sdk/plugin`, and all `plugins/<channel>` must have **zero Eino imports**.
- Host and plugins must not use `adk.NewRunner`. The only loop is `runtime.Service.Run`.
- Only the later A2A slice may let the independent `plugins/a2a` depend on `eino-ext/a2a` **models/transport**. `RegisterServerHandlers(adk.Agent)` is prohibited.
- `AgentAsTool` / DeepAgent are not part of this EPIC.

## Plugins

- Authors may import only `agent-vivy/sdk/plugin`. `import agent-vivy/internal/...` is prohibited.
- `net.Listen` / `http.ListenAndServe` are prohibited. Listen belongs to the Host.
- Do not write telego / discordgo / lark / DingTalk / botgo into a species' default `go.mod`.
- The default submitted `internal/generated/plugins/zz_register.go` must remain `return nil`.
- `pluginhost.Adapt` must not turn `seam: channel` into `tools.Tool`.

## picoclaw Reference (Required Reading for Formal Channel Work)

The five real adapters (CH-C4 / C6 / C7a / C7b / C7c) use **picoclaw's channel implementations as the most complete Go samples**. Read the corresponding package before starting, then rewrite it into `plugins/<name>/`.

- Read-only. Do not `import` picoclaw modules or write `.workspace` into `go.mod`.
- Borrow: `Start` / `Stop` / `Send`, InboundContext / SenderInfo, error classification, and the platform's token usage.
- Do not borrow: `init()` blank-importing into the gateway, allowing an empty `allow_from`, plugins creating their own `net.Listen`, or kernel types such as `TelegramSettings`.
- Paths (use whichever exists): repository `.workspace/picoclaw/pkg/channels/<name>`; local `C:\Users\Administrator\Desktop\morediva\.workspace\picoclaw\pkg\channels\<name>`.
- For Discord, **do not** port `voice.go` / pion. DingTalk uses Stream; do not regress to a webhook text bot. QQ is an official Bot, not a personal account / OneBot.

## Loop and Ledger

- Inbound traffic goes only through `Env.PublishInbound` → ChannelHost → `channel.inbound` → `Message(source=channel)` → `Service.Run`.
- Secrets go only through `token_env` / `*_env`. Values do not enter configuration, Journal, or event payloads.
- Empty `allow_from` = reject Start. `"*"` is prohibited.

## Verification and Submission

- Before delivery, run `just ci` in the root (or this worktree).
- The user-visible surface is `http://127.0.0.1:3015`, not the embedded UI at `:8787`.
- Write `docs/logs/YYYY-MM-DD-<id>/{summary,verification,acceptance}.md`.
- One theme commit per slice. Do not push unless the user explicitly says so.
- Mark the TODO line DONE and point it to the log. Check off the handoff in this slice's PLAN §10.

## Language

Code identifiers are in English. User-facing copy is in Chinese. Iteration logs are in Chinese.
