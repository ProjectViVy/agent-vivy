# N1 acceptance checklist

- Paired migration + head assertions updated to 36 — yes.
- Receipt lookup precedes current-state checks; committed mutations never
  re-run — yes (`notebook_mutations` + post-conflict re-probe).
- CAS via version + base_revision; conflicts carry CurrentVersion /
  CurrentRevisionID — yes.
- Foreign IDs/cursors share `not_found` / `invalid_request` — yes.
- Revisions immutable; adopt mints new head; tombstones retain history —
  yes.
- Bodies fetched individually, never in list rows — yes (`ListRevisions`
  selects `''`).
- Legacy `notes` migrated with exact IDs/content/timestamps; oversized rows
  readable; new saves bounded — yes.
- Per-scope idempotent role-section seeding; no tombstone resurrection — yes.
- `vivy notebook export` uses configured backend read-only; refuses
  overwrite/traversal; reports upgrade on old schema — yes.
