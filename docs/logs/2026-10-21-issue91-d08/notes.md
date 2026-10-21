# D08 deletion closure and handoff

## Residual audit

The live D08 paths no longer compute or carry prompt/persona/frame/policy,
Context candidate/provenance, SkillSource content/provenance, MCP schema/instance/
remote or checkpoint checksum hashes. `PolicySnapshot.Definition` contains the
actual private JSON value; it is never projected as the replacement for the
removed public `policy_hash`. Checkpoint v2 stores only format/engine/time,
existing agentic framing and the Run/schema/composer/Generation binding, beside
opaque Eino bytes. Source IDs and explicit provider versions retain their own
meaning. A local unversioned skill has no fabricated version.

Searches covered the changed Host, runtime/domain/storage, SDK, schemas and UI
owners for retired symbol names, plus hash/digest/checksum implementations in
the D08 source files. The retained matches have these owners:

| Matches | Owner and disposition |
| --- | --- |
| Context-independent source tree, Assembly/Generation and Studio artifact identities | D06; no partial donor worktree was merged. |
| File preconditions/version history, continuity receipt input identity and write output `sha256` | D09. The real-path smoke still sees this existing write-result field; it is not a checkpoint or approval substitute. |
| Mask Definition/Metadata/create-request digests and skill/cognitive mutation CAS | D10. D08 deletes the admitted MaskCaptureCheck/Snapshot digest and compares actual locked definition values; it does not retire the business mutation revision contract. |
| Work, workflow definition/artifact, child authority/business closure and INOFY digests | D11. D08 substitutes actual policy values only inside their existing private authority owner. |
| Migration manifest/checksum metadata and validation | D12. Released 001–036 migrations remain byte-for-byte unchanged. |
| Opaque native Eino payload internals and historical records/tests | Upstream/native or historical evidence; not reinterpreted by Vivy's outer adapter. |

## Finite transition

Paired append-only migration 037 drops `run_prompt_snapshots.payload_sha256` and
`checkpoints.checksum`. Fresh target initialization, recognized normalized 035
and exact accepted D03 036 source shapes, rollback and reopen are covered by the
storage suites. Unlisted historical 036–043 shapes remain unsupported; recognizing
D03's exact paired 036 manifest does not broaden that source matrix.

Offline SQL fragments are outside the runtime embed glob. In a copied target
transaction, D03 approval expiry precedes D08 question/Run settlement and native
checkpoint pointer deletion. Claimed/completed operations, historical checkpoint
generations, answered/history interactions and unrelated Blob pointers survive.
The source copy is unchanged after both success and injected target rollback.
D11 settlement and D12 metadata still need D14's finite assembler; these fragments
are not a publishable converter and add no runtime legacy reader.

## Dependency gate

The issue-91 index remains the sole status source. D04 requires an independently
accepted D06 commit and evidence. D06 is still Blocked in the index; its local
worktrees and branch commits are not acceptance. D04, D05 and D13 therefore remain
unstarted by this lane. Re-read the index and accepted D06 evidence before resuming.
No push is authorized for this delivery.
