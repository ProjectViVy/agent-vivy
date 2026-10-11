# Acceptance

Native parser accepts the approved Vivy manifest and rejects ambiguous JSON,
unsupported fields/schema, invalid identity/path, unresolved recommendations
and excessive version strings. Artifact digests preserve semantic argument
order and exact declared file hashes while ignoring original JSON formatting.
Native Skills accept only name/description frontmatter and literal body.
Catalog accessors do not expose mutable collections.

Complete product acceptance remains pending TK-02–13, both databases, Windows
CI, the real split UI and artifact inspection. This enabling increment adds
no visible installation feature yet.

Acquisition copies only declared regular files into private staging, produces
the independently calculated digest, resolves Git refs to commits, and never
executes repository hooks or scripts. Reopening and reading detects mutated
bytes. Product lifecycle and persistence acceptance remain pending TK-03–13.
