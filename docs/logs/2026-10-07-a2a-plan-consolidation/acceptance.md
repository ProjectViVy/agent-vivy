# Consolidation acceptance

- Read the current plan index: every old Story and the old design/index has
  one successor location. No working link requires the old branch.
- Read design 1.1 and 8.2: unresolved scope choices and the original B replay
  contract are retained explicitly, with a conditional A2A-R1 plan.
- The index remains the sole status/dependency ledger. No functional Story
  is Ready; B requires G0 adoption and its exact wire/error freeze.
- A2A-06 can hand off a standard-server milestone to A2A-R1 without claiming
  complete B acceptance. B repeats artifact evidence after source changes.
- The published consolidation commit must have the newer head as first
  parent and the old source commit as second parent. Its first-parent diff
  must contain only the intended Markdown files; the original source remains
  reachable after the owner removes the old branch.

This is document consolidation acceptance. Product CI, SDK/client execution,
native storage/concurrency behavior and extension interoperability are not
certified by it. The verification record states the actual checks and limits.
