# Verification

Independent Debian Linux-filesystem checkout:
`/home/mastwet/work/vivy-issue91-runtime/agent-vivy`, accepted baseline
`d098d4e2acf0f413c2378d18964de368ac089447` plus explicit D05 patch. Own clean
Laputa pin `ff3936f44ff8cf08c12af2cf698c194cfe474fd3`, Go1.26.4, Node24.21.0,
GOFLAGS=-p=2. No other dirty checkout, tenant Journal or production credentials.

| Check | Result |
| --- | --- |
| Absent warming/steering factory accessor before implementation | RED; accessor absent |
| Runtime/App/compiler/catalog named preservation checks | 21 PASS, zero skip |
| Explicit selected/absent/invalid factory cases and lazy App binding | 8 PASS, zero skip; all new Factories names executed |
| just wiring-ci | 16 Node + 111 Go PASS, zero skip |
| just kernel-ci | vet PASS; 1731 PASS, one existing optional rg skip; real minimal Pack/Inspect/boot1 PASS |
| just race-ci | vet PASS; 1731 PASS, one existing optional rg skip; no race report |
| App factory/config/causal/settings/reload after review correction | 22 PASS, zero skip |
| Minimal Pack/Inspect/boot after App-only correction | 1 PASS, zero skip (7.21s) |
| Fresh independent review plus reload-fix re-review | PASS; no unresolved finding |
| Canonical source sync and git diff --check | PASS |

```sh
go test -json -tags=vivy_headless ./sdk/internal/assembly ./internal/app ./internal/runtime ./internal/moduleport -run 'TestGenerateRuntimeAssembly.*Factory|TestNudgeFactory|TestWarmingSteeringFactory|TestCacheWarm|TestSteer|TestQueue|TestToolResultReachesSecondModelRequest|TestEinoImportsQuarantined|TestPluginWindowCannotImportInternal|TestCoreCatalog|TestPublicSourceCannotProvideCorePort' -count=1 -timeout 10m
go test -json -tags=vivy_headless ./sdk/internal/assembly ./internal/app -run 'TestGenerateRuntimeAssembly.*Factor|TestWarmingSteeringFactoryBindingsAreLazyAndTyped' -count=1 -timeout 3m
just wiring-ci
just kernel-ci
just race-ci
go test -json -tags=vivy_headless ./internal/app -run 'TestLoopFactory|TestNudgeFactory|TestWarmingSteeringFactory|TestToolResultReachesSecondModelRequest|TestCompaction|Test.*Reload|Test.*Settings' -count=1 -timeout 5m
go test -json -tags=vivy_headless ./sdk/internal -run '^TestMinimalKernelArtifactBootAndInspect$' -count=1 -timeout 5m
git diff --check
```

The initial focused selector's singular Factory did not match the new plural
Factories names; the explicit corrected Factor selector above executed them,
as did wiring-ci. The reload helper regression first failed compilation before
implementation, then passed; no unsupported claim of a selected provider App
integration is made. Kernel/race owner source was unchanged by that App-only
correction; App22 and actual minimal boot were rerun after it.

The existing skip is TestBackendGrepRipgrepHonorsGitignore (rg absent in WSL),
not passing evidence. No schema/storage implementation changes: PostgreSQL
revalidation is not required or claimed here. No Windows full/native CI,
full-product just ci or real-token validation ran; these remain separate D14
lanes under the owner's scope. D13 supplies split3015 browser/mock proof.

Real paths retained: actual streaming/idle/off maintenance with savings and
paid cache-write accounting; diagnostic-only failure with no private error text;
live maintenance deadline and Run cancellation; same-Run native steering and
turn.steered; durable queue replay after restart; production App nonce read into
request2, final, exactlyone terminal and idle. No factory is invoked by these
checks. Review record documents the only correction.
