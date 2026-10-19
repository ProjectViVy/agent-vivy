# W01: Extract the pure warming policy

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Preserve the user's execution choice; use one active write lane and fresh independent reviewers for final acceptance.

**Goal:** Move the warming *decision* out of core into the pure, Eino-free policy of `plugins/exp/warming`: eligibility (mode, declared warmth, cache lifetime), the savings floor and token-floor gate, the idle-refresh decision, the fixed request shape and the paid-request accounting decision, with the baseline formulas pinned by tests.

**Architecture:** The package implements `loopexp.WarmingPolicy.Decide` over the plain DTOs fixed in §11.3. It knows nothing about Eino, the runtime, the Journal, tools, providers, the Run or the filesystem: the Host supplies observed values, and the policy returns a plain decision (`Request == nil` means skip). The single core implementation still serves in this Story; W02 performs the cutover and deletes the core policy body.

**Tech Stack:** Go 1.26.4; no Eino import; `agent-vivy/sdk` contract only; plugin-local Go module.

**Spec:** [Complete deletion design §11.3 and §11.5, C11](../../../plans/2026-10-19-complete-deletion-plan.md). Epic index: [index.md](index.md). Predecessor: [W00](W00-seam-binding.md).

**Story:** W01 · **Epic:** E-WARMING · **Requirements:** R2, R5, R6.

**Immediate predecessors:** W00.

**Readiness gate:** W00 accepted: the package exists, is selectable, and imports only stdlib + SDK. The core warming behavior is untouched and still serving. Current request authorizes planning only.

## Global Constraints

- Pure policy: no `context.Context`, no I/O, no clocks, no goroutines, no Eino/domain/runtime imports, no Journal or provider access; decisions are deterministic given the input.
- Preserve Baseline A behavior exactly: do not "improve" the heuristic, do not add a model-pricing fetch, do not add a cross-Run prefix cache or a background scheduler.
- One policy instance per served warmer; no package-level mutable state.
- No Vivy-owned hash or prefix digest in policy input or decisions; the policy never sees raw prefix text.
- Follow the shared epic gates in [index.md](index.md) (import quarantine, physical omission, T1-only, no second path, no hash).
- The core implementation remains the only serving path in this Story.

## Review Focus

- Behavior equivalence: every skip/eligibility branch yields the Baseline A status/reason (`below_min_savings`, `below_token_floor`, `off`, not warmable, missing lifetime) and the same request scalars.
- Boundary comparisons: `savings < MinSavingsUSD` (equality warms), `PrefixTokens < 2048` on the unpriced path (equality warms); zero/negative `MinSavingsUSD` never skips on price.
- Paid accounting: the decision must not report tokens; only the Host knows real usage. Any token estimate in the package is a decision input, never a billing record.
- Contract shape: the struct shapes stay as §11.3 fixes them; only exported constants may be added (decision A9).
- Field discipline: `CachedInputPerMTokens` is part of the fixed input DTO but the pinned decision does not read it; a reviewer must reject code that silently branches on it (that would be a behavior change) and reject deleting it (that would fork the accepted contract).

## Baseline A mapping (202aac38, 2026-10-07)

All policy to move comes from `internal/runtime/cachewarmer.go` (229 lines, verified):

| Baseline A symbol / behavior | Line(s) | Target |
| --- | --- | --- |
| `cacheWarmModeOff/Streaming/Idle` (`"off"`, `"streaming"`, `"idle"`) | 34-37 | SDK contract mode constants in `sdk/port/loopexp/warming.go` |
| `cacheWarmPrompt = "Reply with one word: ok."` | 42 | `loopexp.WarmingPrompt` (Host passes it in `WarmingRequest.Prompt`) |
| `cacheWarmMaxTokens = 16` | 45 | `loopexp.WarmingMaxOutputTokens` |
| `cacheWarmCallTimeout = 60 * time.Second` | 48 | `loopexp.WarmingTimeoutMillis = 60000` |
| `cacheWarmTokenFloor = 2048` | 51 | `loopexp.WarmingTokenFloor` |
| `cacheWarmIdleHeadroom = 0.8` | 54 | `loopexp.WarmingIdleHeadroom` |
| constructor eligibility: mode `""` → `streaming`; `off` → nil; engine/chat model/mapper non-nil; `ModelInfo.SupportsWarming && CacheLifetimeSeconds > 0` | 80-103 | `Decide` input branches (`Mode == ""` normalization belongs to the Host; the policy receives the effective mode) |
| selected tools resolved against `eng.toolInfos` in selection order | 92-99 | not policy: Host prefix construction (W02) |
| settle gate: priced savings = `PrefixTokens * InputPerMTokens / 1e6`, skip if `< MinSavingsUSD`; unpriced floor `PrefixTokens < 2048` | 124-138 | `Decide` returns the same reasons |
| streaming → schedule now; idle → arm timer at `CacheLifetimeSeconds * 0.8` | 105-151 | streaming decision returns the request; idle decision returns the request and the Host uses `WarmingIdleHeadroom` for its timer (A9) |
| one in-flight guard, cancel-with-run, silent drop, timeout, observer stripped, max tokens, `[System(instruction), User(prompt)]`, cache-write extra read | 153-211 | Host/lifecycle (W02); not policy |
| `cache.warmed` payload assembly and detached persist | 213-228 | Host (W02) |

Per-route behavior to preserve: summary-route model calls never warm; main and child calls do (`service.go:2991-3013`, `3195-3199`). That route filter is Host-owned, not policy.

## Files and ownership

- Modify: `sdk/port/loopexp/warming.go` — add the exported constants from decision A9 only.
- Create/modify: `plugins/exp/warming/policy.go`, `plugins/exp/warming/policy_test.go`.
- Create/modify: `plugins/exp/warming/policy_trace_test.go` (table of input → exact `WarmingDecision`).
- Do not touch `internal/runtime/*`, `internal/config/*`, `internal/rpc/*`, `internal/app/*` in this Story.

## Contract

```go
// plugins/exp/warming/policy.go — Eino-free; imports only agent-vivy/sdk/port/loopexp.
type policy struct{}
func NewPolicy() loopexp.WarmingPolicy { return policy{} }
func (policy) Decide(in loopexp.WarmingInput) loopexp.WarmingDecision
```

Decision table (verbatim contract):

| Effective mode | Condition | Reason | Request |
| --- | --- | --- | --- |
| `off` | always | `off` | nil |
| `streaming`, `idle` | `!SupportsWarming` | `not_warmable` | nil |
| `streaming`, `idle` | `CacheLifetimeSeconds <= 0` | `no_cache_lifetime` | nil |
| `streaming`, `idle` | `InputPerMTokens > 0 && PrefixTokens*InputPerMTokens/1e6 < MinSavingsUSD` | `below_min_savings` | nil |
| `streaming`, `idle` | `InputPerMTokens <= 0 && PrefixTokens < WarmingTokenFloor` | `below_token_floor` | nil |
| `streaming`, `idle` | otherwise | `eligible` | `{Prompt: WarmingPrompt, MaxOutputTokens: 16, TimeoutMillis: 60000}` |
| any other mode | compile-time unreachable; defensively | `invalid_mode` | nil |

The policy does not decide *when* to call (immediate vs idle delay); it decides *whether*. The Host owns scheduling, the in-flight guard and cancellation (W02). A non-eligible decision must not produce a maintenance request; the Host decides whether to record a `skipped` diagnostic (Baseline A records it for gate skips, not for `off`).

## Tasks

- [ ] 1. Read `internal/runtime/cachewarmer.go` lines 34-138 and pin every constant and branch in `policy_test.go` before writing implementation (test-first; no behavior invented).
- [ ] 2. Add the A9 constants to `sdk/port/loopexp/warming.go` with doc comments tying each to Baseline A behavior and to §11.5's fixed request shape. Do not change the D05 DTO structs.
- [ ] 3. Implement `plugins/exp/warming/policy.go` as the table above; keep it a pure function of the input (no receiver state, no package globals, no time/context).
- [ ] 4. Add table tests covering: `off`; unknown/empty effective mode; not warmable; missing lifetime; priced below floor; priced exactly at floor (eligible); unpriced at 2047/2048 tokens; negative min savings; zero prefix tokens; idle mode yields the same request as streaming; request literals (prompt text, 16, 60000).
- [ ] 5. Add a decision-trace test that asserts the returned struct has no other fields set and that a skip never carries a non-nil `Request`.
- [ ] 6. Add a package test asserting the policy package imports no forbidden path (stdlib + `agent-vivy/sdk/port/loopexp` only), reusing the W00 `imports_test.go` helper style.
- [ ] 7. Leave `internal/runtime/cachewarmer_test.go` untouched in this Story; W02 moves the real-path tests and deletes the core body in the same cutover change.
- [ ] 8. Confirm the core still serves unchanged: no file under `internal/` is modified by this Story.

## Verification commands and expected read

Implementation-time (toolchain absent in planning environment):

- `go test ./plugins/exp/warming` → exit 0.
- `go vet ./plugins/exp/warming` → exit 0.
- `go test ./internal/runtime -run 'TestCacheWarm'` → still exit 0 and unchanged behavior (the core still serves).
- `rg -n "cloudwego/eino|agent-vivy/internal" plugins/exp/warming` → no match.
- `git diff --check` → exit 0.

## Acceptance, rollback and handoff

The pure policy reproduces every Baseline A eligibility/gate branch and the exact request scalars, with no runtime/Eino/I-O dependency and no state; the core still serves unchanged. Return the decision table evidence, test names and outputs, the constants added, and confirmation that no core file changed.

Roll back by reverting this Story's commit; never keep a policy that differs from Baseline A to make a test pass. Hand off to W02: the package's `NewPolicy` must be the only decision implementation the adapter will use, and the Host contract must be able to supply mode, capabilities, prices, settled prefix tokens and settings without the policy reading anything itself.
