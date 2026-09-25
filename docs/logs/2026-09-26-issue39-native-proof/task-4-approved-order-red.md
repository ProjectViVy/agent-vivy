# Task 4 owner-approved Service/Eino runtime RED

## Outcome and approved order

This delivery establishes only the compileable real-Service/Eino RED boundary.
It does not decide G0 GO or NO-GO.

The owner approved this proof order after the earlier test-only investigation
found no compileable Service/Eino lifecycle seam:

1. PIN the unchanged real Service approval path.
2. Write the desired real Eino Workflow integration test.
3. Add only a package-private Service lifecycle method with typed request and
   result values that returns `ErrNativeOrchestrationUnimplemented`.
4. Capture the named runtime RED.
5. Leave lifecycle behavior and the G0/G1 gates for Task 5 and later work.

The scaffold implements no child activation, graph lifecycle, policy,
checkpoint recovery, schema, scheduler, or model loop.

## Source and capability check

- Source HEAD before and after the proof checks:
  `b77090618c175c8c1dec3aeafcb3576d5d5e0b38` before the focused commit.
- `go.mod` pins `github.com/cloudwego/eino v0.9.13`.
- Local pinned source inspected:
  `C:\Users\Administrator\go\pkg\mod\github.com\cloudwego\eino@v0.9.13\compose\workflow.go`.
- `Workflow.End()` is implemented at `compose/workflow.go:165`.
- `WorkflowNode.AddInput` is implemented at `compose/workflow.go:197`.
- `WorkflowNode.AddDependency` is implemented at `compose/workflow.go:300`.
- Workflow is backed by `AllPredecessor` DAG execution and does not support
  cycles. The proof uses `End()` and explicit `AddInput` mappings. It does not
  use deprecated `AddEnd`, unsupported `WithMaxRunSteps`, or a custom
  scheduler.
- Checkpoint capability remains available in the pinned module at
  `compose/checkpoint.go` and `compose/resume.go`, but this RED scaffold does
  not wire or claim recovery.

Eino imports remain confined to `internal/runtime`. The test builds a real
`compose.Workflow`, binds both A and B lambda nodes to the method value of the
real `*Service` returned by `newTestService`, explicitly maps both typed node
outputs into a join, and maps the join to `Workflow.End()`. It does not call
independent `Service.Run` methods, use a mocked Service, or claim a standalone
lambda as integration proof.

Review ruling: the scaffold deliberately does not read Service state before
returning the sentinel. Such a read would add behavior beyond the owner's
approved behaviorless entrypoint and still would not prove authority,
checkpoint, approval, or broker integration. This RED proves Eino dispatch to
the Service-owned method; Task 5 must make the lifecycle boundary substantive.
If that scope ruling is wrong, this artifact proves method dispatch but not yet
Service authority, and Task 5 remains the required correction point.

## PIN baseline

The pre-change PIN ran before either Go file was added. Its exact invocation,
bounded to 120 seconds, was:

```text
'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run '^TestServiceApprovalApproveFlow$' -count=1 -v
```

Exit code: `0`.

```text
=== RUN   TestServiceApprovalApproveFlow
--- PASS: TestServiceApprovalApproveFlow (0.94s)
PASS
ok  	agent-vivy/internal/runtime	1.200s
```

This named PASS was captured before either Go file was added.
Its raw command output was visible in the execution session and transcribed
above, but it was not initially retained as an independent raw log artifact.
That chronology is therefore reported truthfully rather than inferred as an
independently durable fact.

### Post-change PIN reproduction and parent-tree proof

After the focused commit, the same bounded PIN command was rerun and its raw
output was captured:

```text
=== RUN   TestServiceApprovalApproveFlow
--- PASS: TestServiceApprovalApproveFlow (1.46s)
PASS
ok  	agent-vivy/internal/runtime	1.666s
```

Exit code: `0`. This is explicitly a **post-change reproduction**, not a
backdated pre-change capture.

The PIN-owning production and test files are byte-identical between the Task 4
parent and the reviewed source:

```text
git diff --exit-code b77090618c175c8c1dec3aeafcb3576d5d5e0b38..HEAD -- internal/runtime/service.go internal/runtime/approval_test.go
```

Exit code: `0`; output was empty. Blob identities also matched:

```text
parent service.go:         927b318d49c747eeeaf5664e326c7e73451b2090
reviewed service.go:       927b318d49c747eeeaf5664e326c7e73451b2090
parent approval_test.go:   95f227ef595a99e3fd260774a1d3dd45566b450d
reviewed approval_test.go: 95f227ef595a99e3fd260774a1d3dd45566b450d
```

Thus the post-change reproduction exercised the same `Service` implementation
and `TestServiceApprovalApproveFlow` source as the parent. It strengthens the
unchanged-source characterization without claiming to reconstruct the initial
run's chronology.

## TDD transition and named runtime RED

The desired test was written first. Its initial run failed to compile only
because `nativeOrchestrationRequest`, `nativeOrchestrationResult`, and
`Service.runNativeOrchestration` did not yet exist. The approved scaffold was
then added. The acceptance evidence below is the post-scaffold runtime RED,
not that preliminary compile failure.

Manual-QA invocation, bounded to 120 seconds:

```text
'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run '^TestOrchestrationNative$' -count=1 -v
```

Exit code: `1`, intentionally non-zero.

```text
=== RUN   TestOrchestrationNative
    orchestration_conformance_test.go:53: invoke native orchestration workflow: [NodeRunError] runtime: native orchestration unimplemented
        ------------------------
        node path: [b]
--- FAIL: TestOrchestrationNative (0.96s)
FAIL
FAIL	agent-vivy/internal/runtime	1.211s
FAIL
```

Binary observable: the exact named test was discovered, the Eino Workflow
entered a bound real-Service node, Eino reported the node path, the typed
sentinel cause surfaced as `runtime: native orchestration unimplemented`, and
the test confirmed it with `errors.Is`, and the process exited non-zero. There
was no `[no tests to run]`, import failure, compile failure, or acceptance of an
unrelated runtime error.

Determinism invocation:

```text
'/mnt/c/Program Files/Go/bin/go.exe' test ./internal/runtime -run '^TestOrchestrationNative$' -count=3 -v
```

Exit code: `1`. `TestOrchestrationNative` ran three times and the sentinel
cause appeared three times. The executions reported node paths `[a]`, `[b]`,
and `[b]`. Eino may schedule A or B first; both are the same real Service-owned
lifecycle boundary and both return the same sentinel.

## Static checks and deferred gate

```text
'/mnt/c/Program Files/Go/bin/go.exe' vet ./internal/runtime
```

Exit code: `0`; no diagnostics.

```text
git diff --check
```

Exit code: `0`; no diagnostics.

Full `just ci` is intentionally deferred until Task 5 turns this approved RED
proof green. A product-wide green gate cannot be truthful while the required
named proof is intentionally failing.

## Identity accounting at this RED stage

The proof intentionally has no graph Run ID, child/activation Run IDs,
ChildSession IDs, tool-call/effect IDs, or graph checkpoint ID. Creating those
identities would be lifecycle behavior beyond the approved sentinel scaffold.
Their absence is the boundary exposed for Task 5; it is not evidence for a G0
verdict.

## UltraQA probes

- `stale_state`: passed. HEAD was
  `b77090618c175c8c1dec3aeafcb3576d5d5e0b38` before the focused commit and
  Eino was v0.9.13 before and after implementation.
- `dirty_worktree`: passed. Initial status contained only the prior untracked
  `docs/logs/2026-09-26-issue39-native-proof/` directory. The prior
  `task-4-verification.md` SHA-256 remained
  `fe0d3e4578a811d941d00e40cc0544136e093232953d9a2f224fcb081e96a92a`.
  Final status also contains an untracked `.omo/` directory created by the
  independent reviewer. It is orchestrator-owned evidence state. Neither path
  was staged, edited, deleted, or treated as source by this delivery.
- `hung_or_long_commands`: passed. Every Go invocation used `timeout 120s` and
  completed within the bound.
- `flaky_tests`: passed for the intended RED. The three-count run discovered
  the named test three times and surfaced the same sentinel three times.
- `misleading_success_output`: passed. The manual result was inspected for the
  named test, Eino node path, sentinel, and non-zero exit; exit-zero
  `[no tests to run]` is rejected.
- `malformed_input`: not applicable; no parser or external input boundary was
  added.
- `prompt_injection`: not applicable; no prompt or untrusted model input was
  added.
- `cancel_resume`: not applicable at this RED stage; the scaffold implements
  no lifecycle or recovery operation.
- `generated_cache`: not applicable; no generator or cache was added.
- `repeated_interruptions`: not applicable; no interrupt/resume behavior or
  long-running QA process was added.

## Cleanup receipt

No server, browser, container, or background service was started. A final
process scan found no `go.exe test` or `runtime.test.exe` process. Temporary
command captures were kept only under `/tmp` and are not staged. No generated
cache or repository temporary artifact was created. The prior Task 4 report
was not modified. The untracked reviewer-owned `.omo/` evidence and inherited
`task-4-verification.md` report are deliberately preserved; deleting or staging
either would violate their ownership and the task's preservation requirement.
