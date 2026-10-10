# Verification

RED records: `chain-unresolved-settlement-red.jsonl` (partial/unknown completed window advanced), `chain-unresolved-identity-red.jsonl` (original recovery identity cleared), `chain-rejected-app-red.jsonl` (public status omitted pending window/reason). Earlier isolation failure exposed the separately repaired literal SQLite URI path bug; its failure log is preserved independently.

Final directly affected commands and results:

- `go test -json ./internal/runtime -run '^(TestCognitive.*|TestINOFY.*|TestWorkflow.*|Test.*OneShot.*|TestUnknownOutcomeNoNewAttempt)$' -count=1`: 81 pass, zero skip/fail, exit 0.
- Diagnostic DIVA composition named-file run (excluding default_generation_test.go), reflection/large-source/rejected-update/Restart/control tests: 12 pass, zero skip/fail, exit 0. `chain-recovery-final-composition.jsonl`.
- `go test -json ./internal/app -count=1` with the untouched default recipe: 133 pass, 11 conditional skips, zero fail, exit 0. DIVA-only composition, opt-in capture and helper entrypoints require their diagnostic overlay/environment; default recipe skips do not constitute product acceptance.
- `TestINOFYWorkflowCancelPropagates -count=10`: 10 pass, zero skip/fail, exit 0; actual unresolved ledger distinguishes both legitimate race outcomes.

Final raw records live in the paired DIVA v0.3 recovery checkpoint. The diagnostic overlay is an earlier generated development artifact, not an attestation of new source. Required final just ci, conformance reproduction, regenerated pack/Inspect and native identity validation remain pending until the local chain is frozen.
