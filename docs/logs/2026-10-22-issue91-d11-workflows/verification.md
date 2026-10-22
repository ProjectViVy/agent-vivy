# Verification

The author branch starts at ecaf7943. Main verification used independent Linux-filesystem checkout /home/mastwet/work/vivy-issue91-d11-wf/agent-vivy and its own clean sibling Laputa 9acc999301d5. GOFLAGS=-p=2 and Go 1.26.4 were process-local.

- RED: go test ./internal/storage/sqlite -run TestWorkflowCommitRejectsChangedActualEvent -count=1 -timeout 5m failed as intended: the old digest receipt accepted a changed actual event body under the same CommitID.
- PASS: go test ./internal/storage -count=1 -timeout 5m. Exact-input semantic change matrix covers private admitted input, resume key/answers, ProgramID, HostBindingID, result, checkpoint and event changes; exact integers above 2^53 remain distinct.
- PASS: go test ./internal/storage ./internal/storage/sqlite -run 'Workflow|INOFY' -count=1 -timeout 5m. Includes actual receipt conflict, lifecycle/epoch/terminal invariants, rollback of conflicting result rows, scoped publication, actual implementation identity, same-timestamp draft CAS, private resume changed-answer conflicts and restart persistence, and migration fixture paths.
- PASS: go build ./internal/runtime ./internal/rpc with actual parent/D10/child integration source borrowed into the Linux test checkout.
- Formatting: gofmt on owned changed Go paths; git diff --check is required immediately before the focused commit.

The Linux test checkout borrowed actual D10 042/043 migrations (subsequently committed as 878b7e13), child/Work b2955a6 source and 046 migration, and the parent's actual shared conformance/cognitive/service/mapper/control code. No fake migrations, substitute schemas, or production state were used. These borrowed dependencies are not included in this author commit.

Selected runtime/RPC tests were attempted but not completed in this lane: initial shared cognitive signatures and workflow fixtures were missing; later integration reached the shared SkillEnableRequest/mountSkillOps and removed SkillSummary.Hash fixture mismatches. The parent subsequently fixed those shared fixtures and owns the whole-candidate run. No selected runtime/RPC test pass is claimed here.

Not run in this lane: real PostgreSQL DSN tests, full wiring-ci/kernel-ci/d02 gates, UI tests/typecheck, split Vite browser smoke, race, Docker and native Windows acceptance. These remain explicit integration obligations. No full product/release acceptance is claimed from the focused owner tests.
