# Canonical Laputa dependencies and fresh-clone bootstrap

VIVY now imports the three canonical `github.com/ProjectViVy/laputa/*`
modules and pins their source checkout to Laputa main `ff3936f44ff8cf08c12af2cf698c194cfe474fd3`.
`laputa-source.lock.json` is the single source of the repository/commit pin.

The shared PowerShell bootstrap prepares all three modules before setup,
development, build, and verification recipes invoke Go. CI uses that same
entry point. Existing unrelated repositories are rejected; a checkout at a
different revision is updated only when clean. Local edits are preserved.

The SDK's complete Git source closure and its existing local replacements are
retained. Sealed go-host packaging continues to snapshot and verify the owning
Laputa repository. Placeholder module requirements were replaced by real
published revisions, and the checked-in provider-conformance digest was rebound
to the import-path changes.

The split development server builds a headless backend before applying the
readiness timeout, so a first start requires neither embedded `ui/dist` nor a
warm Go build cache. UI dependencies use the frozen lockfile. `just dev` also
forwards the script's existing `-NoBrowser` switch.

The backend CI lane now checks fresh-clone `just setup`, removes its prepared
sibling from the next check's path, and independently checks `just dev
-NoBrowser` until both HTTP endpoints return 200. README documents the source
bootstrap, prerequisites, source lock, failure recovery, and direct-script use.

No runtime, memory, persona, migration, or SDK authority contracts were changed.
