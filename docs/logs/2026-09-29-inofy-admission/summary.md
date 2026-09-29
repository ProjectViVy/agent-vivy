# INOFY Definition admission checkpoint

The `workflow` tool, workflow propose/start RPC and first-party inspector now submit an `inofy.workflow/v1` Definition. VIVY projects the tool schema from INOFY's exported schema, limits the catalog to `vivy.child-task@1`, and checks graph bounds and the parent's read-only tool ceiling with the canonical INOFY decoder/compiler. Unknown fields, legacy descriptor payloads, duplicate fields, unsupported retry/fallback and oversized literal inputs are rejected.

The INOFY library is pinned to published commit `6acfcc6b1a51921c45316a15eb0846bc0e775a9e`. This is an admission checkpoint, not production cutover. Start explicitly fails before creating a Run until S11-C atomic storage and S11-D native child execution are installed. Historical descriptor inspection and the old executor remain for legacy recovery and existing tests until S11-E. No reusable definition catalog or visual editor was added here.

The pinned Eino v0.9.13 compose Workflow remains in the old implementation under `internal/runtime`. INOFY's compiled Program uses its pinned Eino runtime; the new host admission uses the INOFY public Compile API without building a second graph scheduler.
