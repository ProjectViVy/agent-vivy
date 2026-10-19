# Verification

The source-split verification results are recorded in the plan directory's split-verification.json and published branch receipt. Checks cover exact tree reconstruction, per-lane blob/mode/deletion identity, unchanged unassigned executable paths, parallel donor-base consistency, all hunk locators, full history reachability, and new human-authored commit identities.

An independent read-only reviewer found and prompted fixes for generic dev-environment misrouting, one INOFY test misrouting, the parallel source manifest's mismatched before-base, and deleted-hunk source locators. These are source/metadata corrections, not product behavior changes.

No go test/just ci/browser/PostgreSQL/provider/BML product pass is claimed: shared bindings and accepted target B are absent. The source-only branches are deliberately marked unaccepted, not silently shipped.
