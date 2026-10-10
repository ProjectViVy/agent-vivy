# Verification

Final serialized actual composition: 45 named tests/subtests pass, zero skips, exit 0 (chain-recall-serial-composition.jsonl). Final default App/module/catalog regression: 163 pass, 26 conditional skips, exit 0 (chain-recall-final-default.jsonl); skips are not acceptance. Runtime focused regression: 83 pass, zero skips, exit 0. Runtime race selection: 38 pass, zero skips, exit 0. Actual degraded App race: one pass, zero skips, exit 0 (chain-recall-degradation-race-close-trace.jsonl).

Retain the original three-profile absent-evidence RED, missing source/admission compile REDs, Eino text-part diagnostic, initial native collection-filter failure, disabled-control hydration assumptions, positive race deadline failures, native child race diagnostic, first Close timeout and first combined correction transport failure. Native contract RED/GREEN logs and focused commits are separate. These are developer diagnostics, not formal complete S08/S09/S11 or source attestation.

Use the task environment, named App source files excluding default_generation_test.go and official generated diagnostic overlay:

```bash
source /workspace/work/memory-loop/tools/environment.sh
mapfile -t memory_loop_sources < <(rg --files --maxdepth 1 internal/app -g '*.go' -g '!default_generation_test.go' | sort)
go test -overlay /workspace/work/memory-loop/tools/recall-diagnostic-overlay/overlay.json -json "${memory_loop_sources[@]}" -run '^(TestMemoryLoopRecallAfterProcessRestart|TestMemoryLoopRecallNegativeControls|TestMemoryLoopCorrectionAndDeletionInModelInput|TestMemoryLoopRecallDeadlineDegradesSafely)$' -count=1
go test -race -overlay /workspace/work/memory-loop/tools/recall-diagnostic-overlay/overlay.json -json "${memory_loop_sources[@]}" -run '^TestMemoryLoopRecallDeadlineDegradesSafely$' -count=1
go test -json ./internal/app ./internal/modules/diva-cognitive ./internal/modules/defaults -count=1
go test -race -json ./internal/runtime -run '^(TestContextSourceAdmissionCannotBeForgedByRequest|TestCognitive.*|TestMission.*|TestUnassignedMission.*)$' -count=1
```

Loopback permission is required. The 45-case full exact selection is archived in the DIVA checkpoint. Pulse/Recap's known untracked RED is deliberately outside this developer selection; formal S05 must execute it. Scripted responses are not live-model evidence.

The diagnostic overlay is emitted by the official SourceCatalog/Compiler/GenerateRuntimeAssembly/SealManifest APIs. Its temporary SDK helper is removed from the product checkout and retained as tools/recall-diagnostic-overlay/helper.go.txt. Recreate that temporary sdk/internal/cmd/memory-loop-diagnostic/main.go only to regenerate the diagnostic overlay, then remove it. Its manifest has no conformance/artifact attestation and its generation hash cannot attest later source. Full CI/conformance/SDK pack/Inspect/native identity/review are still due after source freeze.
