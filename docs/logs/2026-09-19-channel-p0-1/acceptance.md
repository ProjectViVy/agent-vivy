# CH-P0-1 acceptance

## Observable result

An implementation worker can begin CH-P0-2 without choosing Channel's owner,
composition surface, RPC attachment shape, state vocabulary, or conditional
Host rule. Those choices are now normative and compile-tested.

## Acceptance map

| Requirement | Evidence | Result |
|---|---|---|
| CH-01 canonical T1 owner and `0..1` cardinality | Module/Port contracts; public-core and duplicate-Host tests | PASS |
| CH-02 conditional Host | Provider-without-Host and zero-Provider Host tests | PASS |
| CH-03 one factory/owned seam | `internal/channelcontract` compile assertions | PASS |
| CH-04 one existing Run path | frozen dependency contract; app/runtime wiring unchanged | PASS |
| CH-05 typed RPC contribution | `MethodBinding`, `Contribution`, validation matrix | PASS |
| CH-06 honest absence contract | empty contribution validates and contributes zero bindings; runtime dispatch remains unchanged | PASS at contract level |
| CH-07 distinct state truth | state test separates `Compiled` from `ProcessAvailable` | PASS |
| CH-08 dormant settings retained | unrelated update preserves an omitted Provider overlay | PASS |
| CH-09 generated imports own inclusion | normative Assembly rule; generated files unchanged | PASS at contract level |
| CH-10 generic UI projection | Go serialization, TypeScript typecheck, UI tests | PASS |
| CH-11 lifecycle/rollback contract | `Owned` embeds `module.Instance`; normative ordering | PASS at contract level |
| CH-12 default product unchanged | production wiring/UI/Recipes unchanged; focused and manual CI-equivalent gates | PASS, subject to literal `just ci` environment limitation |

## Human review

Confirm that the diff contains contracts, catalog metadata, tests, the generic
wire type, and evidence only. In particular, it must not contain a real
`internal/modules/channel` implementation, generated Assembly changes,
reduced Recipes, or Channel settings UI movement.

CH-P0-1 acceptance does not imply CH-P0-2 through CH-P0-5 completion.

