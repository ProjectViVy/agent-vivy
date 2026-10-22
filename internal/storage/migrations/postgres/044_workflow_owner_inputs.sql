-- D11: exact immutable workflow owner inputs. Released SQL is unchanged.
-- The offline copy converter copies protected outputs before target cutover.
-- Schema-1/2 history remains unbound; pending execution needs explicit recovery.
ALTER TABLE workflow_revisions DROP COLUMN descriptor_digest;
ALTER TABLE workflow_revisions DROP COLUMN authority_digest;
ALTER TABLE workflow_revisions DROP COLUMN program_digest;
ALTER TABLE workflow_revisions DROP COLUMN catalog_digest;
ALTER TABLE workflow_revisions DROP COLUMN input_digest;
ALTER TABLE workflow_revisions ADD COLUMN program_id TEXT;
ALTER TABLE workflow_executions DROP COLUMN checkpoint_digest;
ALTER TABLE workflow_executions DROP COLUMN checkpoint_blob_id;
ALTER TABLE workflow_executions DROP COLUMN resume_answers_digest;
ALTER TABLE workflow_executions ADD COLUMN resume_answers_json BYTEA;
ALTER TABLE workflow_results DROP COLUMN digest;
ALTER TABLE workflow_results DROP COLUMN blob_id;
ALTER TABLE workflow_results ADD COLUMN output BYTEA;
DROP INDEX workflow_definition_revisions_dedup_idx;
ALTER TABLE workflow_definition_drafts DROP COLUMN definition_digest;
ALTER TABLE workflow_definition_drafts DROP COLUMN artifact_digest;
ALTER TABLE workflow_definition_revisions DROP COLUMN definition_digest;
ALTER TABLE workflow_definition_revisions DROP COLUMN artifact_digest;
ALTER TABLE workflow_definition_revisions DROP COLUMN used_catalog_digest;
