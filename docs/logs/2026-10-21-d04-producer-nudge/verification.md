# Verification

Venue: independent Debian Linux-filesystem checkout
`/home/mastwet/work/vivy-issue91-runtime/agent-vivy`, candidate based on
`30af694dfe6e4f8963a71ce842b80575284f5963` plus the explicit D04 patch.
Own clean Laputa pin: `ff3936f44ff8cf08c12af2cf698c194cfe474fd3`.
Go1.26.4, Node24.21.0, GOFLAGS=-p=2, all Go gates use vivy_headless.
No other WSL checkout or tenant state was reused/reset.

| Check | Observed result |
| --- | --- |
| Exact tuple separator alias test on baseline | RED; distinct tuples collided |
| Corrected native no-consumer test on baseline30af694d | RED; model2 waited for derived acknowledgment |
| Absent typed generator slot before implementation | RED; required accessor absent |
| Initial kernel acceptance | RED; native tool_search producer and same-ID Work invocation paths exposed |
| Three independent-review regressions before correction | RED; malformed identity, missing size bound, duplicate terminal ownership |
| Focused runtime/App approval/operation/nudge/native-orchestration regressions after review | 98 PASS, zero skip |
| Final App lazy-factory, causal nonce, SDK selected/absent factory and import checks | 6 PASS, zero skip |
| just wiring-ci | 16 Node + 107 Go PASS, zero skip |
| just kernel-ci after all behavior corrections | vet PASS; 1729 cases PASS, one existing optional ripgrep skip; minimal Pack/Inspect/boot1 PASS, zero skip |
| Linux race nudge/flight/native metadata/Work identity | 48 PASS, zero skip; no race report |
| Fresh independent review and correction re-review | PASS; no unresolved correctness finding |
| git diff --check / source synchronization | PASS |

Executed focused regression command:

```sh
go test -json -tags=vivy_headless ./internal/runtime ./internal/app -run 'TestNudge|TestToolOperation|TestApproval|TestToolApproval|TestNativeOrchestration|TestServiceResumedRun|TestToolResultReachesSecondModelRequest|TestNudgeFactory' -count=1 -timeout 10m
go test -json -tags=vivy_headless ./internal/app ./sdk/internal/assembly -run 'TestNudgeFactory|TestGenerateRuntimeAssemblyNudge|TestToolResultReachesSecondModelRequest|TestEinoImportsQuarantined|TestPluginWindowCannotImportInternal' -count=1 -timeout 5m
just wiring-ci
just kernel-ci
go test -json -race -tags=vivy_headless ./internal/runtime -run 'TestNudge|TestToolOperation.*(Concurrent|Flight|Claim)|TestModelWorkIdentityUsesEinoCallIDForDistinctCallsAndRetries|TestToolSearchActivatesDeferredTool' -count=1 -timeout 8m
git diff --check
```

The optional skip is `TestBackendGrepRipgrepHonorsGitignore` because rg is absent
in this WSL toolchain; it is not counted as passing. No schema/storage changes
were made, so the D06 integration's real PostgreSQL125 evidence is consumed,
not rerun or relabeled as D04 storage verification. Full-product just ci,
Windows native/full CI and real-token acceptance remain unrun per the owner's
D14 boundary. D13 supplies split3015 browser/mock acceptance. Current real-path
D04 evidence is the production App/provider nonce read through actual Service,
RPC and governed ToolHost: exact request2 result/callID, final, one terminal, idle.

Residue: no FNV, digest tuple, Register/Complete/Seal consumer handoff or parked
mapper completion remains in the owning nudge path. Independent mapper
waitForToolsSettled remains for observed-stream accumulation/marker ordering.
Service continuity admission hashes belong to D09; Work/revision hashes belong
to D11; migration metadata to D12. No unrelated owner was deleted or renamed.
