# remake/acp: raw feature donor

This branch contains 30 dedicated source-file deltas and references 25 shared-file deltas. It is a source donor, not a standalone build or completed feature.

The common plan is [index](../../index.md); the exact extraction state is [extraction](../../extraction.md). `source.json` names the original blobs, source/base revisions and alternate branch-only commits. New source paths are copied byte-for-byte, not adapted to the new core yet. Deleted paths remain deleted.

Shared Service/App/Recipe/pin/Assembly/data glue remains an archive reference; do not copy it wholesale or merge this branch into the stable core. Integrate only the selected behavior onto accepted B using the lane plan and the current no-hash, ownership, approval and data contracts. Product tests and private dependency bootstrap are pending; a successful source reconstruction is not a runtime pass.

The original historical tests and hashes are source material, not permission to restore retired machinery. The Eino/Claude preamble already belongs to #91. No existing branch or user store is changed by this extraction.
