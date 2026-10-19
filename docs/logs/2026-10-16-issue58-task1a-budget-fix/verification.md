# Verification

The regression used an actual bound native 5001-rune record and its real receipt
revision. Before the production change, budgets -1 and minimum int panicked at
the rune-slice bound; five existing budget controls passed. The test recorded
3 failing events (the two subtests and parent), exit 1, 1.421s. After the single
bounds correction, all 8 test events and the package passed, 1.277s.

All commands below use the unchanged local-only runner with Go 1.26.4, offline
dependencies, credential stripping, and a rejecting proxy. Every attempt had
zero attempted or forwarded external requests. Shell: absolute `/bin/bash`,
`login:false`. Prefix each command with:

```text
python3 /tmp/issue58-localonly-runner.py /workspace/scratch/5dda642e718d/agent-vivy-issue58-recovery
```

```text
issue58-task1a-fix-budget-red go test -json -mod=readonly -p=1 ./internal/modules/memory -run '^TestIssue58BMLExpansionBudgetBounds$' -count=1 -timeout=2m
issue58-task1a-fix-budget-green go test -json -mod=readonly -p=1 ./internal/modules/memory -run '^TestIssue58BMLExpansionBudgetBounds$' -count=1 -timeout=2m
issue58-task1a-fix-normal go test -json -mod=readonly -p=1 ./internal/modules/memory github.com/ProjectViVy/agent-vivy/bml -count=1 -timeout=5m
issue58-task1a-fix-race go test -json -race -mod=readonly -p=1 ./internal/modules/memory github.com/ProjectViVy/agent-vivy/bml -count=1 -timeout=5m
issue58-task1a-fix-vet go vet -mod=readonly -p=1 ./internal/modules/memory github.com/ProjectViVy/agent-vivy/bml
issue58-task1a-fix-gofmt gofmt -l internal/modules/memory/backend.go internal/modules/memory/backend_budget_test.go
issue58-task1a-fix-diffcheck git diff --check
```

Normal: PASS 250 test events + 2 packages, 1.834s. Race: PASS 250 + 2 packages,
11.930s, zero races. Both include unchanged real BML and Mentle memorytest
conformance, the new budget controls, and all 162 native BML tests under root
SQLite1.59 MVS. No tests skipped. Vet PASS 0.466s; gofmt output empty; diff check
PASS. Native module source/dependencies were unchanged; its separate SQLite1.56
qualification was not rerun for this adapter-only fix.

Runner candidate metadata names parent `9ae26307`; these are final hashed
working-source results, not an immutable-parent test claim. Original root
reproducer, overlay and RED evidence are copied byte-for-byte into the external
fix evidence directory. The parent will rerun the unchanged original on the
immutable fixed checkpoint. Full CI, actual Windows, browser, Task1b and Task2
checks were NOT RUN in this bounded fix phase.
