# Acceptance

How a human confirms it:

1. Open http://127.0.0.1:3015 → Settings → Vivy features → Open lifecycle: the top of the
   page shows "Vivy Studio is authoritative for evolution; this page is read-only inspection
   only and provides no create/evaluate/promote entry points."
2. The Generations / Evals / Promotions tabs are view-only: there is no Create Generation
   form, no "Reject" button, no start/record-evaluation form, and no "Confirm promotion"
   form.
3. The inspect information in the Species tab (protocol, Generation, Artifact SHA, Policy,
   Recipe, tools, and Grants) matches before the change.
4. Lifecycle operations happen in Vivy Studio (pack → eval → release → install → rollback);
   everyday Vivy is no longer the evolution-operations entry point.
