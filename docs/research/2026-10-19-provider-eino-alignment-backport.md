# Eino and provider alignment backport assessment

Date: 2026-10-19

Status: research only. No product code, pin, migration or workflow changed by
this note.

Question: current main carries a provider update and pins Eino to the newest
version. That work is infrastructure alignment, not a capability increment, and
the newer chain design is cleaner, so it should be moved back to the stable
baseline-A branch instead of waiting for the gated re-intake batches.

## 1. What actually exists on main

Observed head: `9cd2801f` (merge of PR #92, 2026-10-19). Baseline A is
`202aac38` (2026-10-07).

| Commit | Date | What it is | Non-capability? |
| --- | --- | --- | --- |
| [`8f263d36`](https://github.com/ProjectViVy/agent-vivy/commit/8f263d36) | 2026-10-18 | Eino `v0.9.13` -> `v0.9.21` plus a pending-run policy across an engine-version change: recovery cancels the pending approval/question (before the terminal event) and the Plan review (after it), and closes the run with a "Vivy was updated" message on `ErrCheckpointEngineVersionMismatch`. Adds `recovery_engine_version_test.go` and the HITL policy section. Closes `EINO-CORE-UPGRADE` and the upstream ToolsNode race (`EINO-TOOLSNODE-ERR-RACE`). | Yes: dependency + recovery policy, no new product capability |
| [`8ec876d3`](https://github.com/ProjectViVy/agent-vivy/commit/8ec876d3) | 2026-10-19 | Vendors `third_party/eino-claude` (production sources of `eino-ext/components/model/claude` v0.1.25 plus one local race fix: per-worker `ConcatMessages` error variable) and replaces only that module; one Dockerfile `COPY` line. PATCHES.md states no public API, protocol, callback or orchestration change; remove when upstream fixes it. | Yes: race fix, no behavior/API change |
| [`f5322c92`](https://github.com/ProjectViVy/agent-vivy/commit/f5322c92) | 2026-10-15 | Custom provider runs from its own endpoint (`provider/catalog.go`, `resolving.go`, `app/provider_selection.go`). | Bug fix; portable in principle |
| [`1bdfdca4`](https://github.com/ProjectViVy/agent-vivy/commit/1bdfdca4) | 2026-10-17 | Bounds model discovery results (`provider/discover.go` + test). | Bug fix; self-contained |
| [`773dc741`](https://github.com/ProjectViVy/agent-vivy/commit/773dc741) | 2026-10-16 | Honors a trusted per-call output cap after thinking options (`domain/model_output_limit.go`, `provider/resolving.go`). | Bug fix; self-contained |
| [`8a37e059`](https://github.com/ProjectViVy/agent-vivy/commit/8a37e059) | 2026-10-18 | Pins stream behavior in tests only (`provider_stream_test.go`). | Test-only; portable |
| [`4c4e605a`](https://github.com/ProjectViVy/agent-vivy/commit/4c4e605a) | 2026-10-16 | Recap/refused-evidence model route pin. | No: depends on post-baseline recap code |

The "one message generation" design (Eino `AgenticModel` /
`schema.AgenticMessage`, `openai-responses` executable, zero protocol
conversion) is **not implemented on main**. It exists only as the
`docs/plans/provider-agentic/` spike, signed `GO-CONDITIONAL` on 2026-10-18,
with migration phases P2-P4 unstarted and blockers B5-B7 open (Claude usage
total, Claude cache semantics, chat-completions single-part content). P0/P1 -
the decision and the Eino bump - are the only delivered phases. So there is no
new production provider chain to transplant yet; only the infrastructure pieces
above.

## 2. Why it counts as non-capability

- P1 moves one dependency and writes down a restart policy. It adds no model,
  tool, UI surface or protocol.
- The Claude patch keeps the upstream component and behavior; it only removes a
  data race that the race detector already reproduces on the current stack.
- The agentic P2 message-generation boundary *would* be a capability/protocol
  change (making `openai-responses` executable). It stays in the reviewed
  re-intake track, not in this backport.

## 3. Feasibility probe on baseline A (measured 2026-10-19)

A disposable worktree at `202aac38` was used, with the repository-pinned
toolchain (Go 1.26.4) and `GOPROXY=https://goproxy.cn,direct`:

- `go get github.com/cloudwego/eino@v0.9.21` changed only the `eino` require
  line and two `go.sum` hashes; the remaining diff is `require` block
  reordering already present in the module. No transitive pin changed.
- `go build ./internal/...` fails only on `ui/embed.go: pattern all:dist: no
  matching files found` - the known fresh-checkout constraint (`just ci` builds
  `ui/dist` first). `go test -run '^$' ./internal/runtime/ ./internal/provider/`
  compiled both test binaries against v0.9.21.
- `go test ./internal/provider/ -count=1` passed (0.99s).
- Adding `third_party/eino-claude` from `8ec876d3` plus the `replace` line:
  `go build ./internal/provider/ ./internal/runtime/` passed and
  `go test ./internal/provider/ -count=1` passed (7.5s).
- No baseline-A source adaptation was needed for Eino API drift in these
  packages.

Limits: the UI embed and `cmd/vivy` link path, the full runtime suite (35m),
`-race`, real-provider and Docker builds were not run. The probe proves
compilation and provider test parity, not product acceptance.

## 4. Recommended backport

A separate non-deletion track, executed as a wave-2 preamble:

| Item | Source | Work | Gate |
| --- | --- | --- | --- |
| E1 Eino v0.9.21 + pending-run policy | `8f263d36` | Bump the pin; port the `service.go` recovery policy onto baseline-A code; port `recovery_engine_version_test.go` and its helpers; add the HITL policy section; move the two backlog rows; re-cite the plan's Eino capability check and §11.10 at v0.9.21 | L2 (runtime + storage), targeted recovery tests, `just vet` |
| E2 Claude race patch | `8ec876d3` | Copy `third_party/eino-claude`, add the `replace` and the Dockerfile `COPY` line, keep PATCHES.md | depends on E1 (vendored module requires `eino v0.9.21`); `just docker-build` when available |
| E3 Provider bug-fix set (optional) | `f5322c92`, `1bdfdca4`, `773dc741`, `8a37e059` | Triage each against baseline-A contracts; take only self-contained fixes, not the recap route pin | L1 provider tests |

Not backported: agentic P2-P4, the Laputa/Studio pin bumps, and any commit that
cites post-baseline runtime code (`4c4e605a` and the recap family).

Placement: E1/E2 do not block D01/D02/D03/D06/D12, but should land **before**
D04/D05 so the retained nudge suite is race-clean during the seam work and the
dependency boundary moves once, not twice.

## 5. Risks and consequences

- **Recovery semantics change.** A paused run whose checkpoint is unreadable now
  cancels its pending interaction and fails with a specific message. This is a
  behavior change for upgraded stores and must appear in the D14 behavior audit,
  not only in unit tests.
- **One version boundary, not two.** The plan audit pinned `eino v0.9.13`; after
  E1 every citation (plan §11, AGENTS-era capability checks, Eino-quarantine
  citations) must read `v0.9.21`. Leaving both is a second source of truth.
- **Vendored patch ownership.** The copy is upstream Apache-2.0 source; it needs
  the same license file, a removal condition (upstream race fix) and a review
  checkpoint, otherwise it silently becomes a fork.
- **Docker path.** The new Dockerfile `COPY` line is required for any container
  build; omitting it fails the build rather than degrading.
- **Data.** No schema change and no checkpoint conversion; the version check
  stays fail-closed. Existing baseline-A stores fail their paused run closed
  once and continue.

## 6. Owner decision (2026-10-19)

The owner approved the infrastructure alignment for this round: E1 and E2 land
as a Phase-A preamble before D04/D05, and E1 re-cites the plan's Eino
references at v0.9.21 in the same change. They are dependency/recovery
alignment, not capability increments, so they do not wait for the re-intake
batches. E3 remains a separate triage: take self-contained fixes only, never
commits that cite post-baseline runtime code. Recorded in
[owner decisions](../logs/2026-10-19-baseline-a-epic-split/owner-decisions.md)
item 11.
