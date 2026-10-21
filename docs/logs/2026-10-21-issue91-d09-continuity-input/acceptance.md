# Human acceptance

1. Repeat a task, reference or presentation request with the same stable request
   ID and identical intended input. It returns the original run/reference/set,
   with no duplicated event or work.
2. Change intended text, source selection or presentation inputs under the same
   ID. It must conflict even when the values have the same length.
3. Repeat a committed attachment after its per-run budget is full, or a committed
   presentation after the original workspace disappears. The destination-owned
   original result remains recoverable without a new source read or new effect.
4. Reopen the disposable store and repeat the same IDs. Results and conflicts
   remain. A historical receipt without exact input must not authorize a retry.
5. Delete the destination session. Its private receipt input is removed through
   that same transaction. Historical source deletion alone keeps destination
   copies; it never grants permission to read fresh source records.

These are reproducible acceptance steps, not human execution claims. D09 Story
acceptance, owner-deferred3015 smoke and D14 finite artifact/release stay open.
