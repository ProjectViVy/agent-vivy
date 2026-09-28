# Issue #1 ACP Face delivery plan

Plan package for [Issue #1](https://github.com/ProjectViVy/agent-vivy/issues/1). The [reviewed architecture](../../specs/2026-09-28-issue1-acp-face-design.md) is the design source; baseline is `main@3c4ed66`. This index is the single owner of Story status and dependency edges. It does **not** schedule the UNSCHEDULED issue, assert G0 acceptance, or authorize G1 implementation.

## Epics and traceability

| Epic | Outcome | Requirements | Stories |
| --- | --- | --- | --- |
| E0 Contract freeze | Reproducible ACP v1 subset and reviewed product contract | A1–A5 | ACP-01 |
| E1 Local ACP Face | Selected Face starts safely, uses one Vivy runtime and handles actual turns/reviews | A1, A2, A3, A5 | ACP-02, ACP-03, ACP-04 |
| E2 Generation acceptance | Recipe selection, physical omission, real client and release evidence | A1–A5 | ACP-05 |

## Story dependency ledger

| Story | Outcome | Immediate predecessor and required accepted output | Plan | Execution status and blocker |
| --- | --- | --- | --- | --- |
| ACP-01 | G0 compatibility fixture, normative contract and owner review | None; Issue #1 plus reviewed design | [ACP-01](ACP-01.md) | **Blocked:** Issue is UNSCHEDULED; no Go toolchain in current environment. Documentation is drafted; compatibility test/review has not run. |
| ACP-02 | Generic stdio Face launch and ACP build's TUI import isolation | ACP-01: frozen wire/limits/packaging contract and G0 owner review | [ACP-02](ACP-02.md) | **Blocked:** G0 output and G1 scheduling absent. |
| ACP-03 | One ACP connection, real sessions, prompt, Journal event projection | ACP-02: `Options.In`, Face startup, and shared private-instance preparation | [ACP-03](ACP-03.md) | **Blocked:** predecessor not accepted. |
| ACP-04 | Permission, Ask User, cancellation, fail-closed lifecycle | ACP-03: owned session/run state, event dispatch, bounded connection writer | [ACP-04](ACP-04.md) | **Blocked:** predecessor not accepted. |
| ACP-05 | Hashed Recipe, Verify/Pack/Inspect, real IDE, full CI and logs | ACP-04: complete Face including review and cancellation behavior | [ACP-05](ACP-05.md) | **Blocked:** predecessor not accepted; real client and CI environment required. |

The immediate-edge DAG is `ACP-01 → ACP-02 → ACP-03 → ACP-04 → ACP-05`. Topological waves are `{ACP-01}`, `{ACP-02}`, `{ACP-03}`, `{ACP-04}`, `{ACP-05}`. All IDs are unique; all edges refer to existing Stories and have no cycle or redundant transitive edge. Serial execution is intentional: ACP-03/04 edit the same adapter state, and ACP-05 seals its final source hash. Wave order is **not** permission to execute; each predecessor requires actual acceptance evidence. No duration estimates or time-based critical path are asserted.

## Decisions and gates

- Compiled Module: `projectvivy/acp` (T2), `std/face@v1` through `core/face-host@v1`; effective grant `rpc.client`; one selected Face. ACP is a local stdio transport over the existing private Control RPC and single `Service.Run`/Journal/Policy authority.
- G0 owns the supported stable v1 wire subset, payload allowlist, version/capability checks, bounds, SDK compatibility and physical omission interpretation. G1 Stories must be revised if the executed G0 fixture changes these contracts; do not assume that a Go SDK method name determines the wire method.
- The current packer always builds/stages `ui/dist`; ACP selection must omit the built-in TUI *implementation imports* by an overlay. G0 records UI assets separately from Face code; do not report that selected ACP removes `ui/dist`.
- Preserve the default gateway, existing `vivy tui`, and `vivy run` behaviors. No new listener, subprocess or dynamically loaded Module. A non-ACP generation must not gain an ACP dependency.
- Issue #1 remains UNSCHEDULED. Owner's G0 review and explicit G1 scheduling are release gates. Plan design alone does not satisfy them. Current environment has no `go`, `just`, or `pwsh`; all runtime and CI checks remain pending.
- For implementation use `.agents/skills/vivy-plugin/SKILL.md` and `.agents/skills/vivy-kernel-ci/SKILL.md` and Superpowers execution/verification. Document honest evidence in `docs/logs/`; do not mark conformance or acceptance by declaration.

## Supervisor acceptance

| Requirement | Acceptance owner / evidence |
| --- | --- |
| A1 | ACP-03 transcript plus ACP-05 real IDE/client subprocess prompt |
| A2 | ACP-03 real ID/Journal tests and ACP-04 review/cancel/expiry tests |
| A3 | ACP-02 early-startup stdout test, ACP-03 committed seq/replay test, ACP-05 process capture |
| A4 | ACP-02 selected import test, ACP-05 two Recipes, `verify`, `pack`, `inspect-artifact`, `go list -deps` and hash proof |
| A5 | ACP-01 freeze and ACP-03/04 invalid-input, limits, disconnect and fail-closed tests |

On each completed Story update this table using test outputs, review notes, and commit/CI references. If G0 changes a contract, review every downstream plan before changing its execution state. Epic acceptance requires the integrated A1–A5 evidence, not only the individual Story checkboxes.
