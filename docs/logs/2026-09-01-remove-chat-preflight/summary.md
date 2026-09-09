# Summary: remove chat preflight gate

## What changed

The chat-turn preflight feature was removed entirely, per maintainer
decision: the feature was never requested, and every message send surfaced an
amber "Preflight found warnings" confirmation banner before the
run started.

Removed:

- `internal/runtime/preflight.go`, `internal/runtime/preflight_test.go`
  (`Service.Preflight`, `PreflightResult`, `PreflightStatus`, `PolicyPreview`).
- RPC method `preflight/run` in `internal/rpc/control.go`: route case,
  `preflight` handler, `preflightResult` DTO, `toPreflightResult`, and the
  capability-list entry. Corresponding assertions in
  `internal/rpc/control_test.go`.
- UI preflight gate in `ui/src/components/chat/ChatView.tsx`: the submit flow
  now calls `startRun` directly; the pending-preflight state, the amber
  warning/blocked banner, and the Continue/Cancel confirmation step are gone.
- `preflight` client, `Preflight` type, and `preflight/run` entry in
  `ui/src/lib/api.ts` (+ test).
- i18n keys `chat.preflightHint`, `chat.preflightBlocked`,
  `chat.preflightWarned`, `chat.continue` in `ui/src/i18n/zh.ts` / `en.ts`;
  the empty-state hint line that advertised the preflight was dropped.

## What was NOT changed

- Runtime policy enforcement is untouched: deny/prompt decisions, plan-mode
  restrictions, and tool hooks are enforced at tool-execution time by the tool
  adapter chain (`internal/runtime/engine.go`, `internal/runtime/policy.go`,
  `internal/tools`). The preflight was a side-effect-free preview only; no
  safety gate was lost.
- Prompt safety scanning (`tools.ScanPrompt`) still runs; its findings simply
  no longer have a preflight consumer.
- Historical records (`docs/logs/…`, `docs/research/AGENT-LOOP-PORT-COMPARISON.md`)
  keep their preflight mentions — they are archives, not live contracts.

## Why not "make it a plugin"

`docs/architecture/VIVY-PLUGIN-SPEC.md` whitelists seams to
`tool` / `tool-world` / `provider` / `channel` and explicitly forbids a
`policy` seam. A send-time admission preview is a policy/loop concern, so
pluginizing it would require inventing a new seam — larger than the feature
itself.

## History

- Backend introduced in `bbcfc02` (2026-08-10, "feat: add harness preflight
  hooks") as part of the DeepSeek Harness parity track.
- UI gate introduced in `150922a` (2026-08-24, React UI migration).
