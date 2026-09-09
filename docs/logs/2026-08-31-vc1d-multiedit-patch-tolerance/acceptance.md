# Acceptance — VC-1d multiedit + patch tolerance

## How a human can tell it worked

1. Start the split pair (`just dev`) and open `http://127.0.0.1:3015`.
2. Ask the model to make several small changes to one file
   (“Change the three TODOs in this file to their corresponding explanations”) — the transcript should show a
   single `multiedit` call (one approval card for one write), not three
   sequential `patch` calls, and the file should end with all changes.
3. Ask for an edit where the model typically mis-guesses indentation — the
   patch should still land with the file's own indentation intact instead of
   failing with "old_string was not found".
4. If the model names text that does not exist, the edit must fail with no
   partial write (open the file and confirm it is untouched).

## What to look for

- `multiedit` appears in the default tool surface without config edits.
- A multiedit proposal card shows the combined diff before approval; after
  approval the file changes once.
- An interrupted/failed multiedit never leaves the target half-edited.

## Rollback

Single commit on `feat/vc1a-bash-tool`; revert to remove the multiedit tool,
its backend methods, the patch fallback engine, and the config default
addition. Exact-only patch semantics return with the revert.
