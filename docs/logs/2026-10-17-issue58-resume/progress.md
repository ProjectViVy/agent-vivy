# SDD ledger — plan: docs/superpowers/plans/issue-58/index.md

Ruling: execute serially and self-review; explicit user prohibits subagents. Existing remote implementation is retained, not rewritten for TDD ritual.
Pre-flight: S3/S4 consume native C1/C2; S5 Task1a adapter exists, Task1b/2 unfinished. Use retained authority and avoid ProfileID-to-subject inference.
Ruling: isolate both repositories under /workspace/issue58 so checked-in relative replacements resolve exact lock without modifying the original Laputa checkout.
Current: merge main into recovery candidate; resolve TODO with both current rows; regenerate UI assembly via SDK.
Next: stage UI, commit integration, merge PR71 and inspect conflicts/pins.
