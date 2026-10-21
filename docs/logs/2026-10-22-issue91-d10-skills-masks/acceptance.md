# Acceptance

1. Read an installed skill through authorized skills/get with explicit path SKILL.md. Submit the exact text and expected_exists=true to the immediate toggle. An unchanged read toggles successfully; same-size content edits, frontmatter-only edits or existence changes reject and refresh rather than overwrite.
2. Stage a skill edit or supporting-file action. Review it, apply unchanged bytes, reject a proposal without mutation, and restore exact prior bytes by rollback after restart. A missing supporting file and an existing empty file remain different states.
3. Stage whole-skill deletion, then change any supporting file or empty directory before approval. Apply must reject. An unchanged delete and restart-safe rollback restore the full manifest; changed retained trash must reject restoration.
4. Edit a custom mask with its current expected revision; stale update/delete/selection requests conflict. An already captured mask remains its actual admitted definition. Reusing a create operation with changed original fields conflicts even after editing that mask.
5. Upgrade a disposable supported source through the finite converter. Keep statuses/history. Unavailable old skill before states and old edited-mask create intents require fresh review rather than inferred authority. Reopen must succeed, and injected migration failure must leave the preceding owner boundary intact.

Automated owner evidence is in verification.md. Final shared adapters and split-origin browser acceptance remain integration responsibilities. Roll back code through Git; never run an old binary on the converted target or discard new history. Use the approved quiescent backup/conversion strategy for data.
