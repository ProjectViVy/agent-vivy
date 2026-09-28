# Merge-repair implementation package

Epic: repair the semantic damage left by the post-re-upload merge chain on
`main` (`a74ad98d`..`6aa00d24`).
Code baseline: `6aa00d24`; design:
[2026-09-28-merge-repair-design.md](../../specs/2026-09-28-merge-repair-design.md).
Authorization: the maintainer asked for the merges to be decomposed, analyzed,
and repaired in the same session. Publication happens only as a human-authored
PR; no push to `main`, no AI-attributed commits.

## Read order and authority

1. Read repository `AGENTS.md`, then the design doc (defect table D1–D15 is the
   contract).
2. Each Story file owns its own repair steps and evidence.

This index owns Story state and dependencies. All stories are sequential in one
lane — the `internal/` source digest pin in the SDK golden artifacts must be the
last write before verification.

## Story graph and current state

| Story | Defects | Outcome | Plan | State |
| --- | --- | --- | --- | --- |
| MR-1 | D1–D4 | restore compilation: dedupe merged struct/literal blocks, restore `EventToolMounted`, re-pin migration manifest | [MR-1](MR-1.md) | Implemented |
| MR-2 | D5, D6 | restore channel plugin capability surface and re-derive all source digests (descriptors, yaml, SDK goldens) | [MR-2](MR-2.md) | Implemented |
| MR-3 | D7, D8 | restore tool-set union; make durable ops interrupt- and refusal-transparent, exempt self-deduping model-work tools | [MR-3](MR-3.md) | Implemented |
| MR-4 | D9, D10 | fix child-run recovery context and child terminal event classification | [MR-4](MR-4.md) | Implemented |
| MR-5 | D11, D12 | reconcile pre-invariant fixtures and non-allocating message writers with the session-primary + `history_positions` contracts | [MR-5](MR-5.md) | Implemented |
| MR-6 | D13, D14 | repair latent test/context budgets and channelhost durable-delivery fixture | [MR-6](MR-6.md) | Implemented |
| MR-7 | gate | full verification: `go build`, `go test ./...` (74 pkgs), conformance re-run, golden re-pin; open human-authored PR | [MR-7](MR-7.md) | Implemented locally |

## Dependency notes

MR-1 precedes everything (nothing compiles until it lands). MR-2's digest
re-pinning is invalidated by any later change under `internal/` or
`plugins/` — its goldens were re-derived last, after MR-3..MR-6 settled.
