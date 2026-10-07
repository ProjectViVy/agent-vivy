# VCP A2 — verification

All commands run on `feat/vivy-code-parity` at commit time, repo root.

## Focused suites (all PASS)

```text
go test ./internal/app/ -run 'TestCodeFace' -count=1 -v
  PASS TestCodeFacePrintModeStreamsAssistantText   — full kernel + real face + scripted
      OpenAI-protocol SSE model; stdout == "hello vivy\n" (assistant text only)
  PASS TestCodeFaceJSONModeEmitsOrderedRecords     — every line parses, all carry "v":1;
      ordered contract session→turn_start→agent_start→message_start→message_update+
      →message_end→turn_end→agent_end→agent_settled; reassembled deltas == "hi there"
  PASS TestCodeFacePrintModeCancelsBlockedRunLoudly — write_note prompt-gated;
      status "cancelled", stderr carries "requires human approval", no post-cancel text
  PASS TestCodeFaceJSONModeContinuesNewestSession  — second invocation with -c shares
      the first run's session id

go test ./sdk/facerun/ -count=1 -v
  PASS TestJSONLSinkMappingTable      — 23-case vivy-type → record-name coverage incl.
      passthrough (journal_event) and semantic-sink skips
  PASS TestJSONLSinkLifecycleRecords  — session/turn_start/turn_end/agent_end/agent_settled

go test ./faces/headless/...        ok (delegation preserved behavior, incl.
                                    digest-verification and blocked-run tests)
go test ./sdk/tui/face -count=1     ok
go test ./internal/codeface         ok
go test ./cmd/vivy-code             ok
go test ./sdk/...                   ok (all packages; conformance below)
```

## Conformance gate

```text
go test ./sdk/internal/conformance/ -run TestCheckedInProviderConformanceMatchesExecutedSuites
  ok (69.9s) — executed suites byte-identical to checked-in artifact after re-pinning
```

Digest toolchain used: `go run ./sdk/internal/cmd/source-hash <dir> <declared|''>`.
faces/headless: descriptor digest d9b48817 (self-zeroed) vs conformance tree digest
24ba4a2b (declared=""); reproduction_test.go pins the tree digest.

## Manual smoke

Deferred to story-level `just ci` per repo convention (35m+ sweep). The binary-level
check performed during A1 (`vivy-code --mode json -p hi` → ModeUnavailableError) is
superseded: the mode now runs through the same RunFaceWithAppOptions path the E2E
tests drive.

## Acceptance mapping

- `vivy-code --mode json -p "hi"` full ordered sequence on a real generation →
  E2E via in-process kernel + scripted SSE provider (wire-faithful).
- `print` returns assistant text only → stdout asserted byte-exact.
- Session flags: `--session-id`/`-c` honored; picker/fork/session-dir/no-session/
  export fail with named errors (documented deviation, spec RQ-CLI follow-up C1).
- `--no-session leaves no Journal session` → cannot be honored: every VIVY run is
  journaled; the flag errors instead of silently journaling.
