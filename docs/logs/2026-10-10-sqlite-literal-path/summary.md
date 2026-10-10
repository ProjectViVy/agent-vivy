# Literal SQLite path identity

The storage backend constructed a SQLite URI by concatenating an unescaped filesystem path. Literal `#` and `?` names could truncate to the same database, and literal percent sequences changed the requested filename. This surfaced when an automatically named Go subtest directory containing `#00` reopened a prior test's state outside its owned temporary directory.

Encode the filesystem path with the standard `net/url.PathEscape` before adding the SQLite URI prefix. The driver decodes one filename; filesystem names cannot supply URI fragments/query parameters. No database schema, authority, storage contract or user data migration changes.

Tests open distinct real databases through the existing Backend and Snapshot APIs, verify no snapshot crosses paths, and verify the exact literal files exist. Windows rejects `?` as an invalid filename; its test asserts failure and absence of a shortened database instead of skipping the case.
