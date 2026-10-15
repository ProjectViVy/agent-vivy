# S3 Task1 independent review

Reviewed3d5fede4a2d056e750001133d004148a3a501db7 against f3be3844ec4062cd33bab31794c34bfab8269a54. Reviewer: independent GPT-6 Astra High, read-only. No Critical findings. Task2 was explicitly outside this review.

| Priority | Finding | Smallest required repair |
|---|---|---|
| Important I1 | recap_context.go returned success on missing LIFE run pin, skipping source/epoch/capacity checks | Permit missing only for positively identified ordinary/delegated runs; resident marker/policy must deny; native second-leg/resume no-send tests |
| Important I2 | service.go FaceCode skipped governed primary preparation for trusted LIFE | Use trusted singleton policy independently of face; preserve Frozen Core/first-arrival contract |
| Important I3 | recap_capacity.go session fallback applied LIFE output policy to a child sharing main Session | Identify delegated execution before resident fallback; no implicit resident input or cap |
| Minor M1 | output_limit_test.go bound nil tools and failed before HTTP | Bind valid nonempty schema; retain exact HTTP and single-LiveSpec assertions |

The author independently found I2/I3 with named isolation RED fixtures before fixes. The independent reviewer corroborated their source paths at3d; later fixes were outside the reviewed SHA.

Independent exact3d command through local-only runner: `go test ./internal/runtime ./internal/provider -run '^TestIssue58' -count=1`. Runtime PASS7.002s; provider FAIL M1 (`reads=1 HTTP=0`, no tools to bind). External forwarding0, denied requests0. `git diff --check BASE HEAD` passed. Raw review evidence is in the root evidence manifest.

## Root acceptance of one fix pass — still not Task1 acceptance

Production fix f8d61b315dce27bfdd8a54548c30f872bdf85286, remote f8cb53ba10367523772f7976fe88630117ef9140, treeaae24648366b00f3eedeceeb6d3f36652d46c5b8. The shared resident classifier fences legacy compression on unavailable policy; final leg validation rejects missing resident pins/markers. Trusted LIFE primary preparation is independent of FaceCode, while children retain ordinary output policy. M1 now reaches real native bound HTTP with a valid tool.

Root independently executed six fix groups on isolated exactf8: `go test -json ./internal/runtime ./internal/provider -run '^TestIssue58(MissingResident|CodingWorkerWorkspaceIsolation|OneLiveSpecPerInvocationActualHTTP|RunPinFailureCannotSelectLegacyCompaction|RecapCheckpointResume)' -count=1 -timeout=120s`. PASS21 test/subtest events, exit0, external rejected0/forwarded0. This verifies the four findings only.

The subsequent author prescribed focused gate atf8 actually FAILed:146 pass events and5 fail events representing three old positive app cases plus parent/package. Their fixtures lack the newly required real bound Recap/capacity dependencies. Preserve positive success/seal/grant assertions and repair only fixture setup; no production rejection is weakened. Focused/race gates on the repaired test candidate remain pending.

S3 Task2 fold/reconcile/CAS/positive receipts, installed #66 binding, live model quality, Windows and PostgreSQL remain unverified. No overall S3 or Issue58 acceptance is claimed.
