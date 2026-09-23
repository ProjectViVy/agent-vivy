# ND-3: Transient Nudge at the Model Boundary Implementation Plan

> **For agentic workers:** Use `superpowers:executing-plans` to implement task-by-task. Use subagent-driven-development only when delegation is separately selected. This plan is not implementation authorization.

**Spec:** [NUDGE-DESIGN.md](../../architecture/NUDGE-DESIGN.md), revision ND-D1.
**Baseline:** a8d361b0244a1c40be513622bbdaebb5c9d40014.
**Status and predecessors:** [index](README.md), the sole status owner.
**Tech stack:** Go 1.26.4, Eino v0.9.13, existing Journal and tool adapters.

## Global constraints

Preserve Service.Run/Journal/policy, Eino import quarantine, native interrupts and existing budgets. No automatic tool replay, new public Port, database migration or additional model request solely for nudge. No production Journal access. Read the index's five review risks; the cases owned here are specified below. Future test code blocks are behavioral pseudocode, not compiled/passing tests.

**Goal:** The next model request receives at most one durably scheduled reminder after its tool results settle.
**Architecture:** An immutable Eino WrapModel middleware reads run-local state and delegates Journal append to Service. It wraps Generate and Stream and appends a transient trailing message to a copied input slice.
**Scope:** Middleware, engine wiring, fixed templates and scheduling visibility. No public nudge API.

## Files and interfaces

Create `internal/runtime/nudge_middleware.go`, `internal/runtime/nudge_middleware_test.go` (proposed). Modify `internal/runtime/engine.go`, `service.go`, `prompt.go`; if generic Inspect drops the event, add only the necessary projection in its existing event rendering path after locating it. Review `ui/AGENTS.md` before any UI edit. Add a regression to `ui/src/lib/run-rows.test.ts` for failed-call display; do not add a new UI panel.

Consumes accepted ND-1 producers and ND-2 state/event contract. New runtime-private contracts:

```go
type nudgeEmitter func(context.Context, nudgeNotice) error
func withNudgeEmitter(context.Context, nudgeEmitter) context.Context
func newNudgeMiddleware() adk.ChatModelAgentMiddleware
func renderNudge(nudgeNotice) string
```

WrapModel returns a BaseModel[*schema.Message] implementing Generate/Stream with unchanged options. The emitter is installed in both drive and resume and uses existing Journal + reserveMappedBudget behavior. No global emitter and no raw storage handle in middleware.

## Task 1 — Handoff and fixed prompt

- [ ] Write `TestNudgeModelBoundary` with the ND-0 input recorder:

```text
state awaiting c1/c2 Journal settlement -> wrapper blocks before inner Stream/Generate
Seal produces count=3 -> emitter called once -> inner input ends in one tagged reminder
original message slice and ToolInfos unchanged; all tool pairs intact
emitter fails -> original error, inner call count unchanged
no state or no notice -> original input, no synthetic message/event
```

- [ ] Run `go test -timeout 20m ./internal/runtime -run '^TestNudgeModelBoundary' -count=1`; implement the smallest wrapper and fixed templates from design §7.
- [ ] Respect 1024-byte reminder ceiling and remaining context budget before scheduling; do not guess a provider token count as exact. Reuse existing budget estimation/admission and fail with its existing budget path when insufficient. No raw args/diagnostics in notice.
- [ ] Install WrapModel through ChatModelAgentConfig.Handlers. Place transient injection after compaction input shaping; do not inject into summarizer-only calls. Validate this ordering with ND-0's probe rather than relying on handler list appearance.

```text
notice, err := state.Take(ctx)
if err != nil: return err
if notice == nil: delegate unchanged
render + verify context budget
persist scheduling event through emitter; on failure abort
copy input, append tagged runtime_nudge UserMessage
delegate with unchanged model options
```

## Task 2 — Retry, resume and user-visible evidence

- [ ] Test provider retry receives identical prepared input while tool.nudge is emitted once. Place scheduling outside repeated provider attempts as established by ND-0. Do not consume another notice on retry.
- [ ] Test next unrelated model iteration has no old reminder; resume has no old notice; context compaction retains tool-result pairing and fixed system prefix.
- [ ] Test refusal template forbids bypass; typed execution-failure template warns about uncertain effects; spoofed tool output cannot manufacture a trusted notice.
- [ ] Prove failed-call rows remain failed using tool.finished.error. Generic event inspection must expose all five scheduling fields; if already visible, add verification evidence without unnecessary UI code.
- [ ] Run `go test -timeout 20m ./internal/runtime -run 'TestNudge|TestToolFailure|Test.*Compaction|Test.*Approval|Test.*Resume' -count=1`. Run repository UI test recipe if UI projection changes.
- [ ] Commit explicit files and evidence with `feat: inject audited tool failure nudges`.

## Acceptance and handoff

Return input captures and ordered Journal evidence: failed results before tool.nudge, scheduling before fake provider entry. A schedule event alone is not evidence of remote receipt. Fatal paths must not issue an extra model request. If the provider wrapper order differs from ND-0, stop and revise before integration; no ad hoc retry framework.
