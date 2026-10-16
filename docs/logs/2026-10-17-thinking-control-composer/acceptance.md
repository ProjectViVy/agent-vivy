# Acceptance

Human view, in the split dev pair (`just dev`, `http://127.0.0.1:3015`) or the
embedded UI.

1. **Unsupported model (this host's current selection: DeepSeek-V4.1-Flash over
   a third-party `base_url`).** The composer toolbar shows a 思考模式 button
   immediately beside 智能. It is grey, disabled, and its tooltip reads
   `当前模型未声明扩展思考支持`. Clicking it opens nothing. It no longer
   disappears — the capability gap is visible instead of implicit.
2. **Declared thinking model** (e.g. select `deepseek-reasoner`, or any
   Anthropic-family model). The button is colored: `跟随默认 / 自动` stay grey,
   `minimal..medium` use a lighter primary, `high` uses full primary, and
   `xhigh/max/开启` use a filled icon. Opening it lists only the levels that
   model declares (a model with declared `low,high` shows exactly
   `跟随默认 / 自动 / low / high / 开启 / 关闭`).
3. **The saved default now works.** Set Settings → 通用 → 思考级别 to `high`,
   return to chat, do not touch the composer, and send a message: the run uses
   `high` (before this cut the composer forced `auto` on every turn). Picking a
   level in the composer overrides the default for that turn.
4. **No stale gate.** Change the model in Settings → 模型 (or the top-bar model
   switcher) and return to the chat page: the button's enabled/grey state
   matches the new model immediately, without sending a message first.

Regression checks: 附件, 执行模式, 审批中心 and the context ring in the same
toolbar behave exactly as before; a queued/dequeued turn still restores the
level it captured when it was queued.
