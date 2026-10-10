# Verification

Baseline: targeted existing payload-limit and missing-checkpoint tests passed (0.146s) before edits. The new regression failed on the intended bug: a real acknowledged steer needed a 65,538-byte future restoration marker against the 65,536-byte ceiling. Both encoded track fields account for the eight-byte growth.

Focused command: GOMAXPROCS=2 go test -p 2 ./internal/runtime -run 'TestSteerEnqueueReservesFutureRestorePayloadBoundary|TestBoundedSteerPayloadAdmitsAfterMissingCheckpointFallback|TestQueuePayloadLimit|TestSteerMissingCheckpoint' -count=1. PASS (0.287s).

The boundary regression probes both sides using real enqueueTurn, checks rejection leaves live queue and journal unchanged, reconstructs the queue, and verifies actual persisted track stays steer. The second regression uses the existing scripted native tool/model flow, accepts a turn just below the restoration boundary, disables checkpoint resume, and verifies successful durable follow-up admission.

The real Go toolchain and existing verified Laputa/module cache were reused. Sourcehash inputs affected: internal/runtime/queue.go and internal/runtime/queue_payload_boundary_test.go (the canonical internal source tree includes test files). No manifest/hash/generated artifact is edited in this lane. Root owns final required just ci, source sealing/regeneration, and clean packaging after integration.

Focused race command: the same four-test selection with go test -race -p 2 and -count=1. PASS (5.152s), with no race reports. git diff --check: PASS. Root's independent read-only review of the final diff passed with no blocking finding.
