# Super Channel EPIC — Sub-AGENT PLAN Package

AGENTs taking over implementation should claim work here; do not infer the architecture from chat history.

## Reading Order

1. `docs/architecture/VIVY-CHANNEL-PACK.md` — Contract
2. `docs/architecture/VIVY-CHANNEL-EVOLUTION.md` — Evolution Tree
3. `00-standing-orders.md` — violation means stop
4. This slice: `CH-C*.md`
5. `docs/TODO.md` §0.2 — calendar

## Claiming Work

| Claim Now | PLAN | Stage |
|---|---|---|
| **Next** | [CH-C1.md](CH-C1.md) | A Genetic Material |
| After that | [CH-C2.md](CH-C2.md) | B Species Window |
| After that | [CH-C3.md](CH-C3.md) | C World Entry |
| After that (template) | [CH-C4.md](CH-C4.md) | D ABI |
| After C4 | [CH-C5.md](CH-C5.md) | E Visibility |
| Can branch from C4 | [CH-C6.md](CH-C6.md) | F Domestic |
| Open after C4 merges | [CH-C7a.md](CH-C7a.md) / [CH-C7b.md](CH-C7b.md) / [CH-C7c.md](CH-C7c.md) | F/G |
| Do not claim | [CH-C8.md](CH-C8.md) / [CH-C9.md](CH-C9.md) | H Post-Cutover Notes |

For implementation, **open a separate** branch such as `feat/channel-c1`; do not pile code into `feat/channel-super-contract`.

**When implementing channels formally (C4 / C6 / C7*):** use picoclaw's implementation as the most complete reference; rewrite it read-only, and do not import it. See 「picoclaw reference」 in `00-standing-orders.md`.

## Ten Sections in Each PLAN

Identity · Goal · Current State · Target Structure · File Inventory · Steps · Acceptance · Prohibitions · Risks · Handoff
