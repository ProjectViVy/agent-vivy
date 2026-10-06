# Verification

## Publication checks

The branch is based on `main` commit `dd78fcf142f384d47ce5cfefb43738fdb9a7346d`. Repository and scoped instructions were checked before preparing the files. The Git tree contained no additional documentation-scoped `AGENTS.md`. An exact branch lookup confirmed that `ACP` did not yet exist.

The following focused checks were run against the four publication files in a temporary local staging repository:

- `git diff --cached --check`: passed, with no whitespace errors.
- `git diff --cached --stat` and explicit path review: only the design and this iteration's three records were added.
- A one-off Python document check: 14 numbered sections, balanced fenced blocks, 12 unique acceptance IDs (`ACP-01` through `ACP-12`), retained scope approval and open G0 status, and valid relative links between the new files.
- Compared the publication draft with the current standalone revision: only repository-location metadata, hard-break formatting and the publication-status paragraph changed.

These checks establish document integrity and publication scope. They do not establish protocol, SDK or runtime correctness. The source inspection recorded in the design is research evidence, not executable test evidence.

## Checks not run

`just ci`, SDK fixtures, Recipe packing, Inspect/conformance and real-client smoke were not run. This commit publishes a proposed design under `docs/superpowers/specs/` and delivery records; it changes no executable behavior, dependency, configuration or adopted product contract. The required implementation and canonical-contract gates remain listed in the design.

## Remote verification procedure

After creating the commit and branch, read the remote branch and commit back. Confirm the expected parent, the four added paths, exact file content and human commit attribution. Confirm that `main` has not been moved by this publication. Report the resulting branch and commit to the owner.
