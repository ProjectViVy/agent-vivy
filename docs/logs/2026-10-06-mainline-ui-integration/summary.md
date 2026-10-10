# Mainline UI integration

The authorized delivery merges `f7262fbe` (restored mask library and synchronized quick switching) and `4e8de423` (conditional composer work dock and slash commands) onto remote main at `1c6e32af`.

The latest mainline Laputa bootstrap changes are preserved. The only merge conflict is the existing source-bound conformance bundle; its five internal-rooted source digests are recomputed from the combined tree. No functional implementation is changed during integration.

Publish the validated merge to `origin/main` with a regular push. Both original fix commits remain in history.
