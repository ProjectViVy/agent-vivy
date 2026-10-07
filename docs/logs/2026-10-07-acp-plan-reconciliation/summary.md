# ACP Planning Reconciliation

Date: 2026-10-07 (Asia/Shanghai)
Issue: [#1](https://github.com/ProjectViVy/agent-vivy/issues/1)

The owner requested completion of ACP so the older docs/issue1-acp-face-design branch could be retired. Consolidated the [single design](../../superpowers/specs/2026-10-07-acp-stdio-face-design.md) and [five-Story package](../../superpowers/plans/issue1-acp-face/index.md). ACP-01 through ACP-05 preserve the original delivery identity while absorbing the October probe, concurrency, privacy, lifecycle and artifact detail. Previous full drafts are replaced by redirects; historical delivery records are explicitly dated to their original commits.

Preserved the old documentation head abed12f7749cf3ec0258ec3dc911639f821fa55b as a second parent of the reconciliation commit; the first parent is ACP at 45c466c6aea1622a4465a835f6a06540870a13ef. Resolve the tree from ACP's current product baseline dd78fcf142f384d47ce5cfefb43738fdb9a7346d plus documentation edits only. No September executable source is restored.

Corrections: honor session cwd; retain protocol version/cancel fixes; restore bounded non-file ResourceLink references without adapter fetch; separate G0 acceptance from G1 scheduling; require real-client positive Ask User evidence; rename scenarios AC-01 through AC-12. Prefer evaluation of the smaller existing pack overlay. The new Recipe field/isolated target, presentation facet, line-buffering tradeoff, SDK pin and bounds remain explicit G0 decisions.

This is planning maintenance, not a product release. No runtime, dependency, canonical product contract, generated artifact, Issue state or repository instruction changed. No SDK compatibility or CI success is claimed. The old branch itself is left for the owner to delete after remote verification.
