ALTER TABLE schema_migrations ADD COLUMN name TEXT NOT NULL DEFAULT '';
ALTER TABLE schema_migrations ADD COLUMN checksum TEXT NOT NULL DEFAULT '';
