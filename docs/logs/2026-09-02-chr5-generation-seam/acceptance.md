# Acceptance — CH-R-5

How a human can tell it worked:

1. Pack any plugin: `vivy-sdk pack --with hello-fs`
2. Open the new generation's `dist/gen-*/generation.json`. It now contains a
   `plugins` array; for hello-fs the entry is:

   ```json
   {
     "name": "hello-fs",
     "version": "0.1.0",
     "seam": "tool-world",
     "grants": ["fs.read"],
     "source_ref": "file:...plugins/hello-fs",
     "tree_hash": "<64 hex chars>"
   }
   ```

3. Pack a channel plugin (`vivy-sdk pack --with telegram`): the telegram
   entry reads `"seam": "channel"` and carries `"transport": "poll"` — the
   manifest is classified by seam, per VIVY-CHANNEL-PACK.md §10.
4. Edit one byte inside a plugin source file and re-pack: that plugin's
   `tree_hash` changes while everything else stays stable.
5. `vivy-sdk inspect-artifact dist/gen-*` prints the same manifest
   (round-trip via generation.json).

Regression guarantees: `recipe` and `tools` fields keep their previous
shape; `InspectArtifact` still validates old fixtures (Studio ledger
generation.json files remain readable).
