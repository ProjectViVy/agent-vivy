# Verification

Run from this checkout with task-local environment.sh, exact native ONNX models, and the existing DIVA diagnostic overlay. Use the named root internal/app Go files excluding default_generation_test.go; default Generation is tested separately. Exact raw commands/outcomes/hashes are retained in DIVA docs/logs/2026-10-memory-loop-chain/v0.9.0-lifecycle-and-replay/.

Target lifecycle1pass/0skip/exit0; capture replay1pass/0skip/exit0. Final selected composition32pass/0skip/exit0. Default App133pass/22conditional skips/exit0; skipped overlay cases are not positive acceptance.

Initial lifecycle failures were fixture compile/type assumptions and incorrect public error/create-ID assertions, not product defects. The native facade intentionally mints create IDs; deleted evidence returns effect_not_found; stale CAS returns revision_conflict. All initial logs remain. Initial replay lacked an explicitly supported model mode; corrected to reflection with policy still disabled.

The selected composition explicitly omits unresolved actual S05 Pulse/Recap RED, preserved at the v0.7 checkpoint. Controlled cursor rewind is not a six-cut crash recovery proof. Full just ci/new-source conformance/SDK pack/Inspect/native identity and fresh whole-phase review remain due after local freeze. Existing generated source hashes do not attest this tree.
