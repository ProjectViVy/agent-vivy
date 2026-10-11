# SSH-01 worker wire implementation plan

> **For agentic workers:** Use superpowers:executing-plans for native execution. Steps use checkboxes; follow the index's readiness gates before coding.

**Goal:** Establish a strict, bounded and versioned data contract shared by worker and plugin.
**Architecture:** Independent worker module; data-only wire package with no Host, execution, credential or LLM dependency.
**Tech Stack:** Go 1.26.4 standard library, JSON framing and SHA-256.
**Spec:** [SSH-D1](../../specs/2026-10-11-code-ssh-design.md) §5; [SSH-C1](contracts.md); baseline `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`.
**Epic / requirements:** E0 / R1, R3-R5, R8. State and predecessors: [index](index.md).

## Global constraints and review focus

Do not implement worker side effects or imply a Host-issued identity exists.
Strict envelope and payload parsing precede dispatch. Count decoded bytes and
encoded frame independently. Review duplicate keys, version mismatch, malicious
frame lengths, arbitrary operation IDs and same-content/different-ID commands.

## Prerequisites and contracts

No Story predecessor. Execution needs accepted SSH-D1/SSH-C1. Consume their
exact fields, method table and limits. Produce wire.Binding/CallScope/Request/
Reply/Fault/Limits/Receipt and Encode/Decode/EncodeReply/DecodeReply/
CanonicalRequestDigest/BindingDigest/DecodePayload signatures. Worker and plugin
consume the same package; no duplicate protocol structs in transport code.

## Task 1: Typed shapes, framing and digest

**Files:** NEW proposed `extensions/ssh-worker/go.mod`,
`wire/{types,payloads,codec,digest}.go`, `wire/{codec,digest}_test.go`.

- [ ] Create the independent module (module path in SSH-C1, Go 1.26.4) and
  table-driven tests `TestFrameRejectsBeforeDispatch`, `TestPayloadClosedSchema`,
  `TestRequestDigestIncludesBindingAndScope`, `TestIdenticalCommandsDifferentIDs`.
  Cases cover duplicate nested keys, unknown fields/method/version, invalid UTF-8,
  truncated frame, zero/oversized length, trailing JSON, integer overflow,
  base64 overhead, and payload limit. Assert errors before a dispatch spy runs.
- [ ] In the module directory run `go test ./wire -count=1`; capture meaningful
  red compiler/assertion output. Missing Go/network/dependencies are environment
  failures, not proof the target behavior was tested.
- [ ] Implement length-prefix read with allocation check first, strict recursive
  key/type checks, typed method payloads and deterministic typed digest. Exclude
  only request_digest; include binding/scope/payload. Decoder accepts one JSON
  value and never coerces unknown fields or numbers. Implement reply codec and
  validate terminal result/fault shape and request/reply identity agreement.
- [ ] Run `go test ./wire -count=1` and `go vet ./wire`. All malformed frames
  fail safely; valid requests round-trip exact bytes/opaque strings.
  Commit as `feat: define strict SSH worker wire contract`.

## Task 2: Bounds, compatibility and parser pressure

**Files:** NEW proposed `wire/codec_fuzz_test.go`, `wire/testdata/fuzz/` seeds;
UPDATE `contracts.md` only for a reviewed correction; proposed Story log.

- [ ] Add `TestLimitsUseHostWorkerMinimum`, `TestReplyDigestMismatchRejected`,
  `TestUnknownVersionRefusesAdmission`, `FuzzDecodeRequest` and `FuzzDecodeReply`.
  Seed deeply nested JSON, duplicate payload keys, hostile lengths and malformed
  base64. Assert no panic, no unbounded allocation and no valid dispatch on errors.
- [ ] Run `go test ./wire -count=1`, then each fuzz target with
  `go test ./wire -run '^$' -fuzz '^FuzzDecodeRequest$' -fuzztime 30s` and the
  equivalent Reply command. Record budget/corpus/results; no exhaustive-security
  claim follows from a short fuzz run. Run `go test -race ./wire -count=1` on a
  race-capable platform. Wire contains no side effects or global mutable state.
- [ ] Capture A02 and module/dependency/import closure. Verify wire does not
  import worker execution, agent-vivy/internal or Eino. Run `git diff --check`;
  commit as `test: cover SSH wire bounds and malformed frames`.

## Acceptance and handoff

Return exact wire signatures, protocol revision and A02 red/green/fuzz results
to SSH-02/04. Contract changes update all consumers and invalidate readiness.
No listener, process, SDK Port or product Recipe is created. Rollback removes
only these new source paths; it has no operator data to migrate.
