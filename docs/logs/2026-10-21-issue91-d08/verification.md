# D08 verification

## Candidate and venue

Implementation started from `stable@c065a82b4618e34dddd32893abd0557bd79c5b44`
(accepted D03). The Windows stable checkout is the sole writer. Tests use the
independent Linux-filesystem checkout
`/home/mastwet/work/vivy-issue91-runtime/agent-vivy` in Debian WSL, with its own
clean Laputa at `ff3936f44ff8cf08c12af2cf698c194cfe474fd3`. The old dirty D03
WSL checkout and other worktrees remain untouched. Go is 1.26.4, Node is 24.21.0,
pnpm is 10.33.0; `GOFLAGS=-p=2` limits build parallelism. Go owner tests use
`-tags=vivy_headless`. Candidate paths are synchronized explicitly; no Go/Node
verification runs on the mounted Windows checkout.

Gate output identifies the base commit plus a dirty candidate patch. A direct byte comparison passed for all 129 changed non-documentation paths;
Go formatting was clean and the verification checkout had no extra candidate
paths. The final source comparison and delivery commit bind that tested patch
to the Story commit; the tests were not misrepresented as having run on a future SHA.

## Required and affected checks

| Command / lane | Observed result |
| --- | --- |
| `just wiring-ci` | PASS: 16 Node tests, 105 Go cases, zero skips. Subsequent changes did not alter compiler/Assembly/bootstrap wiring. |
| `just test-postgres` (final candidate) | PASS: 123 cases, zero skips; actual disposable PostgreSQL 16 at loopback port 55438, no volume. Includes copied conversion, mask admission and continuity atomicity cases. |
| `just kernel-ci` (final candidate) | PASS: vet, 1730 Go cases, plus one minimal Recipe pack/inspect/boot case. One existing optional-ripgrep test skipped, identified below. No UI installation/build is part of this gate. |
| `just race-ci` (final candidate) | PASS: vet and 1730 race-instrumented Go cases; no race report. The same single optional-ripgrep case skipped. |
| `go test -tags=vivy_headless ./internal/storage/... -count=1 -timeout 35m` | PASS. No-DSN PostgreSQL skips in this preliminary lane are not counted as PostgreSQL acceptance; the separate real gate above supplies it. |
| `go test -tags=vivy_headless ./internal/contexthost ./internal/skillhost ./internal/mcphost ./internal/toolhost ./sdk/port/skillsource -count=1 -timeout 5m` | PASS, all five owners. |
| `go test -tags=vivy_headless ./internal/app ./internal/rpc ./internal/studio -run 'TestProductionMCP|TestToolApproval|TestToolEnvelope|TestControlHandlerUsesVersionedSnakeCaseContracts|TestInspect|TestHeadless|TestContinuityCodingLoop' -count=1 -timeout 5m` | PASS in all three packages; named assertions executed. |
| `pnpm --dir ui run typecheck` | PASS, actual generated UI staged by the existing pretypecheck owner. |
| `pnpm --dir ui exec vitest run src/lib/model-completion.test.ts src/lib/store.test.ts src/lib/run-subscription.test.ts src/lib/run-rows.test.ts src/components/chat/ChatView.test.tsx` | PASS, 5 files / 102 tests. |
| Split-browser approve, deny, hard restart while awaiting approval | PASS with actual backend and loopback mock; see `acceptance.md`, `browser-result.json` and screenshots. |
| Independent read-only review | One stale TODO finding corrected; no actionable code findings. See `review.md`. |
| `git diff --check` | PASS at final stage, including staged paths. |

`TestBackendGrepRipgrepHonorsGitignore` skips because `rg` is absent inside this
WSL distro. This existing external search-tool lane belongs to D09; it is not
approval/checkpoint coverage. No dependency was installed to turn an unrelated
optional lane green. All required D08 tests execute. Windows native packaged
acceptance is not run by this WSL lane.

## Failure-first and regression evidence

- New MCP name-collision, Context query identity and unversioned SkillSource
  tests failed on baseline behavior, then passed after direct-value changes.
- Checkpoint v2/missing-admission assertions first rejected baseline checksum and
  unbound writes. New tests cover v2-only framing, no write before admission,
  foreign Run and schema/composer/Generation mismatch. Existing tests cover
  engine mismatch, native decode, atomic generation rollback and reopen.
- `TestApprovalRecoveryKeepsPrivateAdmittedPolicy` and
  `TestApprovalRecoveryReusesAdmittedPromptAfterMaskEdit` pass: restart retains
  private actual values, changed policy refuses effect, current resolver/frame
  cannot replace the admitted prompt, and canaries stay out of public events.
- `TestBackendConformance/admission-rejects-forged-definition-at-same-revision`
  first showed forged name/body accepted. The shared transaction guard now
  rejects those values without a Run and accepts unchanged exact content.
- Shared continuity atomic tests first exposed missing prompt persistence and
  missing failure injection. SQLite/Postgres now reject after the prompt step
  without leaking a row and persist/reopen the exact prompt on success.
- Copied SQLite/Postgres conversion tests verify fresh target schema, recognized
  sources, failure rollback, source preservation, one terminal, interaction
  expiry, preserved operation/history/generations and target reopen. Existing
  released migrations 001–036 are unchanged.
- A full-runtime stall was traced with SIGQUIT to private-policy loading while a
  Journal cursor held SQLite's sole connection. Closing that cursor before the
  private load fixed recovery. The full runtime now passes inside kernel CI.
- Affected app recovery exposed a legacy unbound approval left approved after
  rejection. Registering the existing stale reporter before envelope decode now
  expires it before the Run terminal. Cached sibling, same-Run steering, direct
  checkpoint and external Plan fixtures were brought under real admission.

## Scope and limitations

The owner explicitly selected WSL wiring/kernel/race, real PostgreSQL and
affected owner/UI/real-path gates. Full-product `just ci` and human real-token
proof remain D14. Windows full CI was not run. This overrides the legacy kernel
skill's broader default without claiming release acceptance.

Valid outer framing does not independently detect changes to opaque native
payload bytes after checksum removal. Eino owns native decoding; Vivy enforces
format, engine and admitted Run/snapshot ownership. D08 delivers converter
fragments only; D11/D12/D14 still own final upgrade integration/publication.
No push was performed. D04 remains gated on accepted D06 evidence.

Final orchestration note: all three final `just` gates completed successfully
under `set -e`. The wrapper's last log-display `tail` used a trailing CRLF and
returned 1 after the race gate had completed. Reading the correctly named log
confirmed the runner's 1730-pass summary; this display failure did not rerun or
invalidate the successful gate. The log is retained in `gate-output.txt`.

The temporary browser tab, owned split backend/mock/Vite, disposable Postgres
container with its anonymous data and temporary smoke home were removed after
proof collection. No tenant home, unrelated server or dirty checkout was touched.
