# Windows Issue58 binding failure diagnosis — phases 1–3 only

**Finding:** the shared conformance identity fixture persists Unix-only `owner_path: "/trusted/owner"`. Windows correctly rejects that identity before source admission because it has no drive/UNC volume and is not a native cleaned absolute path. The reported Issue58 failure cluster is therefore a fixture portability failure at the existing identity guard, not evidence that Recap CAS, source grants or native Windows owner replacement failed. Do not relax any production validation.

Scope: read-only systematic-debugging phases 1–3, using the skill's root-cause/pattern/hypothesis sequence. Inspected fixed clean Vivy `fdcb25a554df79c7938287b89a539c905804ca63` at `agent-vivy-issue58-native-pin-validation`. No source/test/index/dependency edits, commits, builds or diagnostic execution were made. The existing Windows CI log is the native reproduction; local Go source inspection establishes the platform rule. No new Windows pass is claimed.

## Evidence and data flow

Original preserved log: `/tmp/issue58-windows-backend-9a1a21-job116707597299.log`, 143690 bytes, SHA256 `ecf8437fbf17a66cd02669f5ce05f32656f903bf295c05f58240f07e8b6921a2`. Root identifies run 38817922217, backend job 116707597299, branch 9a1a / merge 008aa27 into main 571. The abbreviated CI commit object is not available in this local Git object database (`git show 9a1a21:...` returned invalid object); source inspection below is explicitly fixed fdcb, not an invented byte-for-byte read of that CI commit.

| Boundary | Inspected source / observed output | Consequence |
|---|---|---|
| Shared test bootstrap | `internal/storage/conformance/subject_history.go:Issue58Identity` directly writes fixed UUIDs and the literal `/trusted/owner` JSON into `organism/subject/v1`. It bypasses App `ResolveSubjectBinding`. | Valid Unix fixture identity; invalid Windows identity. All users of this helper share the defect. `rg 'owner_path' internal` finds this single hard-coded fixture path. |
| Domain identity check | `internal/domain/subject_history.go:SubjectIdentity.Valid` requires both `filepath.IsAbs(OwnerPath)` and `filepath.Clean(OwnerPath)==OwnerPath`, in addition to exact UUID/domain constraints. | `/trusted/owner` is rejected on Windows. The UUID/domain values themselves match the source fixtures. |
| Actual platform semantics | Go 1.26.4 `src/internal/filepathlite/path_windows.go:IsAbs`: zero `volumeNameLen` returns false. `/trusted/owner` has neither drive prefix nor double-separator UNC prefix, so volume length is 0. `src/path/filepath/path_test.go:TestIsAbs` explicitly says all non-Windows test paths without volume letters must fail on Windows; Windows cases include `/`→false and `\Windows`→false. | This is a deterministic host-path contract, not a hypothesis about slash handling in SQL. `Clean` would also normalize slash separators, so changing only the IsAbs check would still not establish the required canonical identity. |
| Shared SQL identity guard | `internal/storage/subjectsql/subjectsql.go:identity` reads the snapshot and returns `ErrSubjectBinding` on `!id.Valid()`. `RegisterAdmissionTx` calls it after source DTO validation. SQLite `run_admission.go:commitRunAdmission` calls that registration before inserting/pinning a new admitted run; continuity uses the same registration. | The first source admission is refused before provider work or C2 sealing. This guard is shared by SQLite/PostgreSQL, so a backend-specific grant/string comparison is not needed to explain the cluster. |
| Observed App symptom | Log lines 660–673: selected continuity shows `messages=0 runs=0 provider calls=0`; SingletonAdmission, ExactTextInteractionSeals and FormattedGrantAuthorizedRead fail at first primary commit with `storage: subject binding mismatch`. Their fixtures call Issue58Identity. | Failure precedes native summary/provider/owner receipt paths. Rewriting grants or capacity logic would be a symptom fix. |
| Observed runtime/storage symptom | Log lines 763ff: source/provenance/compaction fixtures fail at initial source creation. Lines 1212–1249: SQLite SubjectSealAcrossHandles, InteractionOrderAndDedup, SubjectContextSnapshot/race/copy etc fail at admission. These share Issue58Identity. | Breadth follows shared bootstrap, not many independent Windows source-retention failures. Raw log contains 184 binding-mismatch lines and 214 Issue58 fail headings across 99 distinct top-level names (headings include subtests; these are diagnostic counts, not an executed acceptance count). |

## Working-pattern contrast

Production `internal/app/assembly_cognitive.go:cognitiveBundleForAssembly` uses the sealed generated factory and passes its **actual** ACTMEM root into `ResolveSubjectBinding`. `internal/app/subject_binding.go` obtains an absolute path, resolves existing ancestors with EvalSymlinks, rejoins missing suffixes with filepath.Join, then marshals a domain.SubjectIdentity. A reopened App validates that identity and exact canonical owner path. It does not invent a Unix path or trust a caller-provided subject.

`TestIssue58RealAssemblyRecapNativeToolsAndRestart` uses that actual App constructor and reads its bound owner's subject/domain for the trusted main resolver. It does **not** call Issue58Identity. The Windows log shows its actual temporary workspace twice (`D:\a\_temp\TestIssue58RealAssemblyRecapNativeToolsAndRestart...\001\workspace`, lines 602–603), consistent with the close/reopen path, and does not list this test as failing. Ordinary `go test` output does not print each passing test, so this is a source/trace contrast, not a newly fabricated individual PASS event. Existing SubjectIdentityStable/path-mismatch/concurrent-first-registration tests also use ResolveSubjectBinding with filepath.Join(t.TempDir(),...), rather than the faulty literal.

The two App crash/close tests use an intentionally direct test bundle and call Issue58Identity while seeding their core source rows. They fail there (log 597/600), before CompactSession or shutdown settlement assertions. Subsequent Windows TempDir cleanup messages about `garden.db` being in use follow that early Fatal before their normal CloseContext call; they do not prove a second production owner or a failed tested shutdown barrier. Registering fixture cleanup immediately upon open would make such failures cleaner, but is secondary to the invalid identity.

## Hypothesis test and smallest correction/reproduction

Single hypothesis: replace only the shared fixture's Unix literal with an OS-native, clean absolute test-owned owner path; unchanged production admission should then accept the same UUIDs/grants on Windows. Static evaluation of the exact domain guard against Go's Windows IsAbs implementation confirms the rejection mechanism. Linux passes are consistent because `/trusted/owner` is absolute and already clean there. No local simulation/cross-compile is represented as native Windows execution.

Smallest proposed source-lane change: in `conformance.Issue58Identity`, construct `domain.SubjectIdentity` with existing constant IDs/domain and `OwnerPath: filepath.Join(t.TempDir(), "owner")`, assert `id.Valid()` in the fixture, and `json.Marshal` it before Snapshot.Put. JSON marshaling is necessary for native Windows backslashes; do not interpolate unescaped paths into raw JSON. The persisted identity remains stable for that disposable backend's reopen. For a fixture intentionally testing actual App owner-path equality, use that App's real resolver/root instead; do not overwrite a real registered identity.

Do **not** use `filepath.FromSlash("/trusted/owner")` alone (still has no Windows volume), accept root-relative paths, drop canonical/equality checks, infer a new tenant binding, or special-case production SQL validation for tests. This is not a tenant migration and must touch only disposable fixture construction.

Existing native Windows RED reproduction is already recorded. After the sole source writer is released, use TDD at the fixture boundary and rerun on actual Windows at one frozen candidate:

- A fixture identity round-trip must be Valid, absolute/native-clean and retain identical serialized identity after reopening the same core database. Include a negative assertion that a root-relative/unclean identity stays rejected; keep existing alias/changed-owner tests unchanged.
- Minimal existing regression commands: `go test ./internal/storage/sqlite -run '^TestIssue58InteractionOrderAndDedup$' -count=1` and `go test ./internal/app -run '^TestIssue58(SingletonAdmission|RealAssemblyRecapNativeToolsAndRestart)$' -count=1`, then the approved broader Windows gates. Preserve the original RED and every subsequent result. No Windows qualification until these execute natively.

This explains the subject-binding cluster only. The same backend log independently reports `TestDefaultGenerationBaselineInventory` expecting an older inventory without Recap module/tools/actions (lines 587–588); that is a separate fixture failure and would not be fixed by the path correction. The separately assigned Web key-classification/UI failure is outside this diagnosis. Any remaining Windows failure after the fixture correction needs its own preserved evidence; do not declare the whole job fixed from this analysis.

## Root subsequent remote-source verification and correction

Root fetched all three actual PR merge008aa27b source files directly through
GitHub: conformance/subject_history.go, domain/subject_history.go and
subjectsql/subjectsql.go. Each is byte-for-byte equal to fixedfdcb. SHA256 proof
is retained in the companion CI-fix archive; this closes the source-identity gap
noted in the original read-only analysis without pretending the merge object
was locally available to the analyst. Root changed only the shared test fixture
to native filepath.Join plus typed JSON marshal/Valid assertion. Production
SubjectIdentity.Valid, SQL identity checks and permissions remain unchanged.
The independent stale inventory fixture is explicitly updated for the previously
reviewed Recap module, five tools and seven actions. Web classification adds the
two missing existing keys, without validator changes.

Source7f10ec6649eaae1e51aa15c4ae04ee12cec6a743/tree6d70b711 is published
as remote be19b94f8ca633fbf9f51c327023a652aeffd83a with exact tree verified.
Root fixed-candidate App/runtime/SQLite selection PASS269tests+3packages,
1opt-in SplitFixture skip,0fail,34.767s. Cross-face classification, completeness
and eight unchanged validator tests pass separately. These are Linux executions;
actual Windows rerun is pending. No final product CI or BML acceptance is inferred.
