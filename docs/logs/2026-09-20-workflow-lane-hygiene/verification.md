# Verification: root-tree lane hygiene

Environment: Windows, repository root, 2026-09-20. `just` gate from the
repository root; Go toolchain on PATH.

## 1. The strays were the only thing the carve-out was hiding

Before removal, `go vet` on the two packages a root-tree reader would run:

```text
go vet ./internal/workflow/ ./internal/domain/
# agent-vivy/internal/workflow
# vet.exe: internal\workflow\workflow_test.go:88:31: undefined: ValidationContext
```

After removal, with no exclusion anywhere:

```text
go build ./...
# exit 0

go vet ./...
# exit 0
```

`go build ./...` passed even before the removal: `internal/workflow` had no
non-test file on `main`, so the breakage was test/vet-only. That is exactly
why a package-name exclusion was the wrong mitigation — it made the gate look
at everything except the one broken thing.

## 2. The digest moved because untracked files were in it

```text
cd sdk
go run ./internal/cmd/source-hash ../internal ""

# before removal (two stray files present):
af2bcb75e3f80a04f22ee29c59226ad1cef8b4ae46737cce12be0813e65516f3

# after removal:
724b0f9a037f38c96a3cf03289560bb0cb46fe9eb8729d0bdc6fa3d1ce0d7e8d
```

`af2bcb75…` was also what `sdk/internal/assembly/conformance_results.json`
carried in its five `internal` suite rows, i.e. the checked-in evidence
matched a tree that included another lane's WIP. All five rows now carry
`724b0f9a…`.

## 3. The provider conformance producer gate passes

```text
go test ./sdk/internal/conformance/ -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1 -timeout 20m
ok  	agent-vivy/sdk/internal/conformance	145.669s
```

This gate recomputes the `internal` digest from the live tree, runs every
Provider/Host suite in `releaseSuiteCases`, and byte-compares the result with
the embedded artifact. It is the check that would fail if the new value were
wrong, and it passed.

## 4. `just ci` — first run red on two approval-durability tests

```text
just ci
--- FAIL: TestToolApprovalBindingCannotBeBypassedWhenResumeBecomesAllowed (4.28s)
    --- FAIL: TestToolApprovalBindingCannotBeBypassedWhenResumeBecomesAllowed/changed_arguments (1.81s)
        assembly_governance_e2e_test.go:644: runtime: approval is not durable yet
--- FAIL: TestServiceApprovalDenyFlow (0.47s)
    approval_test.go:602: decide: runtime: approval is not durable yet
FAIL	agent-vivy/internal/runtime	283.943s
```

Both failures are the same product error from `internal/runtime/service.go:2282`
(`runtime: approval is not durable yet`), raised while the test polls a
`decisionDeadline`. Run in isolation, the same selection is green:

```text
go test ./internal/runtime -run 'TestToolApprovalBindingCannotBeBypassedWhenResumeBecomesAllowed|TestServiceApprovalDenyFlow' -count=1 -timeout 10m
ok  	agent-vivy/internal/runtime	0.740s
```

The failure is load-sensitive and does not match this change: nothing here
touches approval persistence, and the two packages involved
(`internal/app`, `internal/runtime`) pass their own conformance and
approval tests when the machine is not running the whole suite in parallel.
Run 1's scope was exactly these two tests — `FAIL agent-vivy/internal/app
83.367s` and `FAIL agent-vivy/internal/runtime 283.943s`, one approval test
each; every other package, including `internal/app`'s assembly governance e2e
(the check that would catch a wrong digest), reported `ok`. One full-suite
observation is not enough to call it a flake, so further runs are recorded
below.

A second full run started immediately afterwards and was killed by the
session environment during the `test` step (log stops mid-suite with no `go`
process left), not by a test result; its log is
`.workspace/ci-hygiene-2.log`. Run 1 is also the only one that reached the
`test` step at all, so `headless-compile` and `plugin-ci` were still
unverified at that point.

## 5. `just ci` — further runs

Run 3 reached the `test` step and was then stopped on operator instruction
(no further testing; finalize and publish), so it produced no result.
`.workspace/ci-hygiene-3.log` holds the partial run.

Final state of the evidence for this change, with the limitation stated
plainly:

- **Verified:** `go build ./...`, `go vet ./...` with no exclusion, the
  provider conformance producer gate (§3, green in 145.7s), and run 1's
  `fmt-check → ui-ci → vet` prefix, which completed before `test` failed.
- **Not verified green in this session:** a complete `just ci`, and the
  `headless-compile` and `plugin-ci` steps — run 1 never reached them because
  it failed at `test`, and runs 2–3 were stopped.
- **Known red, unrelated to this change:** the two approval-durability tests
  in §4, recorded on the board as `TFLAKE-APPROVAL-DURABILITY`.

No run contradicted §4 or produced a different failure: every package that
reported before each run was cut off reported `ok`, and the change itself has
no Go diff outside the embedded `conformance_results.json` evidence value.