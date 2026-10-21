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

## Final scoped runtime gates and real path

Code candidate: `17e3cb242376418a9c40eeec16d77cd067a4c145`, independently checked
at `/home/mastwet/work/vivy-issue91-data-features/agent-vivy` with its own clean
Laputa pin. Later log-only commits do not change this tested source.

| Check | Observed result |
| --- | --- |
| `just wiring-ci` | Pass: 16 Node guard cases and 105 Go cases; zero skips |
| `just kernel-ci` | Vet + 1654 Go cases pass; minimal Pack/Inspect/boot case passes separately |
| Kernel optional case | `TestBackendGrepRipgrepHonorsGitignore` skips because this Linux host lacks ripgrep; not counted as passing |
| Independent Linux BML module test/vet | Pass: 113 executed cases; zero skips |
| `just test-postgres` | Pass: 115 executed cases; zero skips, task-created postgres:16 container removed afterward |
| Official affected source UI suite | Pass: view.test.tsx (9) + rules.test.tsx (1), no temporary include override |
| UI typecheck/build | Pass using real Vite assets; existing large-chunk advisory remains |
| Changed-source producer | All 23 Provider suites pass (69.586s); generated conformance_results.json retained through its producer |
| SDK Module verification | `go run ./sdk verify ./plugins/vivy-memory` passes after existing source-hash producer refresh |

The first kernel attempt passed owner tests but failed minimal Pack because the
changed Memory UI source still had its previous declared pin. Refreshing via
`sdk/internal/cmd/source-hash`, reproducing conformance, and staging UI through
the SDK fixes the failure; no generated Assembly was hand edited.

Real split smoke used task-owned ports **8788/3016**, the existing
`VIVY_BACKEND_ADDR` proxy override and an explicit disposable VIVY_USER_HOME.
Standard 8787/3015 were already owned by the independent stable checkout;
those processes were not stopped or reused as candidate evidence. The mandated
**3015-specific smoke remains unrun** and is not claimed by the alternative-port
smoke. No embedded UI, tenant store or provider/model request was used.

Observed browser interactions:

1. Default governance refuses an effectful MEMRULES save and retains the draft.
2. Restart only the disposable test pair with its existing `full_auto` profile;
   the normal authenticated ActionHost policy path permits the test write.
3. Read default handbook, edit and save -> authoritative file-backed text shown.
4. Edit a draft, externally change only this task's synthetic handbook, save ->
   conflict displayed and draft retained.
5. Refresh -> external text shown; edit/retry -> final text saved successfully.
6. Add a synthetic memory -> visible live list row. Read-only inspection of this
   disposable memory DB confirms schema2, one opaque record, no retired JSON
   fields and no memory_apply_journal.

Screenshots: memrules-conflict.png, memrules-saved.png and memory-created.png.
The test pair and synthetic home are task scratch, not a retained tenant.
Full-product `just ci`, D02 Face protocol gates and race are not claimed: this
cut changes BML persistence/management payloads and the Memory UI, with no
runtime/Face orchestration changes. Application metadata parity/conversion still
waits for D03 integration; the passing unchanged PG suite does not accept it.


Owner steering (2026-10-21): standard3015 smoke is deferred for later verification;
continue implementation. This supersedes waiting for D06's port scheduling.
Alternative-port evidence above is retained with its original scope.
