# W02: Bind the adapter, settings, accounting and recovery, and cut over serving

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Preserve the user's execution choice; use one active write lane and fresh independent reviewers for final acceptance.

**Goal:** Serve real warming from `plugins/exp/warming` through a minimal Eino adapter: bind the typed factory slot to a Host that owns the run lifecycle, prefix, paid maintenance request, accounting evidence and recovery behavior; expose settings and the selected/unselected capability distinction; and delete the core policy body in the same change that starts serving from the package.

**Architecture:** `internal/runtime/extensions/warming` converts the SDK factory into the root-runtime factory and translates policy decisions into the existing engine model call, `withoutModelCallObserver` maintenance context, timeout, in-flight guard and `cache.warmed` diagnostic. The Host stays in `internal/runtime` (the only code allowed to touch Eino) and owns the observed input, the reusable prefix, budget/cancellation, and the Journal publisher. One `Service.Run`/Eino/Journal path, one warmer per drive/resume leg, no second scheduler, runner, queue or receipt framework.

**Tech Stack:** Go 1.26.4, pinned Eino v0.9.13 inside runtime/provider only, existing SQLite/Postgres; TypeScript `Settings` view type only (no new control).

**Spec:** [Complete deletion design §11.1–§11.5, C11](../2026-10-19-complete-deletion-plan.md). Epic index: [index.md](index.md). Predecessors: [W00](W00-seam-binding.md), [W01](W01-policy-extraction.md), base-cut [D05](../issue-91/D05.md).

**Story:** W02 · **Epic:** E-WARMING · **Requirements:** R2, R3, R5, R6.

**Immediate predecessors:** W00, W01.

**Readiness gate:** W00 and W01 accepted; the package is selectable and its policy matches Baseline A; the core behavior is still the only serving path and the repository CI is green at that commit. Current request authorizes planning only.

## Global Constraints

- Single serving implementation: this Story's change starts serving from the package **and** deletes the core policy body; no feature flag, fallback, alias or "legacy warmer" remains.
- One maintenance call per eligible settlement, guarded by one in-flight flag owned by the warmer; no timer, background service, cross-Run prefix cache or second scheduler is added.
- Maintenance traffic stays on the raw engine chat model with the model-call observer stripped (`withoutModelCallObserver`), exactly as Baseline A; it never enters context, messages, `model.request`, `model.usage` or `model.call.finished` (decision A1).
- The warm request is the same real paid provider call: same model, prompt text, 16 max output tokens, 60 s outer timeout, tools and system prefix; response discarded except reported tokens.
- Configured-off is compiled-in but inert; unselected is `NOT_COMPILED`. These are distinct states and both must be tested.
- The Host may not grant tool, approval, Journal-write, policy or model-routing authority to the adapter; the adapter may not synthesize a prefix, model input or result.
- No Vivy-owned hash/prefix digest; no production Journal access; no commit/push by the implementing agent.
- Docs/contracts touched only where this Story makes them true.

## Review Focus

- Cutover atomicity: after the change, `rg` finds no core eligibility/gate policy and no second warmer; a reviewer must reject a "keep the old one as fallback" diff.
- Prefix equivalence: the Host derives leading system messages and exactly bound tools from the observed invocation; the warm request on the normal chat path must be byte-identical in messages/tools to Baseline A (A8 test).
- Failure semantics: run-cancelled warm is a silent drop; provider failure records `failed` with the error type; an append failure of the diagnostic keeps Baseline A's existing terminal behavior (the helper calls `emitTerminal`; the caller still ignores the boolean).
- Accounting honesty: only provider-reported tokens are recorded; no estimate is billed. `call_id` in `cache.warmed` is the settled foreground call's CallID (A5), never the maintenance call's.
- Route/eligibility: summary-route calls never warm; child calls do; only successful calls with usage settle.
- Idle lifecycle: the timer is re-armed by each settlement, one timer per warmer, canceled with the run; no goroutine leak.
- Settings: config parse/validate covers invalid mode, negative floor and both `0` and omitted floor; a selected+off build still constructs the package; an unselected build has no warming config read and answers `NOT_COMPILED`.

## Baseline A mapping (202aac38, 2026-10-07)

| Concern | Baseline A path / symbol | W02 target |
| --- | --- | --- |
| Service settings inputs | `ServiceDeps.CacheWarmingMode`, `.CacheWarmingMinSavingsUSD` (`service.go:233-241`); wired from `cfg.Runtime.CacheWarming*` (`app.go:870-871`) | keep the fields; Host consumes them; add `ServiceDeps.WarmingFactory runtime.WarmingFactory` |
| warmer construction | `Service.newRunCacheWarmer` (`cachewarmer.go:78-103`), called for the initial leg (`service.go:2946-2949`) and each resume leg (`service.go:4641-4644`); returns nil for `off`/not warmable/missing lifetime | `Service.newWarmingSettlement(...)` returning the Host; same two call sites; nil only when unselected (policy decides off/not-warmable, same observable no-op/no-event behavior) |
| observer binding | `runModelCallObserver.warmer *runCacheWarmer`, settlement at `service.go:3195-3199` only when `result.Err == nil && usage != nil`; summary observers never carry it (`service.go:3009-3013`) | observer holds the `ModelSettlement` interface; identical condition and route filter |
| observed input | `modelCallInput{Mode, Messages, Tools}` at `Begin`; not retained to `End` (`model_stream_observer.go:61-126`, `service.go:3046-3088`) | retain the call's `modelCallInput` in `observedModelCall`; End passes `ObservedModelInput` to the settlement |
| prefix | engine static `instruction` + selected tool infos resolved against `eng.toolInfos` (`cachewarmer.go:92-99, 187-190`; `engine.go:138-144`) | Host `ReusablePrefix` from the observed invocation (leading system messages + exactly bound tools + estimated tokens); equivalence test vs Baseline A |
| paid request | `eng.chatModel.Generate` with `model.WithMaxTokens(16)`, `model.WithTools`, `[System, User(prompt)]`, `withoutModelCallObserver`, `context.WithTimeout(..., 60s)` (`cachewarmer.go:170-191`) | Host `GenerateMaintenance`; same call shape; cached prompt tokens from the response, cache-write from `einoclaude.GetCacheCreationInputTokens` |
| diagnostic | `cache.warmed` payload `{call_id (declared, never set), provider, model, mode, status, reason, prompt_tokens, cache_write_tokens}` (`payloads.go:191-203`; producer `cachewarmer.go:213-228`) | same payload, `call_id` populated (A5); mode `streaming`/`idle`, status `warmed`/`skipped`/`failed`, same reasons |
| lifecycle | one `inflight` bool, `time.Timer` for idle at `lifetime*0.8`, cancel with run, silent drop | same, using `loopexp.WarmingIdleHeadroom` for the delay; no new scheduler |
| accounting decision | warm is unobserved/unbudgeted; tokens only in `cache.warmed` (A1) | unchanged; `MaintenanceResult` carries reported tokens; no ledger mutation |
| recovery decision | no shutdown wait; detached bounded diagnostic write (A2) | unchanged; document the delta from §11.5 |
| settings UI | none (`ui/src` has zero warming references); no `settings` section; `config.example.yaml` has no key | selected-only `settings/get`/`settings/update` section; `api.ts` view type only; no control (A4) |
| config validation | `config.go:902-909` off/streaming/idle + non-negative floor; defaults at 693-694; no tests | keep, add parse/validate tests and example-config entries |
| capability / NOT_COMPILED | `sdk/generation/manifest.go:95`; `NetworkStates` projection in the generated Manifest | warming capability entry from W00; `NOT_COMPILED` for unselected warming-only settings requests |

## Files and ownership

- Create: `internal/runtime/extensions/warming/adapter.go`, `internal/runtime/extensions/warming/adapter_test.go`.
- Modify: `internal/runtime/extensions.go` (finish the warming factory/Host/DTO slot if D05 left any part; no duplicate definitions), `internal/runtime/cachewarmer.go` (Host + lifecycle; delete policy), `internal/runtime/cachewarmer_test.go` (real-path tests only), `internal/runtime/service.go`, `internal/runtime/model_stream_observer.go`, `internal/runtime/payloads.go` (only if a doc comment changes; payload shape unchanged), `internal/app/app.go` (wire `WarmingFactory` from `assembly_loop_extensions.go`).
- Modify: `internal/config/config.go` (comments/defaults only if needed), `internal/config/config_test.go` (new warming tests), `config.example.yaml`.
- Modify: `internal/rpc/control.go` (+ `internal/rpc/control_test.go`) for the selected-only settings section and `NOT_COMPILED`; `ui/src/lib/api.ts` for the optional `Settings.warming` view type.
- Do not modify `plugins/exp/warming/policy*.go` except to fix a policy bug proven by a Host test (that would return to W01's contract).
- Do not create a UI card, presentation-host slot, schema file or second event producer.

## Contracts

```go
// internal/runtime/extensions/warming/adapter.go — inside the Eino runtime package tree.
func Factory(newPolicy loopexp.WarmingFactory) runtime.WarmingFactory {
    return func(host runtime.MaintenanceHost) runtime.ModelSettlement { ... }
}
// settlement.Settled: build policy input from Host, call Decide; nil request → optional skipped
// diagnostic (never for mode off); else arm the existing mode-specific schedule and, when it
// fires, call GenerateMaintenance then Diagnostic. No I/O, Journal or model access here.
```

`ObservedModelInput` wraps `modelCallInput` in an unexported field; only the Host constructs it. `ReusablePrefix` privately holds the Run/call-bound leading system messages, exactly bound tools and estimated prefix tokens; adapters cannot synthesize it. `MaintenanceResult` = `{CallID, Status, Reason, PromptTokens, CompletionTokens, CachedTokens, CacheWriteTokens}`; `MaintenanceDiagnostic` = the reported status/reason/tokens plus the settled CallID and mode.

Settings projection (selected builds only):

```json
"warming": {"mode": "streaming|idle|off", "min_savings_usd": 0.05, "available": true, "inactive_reason": ""}
```

Unselected builds omit the section and reject a warming-only `settings/update` with the existing `not_compiled` vocabulary; no config value is read.

## Tasks

- [ ] 1. Read `service.go:2946-2949, 2991-3013, 3195-3199, 4637-4644`, `cachewarmer.go` (all), `model_stream_observer.go:59-126, 163-200` and `payloads.go:191-203` before editing; record the exact symbols this Story moves.
- [ ] 2. Extend `observedModelCall` to retain the call's `modelCallInput`; pass it to settlement in `End`. Keep Begin/Chunk/End semantics and summary-route exclusion unchanged.
- [ ] 3. Implement the Host in `internal/runtime/cachewarmer.go`: construct from `runCtx`, mapper, session, engine, selected names and `ServiceDeps.WarmingFactory`; **nil only when the factory is nil (unselected)**. Do not pre-filter construction by mode or model capability: Baseline A's constructor gate (`off`/not warmable/missing lifetime → nil, no event) becomes a policy decision with the same observable result — no maintenance request and no `cache.warmed`; only the gate skips (`below_min_savings`, `below_token_floor`) emit the `skipped` diagnostic. Implement `ReusablePrefix`, `PolicyInput` (normalize empty mode to `streaming` here, not in the policy), `GenerateMaintenance`, `Diagnostic`, plus the existing streaming/idle scheduling, in-flight guard and cancellation. Keep the warm request, observer stripping, timeout, tool binding and error/silent-drop classification exactly as Baseline A.
- [ ] 4. Implement `internal/runtime/extensions/warming/adapter.go` as the pure translator above; it must not import `internal/runtime/extensions/warming`'s siblings, the EXP package, or any Eino component beyond what the root runtime contract already exposes (adapter imports root runtime + `loopexp` only).
- [ ] 5. Replace `runModelCallObserver.warmer *runCacheWarmer` with the `ModelSettlement` interface; keep the `result.Err == nil && usage != nil` condition and the summary exclusion. Update both binding sites to `s.newWarmingSettlement`.
- [ ] 6. Cut over in the same commit: delete `runCacheWarmer.gate`, `settled` policy branching, `armIdle`'s policy constant, and the moved constants from core; any remaining helper must be Host/lifecycle only. `rg -n "below_min_savings|below_token_floor" internal/runtime` must match only the Host's mapping of the policy reason to the diagnostic, never a local decision.
- [ ] 7. Move runtime tests: keep/rewrite `TestCacheWarmStreamingFiresAfterSettle`, `TestCacheWarmIdleArmsBeforeLifetime`, `TestCacheWarmSavingsFloorSkips`, `TestCacheWarmOffMakesZeroCalls` as real-path tests through the selected factory and a scripted `warmSpyModel`; add: exactly one maintenance request under the in-flight guard, `cache.warmed.call_id` equals the settled call, run-cancel silent drop, provider-error `failed` with error type, no messages/context pollution, idle timer canceled at run end, summary route never warms, child route does, unselected factory makes no call. Make timing deterministic (short declared lifetime plus a bounded wait, or inject the idle delay through the Host's clock hook if the existing test harness supports it); no sleeps that can flake.
- [ ] 8. Settings: keep `ServiceDeps` fields and `app.go` wiring; add `internal/config/config_test.go` coverage — omitted floor → 0.05; explicit `0` → 0 and never price-skips; negative floor → validation error; invalid mode → validation error naming `runtime.cache_warming`; each accepted mode round-trips. Add the two keys to `config.example.yaml` with a comment that they require a Recipe selecting `vivy/exp-warming`.
- [ ] 9. RPC capability/settings: in `settings/get`, include the `warming` section only when the generated Assembly selects `vivy/exp-warming` (generated capability projection, not a hand list); in `settings/update`, accept the section only in that build and reject it with `not_compiled` otherwise. Add `Settings.warming?: WarmingSettingsView` to `ui/src/lib/api.ts`. Do not add a settings card.
- [ ] 10. Event parity: populate `CallID` in `payloadCacheWarmed` from the settled call; keep status/reason/token semantics; verify the event is still persisted+publish through the existing owner with the detached bounded context, and that a run-cancelled warm emits nothing. Do not add a schema file or a warming-specific consumer (Baseline A has none); W03 owns the evidence/schema decision.
- [ ] 11. Add the accounting/recovery tests from decision A1/A2: off makes zero calls and zero events; unselected makes zero calls and returns `NOT_COMPILED` for a warming-only settings update; a maintenance failure does not fail the Run; a diagnostic append failure keeps the Baseline A terminal behavior (assert the Run terminal outcome, not the helper's return); no ledger/settlement row is created by a warm (assert the journal contains no `model.request`/`model.usage`/`model.call.finished` for the maintenance call).
- [ ] 12. Add the prefix-equivalence test (A8): record the warm request messages/tools produced by the old core path in a fixture created before the cutover commit (or reconstruct from `eng.instruction`/`eng.toolInfos` in the test) and assert the package-driven Host produces identical messages/tools on the normal chat path.
- [ ] 13. Run the gates: `go test ./internal/runtime ./internal/config ./internal/rpc ./internal/app ./plugins/exp/warming`, the repository `just ci`, and the affected real-path check (browser settings/event surface is unchanged; no card). Record exact commands and outputs.

## Verification commands and expected read

Implementation-time (toolchain absent in this planning environment):

- `go test ./internal/runtime -run 'TestCacheWarm' -count=1` → exit 0, each case above named in output.
- `go test ./internal/config ./internal/rpc ./internal/app ./plugins/exp/warming` → exit 0.
- `rg -n "runCacheWarmer|gate\\(promptTokens" internal/runtime` → no core policy match (Host/lifecycle names only).
- `rg -n "below_min_savings|below_token_floor" internal/runtime` → only diagnostic mapping.
- `rg -n "cacheWarmPrompt|cacheWarmMaxTokens|cacheWarmCallTimeout|cacheWarmTokenFloor" internal/runtime` → no match (constants moved to the SDK contract).
- `git diff --stat internal/runtime` → shows policy deletion and Host/adapter addition, not a parallel old path.
- `just ci` → record the exact result; do not claim a skipped check as passing.

## Acceptance, rollback and handoff

In a selected build, an eligible settled call makes exactly one real maintenance request on the current provider, reports its provider tokens and the settled call id in `cache.warmed`, and the Run/Journal/terminal behavior is unchanged; `off` makes zero calls/events but is compiled; an unselected build has no package/constructor/config read and reports `NOT_COMPILED`; the core policy body is gone with no fallback. Return the exact commit/build, changed paths, the runtime test list and outcomes, the prefix-equivalence evidence, the settings evidence and the config/CI results.

Roll back with the Story commit; never leave both implementations serving and never re-enable the core policy as a fix. Hand off to W03: the surviving code must expose the exact evidence surface (payload fields, adapter diagnostics) that W03 removes hash-only residue from and guards.
