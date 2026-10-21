# Current-pin dependency preparation

Read-only revalidation used this lane's own clean Laputa ff3936f44ff8cf08c12af2cf698c194cfe474fd3
and actual INOFY v0.0.0-20260930141905-71e2c9bbe47d module source. These are the
consumed pins from go.mod, not the historical audit branches. No upstream
messaging, pin change or downstream seam bypass was performed.

| Actual consumer | Required upstream seam |
| --- | --- |
| diva-cognitive factory boundSink.Capture | Garden CaptureRequest.ContentHash reaches ingest.Service.Submit, which requires/computes matching SHA-256, stores it and compares it on replay. |
| BoundDomain/NewExecutor effects | Laputa evolution.Effect.PayloadDigest is mandatory in Validate/NewEffect/CheckReplay; existing effect receipts consume it. |
| Garden memory effects/Mentle adapter | memory.AuthorizedMutation.PayloadDigest is required and forwarded to mutation receipts, alongside existing revision/absence CAS. |
| factory ResolveBinding and workflowNodes | RunBinding.StrategyDigest is mandatory and compared against the actual bound composition. |
| Garden persona effects | ChangeRequest.BaseHash is passed from document ContentHash and checked with owner revision. |
| workflow Program.Run, inofyRunStore and node executor | INOFY compares ExecutionRef.ProgramDigest/compiled metadata; compilation/publication expose definition/catalog/program identities; resume validates input/answer identities and envelope payload checksum. |

A constant or renamed opaque digest would bypass current replay/ownership checks.
Reviewed upstream hash-free exact-input/revision/receipt/RunStore/checkpoint and
publication contracts are required before replacing these mandatory seams.
D08's private actual policy definition does not remove them. PolicyRevision is
itself a revision string, not a mandatory digest validator; Vivy manufacturing
it by hashing policy JSON is a local concern. FrozenCore.SourceHash is not
required by its upstream validator. Do not invent blockers for those fields.

Independent local preparation remains possible after the Story gates permit:
D10's direct toggle reads authorized raw SKILL.md then submits expected exact
bytes/existence; bare skills/get returns a parsed body without frontmatter,
so use explicit path:"SKILL.md". Existing SkillRevision.BeforePayload supports
exact content; directory actions, rollback and restart must remain covered.
BML/memory/mask revisions retain their existing ownership.

D11 prepares only existing Work/workflow/child/notes behavior around stable IDs,
normalized intended input and current unknown-effect fences. Notes are already
append-only with opaque IDs; summary-oriented digest comments do not establish
content-hash authority. No notebook revision, report or toolkit subsystem is
introduced. Index and Story plans remain the sole dependency/status record.
