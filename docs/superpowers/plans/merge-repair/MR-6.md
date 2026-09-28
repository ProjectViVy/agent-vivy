# MR-6: latent test repairs exposed by the fix

Defects: D13, D14.

## Steps

1. `internal/runtime/reference_lifecycle_test.go`: `MaxContextBytes` 256 →
   8192. The prompt-instruction reservation (~785 B) plus preamble and the
   assembled model input (~5.2 kB) already exceed 256 B, so the fold could
   never trigger; 8192 keeps `TriggerPercent` meaningful. This test could not
   have run green on any ancestor containing it — the merge chain made it
   reachable again.
2. `internal/app/shutdown_test.go`: `channelhost.Deps` now requires
   `Deliveries` (migration 026 durable channel deliveries) — pass
   `a.backend`.
3. `internal/app/shutdown_test.go`: `RunFunc` signature gained
   `*domain.Provenance` — update the fixture.

## Evidence

`TestReferenceLifecycleCompactionManifestKeepsIDs`,
`TestAppShutdownStopsGoalAdmissionBeforeChannelDrain` green.
