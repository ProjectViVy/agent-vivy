# Detailed architecture verification

## Source and scope

- Local/remote planning baseline: `96311d16b231c64993670e3bc5860cc17d3910cf`.
- Inspected root AGENTS, current SDK/module Port/import constraints, existing
  assembly factory generation, Eino adapter/observer/tool boundaries, queue,
  warming, message projector, migration runner, BML and Studio ledger owners.
- Read-only external pins: INOFY
  `71e2c9bbe47d5d095b27d756e38eaff565d93d36`, Laputa
  `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24`, Studio
  `ac5532236ddb2e4312d569269fe9b64b5fbd79bb`. Findings and required proposed
  upstream deltas are in shared design §11.10. These are not upgraded pins.
- Architecture stays in the established shared design. Index alone owns status
  and immediate dependencies; no implementation authorization is inferred.

## Read-only domain review and corrections

Runtime review checked optional construction, import direction, native Eino
boundary ordering, outcomes, warming and steering. Incorporated: candidate-aware
steering eligibility; same-ID refused-cancel demotion and persistence failure;
all-ID outcome coverage then fresh-only detector input including replay siblings;
Run-lifetime steering controller; Work/question claim exceptions; current warming
budget-denial and diagnostic append terminal semantics. Existing queue mode
literal remains one-at-a-time.

Persistence/assembly review checked finite source matrix, existing snapshot tuple,
operation rows, checkpoints, consents, BML and separate Studio ledger. Incorporated:
legacy typed outcomes remain NULL only on unbound legacy rows, never guessed from
text; Studio file SourceRef rewrites follow copied binary paths including custom
output locations, without assuming directory ID equals GenerationID.

## Document checks

Focused checks cover all 15 Story headers, eight requirement mappings, local
referenced paths, relative links, exact index/Story dependencies, known endpoints,
no self-edge/cycle, derived topological waves and `git diff --check`. The DAG
remains 15 nodes, 26 edges and six waves. Focused validation passed: 15 Story plans, 15 DAG nodes, 26 edges, six waves,
eight requirements, 59 local links and 161 explicit existing path patterns;
Story structure/status/prerequisites/placeholders and referenced just targets
passed. `git diff --check` passed. Publication uses an expected-head lease;
remote readback is required before reporting completion.

No Go/just product checks, regeneration, SQLite/Postgres converter execution,
BML execution, live provider test, installed-data access or post-deletion residue
audit ran. No product behavior, performance or deletion-completion claim follows
from planning checks.
