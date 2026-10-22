# D11 workflow exact-input owner closure

Workflow admissions now retain exact canonical definition, authority, input and limits at the existing immutable workflow revision owner. New executable admissions use discriminator 3 and an owner-allocated ProgramID: raw/trusted runs allocate an opaque ID once, published workflows use INOFY definitions.ProgramID, and draft snapshots use definitions.DraftProgramID. Repeated operations compare actual input; a different draft snapshot cannot reuse the old operation binding.

The existing atomic workflow step receipt stores canonical semantic commit input, including private admitted input, resume key and answers, outputs and native checkpoints. Same CommitID plus equal actual input replays the original receipt; different actual input conflicts. Writer epoch and event wall-clock timestamps remain receipt metadata. Private inputs and answers are never reconstructed from public events.

Protected output bytes and full native checkpoint envelopes commit directly in existing workflow_results/workflow_executions owners. The old content-addressed blob indirection is removed. Native INOFY/Eino serialization, unknown-effect fences, child authorization and cancellation remain authoritative. One-shot workflow child identity directly encodes its existing operation identity using canonical base64url; it is not content-derived.

Definition publication compares normalized actual artifact and used implementations under existing author/ETag CAS and monotone revision allocation. Draft ETags use the existing monotone owner update position and rotate even for equal-timestamp updates. Public workflow RPC/types/events expose ProgramID and omit retired digest contracts.

Paired incremental migrations 044/045 remove retired workflow digest fields and indexes and add nullable actual owner fields. Released 001..038 SQL is unchanged. The explicit human authorization for these migrations and offline copy conversion followed the reviewed table/column preservation proposal after automatic approval review initially rejected source authoring. No production database was opened or modified.

Scope excludes shared service/mapper/control/client integration, cognitive owner code, Work/continuable-child owners, the standalone D14 converter and final whole-candidate gates. Notes were inspected: baseline note digest text is a prose summary, not hash authority, so note behavior is unchanged. No release is produced by this lane; D14 must supply and verify the finite offline converter before target-format release.

## Eino capability check

Consumed INOFY 6f1397e417fc (v0.0.0-20261021084605) exposes CompileOptions.ProgramID, ProgramMeta identity metadata, definitions.ProgramID/DraftProgramID, private RunCommit.AdmittedInput/ResumeKey/ResumeAnswers and matching RecoveryState fields. Its CheckpointEnvelope binds actual ProgramID, HostBindingID, compiler/Eino versions, CatalogContract and serializer. Pinned Eino v0.9.21 compose.WithCheckPointStore/WithCheckPointID retain native payload ownership. Vivy adapts those existing APIs and does not implement another runner or checkpoint serializer.
