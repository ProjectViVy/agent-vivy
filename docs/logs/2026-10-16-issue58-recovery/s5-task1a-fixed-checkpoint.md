# S5 Task1a fixed checkpoint and remaining scope

Source `f6bcae7df73cd192e352480762621b8bb84ae711`, tree
`28c871b5da28c48f14157b05f12fff6f440c0e70`, is published as remote
`3f547b583b0ac63b1410c6216f251e1a2f7433af`. Root read back and verified the
published tree. The preceding read-status producer is local46a10b3f / remote
74f8788a394d5c583540d0ea3f8b18e9d19e7a41 / treebdf80d0396c0fd46539d5f2c6cbf53d67875cdeb.
Commit metadata differs between local and remote; tree equality is explicit.

The [author delivery](../2026-10-16-issue58-consumer-task1a/summary.md) prepares
an exact App-owned Garden Backend over the accepted BML transaction, preserves
strict native replay, serializes root/core-lease ownership, copies only selected
firstparty providers per App, and releases Home after normal Run cancellation
drains consumers. No LIFE projection is activated. Historical C2 sources
(Task1b), single receipt-aware projection, legacy tool/recall qualification,
no-sink LIFE denial and actual loaded status (Task2) remain unfinished.

Root verified 96 author artifacts, all24 cumulative immutable paths and all16
final tested Go file hashes. Author passes were working-source checks against
HEAD46a with explicit source seals; they are not relabeled as execution of f6.
All original RED/compile/intermediate failures and two earlier source snapshots
remain in the packet. These exposed same-Home/different-core-lease cleanup,
reusable Assembly stale providers, and normal Run retaining Home. A Home-only
early-listener defer was removed before the final source because consumers had
not drained on that branch; earlier passing draft logs are not final evidence.

Root independently executed the actual clean detached f6 candidate with a real
.git directory, Go1.26.4 and the unchanged local-only runner:

| Gate | Actual fixed result | Seconds |
|---|---|---|
| Full memory/diva-cognitive/contexthost/App, documented MemoryLoop exclusion | PASS380 test events +4 packages;3 existing opt-in SKIPs | 30.934 |
| Focused Issue58/Toolkit-reopen/Gatewayless/Headless race | PASS81 +4 packages;1 SplitFixture SKIP;0 races | 88.341 |
| Vet of those packages + cognitivecontract | PASS | 4.554 |
| Full native BML through root MVS SQLite1.59 | PASS162 +1 package;0fail/skip | 1.889 |

The three normal opt-ins are CognitiveActionFixtureCapture,
Issue58RecapSplitFixture and RealProviderSmoke. The separate unmodified
MemoryLoop/DIVA scripts remain final-candidate gates. Normal recorded2 rejected
external proxy attempts and zero forwarded; the other checks recorded zero.
No live-provider quality, DSN-backed PostgreSQL, Windows, browser or installed
product result is inferred. Exact commands/HEAD/exit/times/raw JSONL are archived.
NativeSQLite1.56 normal/race at actual46a separately passed162+1 each in
1.579/8.405 seconds; its resolved-module evidence confirms1.56.0. That is not a
new Windows result or a test of f6's App changes.

Independent Task1a source review is COMPLETE with 0 Critical, 1 Important and
0 Minor findings. I1 identifies negative `Expand.BudgetChars` reaching a negative
slice bound after a legitimate native export. Root reproduced the unchanged
ordinary local fixture at actual f6: FAIL1 test +1 package in1.324s, exact
`slice bounds out of range [:-1]`, no external requests. The original test,
overlay, raw log and command metadata are frozen separately. Task1a requires a
minimal bound rejection and lead verification before scoped acceptance; the
broad passes above did not cover this case. No second review of the same final
fix diff is planned; whole-candidate S6 review remains distinct.

The previous reviewer session failed initialization twice with a system
content-risk message; it produced no review. A fresh-context read-only reviewer
successfully supplied report+manifest. Root checked all39 reviewed source/control
hashes and the report hash. That reviewer executed no tests or network calls.
Preparatory CI wiring at2e627 separately has a completed source-only review with
no findings; neither review establishes actual Windows acceptance.

Evidence SHA256:

- `evidence/s5-consumer-task1a-author.tar.gz`: `ab6ecc1108db3b1f7427edfaed35bdacc8f1f783979dbf0458fb7889c9671d0e`
- `evidence/s5-consumer-task1a-root-fixed.tar.gz`: `1014ddcfb50deeb77c73b755450e36b1f5c67da7cb7bef49d52d43398620fc56`
- `evidence/s5-operation-lookup-root-fixed.tar.gz`: `cc953c7ebf1c5a0149217b8643a5c5a29c15cb0d6398ed1c192000e71c2cdbc5`
- `evidence/s6-ci-independent-review.tar.gz`: `7344c22607846b11181c26e47a63d001514627942e3648262f5574dbcf64906b`

- `evidence/s5-task1a-independent-review.tar.gz`: `1f6704cd62ad93e64a32fc721c3153c07afbee83201e448c7c239918bfa9aadf`
- `evidence/s5-task1a-budget-original-red.tar.gz`: `6726d29dc0eac9b44a3967e61188bda105a1506a1d81122346b4225fd832e910`

Final S5/S6 acceptance still requires one later complete candidate; these scoped
results cannot be aggregated with older stages as whole-Issue GREEN.
