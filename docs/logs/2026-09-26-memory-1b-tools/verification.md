# Verification: MEM-1B memory_* tool providers + MEM-1A defect fixes

Environment: branch `feat/memory`, 2026-09-26; HEAD at closeout `1c32a14e`.

Gates are run by the session controller; see the session ledger for
results. The gate set for this slice:

## 1 — Per-slice gates

```text
gofmt -l internal/modules/memory                                        # clean
go vet ./internal/modules/memory ./internal/modules/defaults ./internal/app   # clean
go test ./internal/modules/memory -count=1
cd bml && go test ./...                                                  # untouched by this slice
```

Coverage: `provider_test.go` (malformed/empty-RunID `run.completed` acks as
`DeliveryCompleted` with no write and the cursor advancing; colliding
sanitized RunIDs dedupe separately; absurd cursor rejected as invalid),
`actions_test.go` (`update` preserves `evidence_refs` when omitted and
applies them when given; output-cap reason is `output_too_large`),
`module_test.go` (second `Open` returns the active service),
`tools_test.go` (definition ids/effects/schemas; invoke happy paths against
a temp store; write effect surfaces `Readonly=false`; unavailable-service
outcome; `limit=0` rejection).

## 2 — Manifest sealing

`vivy/memory-bml-tools` emits exactly one provider per declared
`std/tool@v1` PortRef id; `manifest.Tools` equals the generated provider
set (`memory_add`, `memory_get`, `memory_list`, `memory_search`,
`memory_update`, `memory_remove`); `Requires` marks `vivy/tool-host` used.
The six ids collide with neither `tools.AssemblyControlledToolNames()` nor
existing generated tool ids — `bindGeneratedTools` errors on duplicates, so
the seal is asserted by construction.

## 3 — Final gate

```text
go test ./internal/modules/memory ./internal/app ./internal/modules/defaults -count=1
just ci    # controller-run
```

## 4 — Closeout is docs-only

This commit adds `docs/logs/2026-09-26-memory-1b-tools/` and the index row;
no Go/TS source changes.
