# Verification

Candidate and final gate results will be recorded after the integrated checks.

Observed so far:

- Native independent BML baseline failed on six existing test-owned Home handles
  left open during TempDir cleanup. Updated those tests to close their homes;
  no production lifecycle workaround was added.
- New v2 regressions first failed on emitted digests, schema1, write-before-refusal
  and relocation changing owner. After implementation the complete independent
  BML suite and `go vet ./...` pass.
- New unowned-artifact regression first showed existing output overwritten;
  directory ownership guard fixes it. Exact restart allocation and raw/normalized
  conflict tests pass.
- New linked-directory rollback regression first deleted a foreign marker;
  rejection before touching outputs fixes it. The regression now passes natively.
- MEMRULES focused BML tests, memory package tests and 10 source UI cases passed
  on the isolated consumer branch; integrated checks remain to be recorded.
- GitHub clone failed during pinned bootstrap (host DNS/getaddrinfo failure).
  A local clone of the exact WSL Laputa source was used instead; the repository
  bootstrap verifies clean pin ff3936f44ff8cf08c12af2cf698c194cfe474fd3.

Not yet claimed: independent Linux wiring/kernel/PostgreSQL, actual split UI,
finite converter-to-target reopen, fresh whole-branch review, D12 acceptance,
full-product release or D14 converter assembly.

## Integrated BML and converter results

- Independent BML `go test ./... -count=1` and `go vet ./...` pass on native
  Go 1.26.4 after the final fixes; no skip was observed. Integrated root
  `go test ./internal/modules/memory -count=1 -timeout 10m` passes against this
  lane's independently verified pinned Laputa.
- Independent review found incomplete target DDL/record checks and interrupted
  rollback gaps. Both new regressions failed first; final target preflight checks
  owned DDL constraints/FTS plus actual record JSON before WAL, and rollback keeps
  durable authority until the last deletion. Full BML suite/vet pass afterward.
  Narrow independent rereview found no remaining actionable issue.
- Finite converter source is **f7875b21a65b5a45f72ce83e084f4da300b45487** on
  `codex/issue91-d12-rules`, `tools/bml-upgrade`. Its old readers are deliberately
  absent from this runtime branch. Standalone converter tests/vet pass natively.
- The two cross-version bridge cases pass against the actual current BML target:
  schema1 copy -> OpenExisting/Integrity -> close/relocate/OpenExistingCanonical;
  artifact1 copy -> target manifest parser -> unverifiable-ID rejection -> owned
  rollback. IDs, content, revision8/store accounting, tombstone target and owner
  survive. See target-runtime-bridge_test.go.txt for the executed test source.
- Earlier Linux candidate dcad2bb6: `just wiring-ci` passes 16 bootstrap/guard
  Node tests plus 105 Go cases with zero skips. Final candidate gates follow.

Bridge reproduction: in the separate converter module copy the archived bridge
source to bridge_target_test.go, copy go.mod/go.sum to bridge.mod/bridge.sum,
run `go mod edit -modfile=bridge.mod -require=github.com/ProjectViVy/agent-vivy/bml@v0.0.0
-replace=github.com/ProjectViVy/agent-vivy/bml=<absolute-target-checkout>/bml`,
then `go test -modfile=bridge.mod ./... -run '^TestBridge' -count=1`. Remove the
three temporary bridge files afterward. This intentionally uses a temporary
modfile; production converter code has no BML/root dependency.
