# Notebook/report detailed design

Produced the owner-requested detailed design for current agent-vivy #39 and its
#5 reporting integration on baseline `017ec8cc37970b291e04c619990aed00d5403116`.

Authoritative artifact:
`docs/superpowers/specs/2026-10-09-notebook-reports-design.md`.

The design records the confirmed built-in-only storage choice, editable report
body, custom sections, Markdown editor/preview, independent comments, revision
protection, optional UI and headless access. It defines scoped storage and action
contracts, CAS/idempotency, migration, report-only execution, feedback selection,
scheduling/recovery, acceptance cases, and notebook-first delivery increments.

Source inspection identified concrete integration work: legacy prompt injection,
mock Notebook data, absent scoped persistence in the public action facade, human
edit authorization, INOFY parent-Run admission, Agent-only workflow nodes,
agent-turn-only cron dispatch, and cognitive ingestion provenance.

This delivery is a reviewable design draft. No runtime/UI code, generated
Assembly, repository instruction, normative product contract, issue, or PR was
changed. No executable Story package, implementation readiness, or live runtime
verification is claimed. The next stage is owner review of the written design,
then Story planning with the approved contracts.
