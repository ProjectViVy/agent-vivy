# Issue #32 P0 verification

## Source identities and lockfile

- Read-only GitHub commit API refresh: VIVY `main` is
  `017ec8cc37970b291e04c619990aed00d5403116` (2026-10-09 02:36:45Z); DIVA
  `main` is `518a33ef09858ee1bb190579dd7529aceaa15dd6` (2026-10-05 17:04:53Z).
- Local DIVA checkout: `git rev-parse HEAD` returned the reviewed DIVA SHA;
  `git status --porcelain=v1` was empty before the workspace lock claim.
- `git ls-tree HEAD -- agent-diva-gui/pnpm-lock.yaml` returned a tracked blob.
  `sha256sum agent-diva-gui/pnpm-lock.yaml` returned
  `01ef0ea82f41b54be2103a8bc7cf54a407a2be6b39a48a6137010a8acecea249`, equal
  to `locks.pnpm-lock.yaml` in `build/vivy-sources.lock.json`. The wrapper's
  `build_lock` reads this tracked file at `scripts/build-desktop.py:188`.
- Ruling: remove P1.0 — the plan alleged this input was missing, while the
  reviewed commit contains it with the expected digest. Regenerating it would
  create needless dependency churn; cost if wrong is that a later branch has a
  different source state, caught by P1.2's frozen-install/hash assertions.

## Execution environment

| Tool or target | Observation |
| --- | --- |
| Go | `go1.26.4 linux/amd64` from `/workspace/.vivy-cloud/tools/go/bin/go` |
| just | `1.58.0` from `/workspace/.vivy-cloud/tools/bin/just` |
| Node / pnpm / Corepack | Node `v24.19.0`, global pnpm `11.19.0`, Corepack `0.34.6`; DIVA packageManager pins pnpm `10.33.2` |
| Python | `3.12.14` |
| Runtime / OS | Linux `6.18.44 x86_64`; no Windows runner here |
| PostgreSQL | `VIVY_POSTGRES_TEST_DSN` absent; value was not read or recorded |
| Audio | no `/proc/asound` subsystem visible in this environment |
| PowerShell / Rust | not found on the command PATH; the current DIVA task uses the Go host |
| Isolated worktree | VIVY feature branch `feat/issue32-remediation`, based on the design commit and merged latest VIVY main; `.worktrees` is already git-ignored |

The DIVA `LOCK.md` baseline said RELEASED. Path audit confirmed DIVA tracks
`internal/desktop/runtime_service.go` and its tests; VIVY does not have an
`internal/desktop` package. P1.1 is explicitly marked DIVA in its plan and the
DIVA held scope covers it. The VIVY main merge adds RPC/docs/conformance only.

## Coverage and checks

`python /workspace/work/issue32-design/check_documents.py` passed after updating
its input root to this feature worktree: eight plans, 25 tasks, 146 unchecked
steps, 28 unique issue findings (5 P1 / 21 P2 / 2 P3), 27 planned and one
superseded, and 32 resolvable local links/anchors. `git diff --check` passed.

Temporary defect probes under `/workspace/work/issue32-probes/result.log` were
read as preanalysis evidence; they are outside the repository and are not repair
tests. Their test processes returned PASS because they assert the observed
defects: C1 accepted seq=9 was overwritten to SourceHigh=0; R1 normal append
reported a gap and replayed the first row; R3 serialized 16,447 bytes against
an 8,192-byte ceiling and scanned 4,097,025 bytes against a 4,096,000-byte
budget; R4 Generate returned success and Stream ordinary EOF despite mandatory
settlement errors. These six observations remain reproduction evidence only.

No product or native CI was run as part of P0. H1-H3 and C1-C2 regression
commands remain owned by their task briefs; PostgreSQL, Windows and real audio
remain pending in this environment.
