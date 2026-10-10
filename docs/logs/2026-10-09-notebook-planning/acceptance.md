# Planning acceptance

This record describes how to review the package; it does not assert owner approval
or product acceptance.

1. Open [the index](../../superpowers/plans/notebook-reports/index.md). Its eight
   Story links resolve, and its table/DAG identify the same predecessor edges.
2. Review N0–N3 as an independently deliverable notebook. They remove preamble
   injection, add scoped durable content, bind optional actions/tools and replace
   the mock UI. Reporting is not needed to edit/export notebook content.
3. Review R0–R3 for trusted report execution, bounded evidence-backed generation,
   editable/candidate UI, and calendar scheduling/recovery. First-release human
   body edits and original generated revisions both survive regeneration.
4. Check scope and failure cases: cross-scope access, duplicate/lost responses,
   competing saves, comment versions, move/delete races, source failure versus
   empty data, ambiguous model attempts and crash after publication.
5. Verify each Story names its files, contracts, commands, review focus and
   output evidence. These are future execution instructions, not test results.
6. Confirm every status is Planned, no code deployment/merge is implied, and
   missing CI/UI-skill prerequisites are visible in the index and verification log.

Recommended next step is owner review, followed by sequential execution beginning
with N0 if authorized. Later Stories become Ready only with accepted predecessor
evidence and resolved execution gates. Schedule/UI integration closure waits for
both R2 and R3 evidence even though R3 backend work can begin after R1.
