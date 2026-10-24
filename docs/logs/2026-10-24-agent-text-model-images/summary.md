# Sub-agent 400 (2013): text-only models never receive image parts

## Problem

A delegated child run failed with:

```text
child run failed ... status code: 400 ... message: invalid params, 400 (2013)
node path: [node_1, ChatModel]
```

The parent survived (fault isolation from
`2026-10-24-agent-tool-fault-isolation`), but the sub-agent could not finish.
The operator's route was DeepSeek `deepseek-chat`, a text-only model.

## Root cause

`read_file` attaches png/jpeg/gif/webp content as a multimodal parts envelope
unconditionally, and `enhancedToolAdapter` lifts those parts into the model
request. Nothing consulted the model's declared image capability:

- `deepseek-chat` is declared in the embedded vendor data with no metadata, so
  `SupportsImages` was the zero value and `ContextWindow` was `0`.
- Both attachment gates (`internal/rpc/control.go`, `internal/app/app.go`)
  required `ContextWindow > 0` before refusing images, so an undeclared model
  failed open — by design for custom gateways, but here it let media reach a
  text-only provider.
- `provider.Model.SupportsImages` was a `bool`, so "absent" (unknown) and
  "explicitly false" were indistinguishable. The data schema already promised
  absent-means-unknown; the Go projection could not honor it.

DeepSeek answers an image part on a text-only model with HTTP 400
`invalid params (2013)`, which also poisons every later request in that run.
The deterministic E2E reproduces the owner's exact log line before the fix.

## Fix

1. **Tri-state image capability.** `provider.Model.SupportsImages` is now
   `*bool` with `Model.Images() (supports, known bool)`; `domain.ModelInfo`
   carries `ImagesKnown`. `ResolveModelInfo`, the OpenAI ref and the Claude ref
   project both values.
2. **Declared data.** DeepSeek `deepseek-chat`, `deepseek-coder`,
   `deepseek-reasoner`, `deepseek-v4-pro`, and the Anthropic-compatible
   `deepseek-chat` entry now declare `supports_images: false`. Models with no
   declaration stay unknown.
3. **Gates use known-ness.** The RPC attachment gate rejects and the channel
   gate drops images when `ImagesKnown && !SupportsImages`; unknown routes keep
   the status-quo allow. `ContextStatusResult.ImageSupportKnown` now reports
   `ImagesKnown` instead of a context-window proxy.
4. **Run-scoped tool capability.** `driveWithExecution` binds
   `tools.WithImagesSupported` from `GetModelInfo` when the capability is
   known; the binding covers parent and child runs because both enter through
   the same function. An unbound context keeps the old allow.
5. **Honest degradation.** `read_file` returns a text result naming the file and
   the missing capability instead of an image part when the active model does
   not accept images. The tool description says so.

## Not done

- No UI or i18n change.
- MCP tools cannot currently produce the parts envelope, so no adapter-level
  media filter was added.
- Existing media parts already persisted in a live run are not rewritten; the
  fix prevents producing them for a declared text-only route.

## Related

- `docs/logs/2026-10-24-agent-tool-fault-isolation/` — child failure becomes a
  model-visible tool result instead of a parent terminal.
- `docs/logs/2026-10-24-subagent-turn-budget/` — the child inherits the
  configured turn cap.
