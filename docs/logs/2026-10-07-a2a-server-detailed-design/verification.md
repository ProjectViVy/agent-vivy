# Verification

## Baselines and inspected evidence

- Continued remote branch `docs/issue-2-a2a-architecture` at `246d5aa`.
  Native main remains `dd78fcf142f384d47ce5cfefb43738fdb9a7346d`.
- Re-read issue #2: G0 review precedes an implementation plan; exact replay
  remains in its acceptance text and was not edited by this delivery.
- Inspected primary-run SQL transactions, native RunOptions/admission order,
  question first-writer-wins updates, recovery/deletion paths, Mask capture,
  workspace preparation, FrozenCore preparation, event schemas, native text
  projection, Module identity generation and current migration numbering.
- Rechecked pinned `a2a-go/v2` v2.6.0 request/interface types and the official
  protocol reference. Source inspection does not constitute interoperability.

## Self-review

- Checked type names and seam ownership against the architecture.
- Added explicit pre-busy receipt lookup, caller-scoped task visibility,
  serialized-event bounds, request-wide list scan limits and public-card
  access independent of task authentication.
- Preserved task versus stream terminality, both interrupted states,
  official error mapping, native text-before-tool flush behavior and exact
  committed-watermark rules.
- Identified provisional cognitive-state cleanup as an unverified G0 gate;
  did not invent a distributed transaction or claim existing cleanup support.
- Specified reuse of PostgreSQL's existing per-Run Journal advisory lock for
  atomic answer transitions; a separate Run row lock would not serialize
  with native terminal append. Clarified canonical answer normalization and
  the limit of GetTask when an artifact exceeds the response cap.
- Checked detailed work locations and coverage of the acceptance fixtures.

## Documentation checks

- `git diff --check`: passed.
- Python standard-library/PyYAML checks: four design/log documents have
  balanced code fences; all 24 relative file links, including the updated
  TODO link, resolve; both JSON examples parse with the intended method and
  role; the YAML example parses; 15 distinct acceptance fixtures are present.
- Confirmed the draft/review status, native Journal lock rule and oversized
  GetTask artifact limitation remain explicit after self-review.
- Tool availability check: neither `go` nor `just` is installed.

These are documentation checks, not evidence that the proposed API, config,
transaction or fixture has been implemented or executed.

## Unrun verification

This iteration changes only Markdown. No public Go declarations, SQL,
configuration, fixtures or handlers were implemented. The code/YAML/commands
in the document are proposed contracts and future verification inputs.

Go and `just` are unavailable in this workspace. The official-client SDK
probe, `just ci`, storage/race tests, selected/omitted artifact builds and
performance measurements were not run and are not reported as passes.
No production user data or credential was accessed.
