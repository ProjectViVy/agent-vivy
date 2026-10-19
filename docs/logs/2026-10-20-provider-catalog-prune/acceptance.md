# Acceptance

Date: 2026-10-20

A human can confirm the prune in the running product:

1. **Provider list.** Open Settings → Providers. The catalog shows exactly two
   vendors: DeepSeek and SenseNova. Every former vendor (OpenAI, Anthropic,
   Zhipu, Moonshot, MiniMax, ...) is gone, together with its endpoints and
   models.
2. **Default selection.** A fresh config still resolves to `deepseek` /
   `deepseek-flash` (the shipped default), and DeepSeek's
   `anthropic-messages` endpoint is still selectable as its second variant.
3. **No dead row.** The `openai-responses` protocol is no longer offered by any
   vendor row. It remains visible only as a `DEFERRED-INDEFINITE` capability
   the build cannot execute.
4. **Startup.** `vivy` starts on the two-vendor data; the startup consistency
   gate passes, and a hand edit that declares a fourth adapter still fails
   closed.
5. **Generated Generation.** `internal/generated/assembly/zz_default.go` lists
   two Provider Profile identities, matching the sealed manifest.

What a human cannot confirm here: the full `just ci` candidate gate, the
browser smoke, and real-provider runs; those are PR-stage steps.
