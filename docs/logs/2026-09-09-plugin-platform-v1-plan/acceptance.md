# Plugin platform v1 plan acceptance

A human reviewer can accept this P0 when all of the following are true:

- The four normative documents define the same v1 Module, Port, Trust, Grant,
  Tool, UI, Eino, Assembly, Inspect, rollback, and clean-break rules.
- The program plan lists P0-P9 with dependencies, exact target paths, failure
  paths, verification commands, rollback expectations, and phase exit criteria.
- P1-P9 remain unscheduled and no implementation work is implied by P0.
- `AGENTS.md` routes plugin work to `vivy-plugin` and rejects v0, runtime
  discovery, protected Tool shadowing, UI permission prompts, and unsupported
  Eino reinvention.
- `docs/TODO.md` records PLG-1 with P0 complete, P1-P9 unscheduled, and SCX
  Gates A/B/C.
- `.agents/skills/vivy-plugin/SKILL.md` validates and gives a future worker a
  correct decision path before any plugin code is written.
