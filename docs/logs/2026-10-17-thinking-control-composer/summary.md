# Thinking control in the composer + effective default preference

Date: 2026-10-17
Lane: thinker surface (`ui/` composer + `session/context` thinking projection)

## What changed

- `internal/provider/resolving.go`: new `ThinkingLevelSurface(info)`. One rule
  for "which levels may a face offer", shared by `model/thinking/levels` and
  `session/context.thinking_levels` so the settings card and the composer
  cannot disagree.
- `internal/rpc/control.go`: `thinkingLevels` now calls that helper (identical
  behavior, no second copy of the rule).
- `internal/runtime/compaction_service.go`: `ContextStatusResult` gains
  `thinking_levels` (declared surface, `[]` when unsupported, never `null`) and
  `thinking_default` (the model's own default level), filled from the same
  `GetModelInfo` the run path uses.
- `ui/src/lib/api.ts`: host `SessionContext` carries the two additive fields.
- `ui/src/components/chat/ChatInput.tsx`: the thinking control is now always
  rendered next to 智能. It is greyed and disabled when the active model
  declares no extended-thinking support (tooltip `当前模型未声明扩展思考支持`),
  and in that state it is a plain disabled button — no dropdown is attached, so
  a real pointer click cannot open the menu. When the model declares support the
  same button gets a dropdown whose entries are exactly the declared levels plus
  `跟随默认 / 自动 / 开启 / 关闭`, and its color encodes the level (grey →
  `primary/60` → `primary` → filled icon). `thinkingMode` starts as *unset*: the
  turn then omits the `thinking` field so the kernel applies the `settings.yaml`
  default; an explicit pick still overrides for that turn.
- `ui/src/lib/store.ts`: `applyAuthoritativeSettings` re-reads the session
  context when the active model triple (`provider`/`default_model`/`base_url`)
  changes, so the gate is not stale after a model switch.
- `ui/src/i18n/{zh,en}.ts`: `chatInput.thinkingModeDefault` and
  `chatInput.thinkingUnsupported`; both are registered as Web face-specific keys
  in `scripts/i18n-cross-face-contract.json`, which `just ci` checks.

## Why (chain findings)

The reported symptom — 思考级别 card says *当前模型未声明扩展思考支持* — is the
backend and the UI agreeing on the same (insufficient) metadata, not a field
mix-up: `Service.GetModelInfo` -> `catalog.ResolveModelInfo` feeds
`session/context.thinking_supported`, `model/thinking` and the run path
(`resolvingChatModel.thinkingOptions`) alike. On this host the stored selection
is `provider: openai-completions`, `default_model: DeepSeek-V4.1-Flash`,
`base_url: https://api.wxyungu.com/v1`, and the embedded vendor data declares
only `deepseek-flash|v4-pro|v4-flash|chat|coder|reasoner`. An undeclared model
id resolves to the vendor's default endpoint with empty metadata, so
`SupportsThinking` is false, `decideThinking` returns an empty shape, and no
reasoning field is ever sent.

Four defects were found and fixed in this cut:

1. UI hid the control instead of greying it (contradicted the owner's
   requirement; was pinned by `ui/e2e/thinking-gate.spec.ts`).
2. The persisted default was inert: `ChatInput` always sent `thinking: 'auto'`,
   and `thinkingFor` takes any non-empty per-turn value verbatim, so
   `settings.yaml`'s default could never apply — directly contradicting the
   settings card's own description.
3. The composer menu hardcoded the seven levels instead of the model's declared
   surface.
4. `sessionContext` was not refreshed after a model switch, so
   `thinking_supported` stayed stale until the next run terminal or session
   switch.

## Explicitly not done

- No capability declaration path for third-party/proxy endpoints, and no data
  addition to `internal/provider/data/vendors.yaml`. A proxy or aggregator model
  id that the embedded data does not declare therefore still reads as
  unsupported — it now renders as an honest grey button instead of vanishing.
  Recorded in `docs/TODO.md` §0.1 as `THINK-CAP-UNDECLARED-MODEL`.
- `ThinkingSettingsCard` layout is unchanged (it already names the reason and
  remains the place to set the persisted default).
- `FaceSessionContext` in `sdk/ui/src/module.ts` is unchanged: it is an existing
  partial projection of `session/context` (it already omits
  `token_counts_estimated`, `image_support_known`, `image_supported`), and
  adding the two fields flipped the structural `FaceClientStore` compatibility
  assertions in `ui/src/lib/ui-sdk-face-compat.test.ts` /
  `ui/src/lib/ui-build-provenance.test.ts` to `false`.
- TUI faces keep their own gate (`sdk/tui/live/controller.go`); this cut is the
  web composer.

## Contract note

A turn started from the composer no longer carries `thinking: "auto"` when the
user never picked a level. This is the intended behaviour of the existing
`thinkingFor` merge (explicit per-turn value wins, otherwise the persisted
default). Anyone who stored `thinking: off`/`high` in `settings.yaml` and
relied on the web composer overriding it with `auto` will now get the stored
value; that is the documented semantics of the 思考级别 card.
