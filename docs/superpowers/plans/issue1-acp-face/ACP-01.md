# ACP-01 G0 Contract Freeze Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Story / Epic:** ACP-01 / E0. **Goal:** Establish a reviewed, executable ACP stable-v1 subset contract without starting a functional Face implementation.

**Architecture:** Compare the candidate `github.com/eino-contrib/acp@v0.0.4` on-wire behavior against the official `schema-v1.21.0`, then freeze only the actual supported subset in existing normative documents. Keep the fixture in the repository for regression checks. **Tech Stack:** Go 1.26.4, ACP JSON-RPC 2.0 NDJSON, candidate SDK, existing Control RPC. **Spec:** [reviewed design](../../specs/2026-09-28-issue1-acp-face-design.md), baseline `3c4ed66`. **State / dependencies:** [index](index.md); no predecessor. Execution awaits Issue scheduling and a working Go toolchain. An incompatible candidate is a stop condition, not a passing gate.

## Global Constraints

- Stable ACP v1 `schema-v1.21.0` wire subset; local stdio only; one connection per process; no optional filesystem, terminal, URL elicitation, remote listener, dynamic loading, or extra authority.
- Effective grant `rpc.client`; existing `std/face@v1` and `core/face-host@v1`; real Vivy IDs and committed Journal updates only.
- Keep product normative contracts in existing docs, and record G0 review before scheduling G1. Do not add a functional ACP plugin in G0.
- Follow `.agents/skills/vivy-plugin/SKILL.md` and `.agents/skills/vivy-kernel-ci/SKILL.md`; document evidence, run `just ci` when code/docs change; no tenant `data/` access.

## Review Focus

1. SDK Go method `UnstableCreateElicitation` emits `elicitation/create`: assert the **wire** name with form request/response and optional capability absent.
2. Reverse request during prompt does not deadlock `session/cancel`: exercise concurrent read/write in the fixture.
3. The chosen SDK logger cannot send protocol or secret payloads to stdout/stderr: capture both streams with sensitive sentinel content.
4. SDK inbound-size/write-timeout/EOF behavior matches frozen limits: test over-limit frame and blocked/broken writer.
5. UI assets remain in current Pack even if TUI Face code is omitted: document distinct evidence for packages vs artifact files.

---

### Task 1: Pin and execute the wire compatibility fixture

**Files:** Create (proposed) `sdk/internal/acpcompat/compat_test.go`; modify `go.mod`, `go.sum` only to pin the actually tested SDK if suitable; write evidence `docs/logs/<date>-issue1-acp-g0/verification.md` (proposed; date at execution).

**Interfaces:** Consumes official stable v1 schema/docs and SDK v0.0.4 transport and generated types. Produces the checked `initialize`, `session/new`, `session/prompt`, `session/update`, `session/cancel`, `session/request_permission`, `elicitation/create` field/method matrix and a reproducible Go fixture. The fixture does not import an ACP type into a Vivy Port.

- [ ] Write a failing, table-driven fixture for the supported JSON-RPC envelope and stable schema fields, success/error replies, form capability present/absent, cancel while a reverse request is pending, EOF, an oversized input frame, a blocked/broken writer, and the sensitive logger sentinel. Assert no diagnostics on protocol stdout.
- [ ] Run `go test ./sdk/internal/acpcompat -count=1 -v`; record the initial failure or missing SDK dependency, without marking compatibility accepted.
- [ ] Add the pinned candidate dependency and the minimum test harness using the SDK's actual connection/stdio API. Do a field-by-field comparison against upstream stable schema, capturing serialized bytes and SDK version. Inspect `go list -deps ./sdk/internal/acpcompat` for non-stdio transports and the default logger path.
- [ ] Rerun the focused test and record output/command in the G0 log. Expected: fixtures pass **only** if wire fields, methods, ordering, bounds, logger, and disconnect behavior meet the planned subset. If SDK fails, file the exact mismatch and stop for a dependency decision; never hand-code a second ACP schema as a workaround.

### Task 2: Freeze the normative contract and owner review

**Files:** Modify existing `docs/architecture/ACP-REMOTE-CONTROL-PROPOSAL.md`, `docs/architecture/VIVY-FACE-PACK.md`, `docs/TODO.md` (verify paths before edit); create (proposed) `docs/logs/<date>-issue1-acp-g0/{summary,acceptance}.md`; update [index](index.md) only on actual evidence.

**Interfaces:** Consumes Task 1 fixture output. Produces a single linked normative contract for supported stable v1 methods/types, event allowlist, failure/stop mapping, client capabilities, limits, connection ownership, local threat model and Face-code versus UI-asset omission. ACP-02 through ACP-05 consume the reviewed revision and its evidence link.

- [ ] Write a contract-review checklist against design A1–A5 and make it fail review for any unresolved SDK incompatibility, unauthenticated session mutation, raw Journal forwarding, or stdout log path.
- [ ] Revise the three existing docs to remove contradictory deferred/remote or non-Face language; preserve remote control as deferred. Reference the design and test fixture rather than duplicating changing implementation details.
- [ ] Run `go test ./sdk/internal/acpcompat -count=1`, relevant document checks and `just ci`; expect pass in a full development environment, attach transcripts and the field matrix to the G0 log. Current planning environment cannot run these commands.
- [ ] Request/record the Issue owner's G0 review and only then mark ACP-01 Done in the index. If review requests public scope changes, revise architecture and downstream plans first. No G1 scheduling is inferred from approval of this plan.

**Handoff:** Supply the frozen contract revision, fixture and log, actual SDK/API fields and owner review to ACP-02. Report any untested stable field explicitly. Escalate wire incompatibility, public contract or scope changes rather than treating them as routine implementation choices.
