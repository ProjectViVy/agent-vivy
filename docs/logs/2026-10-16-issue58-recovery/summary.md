# Issue 58 recovery checkpoint

This is a new recovery lane, not product acceptance. Repository access and the attached recovery record were confirmed on 2026-10-16 Asia/Shanghai. The original development environment is not assumed available.

## Source reconciliation

| Classification | Evidence | Disposition |
|---|---|---|
| Remote available | implementation branch `66fabfd27bd83d8de201268ccadc6a0d298d7d6a`; approved revision-3 design and S0–S6 plans; S1 owner/facade and S2 Tasks 1–3 source | Preserve and revalidate, not inherited GREEN |
| Latest main | `5715950ddfc87e05cf60e3d2a0ca54983bef05eb` | Integrated locally at `b91096cca2028a8e8b127f60d41da600d64bb7eb`; provider, thinking and persona updates retained |
| Historical only | accepted source `c113cac50b37e09b9439251bbc6d3320a22bc4a4`, checkpoint `858af73230fd6927fb93cdfe722d23face429fa7`, later S3 files and tests | Both commit fetches rejected as not-our-ref; neither named extant checkout has objects; no source or passing conclusion imported |
| Recovery remnant | cleanup staging contains original directory names | Copied before inspection; zero regular files and no `.git`; no recovered source |
| Reimplementation required | S2 Task 4 input provenance, strict persisted request decoder and grouped assistant derivation; S3 input projection/receipt/capacity and native fold adapter | Test-first rebuilding; historical reports are not acceptance |
| Not delivered | S4 management/UI, S5 selected BML projection and legacy exit, S6 candidate acceptance | Continue in existing plans |

The attachment `issue58-recovery-checkpoint.md` is a continuation record, not a backup. Its instruction to wait for the old mount is superseded by the owner's explicit current recovery instruction. No old mount, commit, review or GREEN is assumed.

## Current findings

1. `internal/storage/subjectsql/evidence.go` rejects Notebook-derived evidence wholesale; retain exact evidence and explicitly classify automatic input availability/exclusion. Unknown dominates exclusion.
2. `internal/runtime/subject_tool_evidence.go` saves full output but text replay returns a compact projection; S3 must prove the next real HTTP request receives the full authorized live output.
3. Existing `prepareRequest` history fallback/folding and budget path remain suitable only for ordinary Coding. LIFE requires fail-closed exact inputs and commit-before-retirement.
4. Invalid UTF-8 must be detected before JSON serialization can substitute bytes. Artifact UTF-8 checks after serialization do not establish exact original arguments.
5. #66 installed singleton producer is absent. Its trusted consumer seam exists; do not expand #58 into sidebar implementation. Installed LIFE acceptance remains blocked.

## Compatibility

Main migrations end at 043; recovery branch has paired044/045, so no current numbering collision. Go module declares Eino0.9.20 and Laputa bb57e93. Committed sibling replacements still apply; actual source SHA must be recorded, not inferred from the module version. No dependency version change is authorized by this checkpoint. Persona uninitialized unnamed admission must remain available; initialized Frozen Core remains authority.

## Execution ownership

One implementation writer, GPT-6.1 Sol High, rebuilds missing S2 provenance. Root owns integration, status and evidence; a separate read-only reviewer audited the baseline. No main merge, deployment, tenant migration or external messaging is authorized. Checkpoints go to `feat/issue-58-recovery-20261016`.
