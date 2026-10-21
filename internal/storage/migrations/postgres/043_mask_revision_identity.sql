-- A revision-1 row still holds its original create inputs. Later revisions
-- cannot prove original intent: retain their history, fence operation replay.
ALTER TABLE mask_definitions ADD COLUMN create_name TEXT;
ALTER TABLE mask_definitions ADD COLUMN create_description TEXT;
ALTER TABLE mask_definitions ADD COLUMN create_body BYTEA;
UPDATE mask_definitions SET create_name = name, create_description = description,
 create_body = body WHERE revision = 1;
ALTER TABLE mask_definitions DROP COLUMN digest;
ALTER TABLE mask_definitions DROP COLUMN create_request_digest;
