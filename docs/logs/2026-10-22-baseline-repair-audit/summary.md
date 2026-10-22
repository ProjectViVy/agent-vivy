# Baseline enhancement and repair intake audit

Audit target: `98b2ea9a`; its executable source is `397b6f17`. The owner
reported successful custom-provider use after the local credential handoff.
This audit checks other retained baseline repairs, without merging features or
changing the running package. It confirms sixteen applicable source-level
repair omissions and two separate verification-automation intake items.
These findings were not newly reproduced at runtime during this audit.

The current D14 repair branches are already included. Most remaining repairs
come from the historical R10/core-fixes and provider-hardening donors, which
remain bounded re-intake material under [issue 83](https://github.com/ProjectViVy/agent-vivy/issues/83).
Their earlier delivery does not constitute acceptance on this candidate.

## Applicable repair omissions, ordered by impact

Locations below are one-based lines at `98b2ea9a`. Donor identifiers identify
the narrow historical repair, not a blanket cherry-pick instruction.

| ID / priority | Current location | Evidence and consequence | Donor and smallest useful correction |
| --- | --- | --- | --- |
| BR01 / P1 | `internal/rpc/control.go:1461`; `internal/runtime/service.go:1750`; `ui/src/lib/store.ts:608` | The browser initialization RPC calls startup `Recover`, although `RecoverBackground` already refuses active/pending ownership. Opening another Face can enter restart recovery during a live Run. | `75459823`: route the RPC through the existing guarded operator API and its busy-error mapping. Keep current D08/D11 recovery internals. |
| BR02 / P1 | `internal/rpc/control.go:3635,3643` | `turn/start` resolves attachment/context paths from process `ProjectRoot`, before resolving the authoritative Session workspace. A Session bound elsewhere can attach an identically named file from the wrong root. | `a21e81a9`: resolve Session ownership first, then use its workspace with current D09 containment and private exact-input capture. |
| BR03 / P1 | `internal/storage/sqlite/sqlite.go:31` | The core database is opened with raw `"file:"+path`; URI delimiters and percent escapes can change literal filesystem identity. The Studio ledger's separate URI repair does not fix this constructor. | `edbb0f5f`: escape the filesystem path when constructing the SQLite DSN. No schema or released-SQL change. |
| BR04 / P1 | `internal/runtime/queue.go:165,246,429`; `internal/rpc/control.go:3709` | Ordinary follow-ups expose in-memory items before durable append success, discard marker errors, and drain text without full submitted turn options. ACK, replay, recall and resend can lose data/options. | `242091b8`, `09fabb05`: re-intake ordinary full-turn queue admission, atomic failure behavior and DTO restore using current private inputs. Keep same-Run steering extraction under issue 93. |
| BR05 / P1 | `internal/app/app.go:1590` | `applySettingsEnv` lacks a `UserDefined` guard. A registered alias matching an embedded vendor can publish its key into that vendor's environment variable. Empty saved addresses also fail to withdraw a previous address written by this process. | `968ae236`: keep registry keys within their owner and withdraw only the process-owned base-URL override. Adapt to the new `UserDefined` field from `397b6f17`. |
| BR06 / P1 | `internal/app/model.go:97,134` | Frozen environment selection begins with `hits[0]`; several credentials, or shared credential/adapter names, can silently select an unintended vendor. | `2f66363c`: use an explicit unique vendor selection or abstain and retain settings; report variable/vendor names without values. |
| BR07 / P2 | `internal/rpc/protocol.go:240` | ID-less inbound notifications are dispatched in separate goroutines and can overtake one another. Outbound serialization does not order handler execution. | `12cef819`: a bounded ordered notification worker, with responses and ID-bearing calls independently dispatched to preserve callback RPC progress. |
| BR08 / P2 | `internal/rpc/work_control.go:121,143,277`; `ui/src/lib/store.ts:784` | Work/Plan reads require the old peer Session binding while Session selection starts message and Work reads concurrently. A legitimate newly selected Session can show a spurious authorization conflict. | `7d4b8b18`: resolve the named authoritative Session for reads; retain binding requirements for mutations. |
| BR09 / P2 | `internal/runtime/tooladapter.go:779,823`; `internal/runtime/service.go:3872`; `internal/runtime/patch_engine.go:28,56` | Correctable patch-argument/proposal mistakes reach suspension/preparation before classification and can become fatal Run failures. The existing invalid-workspace-path repair covers a narrower boundary. | `f9034c04`: preflight allowlisted argument defects before suspension, persist current D03 typed outcomes, and retain fatal infrastructure failures. |
| BR10 / P2 | `internal/runtime/hooks_script.go:201` | Windows uses `exec.CommandContext(ctx, "cmd", "/c", command)` and loses shell-program quoting around executable paths/arguments containing spaces. | `3573d905`: port the platform command helper and the existing unscoped ScriptHook regressions. Do not restore absent Toolkit runners. |
| BR11 / P2 | `internal/tools/bashclass.go:345,502` | Literal-only redirect classification rejects valid statically quoted targets and heredoc/file-descriptor words as unverifiable paths. | `d8c2b839`: operator-aware static-word classification, preserving workspace containment and dynamic-expansion refusal. |
| BR12 / P2 | `sdk/internal/frontend_v1.go:1169,1338` | `goModulePath` and `sourceGoBinding` parse `CombinedOutput`; successful Go stderr noise can contaminate a module ID or reject a valid package. | `c70adfbe`: separate stdout and stderr at these two surviving functions, preserving stderr diagnostics and wrapped subprocess causes. Do not restore three deleted digest-era helpers. |
| BR13 / P2 | `internal/app/settings/settings.go:497,975` | Registry read/upsert does not reconcile a declared default model into its model list. Runtime selection can name a default that the settings list cannot display. | `8d77c0c5`: idempotent read/write reconciliation with its persistence tests, using the backend registry as the sole fact owner. |
| BR14 / P2 | `internal/provider/discover.go:83,91` | A response-byte bound exists, but accepted model count and individual ID length are not bounded before registry/UI use. | `1bdfdca4`: bounded accepted catalogue and ID size, preserving order/deduplication and explicit oversized-catalogue failure. |
| BR15 / P2 | `internal/logging/diagnostics.go:387` | Diagnostic cursors treat every mtime change as a gap; ordinary log append invalidates continuation. D09 history cursors have a different owner. | `01ef1180`: allow append growth while detecting truncation/replacement through actual file identity, including Windows. Preserve current no-digest contracts. |
| BR16 / P2 | `justfile:41` | `setup` still invokes `go env -w GOPROXY=...`, changing the developer's global configuration. | `e31a9ffa`: process-local GOPROXY honoring `VIVY_GOPROXY`, defaulting to direct. No global Go writes. |

## Separate verification automation

- **BC01 — nested BML gate:** `bml/go.mod` remains a separate module, so root
  `go test ./...` does not run its full owner suite. D14 has explicit full BML
  and race evidence; the omission is repeatable automation in ordinary CI.
  Reuse the portable BML runner idea from `c70adfbe`, adapted to current BML2
  and available process helpers.
- **BC02 — scoped CI delivery:** `codex/development-ci-contraction@40d54eb6`
  is unmerged. Its scoped selector, `ci-small`, Recipe-specific UI execution
  and Actions wiring need reconciliation with the current kernel gates and
  current repository rules. Do not merge older AGENTS/contracts wholesale.

## Already present or outside this baseline

- All four D14 repair lanes (action contracts, warming accounting, approval
  registration and runtime residue) are included or equivalent.
- `f78195a3` App shutdown/lifecycle, bounded MCP close, listener failure cleanup,
  native CODE fixtures and SDK scratch repairs survive through their replay.
- Mandatory model-observation settlement failure propagation is already
  present through `modelObservationError`, joined settlement errors and
  `TestObservedCallSettlementAtMaxEvents`.
- The classic Claude streaming return-variable race patch already exists in
  `third_party/eino-claude` and the module replacement, despite absent donor
  commit ancestry.
- The dirty `codex/issue91` worktree equals `f78195a3` after normalization;
  its repairs are accounted for. All 53 tracked dirty changes and the one
  untracked file in `issue91-d06-tests` are already integrated or superseded.
  Both working trees were preserved untouched.
- The later Laputa Recap cancellation repair is not inherited by the current
  `9acc9993` pin, but that pin does not ship the affected Recap implementation.
  Preserve the repair as a dependency for later Recap intake. Likewise, later
  notebook, Toolkit and Face-specific fixtures do not justify restoring their
  absent product families. Issue 93's extraction remains separate.
- Known provider B4/B5 follow-ups without a delivered applicable repair are
  not counted as missed merges in this list.

## Capability and integration boundary

Eino capability check: the candidate uses pinned Eino v0.9.21
`adk.NewTypedChatModelAgent`, `adk.NewTypedRunner`, `runner.Run` and
`runner.ResumeWithParams` (`internal/runtime/engine.go:159,436,562,570`).
Provider construction remains behind the existing EinoExt adapter boundary.
The recovery repair already has a Vivy guarded entry point; the other findings
concern admission, persistence, configuration or transport around that path.
No second runner, custom model protocol, new checkpoint owner or retired hash
mechanism is proposed. Each implementation batch must retain these boundaries.

Recommended order: (1) active-run/data/session/provider isolation, BR01–BR06;
(2) protocol, recoverable tool failures and native shell/SDK repairs,
BR07–BR12; (3) settings/discovery/diagnostics/environment, BR13–BR16, followed
by BC01/BC02 as explicit tooling work. This is an audit recommendation, not a
claim that any repair has been integrated or newly validated.
