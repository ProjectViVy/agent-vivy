# ND-4 Acceptance Record

Date: 2026-09-23. Against [ND-4.md](../../plans/nudge/ND-4.md) acceptance and the N1–N5 requirement trace.

## Plan tasks

| Task | Result | Evidence |
| --- | --- | --- |
| 1. Cases A–J with real Service/Journal/governed tools + fake provider, isolated temp dirs | DONE — all ten pass | `internal/runtime/nudge_acceptance_test.go`; verification §1 |
| 1. Package recipe `go test -timeout 20m ./internal/runtime ./internal/mcphost ./internal/toolhost ./internal/domain -count=1` | PASS | verification §2 |
| 1. Index race command on a supported host | EXECUTED — NOT PASSED, upstream defect | verification §3; `EINO-TOOLSNODE-ERR-RACE` (TODO §0.1, preexisting since ND-0) |
| 2. Full `just ci` | PASS (not a subset) | verification §4 |
| 2. Split-runtime smoke, isolated workspace/database | DONE — `VIVY_USER_HOME=/home/ubuntu/smoke/home`; `data/` never touched | verification §5 |
| 2. Failure → corrected permitted call → completion in one Run | OBSERVED — 3× `not_found` → approved `write_file` → successful `read_file` → `run.completed` | journal dump §5 |
| 2. Event inspection: `tool.finished.error`, scheduled reminder metadata, no secrets, single terminal | CONFIRMED — `tool.nudge{repeat_count:3, template_version:nudge-v1}`, 0 payloads contain the credential, 1 terminal per run | journal dump §5 |
| 2. Cancel while work is waiting | OBSERVED — `approval_required` → `approval_cancelled` → `run.cancelled`, no write, no deadlock | journal dump §5 |
| 2. `git diff --check`, index statuses, TODO completion record | DONE | this commit |

## Requirement trace N1–N5

- **N1 (typed failure feedback)**: A/B/C + smoke — `tool.finished` carries `outcome/reason/effects` for missing-file, exit-code and MCP `IsError` paths.
- **N2 (durable settlement before model)**: F1/F2 + `TestNudgeBoundaryWaitsForDurability` (ND-3) — model waits on batch durability; journal failure and cancel abort cleanly.
- **N3 (single detector, request-order)**: D/G — one detector instance, notices at 3 and 5 only, request-order naming proven under out-of-order journal arrival.
- **N4 (bounded notice, one per boundary)**: D/E/G/H + smoke — exactly one `tool.nudge` per count boundary, no duplicates across resume/checkpoint.
- **N5 (no authority/replay/scope creep)**: E/I/J — refusal never bypasses policy, zero mutation on deny, partial effects not replayed, legacy payloads stay readable, no new public Port or migration.

## Contract-bug check

No test revealed a bug in the ND-1/2/3 contracts; nothing was returned upstream and no acceptance was lowered. The only failing gate is the race gate, whose defect lives entirely inside pinned `cloudwego/eino@v0.9.13` (`tool_node.go:1253`) and predates this work — tracked OPEN as `EINO-TOOLSNODE-ERR-RACE` rather than silently absorbed. It is recorded here as a limitation, never as a pass.

## Deliberate deviations and limitations

- Race gate: executed on a supported host; failed on the preexisting upstream eino defect (above). Not claimed passed.
- Smoke provider: a scripted OpenAI-compatible stub via the documented ENV-session (`VIVY_API_BASE`/`VIVY_MODEL`/`DEEPSEEK_API_KEY`); every other component is production code. Product semantics learned during setup — a custom `base_url` keeps the vendor's credential/env-key, and setting a vendor env key freezes the session — are product behavior, not bugs.
- No claim that the model consumed the reminder text; evidence covers scheduling, durability ordering, and injected-payload shape only.
- Approval paths: `Trusted` preset still prompts for `write_file` ("allowlisted tools auto-run, others still ask") — used as-is; this is what made the cancel-while-waiting exercise possible without extra fixtures.
