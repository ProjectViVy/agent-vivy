# Agent-Vivy Agent Harness Goal Roadmap

> Status: active implementation roadmap
> Baseline: `main@e1b3d79`
> Scope: Agent Harness only

## Scope boundary

This goal improves the complete Agent Harness path in Vivy:

```text
prompt -> preflight -> context/budget -> tool selection -> run mode
       -> provider stream -> tool/approval/question -> durable events
       -> API/SSE/UI -> cancel/recover/audit
```

This phase does not implement Memory, BML, Laputa, AutoDream, Evolution,
long-term memory injection, or a replacement memory abstraction. Existing V1
Notes remain unchanged and are covered only by regression tests. Future
full-strength Laputa integration will provide the long-term memory layer.

## Evidence sources

- `../AGENT-VIVY-DIRECTION.md` and `../AGENT-VIVY-ASSEMBLY-OPTIONS.md` define
  the Vivy-owned boundary and anti-clone rules.
- `docs/IMPLEMENTATION-PLAN.md` and `docs/AGENT-VIVY-ARCHITECTURE-V0.md`
  define the current Eino, Journal, Checkpoint, Approval, API, and UI seams.
- The read-only `agent-diva` research covers prompt-cache layout, dynamic
  tool discovery, micro-compaction, Plan Mode restrictions, worktree
  isolation, dry-run preflight, hook pipelines, sandbox policy, provider
  telemetry, and background sessions.

## Ordered capability slices

| ID | Slice | Depends on | Exit evidence |
|---|---|---|---|
| H0 | Harness event and capability register | — | Contract and status ledger committed |
| H1 | Context budget and prompt layout | H0 | Bounded context, deterministic tests, no memory changes |
| H2 | Tool manifest, schema validation, dynamic selection | H1 | Only selected tools are callable; invalid args fail closed |
| H3 | Physical Plan Mode and run-mode policy | H2 | Effectful tools rejected while planning |
| H4 | Ask User suspend/resume flow | H3 | Durable question lifecycle distinct from approval |
| H5 | Dry-run preflight and lifecycle hooks | H2 | `ready/warning/blocked`, no model/tool side effects |
| H6 | Safety filters and tool policy | H2, H5 | Injection, PII, path, command, and untrusted-result guards |
| H7 | Budget ledger and circuit breaker | H3, H6 | Parent/child and retry budgets cannot be bypassed |
| H8 | Background sessions and subagent isolation | H7 | list/logs/attach/cancel/recover with isolated worktrees |
| H9 | Provider stream and audit observability | H1, H7 | retry/stall/reasoning/token events reach UI and logs |
| H10 | API/SSE/CLI/UI closure | H3, H4, H8, H9 | Browser and headless flows recover after refresh/reconnect |
| H11 | ACP/remote control proposal | H10 | Proposal only unless separately approved |

Every slice follows:

```text
evidence -> Vivy proposal -> contract/tests -> implementation
         -> integration -> real-path verification -> atomic commit
```

## Current progress

- H0: done in `748cd38`.
- H1: done in `c3b8ac4`, `9d7c8b3`, and `15c6eb9`. Transient context is
  bounded, the stable prompt prefix is separated from dynamic run facts, and
  tool results are compacted before entering model context. No durable memory
  behavior was added.
- H2-H11: pending.

## Verification gate

Each implementation slice must pass:

```text
gofmt -l .
go vet ./...
go test -race -count=1 ./...
```

UI slices additionally run `npm run build` and Playwright E2E. Real-provider
and desktop checks are explicit human checkpoints; mocks cannot replace them.

H1 verification for `c3b8ac4`:

- `go test -race -count=1 ./...` passed.
- Context unit tests cover recent-history retention, byte budget, mandatory
  current-message retention, tool-row exclusion, and terminal failure.
- Prompt tests prove dynamic date/Notes facts are excluded from the stable
  instruction prefix while tool guidance remains deterministic.
- Tool-adapter tests prove oversized results retain a UTF-8-safe head/tail
  with an explicit collapse marker and small results remain unchanged.
- Existing `TestServiceCancelPendingRun` was repeated five times under race;
  all five passed after one transient full-suite timing failure.

## Anti-clone and non-memory gates

- No `agent-diva-*` or `.workspace` imports or runtime dependencies.
- No Diva Tauri command or storage schema port.
- No new Memory/BML/Laputa/AutoDream implementation in this goal.
- Durable product history remains the Journal; checkpoint bytes remain engine
  state only.
- New events are append-only and versioned; existing V0/V1 event semantics
  remain compatible.

## Goal completion

The Goal may close only when all selected Harness slices are `done`, every
unselected slice is explicitly `deferred` or `rejected` with a revisit trigger,
the full regression and real-path gates pass, the worktree is clean, and this
document contains commit and verification evidence for each completed slice.
