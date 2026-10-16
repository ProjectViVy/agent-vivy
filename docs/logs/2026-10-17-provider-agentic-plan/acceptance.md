# Acceptance — 2026-10-17 provider agentic spike plan

This is a documentation deliverable. The acceptance question is not "does it
work" but **"can a human read it and act on it, and is its evidence real?"**

## How a human can tell it worked

### 1. The plan is discoverable

Open `docs/DEFER.MD` and find the `PROVIDER-AGENTIC-MIGRATION` row. Its status
reads `DEFERRED · SPIKE PROGRAM AUTHORED 2026-10-17 · SCHEDULING PENDING`, and its
notes name `docs/plans/provider-agentic/` and say the restart condition was met.
You should not have to be told where to look.

Then open `docs/plans/provider-registry/EINO-CAPABILITY.md` — the stale
2026-09-18 capability document — and confirm it now carries a supersession banner
pointing at the new program. If it did not, the repository would hold two
documents stating the same capability facts at different versions, which is the
defect this delivery is supposed to prevent.

### 2. The design concept is answerable in one sentence

Read `docs/plans/provider-agentic/DESIGN-CONCEPT.md` §1. You should be able to
state the rule back: *a protocol family's differences may exist only inside a
pinned Eino component's constructor; Vivy holds data declarations and one
`case`*.

Then read §2.1. The table should convince you the concept is viable rather than
aspirational: all three protocol families — including `anthropic-messages`, which
is the one that would otherwise force a permanent second generation — have an
`AgenticModel` implementation at a named version with a named interface
assertion.

### 3. Every claim can be checked

Pick any three citations in the plan at random and verify them against the module
cache or the repository. Each should resolve to the claimed symbol on the claimed
line. This is the property `verification.md` §"Citation sweep" establishes: 88
module-cache checks and a repository sweep, all passing after ten corrections.

The two corrections that matter most to a reader:

- The 2026-09-18 record's stated obstacle ("no `Message`↔`AgenticMessage`
  conversion exists, so a bridge would be custom") is **true about the model
  components and inapplicable at the assembly layer** — the ADK is generically
  typed. `DESIGN-CONCEPT.md` §2.2 says so and cites both sides.
- The reasoning UI is **not** missing. `ReasoningRow.tsx` exists, is rendered by
  `ChatView.tsx`, and is fed by `model.reasoning_delta`. The gap is that no
  adapter produces reasoning content. `S1` C6 and `S3` §2.6 state the corrected
  version.

### 4. The spike is specified, not executed

Read `S0` through `S4` in order. Each should leave you able to run that stage
without making another design decision:

- `S0` gives the throwaway-module commands and the four questions to answer as
  numbers, and says plainly that it does not contain its own results.
- `S1` gives the contract tables to fill, an eleven-item capability checklist,
  and a closed verdict vocabulary.
- `S2` gives the three fakes, the six assertion channels, and four test-discipline
  rules transferred from the audited Python implementation.
- `S3` gives the measured scale, the per-group inventory, the residue-deletion
  set with a consumer check per item, and a twelve-row risk table.
- `S4` fixes the thresholds **before** the evidence, so a `GO` has to be earned.

None of the four has been run. If you expected working spike code, you are
looking at the wrong deliverable — the user explicitly selected documentation
only.

### 5. Nothing was touched that should not have been

Run:

```text
git status --short -- internal sdk ui cmd plugins faces bml go.mod go.sum
```

It must be **empty**. The `internal` source digest is unchanged, so the sealed
Generation is unaffected — a documentation change that moved the digest would be
a defect regardless of how good the prose was.

## What the reader should be able to decide

After reading, a human should be able to make three decisions without further
investigation:

1. **Whether to schedule the spike at all** — the cost is now stated (S0's
   measurement method, S2's harness) rather than implied.
2. **Whether to accept the one irreversible step** — moving `anthropic-messages`
   to `agenticclaude` is named as such in `DESIGN-CONCEPT.md` §6.2 and assigned
   explicitly to `S4`, rather than buried as a side effect.
3. **What "no residue" means concretely** — three rules, each checkable, with the
   deletion set itemised in `S3` §3 and the preserved list beside it so that
   omission is not mistaken for oversight.

## Known limitations, stated rather than hidden

- **The dependencies are not measured yet.** S0 exists precisely because the
  2026-09-18 "second OpenAI client" objection was an argument, not a number. The
  plan records the prediction that the Anthropic side is near-zero (its
  transitive dependencies are already in `go.mod`) but does not present it as a
  result.
- **`AgenticModel` is upstream-marked Beta.** The concept's own honesty section
  (`DESIGN-CONCEPT.md` §6) lists this first, along with six other boundary
  conditions that would change the plan rather than be worked around.
- **The spike may end in `NO-GO`.** That is an admissible outcome and has a
  defined exit (`S4` §7), not a failed delivery.
- **The `docs/TODO.md` / `docs/DEFER.MD` duplication for this row is untouched.**
  It is a board-wide condition from the 2026-09-26 sweep, and the deliberate
  non-actions are recorded in the new `README.md` and in `summary.md`.

## Records

- Plan: `docs/plans/provider-agentic/`
- Log: `docs/logs/2026-10-17-provider-agentic-plan/`
- Branch: `docs/provider-agentic-plan` (not pushed)
